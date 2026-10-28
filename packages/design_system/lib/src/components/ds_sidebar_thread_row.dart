// SPDX-License-Identifier: Apache-2.0

/// The thread rows of the sidebar tree and the row "All posts" (components.md section 7,
/// 01 6.8 items 1, 5 and 6).
library;

import 'package:flutter/material.dart';

import '../generated/design_tokens.g.dart';
import '../icons/ds_icon.dart';
import '../theme/ds_theme.dart';
import 'ds_channel_row.dart';
import 'ds_common.dart';
import 'view_models.dart';

/// The run state icon of a sidebar thread row and its color, or null when the state shows
/// none (01 6.8 item 6: running, waiting for an answer, failed).
///
/// The run colors of `color.run.*` are paired only with content surfaces; on the sidebar the
/// icon takes the `on-sidebar` step of the same category (`run.running` is spruce,
/// `run.waiting` gorse, `run.failed` rowan), which is paired with every sidebar surface. The
/// shapes differ, so the state never rests on the color alone.
({DsGlyph glyph, Color color})? _runMarker(
  BuildContext context,
  DsRunState? state,
) {
  final c = context.dsColors;
  return switch (state) {
    DsRunState.queued || DsRunState.running => (
      glyph: DsIcons.runRunning,
      color: c.categorySpruceOnSidebar,
    ),
    DsRunState.waitingInput => (
      glyph: DsIcons.runWaiting,
      color: c.categoryGorseOnSidebar,
    ),
    DsRunState.failed => (
      glyph: DsIcons.runFailed,
      color: c.categoryRowanOnSidebar,
    ),
    _ => null,
  };
}

/// The mention counter of a sidebar row, as the channel row draws it (components.md 7).
Widget _mentionBadge(BuildContext context, int count) {
  final c = context.dsColors;
  return DsBadge(
    text: dsFormat(context).count(count),
    background: c.attentionBg,
    foreground: c.attentionOnBg,
  );
}

/// A thread of a channel in the sidebar, nested below the channel row (components.md 7,
/// 01 6.8).
///
/// Height `density.row-channel`, indented one step further than the channel row: the tree
/// line in `color.text.on-sidebar-muted` with `border-width.hairline` runs under the channel
/// icon over the full height, or ends in the middle when the row is the [last] of its group,
/// with a short tick to the row (the `'-` of 01 D1). Before the title the run state icon at
/// `icon.size.sm` (`run.running` for queued and running, `run.waiting` for waiting_input,
/// `run.failed` for failed), never the run dot; the title in `type.body-sm`, one line, cut
/// with an ellipsis, the full title in a tooltip. An empty title reads "New chat" with the
/// "automatic" mark (01 6.8 item 6, as the thread header marks an automatic title).
///
/// The states of the channel row (components.md 7): read
/// (`color.text.on-sidebar-secondary`), unread (`color.text.on-sidebar`, bold, bar on the
/// left; a mark, never a number), mentions (counter on `color.attention.bg`), muted
/// (`color.text.on-sidebar-muted` and `action.mute`), selected
/// (`color.surface.sidebar-selected`), hover (`color.surface.sidebar-hover`) and focus (ring
/// `color.focus.ring-on-sidebar`); a done thread, shown only while selected, reads in
/// `color.text.on-sidebar-muted`.
///
/// One screen reader node with the label "{Title}, {State}, {n} unread, {m} mentions" (the
/// state is the run state word while the icon shows, else "Open" or "Done"; counts only above
/// 0; "muted" at the end like the channel row) and the selected flag; Flutter has no tree role
/// or level, so the tree is the order of the rows below their channel. Enter or a tap calls
/// [onOpen]. The row cannot be dragged (01 6.8 item 10).
///
/// ```dart
/// DsSidebarThreadRow(thread, selected: thread.id == current, onOpen: () => open(thread.id))
/// ```
class DsSidebarThreadRow extends StatefulWidget {
  /// Creates the row of [data].
  const DsSidebarThreadRow(
    this.data, {
    super.key,
    required this.onOpen,
    this.selected = false,
    this.last = false,
    this.focusNode,
  });

  /// The thread.
  final DsSidebarThreadRowData data;

  /// Opens the thread.
  final VoidCallback onOpen;

  /// Whether the thread is the open one.
  final bool selected;

  /// Whether the row is the last of its group; the tree line then ends at its middle. In a
  /// forum group "All posts" is last, so no thread row of a forum is.
  final bool last;

  /// The focus node of the row, so the caller's tree navigation can move the keyboard focus
  /// to it (01 6.8 item 7, D-SB-8); null for one of its own.
  final FocusNode? focusNode;

  @override
  State<DsSidebarThreadRow> createState() => DsSidebarThreadRowState();
}

/// State of [DsSidebarThreadRow]; exposes the resolved state colors to tests.
class DsSidebarThreadRowState extends State<DsSidebarThreadRow> {
  /// The title color of the current state.
  @visibleForTesting
  Color get nameColor {
    final c = context.dsColors;
    final d = widget.data;
    if (d.muted || d.done) return c.textOnSidebarMuted;
    if (d.unreadCount > 0) return c.textOnSidebar;
    return c.textOnSidebarSecondary;
  }

