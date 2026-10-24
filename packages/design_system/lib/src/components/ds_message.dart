// SPDX-License-Identifier: Apache-2.0

/// A message of a thread with all variants of components.md section 1.
library;

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:markdown/markdown.dart' as md;

import '../generated/design_tokens.g.dart';
import '../icons/ds_icon.dart';
import '../theme/ds_theme.dart';
import 'ds_common.dart';
import 'ds_trigger_state.dart';
import 'markdown/ds_markdown.dart';
import 'view_models.dart';

/// An action of the hover bar of a message (Reply, React, Edit, Pin, More).
@immutable
final class DsMessageAction {
  /// Creates the action.
  const DsMessageAction({
    required this.glyph,
    required this.label,
    required this.onPressed,
  });

  /// The icon.
  final DsGlyph glyph;

  /// The label from `l10n` (tooltip and semantics).
  final String label;

  /// Runs the action.
  final VoidCallback onPressed;
}

/// A post in a thread (`messages`) from a person, agent, app, import or the system.
///
/// Variants (components.md section 1): text, continuation ([isContinuation], computed by the
/// caller from `ui.message_group_window`: no avatar and header), voice, live box anchor
/// ([liveBox] at this position), card ([components] on `color.surface.raised`, which draws the
/// agent text below its header, so the content is not drawn a second time),
/// system line, mention (tint `color.attention.subtle` and spine), sending, failed (reason with
/// Resend, Edit, Discard), edited ("(edited)") and deleted ("Message deleted", no hover bar).
///
/// The content renders through [DsMarkdown]; over [collapseLines] lines
/// (`ui.message_collapse_lines`) it is collapsed with "Show more". The hover bar shows
/// [actions] on hover and while focus is inside the message: focus the message, then Tab.
/// Agents, apps, imports and the system get an origin badge after the name (SEC-OUT-3).
/// Foreign names are isolated in the visual text (SEC-IN-4) and plain in the semantics label
/// "{Mentions you}, {Name}, {Time}, {Text}".
///
/// ```dart
/// DsMessage(data, collapseLines: config.messageCollapseLines, onOpenLink: open,
///     actions: [DsMessageAction(glyph: DsIcons.actionReply, label: l.messageActionReply,
///     onPressed: reply)])
/// ```
class DsMessage extends StatefulWidget {
  /// Creates the message of [data].
  const DsMessage(
    this.data, {
    super.key,
    required this.collapseLines,
    required this.onOpenLink,
    this.isContinuation = false,
    this.actions = const [],
    this.liveBox,
    this.components,
    this.attachments,
    this.onResend,
    this.onEdit,
    this.onDiscard,
    this.onPlayVoice,
    this.onTriggerTap,
  });

  /// The message.
  final DsMessageData data;

  /// `ui.message_collapse_lines`: rendered lines before the content collapses.
  final int collapseLines;

  /// Opens an activated link of the content.
  final void Function(Uri) onOpenLink;

  /// Whether the message continues the group of the previous one.
  final bool isContinuation;

  /// The hover bar.
  final List<DsMessageAction> actions;

  /// The live box of `kind = run`.
  final Widget? liveBox;

  /// The components of `kind = card` ([DsMessageComponents]).
  final Widget? components;

  /// Resolves server file references of images in the content.
  final DsAttachmentResolver? attachments;

  /// Resend of a failed message.
  final VoidCallback? onResend;

  /// Edit of a failed message.
  final VoidCallback? onEdit;

  /// Discard of a failed message.
  final VoidCallback? onDiscard;

  /// Play or pause of a voice message.
  final VoidCallback? onPlayVoice;

  /// Tap on the trigger state (jumps to the live box).
  final VoidCallback? onTriggerTap;

  @override
  State<DsMessage> createState() => DsMessageState();
}

/// State of [DsMessage]: hover, focus and the collapse.
class DsMessageState extends State<DsMessage> {
  final FocusNode _self = FocusNode(debugLabel: 'message');
  bool _hover = false;
  bool _within = false;
  bool _expanded = false;

  /// Whether the message itself has the focus.
  @visibleForTesting
  bool get focusedMessage => _self.hasPrimaryFocus;

  /// The row background: the mention tint or none.
  @visibleForTesting
  Color? get background => widget.data.mentionsMe && !widget.data.deleted
      ? context.dsColors.attentionSubtle
      : null;

