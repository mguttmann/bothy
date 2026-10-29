// SPDX-License-Identifier: Apache-2.0

/// The columns of the desktop main view (docs/spec/01-product.md 5.1 D1, D2 and D15,
/// R-PR-17; briefs M4-18 Design note 5 and M4-40 Design note 4): three by default, the pinned
/// forum overview as the optional fourth.
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../../keyboard/app_shortcuts.dart';
import '../../keyboard/column_focus.dart';
import 'column_widths.dart';

/// Sidebar, history and session side by side, with drag handles between them; the forum
/// overview ([postList]) is its own column between sidebar and history only while
/// [listPinned] (01 D15).
///
/// Below [minWidthFourColumns] (`ui.desktop_min_width_four_columns`) the session column
/// collapses. Without the pin, and below [minWidthThreeColumns]
/// (`ui.desktop_min_width_three_columns`) also with it, the overview and the history share the
/// content column like the navigation stack of the phone, and [stackShowsHistory] (a thread is
/// open) decides which one shows. A drag changes the width at once and calls
/// [onWidthsChanged] when the drag ends, so one drag is one write; the dragged widths stay
/// until [widths] changes.
class ColumnLayout extends StatefulWidget {
  /// Creates the layout.
  const ColumnLayout({
    super.key,
    required this.widths,
    required this.minWidthFourColumns,
    required this.minWidthThreeColumns,
    required this.onWidthsChanged,
    required this.sidebar,
    required this.postList,
    required this.history,
    required this.session,
    this.stackShowsHistory = false,
    this.showPostList = true,
    this.listPinned = true,
    this.showSession = true,
  });

  /// The stored widths.
  final ColumnWidths widths;

  /// `ui.desktop_min_width_four_columns`.
  final double minWidthFourColumns;

  /// `ui.desktop_min_width_three_columns`.
  final double minWidthThreeColumns;

  /// Called with the new widths when a drag ends.
  final ValueChanged<ColumnWidths> onWidthsChanged;

  /// The column contents, left to right.
  final Widget sidebar;

  /// The post list.
  final Widget postList;

  /// The history.
  final Widget history;

  /// The session column.
  final Widget session;

  /// In the stacked form: whether the history (a thread is open) or the post list shows.
  final bool stackShowsHistory;

  /// Whether the route has a forum overview; only a forum has one (01 5.1: D2, a text
  /// channel, shows none; brief M4-19 Design note 5).
  final bool showPostList;

  /// Whether the overview is pinned as its own column (01 D15; kept on this device).
  final bool listPinned;

  /// Whether the session column shows when the window is wide enough; the header button
  /// "Column" hides it, and a thread of a text channel collapses it (01 5.1 D1, D2; M4-20).
  final bool showSession;

  @override
  State<ColumnLayout> createState() => _ColumnLayoutState();
}

class _ColumnLayoutState extends State<ColumnLayout> {
  ColumnWidths? _dragging;
  double _total = 0;
  List<String> _sized = const [];

  ColumnWidths get _widths => _dragging ?? widget.widths;

  void _drag(String column, double delta) {
    final widths = _widths;
    final current = widths.of(column);
    // The history keeps at least `DsLayout.columnMin`; the room is computed from the widths of
    // this update, since several updates can arrive before the next build.
    final used = _sized.fold<double>(0, (sum, c) => sum + widths.of(c));
    final room =
        _total - used - _sized.length * DsSpace.s4 - DsLayout.columnMin;
    var next = current + delta;
    final max = current + (room > 0 ? room : 0);
    if (next > max) next = max;
    setState(() => _dragging = widths.withWidth(column, next));
  }

  void _end() {
    final widths = _dragging;
    if (widths != null) widget.onWidthsChanged(widths);
  }

  @override
  void didUpdateWidget(ColumnLayout old) {
    super.didUpdateWidget(old);
    // The dragged widths stay until the stored setting comes back (or another device changes
    // it), so the columns do not jump back while the write travels.
    if (old.widths != widget.widths) _dragging = null;
  }