  /// Whether the unread bar on the left shows.
  @visibleForTesting
  bool get unreadBar => !widget.data.muted && widget.data.unreadCount > 0;

  /// The row background of the selected state, or null.
  @visibleForTesting
  Color? get background =>
      widget.selected ? context.dsColors.surfaceSidebarSelected : null;

  @override
  Widget build(BuildContext context) {
    final l = dsL10n(context);
    final c = context.dsColors;
    final d = widget.data;
    final marker = _runMarker(context, d.runState);
    final untitled = d.title.trim().isEmpty;
    final title = untitled ? l.sidebarNewChat : d.title;
    // A finished run says nothing beyond the thread's own state; only the states that show
    // an icon have a word of their own (01 6.8 item 6).
    final state = marker == null
        ? (d.done ? l.threadRowStateDone : l.threadRowStateOpen)
        : switch (d.runState) {
            DsRunState.queued => l.liveBoxStateQueued,
            DsRunState.waitingInput => l.threadRowStateWaiting,
            DsRunState.failed => l.liveBoxStateFailed,
            _ => l.liveBoxStateRunning,
          };
    final label = [
      title,
      if (untitled) l.threadTitleAutomatic,
      state,
      if (d.unreadCount > 0) l.threadRowUnread(d.unreadCount),
      if (d.mentionCount > 0) l.threadRowMentions(d.mentionCount),
      if (d.muted) l.channelRowMuted,
    ].join(', ');
    final color = nameColor;
    return _SidebarTreeRow(
      label: label,
      selected: widget.selected,
      last: widget.last,
      unreadBar: unreadBar,
      background: background,
      onOpen: widget.onOpen,
      focusNode: widget.focusNode,
      children: [
        if (marker != null) ...[
          DsIcon(
            marker.glyph,
            size: DsIconSizeRole.sm,
            color: marker.color,
            decorative: true,
          ),
          const SizedBox(width: DsSpace.s6),
        ],
        Expanded(
          child: Row(
            children: [
              Flexible(
                child: Tooltip(
                  message: title,
                  excludeFromSemantics: true,
                  child: Text(
                    dsIsolate(title),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: dsTextStyle(DsType.bodySm).copyWith(
                      color: color,
                      fontWeight: unreadBar ? FontWeight.w700 : null,
                    ),
                  ),
                ),
              ),
              if (untitled) ...[
                const SizedBox(width: DsSpace.s6),
                // The mark of an automatic title as in the thread header, in the sidebar
                // pairing of `color.text.on-sidebar-secondary` on `surface.sidebar-hover`.
                // Flexible: in a narrow sidebar at text scale 2 title and mark share the
                // width and both cut with an ellipsis instead of overflowing the row.
                Flexible(
                  child: DsBadge(
                    text: l.threadTitleAutomatic,
                    background: c.surfaceSidebarHover,
                    foreground: c.textOnSidebarSecondary,
                  ),
                ),
              ],
            ],
          ),
        ),
        if (d.mentionCount > 0) ...[
          const SizedBox(width: DsSpace.s6),
          _mentionBadge(context, d.mentionCount),
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
    );
  }
}

/// The last row of an expanded forum group: "All posts", which opens the forum overview D15
/// (01 6.8 item 5, D-SB-4).
///
/// Same indent and tree line as [DsSidebarThreadRow], always the last row (the line ends at
/// its middle). The text in `color.text.on-sidebar-secondary`; it carries the marks of the
/// posts the group does not show: unread as the unread state of the table (bold,
/// `color.text.on-sidebar`, bar on the left) and the mention counter, never a total. The
/// label reads "All posts, {n} unread, {m} mentions" (counts only above 0); Enter or a tap
/// calls [onOpen]. Not draggable.
///
/// ```dart
/// DsSidebarAllPostsRow(unreadCount: hidden.unread, onOpen: () => openOverview(forum.id))
/// ```
class DsSidebarAllPostsRow extends StatefulWidget {
  /// Creates the row.
  const DsSidebarAllPostsRow({
    super.key,
    required this.onOpen,
    this.unreadCount = 0,
    this.mentionCount = 0,
    this.selected = false,
    this.focusNode,
  });

  /// Opens the forum overview.
  final VoidCallback onOpen;

  /// Unread messages of the posts not shown in the group; more than 0 is the state "unread".
  final int unreadCount;

  /// Unread mentions of the posts not shown in the group; more than 0 shows the counter.
  final int mentionCount;

  /// Whether the forum overview is the open view.
  final bool selected;

  /// The focus node of the row, so the caller's tree navigation can move the keyboard focus
  /// to it (01 6.8 item 7, D-SB-8); null for one of its own.
  final FocusNode? focusNode;

  @override
  State<DsSidebarAllPostsRow> createState() => DsSidebarAllPostsRowState();
}

/// State of [DsSidebarAllPostsRow]; exposes the resolved state colors to tests.
class DsSidebarAllPostsRowState extends State<DsSidebarAllPostsRow> {
  /// The text color of the current state.
  @visibleForTesting
  Color get nameColor => unreadBar
      ? context.dsColors.textOnSidebar
      : context.dsColors.textOnSidebarSecondary;

  /// Whether the unread bar on the left shows.
  @visibleForTesting
  bool get unreadBar => widget.unreadCount > 0;

  /// The row background of the selected state, or null.
  @visibleForTesting
  Color? get background =>
      widget.selected ? context.dsColors.surfaceSidebarSelected : null;

  @override
  Widget build(BuildContext context) {
    final l = dsL10n(context);
    final label = [
      l.sidebarAllPosts,
      if (widget.unreadCount > 0) l.threadRowUnread(widget.unreadCount),
      if (widget.mentionCount > 0) l.threadRowMentions(widget.mentionCount),
    ].join(', ');
    return _SidebarTreeRow(
      label: label,
      selected: widget.selected,
      last: true,
      unreadBar: unreadBar,
      background: background,
      onOpen: widget.onOpen,
      focusNode: widget.focusNode,
      children: [
        Expanded(
          child: Text(
            l.sidebarAllPosts,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: dsTextStyle(DsType.bodySm).copyWith(
              color: nameColor,
              fontWeight: unreadBar ? FontWeight.w700 : null,
            ),
          ),
        ),
        if (widget.mentionCount > 0) ...[
          const SizedBox(width: DsSpace.s6),
          _mentionBadge(context, widget.mentionCount),
        ],
      ],
    );
  }
}

/// The frame of a row below a channel: the tree line in the indent, then one tappable line
/// with the states of the channel row (components.md 7).
class _SidebarTreeRow extends StatelessWidget {
  const _SidebarTreeRow({
    required this.label,
    required this.selected,
    required this.last,
    required this.unreadBar,
    required this.background,
    required this.onOpen,
    required this.focusNode,
    required this.children,
  });

  final String label;
  final bool selected;
  final bool last;
  final bool unreadBar;
  final Color? background;
  final VoidCallback onOpen;
  final FocusNode? focusNode;
  final List<Widget> children;

  @override
  Widget build(BuildContext context) {
    final c = context.dsColors;
    final slot = dsChannelArrowSlot(context);
    // One step further than the channel row: past its arrow slot and its icon, so the title
    // starts where the channel name starts and the line runs under the channel icon.
    final indent = slot + DsIconSize.sizeSm;
    return Stack(
      children: [
        Positioned.fill(
          child: CustomPaint(
            painter: _TreeLinePainter(
              color: c.textOnSidebarMuted,
              lineX: slot + DsIconSize.sizeSm / 2,
              tickEnd: indent,
              last: last,
              textDirection: Directionality.of(context),
            ),
          ),
        ),
        Padding(
          padding: EdgeInsetsDirectional.only(start: indent),
          child: DsTappable(
            onTap: onOpen,
            focusNode: focusNode,
            label: label,
            selected: selected,
            button: false,
            onSidebar: true,
            hoverColor: c.surfaceSidebarHover,
            excludeChildSemantics: true,
            child: Container(
              constraints: BoxConstraints(
                minHeight: context.dsDensity.rowChannel,
              ),
              decoration: BoxDecoration(
                color: background,
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
              padding: const EdgeInsets.symmetric(horizontal: DsSpace.s8),
              child: Row(children: children),
            ),
          ),
        ),
      ],
    );
  }
}

/// The tree line of a row below a channel: a vertical hairline at [lineX] over the full
/// height (or to the middle for the [last] row) and a tick from it to [tickEnd] at the middle.
/// Rectangles on whole pixels instead of stroked lines, so the hairline stays one sharp pixel.
class _TreeLinePainter extends CustomPainter {
  const _TreeLinePainter({
    required this.color,
    required this.lineX,
    required this.tickEnd,
    required this.last,
    required this.textDirection,
  });

  final Color color;
  final double lineX;
  final double tickEnd;
  final bool last;
  final TextDirection textDirection;

  @override
  void paint(Canvas canvas, Size size) {
    const w = DsBorderWidth.hairline;
    final paint = Paint()..color = color;
    final mid = (size.height / 2).floorToDouble();
    final rtl = textDirection == TextDirection.rtl;
    final x = rtl ? size.width - lineX - w : lineX;
    canvas.drawRect(
      Rect.fromLTWH(x, 0, w, last ? mid + w : size.height),
      paint,
    );
    final tickStart = rtl ? size.width - tickEnd : lineX;
    canvas.drawRect(Rect.fromLTWH(tickStart, mid, tickEnd - lineX, w), paint);
  }

  @override
  bool shouldRepaint(_TreeLinePainter old) =>
      old.color != color ||
      old.lineX != lineX ||
      old.tickEnd != tickEnd ||
      old.last != last ||
      old.textDirection != textDirection;
}