  @override
  void dispose() {
    _self.dispose();
    super.dispose();
  }

  /// Block elements whose neighbors the label separates with a space, so list items and
  /// table cells are not read as one word.
  static const _blocks = {
    'p',
    'li',
    'ul',
    'ol',
    'blockquote',
    'pre',
    'hr',
    'table',
    'thead',
    'tbody',
    'tr', //
    'td', 'th', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6',
  };

  static String _plainNode(md.Node node) {
    if (node is md.Text) return node.text;
    if (node is! md.Element) return '';
    if (node.tag == 'img') return node.attributes['alt'] ?? '';
    final out = StringBuffer();
    md.Node? previous;
    for (final child in node.children ?? const <md.Node>[]) {
      final block =
          (child is md.Element && _blocks.contains(child.tag)) ||
          (previous is md.Element && _blocks.contains(previous.tag));
      if (previous != null && block) out.write(' ');
      out.write(_plainNode(child));
      previous = child;
    }
    return out.toString();
  }

  String _plain(String source) =>
      _plainNode(md.Element('root', DsMarkdown.parse(source)))
          .replaceAll(RegExp(r'\s+'), ' ')
          .trim();

  String _label(BuildContext context) {
    final l = dsL10n(context);
    final d = widget.data;
    final text = switch (d) {
      _ when d.deleted => l.messageDeleted,
      _ when d.kind == DsMessageKind.system => _systemText(
        context,
        isolate: false,
      ),
      _ when d.content.isEmpty && d.voice?.transcript != null =>
        d.voice!.transcript!,
      // The card body reads the agent text after its server header (SEC-OUT-4); the
      // message label must not read it first.
      _ when d.kind == DsMessageKind.card && widget.components != null => '',
      _ => _plain(d.content),
    };
    final label = text.isEmpty
        ? '${d.authorName}, ${d.timeLabel}'
        : l.messageSemanticsLabel(d.authorName, d.timeLabel, text);
    return d.mentionsMe ? '${l.messageMentionsYou}, $label' : label;
  }

  String _systemText(BuildContext context, {required bool isolate}) {
    final l = dsL10n(context);
    final d = widget.data;
    final name = d.systemName ?? d.authorName;
    return switch (d.systemCode) {
      'thread.reopened' => l.messageSystemThreadReopened(
        isolate ? dsIsolate(name) : name,
      ),
      'agent_session.resume_failed' => l.messageSystemResumeFailed,
      _ => l.messageSystemUnknown(d.systemCode ?? ''),
    };
  }

  String? _originBadge(BuildContext context) {
    final l = dsL10n(context);
    return switch (widget.data.origin) {
      DsMessageOrigin.agent => l.messageOriginAgent,
      DsMessageOrigin.app => l.messageOriginApp,
      DsMessageOrigin.import => l.messageOriginImport,
      DsMessageOrigin.system ||
      DsMessageOrigin.unknown => l.messageOriginSystem,
      DsMessageOrigin.human => null,
    };
  }

