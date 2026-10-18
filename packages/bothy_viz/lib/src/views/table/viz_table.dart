// SPDX-License-Identifier: Apache-2.0

/// The table view of every view (docs/spec/25-visualization.md 7.4; brief M6-12, design note 3).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/semantics.dart';

import '../../marks/viz_texts.dart';

/// One cell: its text and the key it sorts by (a number for numeric columns).
final class VizCell {
  /// Creates a cell; the sort key defaults to [text].
  const VizCell(this.text, {this.sortBy});

  /// The text, the same the view shows.
  final String text;

  /// The key of the sort, for example the number behind a formatted count.
  final Comparable<Object>? sortBy;

  /// The key of the sort: [sortBy], else [text].
  Comparable<Object> get sortKey => sortBy ?? text;
}

/// One table with its caption, column headers and rows.
///
/// A row has exactly one cell per column. The table never holds a value the view does not
/// show (design note 3).
final class VizTableData {
  /// Creates the data.
  VizTableData({
    required this.caption,
    required this.columns,
    required this.rows,
  }) : assert(
         rows.every((r) => r.length == columns.length),
         'one cell per column',
       );

  /// Caption, read before the table (the section or view title).
  final String caption;

  /// Column headers from `l10n`.
  final List<String> columns;

  /// Rows in the order of the view.
  final List<List<VizCell>> rows;

  /// Every cell text, row by row: what the tests compare with the view.
  Iterable<String> get texts => rows.expand((r) => r.map((c) => c.text));
}

/// A data table: header row with sort buttons, one row per entry, readable by screen
/// readers as a table (table, row, column header and cell roles plus a label per cell).
///
/// A tap on a header sorts by that column, a second tap reverses the order. Long tables
/// scroll horizontally inside the view; the rows are never cut.
class VizTable extends StatefulWidget {
  /// Creates the table of [data].
  const VizTable({super.key, required this.data});

  /// The table.
  final VizTableData data;

  @override
  State<VizTable> createState() => _VizTableState();
}

class _VizTableState extends State<VizTable> {
  int? _sortColumn;
  bool _ascending = true;

  List<List<VizCell>> get _rows {
    final column = _sortColumn;
    final rows = [...widget.data.rows];
    if (column == null) return rows;
    rows.sort((a, b) {
      final order = _compare(a[column].sortKey, b[column].sortKey);
      return _ascending ? order : -order;
    });
    return rows;
  }

  static int _compare(Comparable<Object> a, Comparable<Object> b) {
    if (a.runtimeType == b.runtimeType) return a.compareTo(b);
    // Mixed keys (a number and "not shown"): numbers first, so the text stays at the end.
    return a is num ? -1 : 1;
  }

  void _sort(int column) => setState(() {
    if (_sortColumn == column) {
      _ascending = !_ascending;
    } else {
      _sortColumn = column;
      _ascending = true;
    }
  });

  @override
  Widget build(BuildContext context) {
    final texts = VizTexts.of(context);
    final colors = context.dsColors;
    final data = widget.data;
    final cellStyle = dsTextStyle(DsType.bodySm)
        .copyWith(color: colors.textPrimary);
    final headStyle = dsTextStyle(DsType.label)
        .copyWith(color: colors.textSecondary);
    const pad = EdgeInsets.symmetric(
      horizontal: DsSpace.s8,
      vertical: DsSpace.s4,
    );
    // The render table gives the table, row and cell roles itself; the header cells add the
    // column header role, and every cell a label with its column.
    return Semantics(
      container: true,
      label: data.caption,
      explicitChildNodes: true,
      child: SingleChildScrollView(
        scrollDirection: Axis.horizontal,
        child: Table(
          defaultColumnWidth: const IntrinsicColumnWidth(),
          defaultVerticalAlignment: TableCellVerticalAlignment.middle,
          border: TableBorder(
            horizontalInside: BorderSide(
              color: colors.borderSubtle,
              width: DsBorderWidth.hairline,
            ),
          ),
          children: [
            TableRow(
              decoration: BoxDecoration(color: colors.surfaceSunken),
              children: [
                for (var i = 0; i < data.columns.length; i++)
                  Semantics(
                    role: SemanticsRole.columnHeader,
                    button: true,
                    label: texts.l10n.vizSortBy(data.columns[i]),
                    onTap: () => _sort(i),
                    child: ExcludeSemantics(
                      child: InkWell(
                        onTap: () => _sort(i),
                        child: ConstrainedBox(
                          constraints: BoxConstraints(
                            minHeight: context.dsDensity.targetMin,
                          ),
                          child: Padding(
                            padding: pad,
                            child: Row(
                              mainAxisSize: MainAxisSize.min,
                              children: [
                                Text(data.columns[i], style: headStyle),
                                if (_sortColumn == i)
                                  DsIcon(
                                    _ascending
                                        ? DsIcons.actionExpand
                                        : DsIcons.actionCollapse,
                                    size: DsIconSizeRole.sm,
                                    color: colors.textSecondary,
                                    decorative: true,
                                  ),
                              ],
                            ),
                          ),
                        ),
                      ),
                    ),
                  ),
              ],
            ),
            for (final row in _rows)
              TableRow(
                children: [
                  for (var i = 0; i < row.length; i++)
                    Semantics(
                      container: true,
                      label: '${data.columns[i]}: ${row[i].text}',
                      child: ExcludeSemantics(
                        child: Padding(
                          padding: pad,
                          child: Text(row[i].text, style: cellStyle),
                        ),
                      ),
                    ),
                ],
              ),
          ],
        ),
      ),
    );
  }
}

/// Several tables of one view, one below the other.
class VizTables extends StatelessWidget {
  /// Creates the list of [tables].
  const VizTables({super.key, required this.tables});

  /// The tables in the order of the view.
  final List<VizTableData> tables;

  @override
  Widget build(BuildContext context) => Column(
    crossAxisAlignment: CrossAxisAlignment.start,
    mainAxisSize: MainAxisSize.min,
    children: [
      for (final table in tables) ...[
        Padding(
          padding: const EdgeInsets.only(bottom: DsSpace.s4, top: DsSpace.s8),
          child: ExcludeSemantics(
            child: Text(
              table.caption,
              style: dsTextStyle(DsType.heading)
                  .copyWith(color: context.dsColors.textPrimary),
            ),
          ),
        ),
        VizTable(data: table),
      ],
    ],
  );
}
