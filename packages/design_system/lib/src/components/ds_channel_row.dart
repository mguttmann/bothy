// SPDX-License-Identifier: Apache-2.0

/// A channel row of the sidebar tree and the kind of a channel (components.md section 7,
/// 01 6.8 items 7, 8 and 11).
library;

import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../generated/design_tokens.g.dart';
import '../icons/ds_icon.dart';
import '../theme/ds_theme.dart';
import 'ds_chat_status.dart';
import 'ds_common.dart';
import 'view_models.dart';

/// The icon of a channel of [kind]: `channel.project` for a forum with a project binding,
/// `channel.forum` for another forum, `channel.dm` for a direct message and `channel.text`
/// for a text channel and a kind this app does not know (01 6.8 item 11, D-SB-10).
///
/// [projectBound] is `Channel.project_bound`; it changes only the icon of a forum.
///
/// ```dart
/// DsIcon(dsChannelGlyph(DsChannelKind.forum, projectBound: true), decorative: true);
/// ```
DsGlyph dsChannelGlyph(DsChannelKind kind, {bool projectBound = false}) =>
    switch (kind) {
      DsChannelKind.forum =>
        projectBound ? DsIcons.channelProject : DsIcons.channelForum,
      DsChannelKind.dm => DsIcons.channelDm,
      DsChannelKind.text || DsChannelKind.unknown => DsIcons.channelText,
    };

/// The sentence that explains the kind of a channel (01 section 4 and 6.8 item 11): the
/// tooltip of its icon, the subtitle of its header and the empty state of a forum.
///
/// A forum with [projectBound] gets the project sentence, another forum the forum sentence;
/// a text channel, a direct message and an unknown kind get the text channel sentence, as a
/// direct message is one shared conversation too.
///
/// ```dart
/// Text(dsChannelKindSentence(dsL10n(context), channel.kind));
/// ```
String dsChannelKindSentence(
  AppLocalizations l,
  DsChannelKind kind, {
  bool projectBound = false,
}) => switch (kind) {
  DsChannelKind.forum =>
    projectBound ? l.channelKindSentenceProject : l.channelKindSentenceForum,
  DsChannelKind.text ||
  DsChannelKind.dm ||
  DsChannelKind.unknown => l.channelKindSentenceText,
};

/// The kind of a channel as one word inside a screen reader label ("text channel", "forum",
/// "project forum"; components.md 7), chosen like [dsChannelKindSentence].
///
/// ```dart
/// final label = '${data.name}, ${dsChannelKindName(l, data.kind)}';
/// ```
String dsChannelKindName(
  AppLocalizations l,
  DsChannelKind kind, {
  bool projectBound = false,
}) => switch (kind) {
  DsChannelKind.forum =>
    projectBound ? l.channelKindNameProject : l.channelKindNameForum,
  DsChannelKind.text ||
  DsChannelKind.dm ||
  DsChannelKind.unknown => l.channelKindNameText,
};

/// The width of the leading slot of every [DsChannelRow], which holds the group arrow: one
/// `density.target-min`, the arrow's hit area.
///
/// Every row keeps the slot, with or without a group, so the kind icons of a space stand in
/// one column (01 D1: `> {} apps` over `# releases`). A thread row below the channel indents
/// by this slot plus `icon.size.sm` and draws its tree line under the channel icon
/// (components.md 7 "indented one step further").
double dsChannelArrowSlot(BuildContext context) => context.dsDensity.targetMin;

/// Whether the ambient density is `touch`: the only density whose `density.target-min` is
/// the touch target (48; comfortable and compact use 24). Compared by value, because an
/// animated density switch leaves a lerped `DsDensity` instance in the theme.
bool _touch(BuildContext context) =>
    context.dsDensity.targetMin >= DsDensity.touch.targetMin;