  @override
  Widget build(BuildContext context) {
    final d = widget.data;
    if (d.kind == DsMessageKind.system && !d.deleted) return _system(context);
    final colors = context.dsColors;
    final density = context.dsDensity;
    final showBar =
        !d.deleted && widget.actions.isNotEmpty && (_hover || _within);

    final body = <Widget>[
      if (!widget.isContinuation) _header(context),
      if (d.replyToName != null && !d.deleted) _reply(context),
      ..._content(context),
      if (d.attachmentNames.isNotEmpty && !d.deleted) _attachments(context),
      if (d.reactions.isNotEmpty && !d.deleted) _reactions(context),
      if (d.delivery != DsDelivery.sent) _delivery(context),
    ];

    Widget row = Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (widget.isContinuation)
          const SizedBox(width: DsSize.avatarMd)
        else
          DsAvatar(name: d.authorName, origin: d.origin),
        const SizedBox(width: DsSpace.s12),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            mainAxisSize: MainAxisSize.min,
            children: body,
          ),
        ),
        // A continuation has no header, but a triggering message always shows its state
        // (01 6.3); it moves to the end of the row.
        if (widget.isContinuation && d.triggerState != null) ...[
          const SizedBox(width: DsSpace.s8),
          DsTriggerStateIcon(
            state: d.triggerState!,
            reason: d.triggerReason,
            onTap: widget.onTriggerTap,
          ),
        ],
      ],
    );
    if (d.delivery == DsDelivery.sending || d.delivery == DsDelivery.delayed) {
      // Reduced opacity marks the unconfirmed message; the line "Sending" says it in text.
      row = Opacity(opacity: _sendingOpacity, child: row);
    }

    final focused = _self.hasPrimaryFocus;
    return MouseRegion(
      onEnter: (_) => setState(() => _hover = true),
      onExit: (_) => setState(() => _hover = false),
      child: Focus(
        canRequestFocus: false,
        skipTraversal: true,
        onFocusChange: (has) => setState(() => _within = has),
        child: Focus(
          focusNode: _self,
          onFocusChange: (_) => setState(() {}),
          child: Container(
            constraints: const BoxConstraints(
              maxWidth: DsLayout.messageMaxWidth,
            ),
            padding: EdgeInsets.symmetric(
              horizontal: DsSpace.s8,
              vertical: density.messagePadY,
            ),
            decoration: BoxDecoration(
              color: background,
              border: Border(
                left: BorderSide(
                  color: background != null
                      ? colors.attentionBorder
                      : Colors.transparent,
                  width: DsBorderWidth.spine,
                ),
              ),
            ),
            // Always a decoration, so a focus change never changes the widget structure (a
            // rebuilt subtree would drop the focus of the hover bar).
            foregroundDecoration: BoxDecoration(
              border: focused
                  ? Border.all(
                      color: context.dsAccent.focusRing,
                      width: DsBorderWidth.focus,
                    )
                  : null,
              borderRadius: BorderRadius.circular(DsRadius.sm),
            ),
            child: Stack(
              clipBehavior: Clip.none,
              children: [
                Semantics(
                  container: true,
                  explicitChildNodes: true,
                  label: _label(context),
                  child: row,
                ),
                if (showBar)
                  PositionedDirectional(
                    top: 0,
                    end: 0,
                    child: _HoverBar(actions: widget.actions),
                  ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  /// The opacity of an unconfirmed message; one step of the motion-free state change, kept
  /// high enough that the text keeps its contrast.
  static const double _sendingOpacity = 0.85;

  Widget _system(BuildContext context) {
    final colors = context.dsColors;
    final style = dsTextStyle(DsType.caption)
        .copyWith(color: colors.textSecondary);
    return Semantics(
      container: true,
      label: _label(context),
      excludeSemantics: true,
      child: Padding(
        padding: EdgeInsets.symmetric(
          horizontal: DsSpace.s8,
          vertical: context.dsDensity.messagePadY,
        ),
        child: Row(
          children: [
            DsIcon(
              DsIcons.stateInfo,
              size: DsIconSizeRole.sm,
              color: colors.textSecondary,
              decorative: true,
            ),
            const SizedBox(width: DsSpace.s8),
            Expanded(
              child: Text(_systemText(context, isolate: true), style: style),
            ),
            Text(widget.data.timeLabel, style: style),
          ],
        ),
      ),
    );
  }

  Widget _header(BuildContext context) {
    final colors = context.dsColors;
    final d = widget.data;
    final badge = _originBadge(context);
    // Name and time are in the message label already; the badge and the trigger state add
    // to it as their own parts.
    return Padding(
      padding: const EdgeInsets.only(bottom: DsSpace.s2),
      child: Wrap(
        spacing: DsSpace.s6,
        crossAxisAlignment: WrapCrossAlignment.center,
        children: [
          ExcludeSemantics(
            child: Text(
              dsIsolate(d.authorName),
              style: dsTextStyle(DsType.label)
                  .copyWith(color: colors.textPrimary),
            ),
          ),
          if (badge != null)
            DsBadge(
              text: badge,
              background: colors.surfaceSunken,
              foreground: colors.textSecondary,
            ),
          ExcludeSemantics(
            child: Text(
              d.timeLabel,
              style: dsTextStyle(DsType.caption)
                  .copyWith(color: colors.textSecondary),
            ),
          ),
          if (d.triggerState != null)
            DsTriggerStateIcon(
              state: d.triggerState!,
              reason: d.triggerReason,
              onTap: widget.onTriggerTap,
            ),
        ],
      ),
    );
  }

  Widget _reply(BuildContext context) {
    final colors = context.dsColors;
    final d = widget.data;
    return Semantics(
      container: true,
      label: dsL10n(context).messageReplyTo(d.replyToName!),
      excludeSemantics: true,
      child: Container(
        margin: const EdgeInsets.only(bottom: DsSpace.s4),
        padding: const EdgeInsets.only(left: DsSpace.s8),
        decoration: BoxDecoration(
          border: Border(
            left: BorderSide(
              color: colors.borderDefault,
              width: DsBorderWidth.spine,
            ),
          ),
        ),
        child: Text(
          '${dsIsolate(d.replyToName!)}: ${d.replyExcerpt ?? ''}',
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
          style: dsTextStyle(DsType.caption)
              .copyWith(color: colors.textSecondary),
        ),
      ),
    );
  }

  List<Widget> _content(BuildContext context) {
    final l = dsL10n(context);
    final colors = context.dsColors;
    final d = widget.data;
    if (d.deleted) {
      return [
        Text(
          l.messageDeleted,
          style: dsTextStyle(DsType.bodySm)
              .copyWith(color: colors.textMuted, fontStyle: FontStyle.italic),
        ),
      ];
    }
    final out = <Widget>[];
    if (d.kind == DsMessageKind.voice && d.voice != null) {
      out.add(_voice(context));
    }
    // A card draws its agent text inside [components], below the header built from server
    // fields (SEC-OUT-4); drawing it here too would put agent text above that header.
    final cardBody = d.kind == DsMessageKind.card && widget.components != null;
    if (d.content.isNotEmpty && !cardBody) {
      final style = dsTextStyle(DsType.body).copyWith(
        color: colors.textPrimary,
        fontSize: context.dsDensity.messageFontSize,
      );
      // The content keeps its own semantics, so links and the copy button of code blocks
      // stay reachable for screen readers below the message label.
      final markdown = DsMarkdown(
        d.content,
        onOpenLink: widget.onOpenLink,
        attachments: widget.attachments,
        style: style,
        mentionHandles: d.mentionHandles,
      );
      out.add(
        _expanded
            ? markdown
            : DsCollapsible(
                maxLines: widget.collapseLines,
                lineStyle: style,
                onExpand: () => setState(() => _expanded = true),
                child: markdown,
              ),
      );
    }
    if (d.edited) {
      out.add(
        Text(
          l.messageEdited,
          style: dsTextStyle(DsType.caption).copyWith(color: colors.textMuted),
        ),
      );
    }
    if (d.kind == DsMessageKind.run && widget.liveBox != null) {
      out.add(
        Padding(
          padding: const EdgeInsets.only(top: DsSpace.s4),
          child: widget.liveBox,
        ),
      );
    }
    if (d.kind == DsMessageKind.card && widget.components != null) {
      out.add(
        Container(
          margin: const EdgeInsets.only(top: DsSpace.s4),
          padding: const EdgeInsets.all(DsSpace.s12),
          decoration: BoxDecoration(
            color: colors.surfaceRaised,
            border: Border.all(
              color: colors.borderSubtle,
              width: DsBorderWidth.hairline,
            ),
            borderRadius: BorderRadius.circular(DsRadius.md),
          ),
          child: widget.components,
        ),
      );
    }
    return out;
  }

  Widget _voice(BuildContext context) {
    final l = dsL10n(context);
    final colors = context.dsColors;
    final v = widget.data.voice!;
    return Padding(
      padding: const EdgeInsets.only(bottom: DsSpace.s4),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          Row(
            children: [
              DsTappable(
                onTap: widget.onPlayVoice,
                label: v.playing ? l.messageVoicePause : l.messageVoicePlay,
                radius: DsRadius.pill,
                excludeChildSemantics: true,
                child: Center(
                  widthFactor: 1,
                  heightFactor: 1,
                  child: DsIcon(
                    v.playing ? DsIcons.actionStop : DsIcons.actionVoice,
                    color: context.dsAccent.accentFg,
                    decorative: true,
                  ),
                ),
              ),
              const SizedBox(width: DsSpace.s8),
              Expanded(
                child: LinearProgressIndicator(
                  value: v.progress.clamp(0, 1).toDouble(),
                  color: context.dsAccent.accentFg,
                  backgroundColor: colors.surfaceSunken,
                ),
              ),
              const SizedBox(width: DsSpace.s8),
              Text(
                dsFormat(context).elapsed(v.duration),
                style: dsTextStyle(DsType.codeSm)
                    .copyWith(color: colors.textSecondary),
              ),
            ],
          ),
          Text(
            v.transcript ?? l.messageVoiceTranscriptPending,
            style: dsTextStyle(DsType.bodySm).copyWith(
              color: v.transcript == null
                  ? colors.textMuted
                  : colors.textPrimary,
            ),
          ),
        ],
      ),
    );
  }

  Widget _attachments(BuildContext context) {
    final l = dsL10n(context);
    final colors = context.dsColors;
    return Padding(
      padding: const EdgeInsets.only(top: DsSpace.s4),
      child: Wrap(
        spacing: DsSpace.s4,
        runSpacing: DsSpace.s4,
        children: [
          for (final name in widget.data.attachmentNames)
            Semantics(
              container: true,
              label: l.messageAttachment(name),
              excludeSemantics: true,
              child: Container(
                padding: const EdgeInsets.symmetric(
                  horizontal: DsSpace.s8,
                  vertical: DsSpace.s4,
                ),
                decoration: BoxDecoration(
                  color: colors.surfaceSunken,
                  borderRadius: BorderRadius.circular(DsRadius.sm),
                ),
                child: Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    DsIcon(
                      DsIcons.actionAttach,
                      size: DsIconSizeRole.sm,
                      color: colors.textSecondary,
                      decorative: true,
                    ),
                    const SizedBox(width: DsSpace.s4),
                    Text(
                      dsIsolate(name),
                      style: dsTextStyle(DsType.bodySm)
                          .copyWith(color: colors.textPrimary),
                    ),
                  ],
                ),
              ),
            ),
        ],
      ),
    );
  }

  Widget _reactions(BuildContext context) {
    final l = dsL10n(context);
    final colors = context.dsColors;
    return Padding(
      padding: const EdgeInsets.only(top: DsSpace.s4),
      child: Wrap(
        spacing: DsSpace.s4,
        runSpacing: DsSpace.s4,
        children: [
          for (final r in widget.data.reactions)
            Semantics(
              container: true,
              label: l.messageReaction(r.count, r.emoji),
              selected: r.mine,
              excludeSemantics: true,
              child: Container(
                padding: const EdgeInsets.symmetric(horizontal: DsSpace.s6),
                constraints: const BoxConstraints(
                  minHeight: DsSize.badgeHeight,
                ),
                decoration: BoxDecoration(
                  color: r.mine
                      ? context.dsAccent.accentSubtle
                      : colors.surfaceSunken,
                  border: Border.all(
                    color: r.mine
                        ? context.dsAccent.accentFg
                        : colors.borderSubtle,
                    width: DsBorderWidth.hairline,
                  ),
                  borderRadius: BorderRadius.circular(DsRadius.pill),
                ),
                child: Text(
                  '${r.emoji} ${dsFormat(context).count(r.count)}',
                  style: dsTextStyle(DsType.label)
                      .copyWith(color: colors.textPrimary),
                ),
              ),
            ),
        ],
      ),
    );
  }

  Widget _delivery(BuildContext context) {
    final l = dsL10n(context);
    final colors = context.dsColors;
    final d = widget.data;
    if (d.delivery == DsDelivery.sending || d.delivery == DsDelivery.delayed) {
      return Row(
        children: [
          DsIcon(
            DsIcons.triggerQueued,
            size: DsIconSizeRole.sm,
            color: colors.textSecondary,
            decorative: true,
          ),
          const SizedBox(width: DsSpace.s4),
          // Wraps instead of overflowing at a large text scale (01 8).
          Flexible(
            child: Text(
              d.delivery == DsDelivery.delayed
                  ? l.messageSendingDelayed
                  : l.messageSending,
              style: dsTextStyle(DsType.caption)
                  .copyWith(color: colors.textSecondary),
            ),
          ),
        ],
      );
    }
    Widget button(String label, VoidCallback? onTap) => DsTappable(
      onTap: onTap,
      label: label,
      excludeChildSemantics: true,
      child: Center(
        widthFactor: 1,
        heightFactor: 1,
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: DsSpace.s6),
          child: Text(
            label,
            style: dsTextStyle(DsType.label)
                .copyWith(color: context.dsAccent.textLink),
          ),
        ),
      ),
    );
    return Padding(
      padding: const EdgeInsets.only(top: DsSpace.s4),
      child: Wrap(
        spacing: DsSpace.s4,
        crossAxisAlignment: WrapCrossAlignment.center,
        children: [
          Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              DsIcon(
                DsIcons.stateError,
                size: DsIconSizeRole.sm,
                color: colors.dangerFg,
                decorative: true,
              ),
              const SizedBox(width: DsSpace.s4),
              Flexible(
                child: Text(
                  d.failureText ?? '',
                  style: dsTextStyle(DsType.bodySm)
                      .copyWith(color: colors.dangerFg),
                ),
              ),
            ],
          ),
          button(l.messageResend, widget.onResend),
          button(l.messageEdit, widget.onEdit),
          button(l.messageDiscard, widget.onDiscard),
        ],
      ),
    );
  }
}

