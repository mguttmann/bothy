// SPDX-License-Identifier: Apache-2.0

/// A message of the history: the mapping of a stored message to `DsMessageData` (origin badge,
/// tombstone, system line, mentions, reply, reactions) and the tile with the hover bar, the
/// menu, the inline editor, the highlight of a jump and the thread starter lines (brief M4-20
/// Design notes 1 and 4; components.md 1; SEC-OUT-3).
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:l10n/l10n.dart';

import '../agents/trigger_state_mapping.dart';
import '../composer/mention_suggestions.dart';
import 'message_menu.dart';
import 'select_text_view.dart';

/// [message] as `DsMessageData` for the reader [me]: the author's name from [principals]
/// (a missing one shows the handle-less placeholder of the system), [timeLabel] formatted by
/// the caller, the [reactions], the quoted [replied] message, and the tokens of `mention_ids`
/// as mentions (03 content format item 4). Only a server message of kind `system` is a
/// system line (SEC-OUT-3).
DsMessageData messageData(
  Message message, {
  required Map<PrincipalId, Principal> principals,
  required PrincipalId? me,
  required String timeLabel,
  List<ReactionSummary> reactions = const [],
  Message? replied,
  DsDelivery delivery = DsDelivery.sent,
  String? failureText,
}) {
  final author = principals[message.authorId];
  final name = author?.displayName ?? message.authorId.value;
  final reply = replied;
  final systemLine =
      message.kind == MessageKind.system &&
      message.origin == MessageOrigin.system;
  return DsMessageData(
    id: message.id.value,
    authorName: name,
    origin: DsMessageOrigin.fromCode(message.origin.code),
    // A `system` kind from anything but the server renders as text (SEC-OUT-3).
    kind: systemLine
        ? DsMessageKind.system
        : message.kind == MessageKind.system
        ? DsMessageKind.text
        : DsMessageKind.fromCode(message.kind.code),
    timeLabel: timeLabel,
    content: message.contentMd,
    mentionsMe: me != null && message.mentionIds.contains(me),
    mentionHandles: mentionHandles(message.mentionIds, principals),
    edited: message.editedAt != null && message.deletedAt == null,
    deleted: message.deletedAt != null,
    triggerState: triggerOf(message)?.state,
    triggerReason: message.triggerStateReason,
    delivery: delivery,
    failureText: failureText,
    replyToName: reply == null
        ? null
        : principals[reply.authorId]?.displayName ?? reply.authorId.value,
    replyExcerpt: reply == null ? null : excerpt(reply.contentMd),
    reactions: [
      for (final r in reactions) DsReaction(r.emoji, r.count, mine: r.me),
    ],
    systemCode: systemLine ? message.systemCode?.code : null,
    systemName: systemLine ? name : null,
  );
}

/// The first line of [text] for a quote line; the reply line cuts it with an ellipsis.
String excerpt(String text) => text.trim().split('\n').first.trim();

/// One message of the history.
///
/// The hover bar offers Reply, React and More; More, a right click, the menu key and Shift+F10
/// open the menu of [actions]. Option+Click (Alt+Click) calls [onAction] with "Mark as
/// unread" (01 7). On the phone ([longPressMenu]) a long press opens the same menu, because a
/// tap on a code or quote block copies that block and there is no hover (01 6.12 item 2). While
/// [editing], an inline editor replaces the content.
///
/// Text selection (01 6.15, R-PR-29; brief M4-50): on the desktop the content is selectable
/// with the mouse, and while text is selected the menu starts with "Copy", which copies it; on
/// the phone the menu has "Select text" after "Copy text", which opens the message in a view
/// with the system selection ([showSelectText]). The tile runs both itself; [onAction] never
/// hears them.
class MessageTile extends StatefulWidget {
  /// Creates the tile.
  const MessageTile({
    super.key,
    required this.data,
    required this.continuation,
    required this.collapseLines,
    required this.actions,
    required this.onAction,
    required this.onOpenLink,
    this.highlighted = false,
    this.editing,
    this.started = const [],
    this.attachments,
    this.resolver,
    this.components,
    this.longPressMenu = false,
  });

  /// The message.
  final DsMessageData data;

  /// Whether it continues the group before.
  final bool continuation;

  /// `ui.message_collapse_lines`.
  final int collapseLines;

  /// The actions the server allows.
  final List<MessageAction> actions;

  /// Runs an action.
  final ValueChanged<MessageAction> onAction;

  /// Opens a link of the content.
  final void Function(Uri) onOpenLink;

  /// Whether a jump points at this message.
  final bool highlighted;

  /// The inline editor while the message is edited, or null.
  final Widget? editing;

  /// The starter lines of the threads started here.
  final List<Widget> started;

  /// The files of the message (M4-22), below its content.
  final Widget? attachments;