/// A channel in the sidebar, one line in `density.row-channel`.
///
/// The icon follows the kind ([dsChannelGlyph]: `channel.text`, `channel.forum`,
/// `channel.project`, `channel.dm`) and has a tooltip with the kind sentence, which a screen
/// reader gets as the tooltip of the row (01 6.8 item 11); the name in `type.body-sm`. States
/// of the table in components.md 7: read (`color.text.on-sidebar-secondary`), unread
/// (`color.text.on-sidebar`, bold, bar on the left), mentions (counter on
/// `color.attention.bg`), active run (dot `category.spruce.on-sidebar` that pulses with
/// `motion.duration.pulse`, still in reduced motion), muted (`color.text.on-sidebar-muted` and
/// the `action.mute` icon), selected (`color.surface.sidebar-selected`), hover
/// (`color.surface.sidebar-hover`, over the whole row) and focus (ring
/// `color.focus.ring-on-sidebar`). The most urgent unseen agent state of the channel's chats
/// ([DsChannelRowData.status], 01 6.17 item 6) shows as a [DsChatStatusMark] before the mention
/// counter and replaces the unread bar. The label reads "{Name}, {kind}, {n} unread, {m}
/// mentions, run active, muted, {status words}"; Enter or a tap calls [onOpen].
///
/// A channel with thread rows passes [groupExpanded]: the leading slot
/// ([dsChannelArrowSlot]) then holds its own arrow (`action.expand` when expanded,
/// `action.collapse` when collapsed), a button named "Hide threads" or "Show threads" that
/// calls [onToggleGroup] with the requested state, and the row reads as an expandable tree
/// node (01 6.8 item 7). The row shows [groupExpanded] until the caller passes the new value
/// (the state is kept on the device by the caller, D-SB-6).
///
/// A forum row passes [addLabel] and [onAdd] for the action "+" (`action.new`) at the trailing
/// edge (01 6.8 item 8). It is built only while the pointer is over the row, while the
/// keyboard focus is on the row or inside it, while the row is [selected], and always in the
/// density `touch` (components.md 7); otherwise it is not in the tree at all, so neither a
/// screen reader nor Tab reaches an invisible button. Arrow and "+" have the hit area
/// `density.target-min` and keep the row at `density.row-channel` in comfortable and compact.
/// Nothing in the row can be dragged (01 6.8 item 10, D-SB-7).
///
/// ```dart
/// DsChannelRow(
///   channel,
///   selected: channel.id == current,
///   groupExpanded: expanded,
///   onToggleGroup: (value) => setExpanded(channel.id, value),
///   addLabel: l.sidebarNewChatIn(channel.name),
///   onAdd: () => newChat(channel.id),
///   onOpen: () => open(channel.id),
/// )
/// ```
class DsChannelRow extends StatefulWidget {
  /// Creates the row of [data].
  ///
  /// [addLabel] and [onAdd] come together: both or neither.
  const DsChannelRow(
    this.data, {
    super.key,
    required this.onOpen,
    this.selected = false,
    this.current = false,
    this.focusNode,
    this.groupExpanded,
    this.onToggleGroup,
    this.addLabel,
    this.onAdd,
  }) : assert(
         (addLabel == null) == (onAdd == null),
         'addLabel and onAdd come together',
       );

  /// The channel.
  final DsChannelRowData data;

  /// Opens the channel.
  final VoidCallback onOpen;

  /// Whether the channel is the open one.
  final bool selected;

  /// Whether the open view lies in this channel, one of its threads being the selected row:
  /// the forum of the open post shows "+" like the selected forum (01 D1 draws
  /// `{} server [+]` over its selected post), without the selected background and flag.
  final bool current;

  /// The focus node of the row itself (not of the arrow or "+"), so the caller's tree
  /// navigation can move the keyboard focus to it (01 6.8 item 7, D-SB-8); null for one of
  /// its own.
  final FocusNode? focusNode;

  /// Whether the channel's group of thread rows is expanded, or null when the channel has no
  /// thread rows (no arrow, not an expandable node).
  final bool? groupExpanded;

  /// Called with the requested expanded state when the arrow is pressed; null disables the
  /// arrow.
  final ValueChanged<bool>? onToggleGroup;

  /// The label and tooltip of the action "+" ("New chat in {forum}" or "New post in
  /// {forum}"), or null for a row without it.
  final String? addLabel;

  /// Starts a new chat or post in the forum (the "+" action), or null for a row without it.
  final VoidCallback? onAdd;

  @override
  State<DsChannelRow> createState() => DsChannelRowState();
}

/// State of [DsChannelRow]; exposes the resolved state colors to tests.
class DsChannelRowState extends State<DsChannelRow> {
  bool _hovered = false;
  bool _focusInside = false;