/// The hover bar of a message.
class _HoverBar extends StatelessWidget {
  const _HoverBar({required this.actions});

  final List<DsMessageAction> actions;

  @override
  Widget build(BuildContext context) {
    final colors = context.dsColors;
    return Container(
      decoration: BoxDecoration(
        color: colors.surfaceOverlay,
        border: Border.all(
          color: colors.borderSubtle,
          width: DsBorderWidth.hairline,
        ),
        borderRadius: BorderRadius.circular(DsRadius.md),
        boxShadow: context.dsElevation.menu,
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          for (final a in actions)
            Tooltip(
              message: a.label,
              excludeFromSemantics: true,
              child: DsTappable(
                onTap: a.onPressed,
                label: a.label,
                excludeChildSemantics: true,
                child: Center(
                  widthFactor: 1,
                  heightFactor: 1,
                  child: Padding(
                    padding: const EdgeInsets.all(DsSpace.s4),
                    child: DsIcon(
                      a.glyph,
                      size: DsIconSizeRole.sm,
                      color: colors.textSecondary,
                      decorative: true,
                    ),
                  ),
                ),
              ),
            ),
        ],
      ),
    );
  }
}

/// The avatar of a message author: a person round, an agent as a rounded square
/// (`radius.md`), an app with a small badge; initials on a category tile chosen from the name.
class DsAvatar extends StatelessWidget {
  /// Creates the avatar of [name].
  const DsAvatar({super.key, required this.name, required this.origin});