  /// Resolves the internal references of images in the content to the message's own files
  /// (M4-07 Design note 2, D-M4-22-7); null leaves every image as text.
  final DsAttachmentResolver? resolver;

  /// The body of a card (`kind = card`, M4-21): the components with their header from server
  /// fields and the agent's text below the divider (SEC-OUT-4).
  final Widget? components;

  /// Whether a long press opens the menu of [actions] (the phone, 01 6.12 item 2); without it
  /// (the desktop) the content is selectable with the mouse (01 6.15).
  final bool longPressMenu;

  @override
  State<MessageTile> createState() => _MessageTileState();
}

class _MessageTileState extends State<MessageTile> {
  /// The text selected in the content now, empty without a selection (the desktop).
  String _selected = '';

  /// Whether a right click is down; the menu opens when it is released (see [build]).
  Offset? _rightClick;

  /// The menu's actions: "Copy" first while text is selected, "Select text" after "Copy
  /// text" on the phone (01 6.15 items 2 and 5).
  List<MessageAction> _menuActions(String selected) {
    final actions = widget.actions;
    final selectText = widget.longPressMenu && widget.data.content.isNotEmpty;
    return [
      if (selected.isNotEmpty && widget.editing == null)
        MessageAction.copySelection,
      for (final a in actions) ...[
        a,
        if (selectText && a == MessageAction.copyText) MessageAction.selectText,
      ],
      if (selectText && !actions.contains(MessageAction.copyText))
        MessageAction.selectText,
    ];
  }

  Future<void> _menu(BuildContext context, Offset at) async {
    if (widget.actions.isEmpty) return;
    // The menu takes the focus, which ends the selection; its copy is the text selected when
    // the menu opened.
    final selected = _selected;
    final chosen = await showMessageMenu(context, at, _menuActions(selected));
    if (!context.mounted) return;
    switch (chosen) {
      case null:
        return;
      case MessageAction.copySelection:
        await Clipboard.setData(ClipboardData(text: selected));
        if (!context.mounted) return;
        ScaffoldMessenger.maybeOf(context)?.showSnackBar(
          SnackBar(content: Text(AppLocalizations.of(context).messageCopied)),
        );
      case MessageAction.selectText:
        await showSelectText(
          context,
          content: widget.data.content,
          mentionHandles: widget.data.mentionHandles,
          resolver: widget.resolver,
          onOpenLink: widget.onOpenLink,
        );
      default:
        widget.onAction(chosen);
    }
  }

  Offset _anchor(BuildContext context) {
    final box = context.findRenderObject() as RenderBox?;
    return box == null
        ? Offset.zero
        : box.localToGlobal(Offset(box.size.width / 2, box.size.height / 2));
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final data = widget.data;
    final editing = widget.editing;
    final actions = widget.actions;
    final onAction = widget.onAction;
    final longPressMenu = widget.longPressMenu;
    Widget body =
        editing ??
        DsMessage(
          data,
          collapseLines: widget.collapseLines,
          isContinuation: widget.continuation,
          onOpenLink: widget.onOpenLink,
          attachments: widget.resolver,
          components: widget.components,
          onSelectionChanged: longPressMenu ? null : (text) => _selected = text,
          actions: [
            for (final a in actions)
              if (messageActionGlyph(a) case final glyph?)
                DsMessageAction(
                  glyph: glyph,
                  label: messageActionLabel(l, a),
                  onPressed: () => onAction(a),
                ),
            if (actions.isNotEmpty)
              DsMessageAction(
                glyph: DsIcons.actionMore,
                label: l.messageActionMore,
                onPressed: () => _menu(context, _anchor(context)),
              ),
          ],
        );
    if (widget.highlighted) {
      body = DecoratedBox(
        key: const ValueKey('history.highlight'),
        decoration: BoxDecoration(
          color: colors.attentionSubtle,
          borderRadius: BorderRadius.circular(DsRadius.sm),
        ),
        child: body,
      );
    }
    if (longPressMenu && editing == null && actions.isNotEmpty) {
      // A tap stays with the content (links, the copy of a block); only the long press is
      // the message's.
      body = GestureDetector(
        key: const ValueKey('message.longPress'),
        onLongPressStart: (d) => _menu(context, d.globalPosition),
        child: body,
      );
    }
    return CallbackShortcuts(
      bindings: {
        const SingleActivator(LogicalKeyboardKey.f10, shift: true): () =>
            _menu(context, _anchor(context)),
        const SingleActivator(LogicalKeyboardKey.contextMenu): () =>
            _menu(context, _anchor(context)),
      },
      child: Listener(
        // Raw pointer events, outside the gesture arena: a recognizer of the content (a
        // link, the text) must not swallow the right click.
        onPointerDown: (e) {
          if (e.buttons == kSecondaryMouseButton) {
            _rightClick = e.position;
            return;
          }
          // 01 7: Option+Click (Alt+Click on Windows) marks unread from this message.
          if (e.buttons == kPrimaryButton &&
              HardwareKeyboard.instance.isAltPressed &&
              actions.contains(MessageAction.markUnread)) {
            onAction(MessageAction.markUnread);
          }
        },
        // The menu of a right click opens on release, after the selection had its turn: on
        // macOS a right click selects the word under the pointer, which "Copy" then copies
        // (01 6.15 item 2). The recognizers see the release after this listener, so the menu
        // waits for the end of the event.
        onPointerUp: (e) {
          final at = _rightClick;
          _rightClick = null;
          if (at != null) {
            unawaited(
              Future(() => context.mounted ? _menu(context, at) : null),
            );
          }
        },
        onPointerCancel: (_) => _rightClick = null,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          mainAxisSize: MainAxisSize.min,
          children: [
            body,
            if (widget.attachments case final files? when editing == null)
              Padding(
                // Under the content, beside the avatar like the content itself.
                padding: const EdgeInsets.only(
                  left: DsSize.avatarMd + DsSpace.s12 + DsSpace.s8,
                  right: DsSpace.s8,
                ),
                child: files,
              ),
            ...widget.started,
          ],
        ),
      ),
    );
  }
}