  /// The name color of the current state.
  @visibleForTesting
  Color get nameColor {
    final c = context.dsColors;
    if (widget.data.muted) return c.textOnSidebarMuted;
    if (widget.data.unreadCount > 0) return c.textOnSidebar;
    return c.textOnSidebarSecondary;
  }

  /// Whether the unread bar on the left shows; the status mark replaces it (01 6.17 item 4).
  @visibleForTesting
  bool get unreadBar =>
      !widget.data.muted &&
      widget.data.unreadCount > 0 &&
      widget.data.status == null;

  /// The row background of the selected state, or null.
  @visibleForTesting
  Color? get background =>
      widget.selected ? context.dsColors.surfaceSidebarSelected : null;

  /// Whether the action "+" is built now: hover, focus on or inside the row, selected or
  /// current, or the density `touch` (components.md 7).
  @visibleForTesting
  bool get addVisible =>
      widget.addLabel != null &&
      widget.onAdd != null &&
      (_hovered ||
          _focusInside ||
          widget.selected ||
          widget.current ||
          _touch(context));

  @override
  Widget build(BuildContext context) {
    final l = dsL10n(context);
    final c = context.dsColors;
    final d = widget.data;
    final status = d.status;
    final label = [
      d.name,
      dsChannelKindName(l, d.kind, projectBound: d.projectBound),
      if (d.unreadCount > 0) l.threadRowUnread(d.unreadCount),
      if (d.mentionCount > 0) l.threadRowMentions(d.mentionCount),
      if (d.runActive) l.channelRowRunActive,
      if (d.muted) l.channelRowMuted,
      if (status != null) dsChatStatusLabel(l, status),
    ].join(', ');
    final color = nameColor;
    final rowHeight = context.dsDensity.rowChannel;
    final slot = dsChannelArrowSlot(context);
    final expanded = widget.groupExpanded;
    final toggle = widget.onToggleGroup;
    // A button on a hovered row shows its own hover one step brighter; on the selected row
    // the hover color is the step that differs from the row.
    final buttonHover = widget.selected
        ? c.surfaceSidebarHover
        : c.surfaceSidebarSelected;
    // Arrow, row and "+" are siblings: a button inside the row's own node would be a control
    // inside a control for a screen reader and would light the row's focus ring with its own.
    final row = DsTappable(
      onTap: widget.onOpen,
      focusNode: widget.focusNode,
      label: label,
      selected: widget.selected,
      expanded: expanded,
      button: false,
      onSidebar: true,
      // The frame paints the hover over the whole line, arrow and "+" included.
      hoverColor: Colors.transparent,
      child: Container(
        constraints: BoxConstraints(minHeight: rowHeight),
        padding: const EdgeInsetsDirectional.only(end: DsSpace.s8),
        child: Row(
          children: [
            // Without a group the empty slot belongs to the row, so a click there opens the
            // channel as the rest of the hovered line does.
            if (expanded == null) SizedBox(width: slot),
            // The kind sentence reaches the screen reader as the tooltip of the row node
            // (01 6.8 item 11); the icon itself is decorative.
            Tooltip(
              message: dsChannelKindSentence(
                l,
                d.kind,
                projectBound: d.projectBound,
              ),
              child: DsIcon(
                dsChannelGlyph(d.kind, projectBound: d.projectBound),
                size: DsIconSizeRole.sm,
                color: color,
                decorative: true,
              ),
            ),
            const SizedBox(width: DsSpace.s8),
            Expanded(
              child: ExcludeSemantics(
                child: Text(
                  dsIsolate(d.name),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: dsTextStyle(DsType.bodySm).copyWith(
                    color: color,
                    fontWeight: unreadBar ? FontWeight.w700 : null,
                  ),
                ),
              ),
            ),
            if (d.runActive) ...[
              const SizedBox(width: DsSpace.s6),
              _RunDot(color: c.categorySpruceOnSidebar),
            ],
            if (status != null) ...[
              const SizedBox(width: DsSpace.s6),
              DsChatStatusMark(status),
            ],
            if (d.mentionCount > 0) ...[
              const SizedBox(width: DsSpace.s6),
              ExcludeSemantics(
                child: DsBadge(
                  text: dsFormat(context).count(d.mentionCount),
                  background: c.attentionBg,
                  foreground: c.attentionOnBg,
                ),
              ),
            ],
            if (d.muted) ...[
              const SizedBox(width: DsSpace.s6),
              DsIcon(
                DsIcons.actionMute,
                size: DsIconSizeRole.sm,
                color: color,
                decorative: true,
              ),
            ],
          ],
        ),
      ),
    );
    final add = addVisible;
    final line = Container(
      constraints: BoxConstraints(minHeight: rowHeight),
      decoration: BoxDecoration(
        color: background ?? (_hovered ? c.surfaceSidebarHover : null),
        borderRadius: BorderRadius.circular(DsRadius.sm),
        border: unreadBar
            ? Border(
                left: BorderSide(
                  color: c.textOnSidebar,
                  width: DsBorderWidth.spine,
                ),
              )
            : null,
      ),
      child: Row(
        children: [
          if (expanded != null)
            SizedBox(
              width: slot,
              child: Center(
                child: _RowButton(
                  glyph: expanded
                      ? DsIcons.actionExpand
                      : DsIcons.actionCollapse,
                  label: expanded ? l.sidebarHideThreads : l.sidebarShowThreads,
                  hoverColor: buttonHover,
                  onTap: toggle == null ? null : () => toggle(!expanded),
                ),
              ),
            ),
          Expanded(child: row),
          if (add) ...[
            _RowButton(
              glyph: DsIcons.actionNew,
              label: widget.addLabel!,
              hoverColor: buttonHover,
              onTap: widget.onAdd,
            ),
            const SizedBox(width: DsSpace.s4),
          ],
        ],
      ),
    );
    // Hover and focus decide whether "+" is built; this node only listens, it never takes
    // the focus or a Tab stop itself.
    return MouseRegion(
      onEnter: (_) => setState(() => _hovered = true),
      onExit: (_) => setState(() => _hovered = false),
      child: Focus(
        canRequestFocus: false,
        skipTraversal: true,
        includeSemantics: false,
        onFocusChange: (focused) => setState(() => _focusInside = focused),
        child: line,
      ),
    );
  }
}