  /// The display name.
  final String name;

  /// The origin; decides the shape.
  final DsMessageOrigin origin;

  @override
  Widget build(BuildContext context) {
    final colors = context.dsColors;
    final tiles = [
      (colors.categorySpruceChipBg, colors.categorySpruceChipFg),
      (colors.categoryLochChipBg, colors.categoryLochChipFg),
      (colors.categoryHeatherChipBg, colors.categoryHeatherChipFg),
      (colors.categoryAmberChipBg, colors.categoryAmberChipFg),
      (colors.categoryMossChipBg, colors.categoryMossChipFg),
      (colors.categoryRowanChipBg, colors.categoryRowanChipFg),
    ];
    final (bg, fg) =
        tiles[name.runes.fold<int>(0, (a, b) => a + b) % tiles.length];
    final initial = name.trim().isEmpty
        ? '?'
        : String.fromCharCode(name.trim().runes.first).toUpperCase();
    final person = origin == DsMessageOrigin.human;
    final tile = Container(
      width: DsSize.avatarMd,
      height: DsSize.avatarMd,
      alignment: Alignment.center,
      decoration: BoxDecoration(
        color: bg,
        shape: person ? BoxShape.circle : BoxShape.rectangle,
        borderRadius: person ? null : BorderRadius.circular(DsRadius.md),
      ),
      child: Text(
        dsIsolate(initial),
        style: dsTextStyle(DsType.label).copyWith(color: fg),
      ),
    );
    return ExcludeSemantics(
      child: origin == DsMessageOrigin.app
          ? Stack(
              clipBehavior: Clip.none,
              children: [
                tile,
                PositionedDirectional(
                  end: -DsSpace.s2,
                  bottom: -DsSpace.s2,
                  child: Container(
                    width: DsSize.statusDot,
                    height: DsSize.statusDot,
                    decoration: BoxDecoration(
                      color: context.dsAccent.accentBg,
                      shape: BoxShape.circle,
                      border: Border.all(
                        color: colors.surfaceCanvas,
                        width: DsBorderWidth.hairline,
                      ),
                    ),
                  ),
                ),
              ],
            )
          : tile,
    );
  }
}

