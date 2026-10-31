// SPDX-License-Identifier: Apache-2.0

/// The thread rows of the sidebar tree, the draft row "New chat" and the row "All posts"
/// (components.md section 7, 01 6.8 items 1, 5, 6 and 8).
library;

import 'package:flutter/material.dart';

import '../generated/design_tokens.g.dart';
import '../icons/ds_icon.dart';
import '../theme/ds_theme.dart';
import 'ds_channel_row.dart';
import 'ds_chat_status.dart';
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
/// A chat whose agent state changed since the person last had it open ([DsSidebarThreadRowData.status],
/// 01 6.17) shows the [DsChatStatusMark] at the trailing edge before the mention counter, and no
/// unread bar while it does (the title stays bold while unread).
///
/// One screen reader node with the label "{Title}, {State}, {n} unread, {m} mentions" (the
/// state is the run state word while the icon shows, else "Open" or "Done"; counts only above
/// 0; "muted" and then the words of the status mark at the end) and the selected flag; Flutter has no tree role
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

  @override
  Widget build(BuildContext context) {
    final l = dsL10n(context);
    final c = context.dsColors;
    final d = widget.data;
    final status = d.status;
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
      if (status != null) dsChatStatusLabel(l, status),
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
        if (status != null) ...[
          const SizedBox(width: DsSpace.s6),
          DsChatStatusMark(status),
        ],
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

/// The sending state of a [DsSidebarDraftRow] (01 6.6 and 6.8 item 8).
enum DsDraftRowState {
  /// The person writes the new post; nothing is sent yet.
  draft,

  /// The post is in the outbox (`thread.create`) and the server has not confirmed it yet.
  sending,

  /// The server refused the post; the forum overview offers Try again.
  failed,
}

/// The draft row "New chat" at the top of a forum group: the person's draft of a new post in
/// this forum (components.md 7, 01 6.8 item 8).
///
/// Same height, indent, tree line and selected, hover and focus states as
/// [DsSidebarThreadRow]. The text is [title], or "New chat" while the title is blank, in
/// italics in `type.body-sm` and `color.text.on-sidebar-secondary`, one line cut with an
/// ellipsis, the full text in a tooltip. [state] [DsDraftRowState.sending] adds the badge
/// "Sending" after the text; [DsDraftRowState.failed] puts the `run.failed` icon
/// (`category.rowan.on-sidebar`, `icon.size.sm`) before the text and the badge "Not sent" after
/// it (01 6.6). The badges take the sidebar pairing of the "automatic" mark and share the width
/// with the text, so nothing overflows in a narrow sidebar at text scale 2.
///
/// One screen reader node with the label "{Text}, {State}" (the state is "Draft", "Sending" or
/// "Not sent") and the selected flag. Enter or a tap calls [onOpen]. The row cannot be dragged
/// (01 6.8 item 10).
///
/// ```dart
/// DsSidebarDraftRow(title: draft.title, selected: true, onOpen: () => openDraft(forum.id))
/// ```
class DsSidebarDraftRow extends StatefulWidget {
  /// Creates the draft row with the draft's [title] (blank reads "New chat"), which [onOpen]
  /// opens; [state] is the sending state, [selected] whether the new chat view is open,
  /// [last] whether the row ends its group and [focusNode] the caller's node, if any.
  const DsSidebarDraftRow({
    super.key,
    required this.title,
    required this.onOpen,
    this.state = DsDraftRowState.draft,
    this.selected = false,
    this.last = false,
    this.focusNode,
  });

  /// The title the person typed for the new post; blank reads "New chat".
  final String title;

  /// Opens the new chat view with the draft.
  final VoidCallback onOpen;

  /// The sending state of the draft.
  final DsDraftRowState state;

  /// Whether the new chat view of this draft is the open view.
  final bool selected;

  /// Whether the row is the last of its group; the tree line then ends at its middle. In a
  /// forum group "All posts" is last, so the draft row of a forum is not.
  final bool last;

  /// The focus node of the row, so the caller's tree navigation can move the keyboard focus
  /// to it (01 6.8 item 7, D-SB-8); null for one of its own.
  final FocusNode? focusNode;

  @override
  State<DsSidebarDraftRow> createState() => DsSidebarDraftRowState();
}

/// State of [DsSidebarDraftRow]; exposes the resolved text style and colors to tests.
class DsSidebarDraftRowState extends State<DsSidebarDraftRow> {
  /// The text color; a draft always reads in `color.text.on-sidebar-secondary`.
  @visibleForTesting
  Color get nameColor => context.dsColors.textOnSidebarSecondary;

  /// Whether the text is in italics; a draft always is (components.md 7).
  @visibleForTesting
  bool get italic => true;

  /// The row background of the selected state, or null.
  @visibleForTesting
  Color? get background =>
      widget.selected ? context.dsColors.surfaceSidebarSelected : null;

  @override
  Widget build(BuildContext context) {
    final l = dsL10n(context);
    final c = context.dsColors;
    final text = widget.title.trim().isEmpty ? l.sidebarNewChat : widget.title;
    final badge = switch (widget.state) {
      DsDraftRowState.draft => null,
      DsDraftRowState.sending => l.messageSending,
      DsDraftRowState.failed => l.sidebarDraftNotSent,
    };
    final label = '$text, ${badge ?? l.sidebarDraftState}';
    return _SidebarTreeRow(
      label: label,
      selected: widget.selected,
      last: widget.last,
      unreadBar: false,
      background: background,
      onOpen: widget.onOpen,
      focusNode: widget.focusNode,
      children: [
        if (widget.state == DsDraftRowState.failed) ...[
          DsIcon(
            DsIcons.runFailed,
            size: DsIconSizeRole.sm,
            color: c.categoryRowanOnSidebar,
            decorative: true,
          ),
          const SizedBox(width: DsSpace.s6),
        ],
        Expanded(
          child: Row(
            children: [
              Flexible(
                child: Tooltip(
                  message: text,
                  excludeFromSemantics: true,
                  child: Text(
                    dsIsolate(text),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: dsTextStyle(DsType.bodySm).copyWith(
                      color: nameColor,
                      fontStyle: italic ? FontStyle.italic : null,
                    ),
                  ),
                ),
              ),
              if (badge != null) ...[
                const SizedBox(width: DsSpace.s6),
                // The pairing of the "automatic" mark of the thread row; Flexible, so text and
                // badge share a narrow width at text scale 2 instead of overflowing the row.
                Flexible(
                  child: DsBadge(
                    text: badge,
                    background: c.surfaceSidebarHover,
                    foreground: c.textOnSidebarSecondary,
                  ),
                ),
              ],
            ],
          ),
        ),
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

/// The row "Show more" of a forum group with more open posts than it shows, above "All posts"
/// (01 6.8 item 3, M4-46); "Show less" while the group lists them all.
///
/// Same indent and tree line as [DsSidebarThreadRow], never the last row ("All posts" follows).
/// The text in `color.text.on-sidebar-secondary`, no counters: the unread marks of the posts
/// not shown stay on "All posts". The label reads "Show more, {n} more posts" ([hiddenCount]);
/// Enter or a tap calls [onToggle], which expands or collapses the list in place. A button, not
/// draggable.
///
/// ```dart
/// DsSidebarMoreRow(hiddenCount: 9, expanded: false, onToggle: () => showAll(forum.id))
/// ```
class DsSidebarMoreRow extends StatelessWidget {
  /// Creates the row.
  const DsSidebarMoreRow({
    super.key,
    required this.hiddenCount,
    required this.expanded,
    required this.onToggle,
    this.focusNode,
  });

  /// The open posts the row shows ("Show more") or hides again ("Show less").
  final int hiddenCount;

  /// Whether the group lists every open post; the row then reads "Show less".
  final bool expanded;

  /// Expands or collapses the list.
  final VoidCallback onToggle;

  /// The focus node of the row, so the caller's tree navigation can move the keyboard focus
  /// to it (D-SB-8); null for one of its own.
  final FocusNode? focusNode;

  @override
  Widget build(BuildContext context) {
    final l = dsL10n(context);
    final text = expanded ? l.sidebarShowLess : l.sidebarShowMore;
    return _SidebarTreeRow(
      label: '$text, ${l.sidebarMorePosts(hiddenCount)}',
      selected: false,
      last: false,
      unreadBar: false,
      background: null,
      onOpen: onToggle,
      focusNode: focusNode,
      button: true,
      children: [
        Expanded(
          child: Text(
            text,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: dsTextStyle(DsType.bodySm)
                .copyWith(color: context.dsColors.textOnSidebarSecondary),
          ),
        ),
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
    this.button = false,
  });

  final String label;
  final bool button;
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
            button: button,
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