/// A small icon button of a channel row (the group arrow, the action "+"): hit area
/// `density.target-min`, tooltip and screen reader label [label].
class _RowButton extends StatelessWidget {
  const _RowButton({
    required this.glyph,
    required this.label,
    required this.hoverColor,
    required this.onTap,
  });

  final DsGlyph glyph;
  final String label;
  final Color hoverColor;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    return Tooltip(
      message: label,
      excludeFromSemantics: true,
      child: DsTappable(
        onTap: onTap,
        label: label,
        onSidebar: true,
        hoverColor: hoverColor,
        excludeChildSemantics: true,
        child: Center(
          widthFactor: 1,
          heightFactor: 1,
          child: DsIcon(
            glyph,
            size: DsIconSizeRole.sm,
            color: context.dsColors.textOnSidebarSecondary,
            decorative: true,
          ),
        ),
      ),
    );
  }
}

/// The dot of an active run; pulses once per `motion.duration.pulse`, still when the motion
/// context is reduced (a zero pulse).
class _RunDot extends StatefulWidget {
  const _RunDot({required this.color});

  final Color color;

  @override
  State<_RunDot> createState() => _RunDotState();
}

class _RunDotState extends State<_RunDot> with SingleTickerProviderStateMixin {
  /// The lowest opacity of the pulse; the dot never disappears, so the active run stays
  /// visible in every frame.
  static const double _pulseLow = 0.4;

  AnimationController? _pulse;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    final pulse = context.dsMotion.durationPulse;
    if (pulse <= Duration.zero) {
      _pulse?.dispose();
      _pulse = null;
    } else {
      (_pulse ??= AnimationController(vsync: this, duration: pulse))
        ..duration = pulse
        ..repeat(reverse: true);
    }
  }

  @override
  void dispose() {
    _pulse?.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    Widget dot = Container(
      width: DsSpace.s8,
      height: DsSpace.s8,
      decoration: BoxDecoration(color: widget.color, shape: BoxShape.circle),
    );
    final pulse = _pulse;
    if (pulse != null) {
      dot = FadeTransition(
        opacity: Tween<double>(begin: 1, end: _pulseLow).animate(pulse),
        child: dot,
      );
    }
    return dot;
  }
}