/// Clips [child] to [maxLines] lines of [lineStyle] and shows "Show more" when it is taller
/// (`ui.message_collapse_lines`, 01 section 6.4).
///
/// The measure is the rendered height, so a long line that wraps counts as several lines;
/// the height scales with the text scaler.
class DsCollapsible extends StatefulWidget {
  /// Creates the collapse around [child].
  const DsCollapsible({
    super.key,
    required this.maxLines,
    required this.lineStyle,
    required this.onExpand,
    required this.child,
  });

  /// The number of lines shown while collapsed.
  final int maxLines;

  /// The style whose line height counts one line.
  final TextStyle lineStyle;

  /// Called by "Show more".
  final VoidCallback onExpand;

  /// The content.
  final Widget child;

  @override
  State<DsCollapsible> createState() => _DsCollapsibleState();
}

class _DsCollapsibleState extends State<DsCollapsible> {
  bool _overflows = false;

  @override
  Widget build(BuildContext context) {
    final scaler = MediaQuery.textScalerOf(context);
    final size = widget.lineStyle.fontSize ?? DsType.body.fontSize!;
    final line = scaler.scale(size) * (widget.lineStyle.height ?? 1);
    final maxHeight = line * widget.maxLines;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        _ClipToHeight(
          maxHeight: maxHeight,
          onOverflow: (value) {
            if (value != _overflows) {
              WidgetsBinding.instance.addPostFrameCallback((_) {
                if (mounted) setState(() => _overflows = value);
              });
            }
          },
          child: widget.child,
        ),
        if (_overflows)
          Align(
            alignment: AlignmentDirectional.centerStart,
            child: DsTappable(
              onTap: widget.onExpand,
              label: dsL10n(context).messageShowMore,
              excludeChildSemantics: true,
              child: Align(
                alignment: AlignmentDirectional.centerStart,
                widthFactor: 1,
                child: Text(
                  dsL10n(context).messageShowMore,
                  style: dsTextStyle(DsType.label)
                      .copyWith(color: context.dsAccent.textLink),
                ),
              ),
            ),
          ),
      ],
    );
  }
}

