// SPDX-License-Identifier: Apache-2.0

/// The four columns of the desktop main view (docs/spec/01-produkt.md 5.1 D1 and D2, R-PR-01;
/// brief M4-18 Design note 5).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import 'column_widths.dart';

/// Sidebar, post list, history and session side by side, with drag handles between them.
///
/// Below [minWidthFourColumns] (`ui.desktop_min_width_four_columns`) the session column
/// collapses; below [minWidthThreeColumns] (`ui.desktop_min_width_three_columns`) the post
/// list and the history share one place like the navigation stack of the phone, and
/// [stackShowsHistory] decides which one is on top. A drag changes the width at once and calls
/// [onWidthsChanged] when the drag ends, so one drag is one write.
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

  @override
  State<ColumnLayout> createState() => _ColumnLayoutState();
}

class _ColumnLayoutState extends State<ColumnLayout> {
  ColumnWidths? _dragging;

  ColumnWidths get _widths => _dragging ?? widget.widths;

  void _drag(String column, double delta, double room) {
    final current = _widths.of(column);
    var next = current + delta;
    final max = room + current - DsLayout.columnMin;
    if (next > max) next = max;
    setState(() => _dragging = _widths.withWidth(column, next));
  }

  void _end() {
    final widths = _dragging;
    if (widths == null) return;
    widget.onWidthsChanged(widths);
    setState(() => _dragging = null);
  }

  @override
  void didUpdateWidget(ColumnLayout old) {
    super.didUpdateWidget(old);
    if (old.widths != widget.widths && _dragging == null) setState(() {});
  }

  @override
  Widget build(BuildContext context) => LayoutBuilder(
    builder: (context, constraints) {
      final total = constraints.maxWidth;
      final four = total >= widget.minWidthFourColumns;
      final stacked = total < widget.minWidthThreeColumns;
      final widths = _widths;
      final sized = <String>[
        AppColumn.sidebar,
        if (!stacked) AppColumn.postList,
        if (four) AppColumn.session,
      ];
      final used = sized.fold<double>(0, (sum, c) => sum + widths.of(c));
      final room = total - used - DsLayout.columnMin;
      Widget fixed(String column, Widget child) => SizedBox(
        width: widths.of(column),
        child: _ColumnFrame(column: column, child: child),
      );
      Widget handle(String column, {bool growsLeft = false}) => _DragHandle(
        key: ValueKey('column.handle.$column'),
        onDrag: (dx) => _drag(column, growsLeft ? -dx : dx, room),
        onEnd: _end,
      );
      final middle = stacked
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
          if (!stacked) ...[
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
/// `column.<name>`; later tickets fill it.
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
        child: child,
      ),
    );
  }
}

/// The draggable border between two columns.
class _DragHandle extends StatelessWidget {
  const _DragHandle({super.key, required this.onDrag, required this.onEnd});

  final ValueChanged<double> onDrag;
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
