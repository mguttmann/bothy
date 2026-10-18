// SPDX-License-Identifier: Apache-2.0

/// A message of the history: the mapping of a stored message to `DsMessageData` (origin badge,
/// tombstone, system line, mentions, reply, reactions) and the tile with the hover bar, the
/// menu, the inline editor, the highlight of a jump and the thread starter lines (brief M4-20
/// Design notes 1 and 4; components.md 1; SEC-OUT-3).
library;

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:l10n/l10n.dart';

import '../composer/mention_suggestions.dart';
import 'message_menu.dart';

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
    triggerState: message.triggerState == null
        ? null
        : DsTriggerState.fromCode(message.triggerState!.code),
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
/// unread" (01 7). While [editing], an inline editor replaces the content.
class MessageTile extends StatelessWidget {
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

  Future<void> _menu(BuildContext context, Offset at) async {
    if (actions.isEmpty) return;
    final chosen = await showMessageMenu(context, at, actions);
    if (chosen != null) onAction(chosen);
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
    Widget body =
        editing ??
        DsMessage(
          data,
          collapseLines: collapseLines,
          isContinuation: continuation,
          onOpenLink: onOpenLink,
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
    if (highlighted) {
      body = DecoratedBox(
        key: const ValueKey('history.highlight'),
        decoration: BoxDecoration(
          color: colors.attentionSubtle,
          borderRadius: BorderRadius.circular(DsRadius.sm),
        ),
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
        onPointerDown: (e) {
          // 01 7: Option+Click (Alt+Click on Windows) marks unread from this message.
          if (e.buttons == kPrimaryButton &&
              HardwareKeyboard.instance.isAltPressed &&
              actions.contains(MessageAction.markUnread)) {
            onAction(MessageAction.markUnread);
          }
        },
        child: GestureDetector(
          behavior: HitTestBehavior.translucent,
          onSecondaryTapDown: (d) => _menu(context, d.globalPosition),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            mainAxisSize: MainAxisSize.min,
            children: [body, ...started],
          ),
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