class _ClipToHeight extends SingleChildRenderObjectWidget {
  const _ClipToHeight({
    required this.maxHeight,
    required this.onOverflow,
    required super.child,
  });

  final double maxHeight;
  final ValueChanged<bool> onOverflow;

  @override
  _RenderClipToHeight createRenderObject(BuildContext context) =>
      _RenderClipToHeight(maxHeight, onOverflow);

  @override
  void updateRenderObject(
    BuildContext context,
    _RenderClipToHeight renderObject,
  ) {
    renderObject
      ..maxHeight = maxHeight
      ..onOverflow = onOverflow;
  }
}

/// Lays the child out with its natural height and keeps at most [maxHeight] of it.
class _RenderClipToHeight extends RenderProxyBox {
  _RenderClipToHeight(this._maxHeight, this.onOverflow);

  double _maxHeight;
  ValueChanged<bool> onOverflow;

  set maxHeight(double value) {
    if (value == _maxHeight) return;
    _maxHeight = value;
    markNeedsLayout();
  }

  bool _clipped = false;

  @override
  void performLayout() {
    final box = child!;
    box.layout(
      constraints.copyWith(maxHeight: double.infinity),
      parentUsesSize: true,
    );
    _clipped = box.size.height > _maxHeight;
    size = constraints.constrain(
      Size(box.size.width, _clipped ? _maxHeight : box.size.height),
    );
    onOverflow(_clipped);
  }

  @override
  void paint(PaintingContext context, Offset offset) {
    if (!_clipped) {
      super.paint(context, offset);
      return;
    }
    layer = context.pushClipRect(
      needsCompositing,
      offset,
      Offset.zero & size,
      super.paint,
      oldLayer: layer as ClipRectLayer?,
    );
  }

  @override
  bool hitTest(BoxHitTestResult result, {required Offset position}) {
    if (!size.contains(position)) return false;
    return super.hitTest(result, position: position);
  }
}
