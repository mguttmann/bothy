// SPDX-License-Identifier: Apache-2.0

/// The plain text of a system selection over a message (01 6.15, R-PR-29; brief M4-50): the
/// selection containers that give the parts of a rendered message their separators.
library;

import 'package:flutter/rendering.dart'
    show MatrixUtils, Selectable, SelectedContent;
import 'package:flutter/widgets.dart';

/// Joins the selected text of the selectables below it with [separator].
///
/// Flutter's selection concatenates the text of neighboring paragraphs without anything in
/// between, so a selection over two paragraphs would copy "firstsecond". The renderer wraps
/// each of its containers in one of these, so a copy keeps the structure the person sees:
/// blocks apart by an empty line, list items and code lines by a line break, the indentation
/// and the rest of one code line by nothing. A part the selection only touches at its edge adds
/// no separator, so a partial selection never starts or ends with one.
///
/// Only meaningful below a [SelectionArea]; the renderer adds it only there.
///
/// ```dart
/// DsSelectionJoin(separator: '\n\n', child: Column(children: blocks));
/// ```
class DsSelectionJoin extends StatefulWidget {
  /// Creates the join of [child] with [separator].
  const DsSelectionJoin({
    super.key,
    required this.separator,
    required this.child,
  }) : _table = false,
       replacement = null;

  /// Creates the join of the cells of a table: a tab between the cells of one row, a line
  /// break between rows; a row is the cells whose tops line up.
  const DsSelectionJoin.table({super.key, required this.child})
    : separator = '\t',
      _table = true,
      replacement = null;

  /// Creates a part whose copy is [replacement] whenever any of it is selected: a list marker
  /// draws "•" but copies "- ", like the copy of a quote (01 6.12 item 4).
  const DsSelectionJoin.replace({
    super.key,
    required String this.replacement,
    required this.child,
  }) : separator = '',
       _table = false;

  /// The text between two selected parts.
  final String separator;

  /// The content.
  final Widget child;

  /// What a selection of this part copies, or null to copy the selected text.
  final String? replacement;

  final bool _table;

  @override
  State<DsSelectionJoin> createState() => _DsSelectionJoinState();
}

class _DsSelectionJoinState extends State<DsSelectionJoin> {
  late final _JoinDelegate _delegate = _make();

  _JoinDelegate _make() => _JoinDelegate(
    widget.separator,
    table: widget._table,
    replacement: widget.replacement,
  );

  @override
  void didUpdateWidget(DsSelectionJoin oldWidget) {
    super.didUpdateWidget(oldWidget);
    _delegate
      ..separator = widget.separator
      ..table = widget._table
      ..replacement = widget.replacement;
  }

  @override
  void dispose() {
    _delegate.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) =>
      SelectionContainer(delegate: _delegate, child: widget.child);
}

class _JoinDelegate extends StaticSelectionContainerDelegate {
  _JoinDelegate(this.separator, {required this.table, this.replacement});

  String separator;
  bool table;
  String? replacement;

  @override
  SelectedContent? getSelectedContent() {
    final parts = <(Selectable, String)>[
      for (final selectable in selectables)
        if (selectable.getSelectedContent() case final SelectedContent data)
          (selectable, data.plainText),
    ];
    // An empty part at an edge is a selection that only touches a paragraph; an empty part
    // between two others is an empty line of code and keeps its line break.
    while (parts.isNotEmpty && parts.first.$2.isEmpty) {
      parts.removeAt(0);
    }
    while (parts.isNotEmpty && parts.last.$2.isEmpty) {
      parts.removeLast();
    }
    if (parts.isEmpty) return null;
    if (replacement case final text?) return SelectedContent(plainText: text);
    if (!table) {
      return SelectedContent(plainText: parts.map((p) => p.$2).join(separator));
    }
    final out = StringBuffer(parts.first.$2);
    var top = _top(parts.first.$1);
    for (final (selectable, text) in parts.skip(1)) {
      final next = _top(selectable);
      // Cells of one row start at the same height; a lower one starts the next row.
      out
        ..write(next - top > _rowTolerance ? '\n' : separator)
        ..write(text);
      top = next;
    }
    return SelectedContent(plainText: out.toString());
  }

  static const double _rowTolerance = 1;

  double _top(Selectable selectable) => MatrixUtils.transformPoint(
    selectable.getTransformTo(null),
    Offset.zero,
  ).dy;
}