/// The line "Thread: `title`, `n` replies" under a message of the main history that opens the
/// thread (01 5.1 D2).
class ThreadStarterLine extends StatelessWidget {
  /// Creates the line of [thread].
  const ThreadStarterLine({
    super.key,
    required this.thread,
    required this.onOpen,
  });

  /// The started thread.
  final Thread thread;

  /// Opens it.
  final VoidCallback onOpen;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    // The first message of a thread from a message is the parent; replies are the rest.
    final replies = thread.messageCount;
    final text = l.threadStarter(dsIsolate(thread.title), replies);
    return Padding(
      padding: const EdgeInsets.only(
        left: DsSize.avatarMd + DsSpace.s12 + DsSpace.s8,
        bottom: DsSpace.s4,
      ),
      child: Align(
        alignment: AlignmentDirectional.centerStart,
        child: DsTappable(
          key: ValueKey('history.starter.${thread.id.value}'),
          onTap: onOpen,
          label: text,
          excludeChildSemantics: true,
          child: Padding(
            padding: const EdgeInsets.symmetric(
              horizontal: DsSpace.s6,
              vertical: DsSpace.s2,
            ),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                DsIcon(
                  DsIcons.thread,
                  size: DsIconSizeRole.sm,
                  color: context.dsAccent.textLink,
                  decorative: true,
                ),
                const SizedBox(width: DsSpace.s4),
                Flexible(
                  child: Text(
                    text,
                    overflow: TextOverflow.ellipsis,
                    style: dsTextStyle(DsType.caption)
                        .copyWith(color: context.dsAccent.textLink),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

/// The inline editor of a message (01 3.3; 04 13.3): Save sends the edit, Cancel closes it.
class InlineEditor extends StatefulWidget {
  /// Creates the editor with [initial].
  const InlineEditor({
    super.key,
    required this.initial,
    required this.onSave,
    required this.onCancel,
  });

  /// The text now.
  final String initial;

  /// Saves the new text.
  final ValueChanged<String> onSave;

  /// Closes the editor.
  final VoidCallback onCancel;

  @override
  State<InlineEditor> createState() => _InlineEditorState();
}

class _InlineEditorState extends State<InlineEditor> {
  late final TextEditingController _text = TextEditingController(
    text: widget.initial,
  );

  @override
  void dispose() {
    _text.dispose();
    super.dispose();
  }

  void _save() {
    final text = _text.text.trim();
    if (text.isEmpty || text == widget.initial) {
      widget.onCancel();
    } else {
      widget.onSave(text);
    }
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    return Padding(
      padding: const EdgeInsets.symmetric(
        horizontal: DsSpace.s8,
        vertical: DsSpace.s4,
      ),
      child: CallbackShortcuts(
        bindings: {
          const SingleActivator(LogicalKeyboardKey.escape): widget.onCancel,
        },
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          mainAxisSize: MainAxisSize.min,
          children: [
            TextField(
              key: const ValueKey('edit.field'),
              controller: _text,
              autofocus: true,
              minLines: 1,
              maxLines: null,
              decoration: InputDecoration(labelText: l.messageEditLabel),
            ),
            const SizedBox(height: DsSpace.s6),
            Wrap(
              spacing: DsSpace.s6,
              children: [
                DsButton(
                  key: const ValueKey('edit.cancel'),
                  label: l.threadCancel,
                  style: DsButtonStyle.ghost,
                  onPressed: widget.onCancel,
                ),
                DsButton(
                  key: const ValueKey('edit.save'),
                  label: l.threadSave,
                  style: DsButtonStyle.primary,
                  onPressed: _save,
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}