  @override
  Widget build(BuildContext context) => LayoutBuilder(
    builder: (context, constraints) {
      final total = constraints.maxWidth;
      final four = total >= widget.minWidthFourColumns && widget.showSession;
      final stacked = total < widget.minWidthThreeColumns;
      final list = widget.showPostList;
      // The overview has a column of its own only pinned and with room for it (01 D1, D15).
      final column = list && widget.listPinned && !stacked;
      final sized = <String>[
        AppColumn.sidebar,
        if (column) AppColumn.postList,
        if (four) AppColumn.session,
      ];
      final widths = fitColumns(_widths, sized: sized, total: total);
      _total = total;
      _sized = sized;
      Widget fixed(String column, Widget child) => SizedBox(
        width: widths.of(column),
        child: _ColumnFrame(column: column, child: child),
      );
      Widget handle(String column, {bool growsLeft = false}) =>
          ColumnDragHandle(
            key: ValueKey('column.handle.$column'),
            onDrag: (dx) => _drag(column, growsLeft ? -dx : dx),
            onEnd: _end,
          );
      final middle = list && !column
          ? (widget.stackShowsHistory
                ? _ColumnFrame(column: AppColumn.history, child: widget.history)
                : _ColumnFrame(
                    column: AppColumn.postList,
                    child: widget.postList,
                  ))
          : _ColumnFrame(column: AppColumn.history, child: widget.history);
      return Row(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          fixed(AppColumn.sidebar, widget.sidebar),
          handle(AppColumn.sidebar),
          if (column) ...[
            fixed(AppColumn.postList, widget.postList),
            handle(AppColumn.postList),
          ],
          Expanded(child: middle),
          if (four) ...[
            handle(AppColumn.session, growsLeft: true),
            fixed(AppColumn.session, widget.session),
          ],
        ],
      );
    },
  );
}

/// The frame of one column: a semantics container with the column's name and the test key
/// `column.<name>`, and a focus group in reading order that F6 moves between (01 8 "Focus",
/// M4-24); later tickets fill it.
class _ColumnFrame extends StatelessWidget {
  const _ColumnFrame({required this.column, required this.child});

  final String column;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final label = switch (column) {
      AppColumn.sidebar => l.shellColumnSidebar,
      AppColumn.postList => l.shellColumnPostList,
      AppColumn.session => l.shellColumnSession,
      _ => l.shellColumnHistory,
    };
    return Semantics(
      key: ValueKey('column.$column'),
      container: true,
      explicitChildNodes: true,
      label: label,
      child: ColoredBox(
        color: column == AppColumn.sidebar
            ? colors.surfaceSidebar
            : colors.surfaceCanvas,
        child: ColumnFocusGroup(
          column: column,
          registry: KeyboardScope.maybeOf(context)?.columns,
          child: child,
        ),
      ),
    );
  }
}

/// The draggable border between two columns: [onDrag] gets the horizontal movement of each
/// update, [onEnd] is called once when the drag ends (01 5.1 D1: every column border can be
/// dragged; M4-42 uses it between the main history and the thread too).
class ColumnDragHandle extends StatelessWidget {
  /// Creates the handle.
  const ColumnDragHandle({
    super.key,
    required this.onDrag,
    required this.onEnd,
  });

  /// Called with the horizontal movement of a drag update.
  final ValueChanged<double> onDrag;

  /// Called when the drag ends.
  final VoidCallback onEnd;

  @override
  Widget build(BuildContext context) => Semantics(
    label: AppLocalizations.of(context).shellResizeColumn,
    child: MouseRegion(
      cursor: SystemMouseCursors.resizeColumn,
      child: GestureDetector(
        behavior: HitTestBehavior.opaque,
        onHorizontalDragUpdate: (d) => onDrag(d.delta.dx),
        onHorizontalDragEnd: (_) => onEnd(),
        child: SizedBox(
          width: DsSpace.s4,
          child: Center(
            child: VerticalDivider(
              width: DsBorderWidth.hairline,
              thickness: DsBorderWidth.hairline,
              color: context.dsColors.borderSubtle,
            ),
          ),
        ),
      ),
    ),
  );
}

/// [widths] fitted to a window of [total] pixels: when the [sized] columns, their handles and
/// the minimum of the history do not fit, the sized columns shrink in proportion to what they
/// have above `DsLayout.columnMin` (a stored width may come from a larger screen, 01 11).
///
/// Example: `fitColumns(stored, sized: [AppColumn.sidebar], total: 800)`.
ColumnWidths fitColumns(
  ColumnWidths widths, {
  required List<String> sized,
  required double total,
}) {
  final available = total - sized.length * DsSpace.s4 - DsLayout.columnMin;
  final used = sized.fold<double>(0, (sum, c) => sum + widths.of(c));
  final excess = used - available;
  if (excess <= 0) return widths;
  final slack = sized.fold<double>(
    0,
    (sum, c) => sum + widths.of(c) - DsLayout.columnMin,
  );
  if (slack <= 0) return widths;
  final keep = excess >= slack ? 0.0 : 1 - excess / slack;
  var fitted = widths;
  for (final c in sized) {
    final extra = widths.of(c) - DsLayout.columnMin;
    fitted = fitted.withWidth(c, DsLayout.columnMin + extra * keep);
  }
  return fitted;
}
