// SPDX-License-Identifier: Apache-2.0

/// The focus order of the columns (docs/spec/01-product.md 7 and 8 "Focus"; brief M4-24
/// Design note 3): each column is a [FocusTraversalGroup] in reading order, and F6 and
/// Shift+F6 move to the first focusable item of the next or previous visible column in the
/// order sidebar, list, history, session. The input belongs to the history column, where Tab
/// reaches it after the messages (01 8).
library;

import 'package:flutter/widgets.dart';

import '../app/layout/column_widths.dart' show AppColumn;

/// The columns in the order of 01 7: sidebar, list, history, session.
const List<String> columnOrder = [
  AppColumn.sidebar,
  AppColumn.postList,
  AppColumn.history,
  AppColumn.session,
];

/// The columns that are on screen, by name, and the move between them.
///
/// A column registers its group node while it is built ([ColumnFocusGroup]); a collapsed
/// column is not built and so is skipped, and a column without a focusable item is skipped
/// too, because F6 must land on something the person can use.
///
/// Example: `registry.move(forward: true)` after F6.
final class ColumnFocusRegistry {
  final Map<String, FocusNode> _nodes = {};

  /// Registers [node] as the group of [column].
  void attach(String column, FocusNode node) => _nodes[column] = node;

  /// Removes [node] if it is still the group of [column].
  void detach(String column, FocusNode node) {
    if (identical(_nodes[column], node)) _nodes.remove(column);
  }

  /// The name of the column that holds [focus], or null.
  String? columnOf(FocusNode? focus) {
    if (focus == null) return null;
    for (final entry in _nodes.entries) {
      final node = entry.value;
      if (identical(focus, node) || focus.ancestors.contains(node)) {
        return entry.key;
      }
    }
    return null;
  }

  /// The first focusable item of [column] in reading order (top to bottom, then in the
  /// reading direction), or null when the column shows none.
  FocusNode? firstOf(String column) {
    final group = _nodes[column];
    if (group == null || group.context == null) return null;
    final rtl = Directionality.maybeOf(group.context!) == TextDirection.rtl;
    final candidates = [
      for (final n in group.traversalDescendants)
        if (n.canRequestFocus && !n.skipTraversal && n.context != null) n,
    ];
    if (candidates.isEmpty) return null;
    candidates.sort((a, b) {
      final ra = a.rect;
      final rb = b.rect;
      final byTop = ra.top.compareTo(rb.top);
      if (byTop != 0) return byTop;
      return rtl ? rb.right.compareTo(ra.right) : ra.left.compareTo(rb.left);
    });
    return candidates.first;
  }

  /// The visible columns with a focusable item, in the order of [columnOrder].
  List<String> get reachable => [
    for (final c in columnOrder)
      if (_nodes.containsKey(c) && firstOf(c) != null) c,
  ];

  /// Moves the focus to the first item of the next (or, with [forward] false, the previous)
  /// reachable column, wrapping at the ends; from outside every column it starts at the first
  /// (or the last). Returns whether the focus moved.
  bool move({required bool forward}) {
    final columns = reachable;
    if (columns.isEmpty) return false;
    final current = columnOf(FocusManager.instance.primaryFocus);
    final index = current == null ? -1 : columns.indexOf(current);
    final String target;
    if (index < 0) {
      target = forward ? columns.first : columns.last;
    } else {
      final n = columns.length;
      target = columns[(index + (forward ? 1 : -1) + n) % n];
    }
    final node = firstOf(target);
    if (node == null) return false;
    node.requestFocus();
    return true;
  }
}

/// The focus group of one column: reading order inside it, and registered with [registry]
/// under [column] for F6 while it is built.
class ColumnFocusGroup extends StatefulWidget {
  /// Creates the group.
  const ColumnFocusGroup({
    super.key,
    required this.column,
    required this.registry,
    required this.child,
  });

  /// The column name ([AppColumn]).
  final String column;

  /// The registry of the shell, or null outside a keyboard scope (the group then only orders
  /// the traversal).
  final ColumnFocusRegistry? registry;

  /// The column content.
  final Widget child;

  @override
  State<ColumnFocusGroup> createState() => _ColumnFocusGroupState();
}

class _ColumnFocusGroupState extends State<ColumnFocusGroup> {
  late final FocusNode _node = FocusNode(
    debugLabel: 'column.${widget.column}',
    canRequestFocus: false,
    skipTraversal: true,
  );

  @override
  void initState() {
    super.initState();
    widget.registry?.attach(widget.column, _node);
  }

  @override
  void didUpdateWidget(ColumnFocusGroup old) {
    super.didUpdateWidget(old);
    if (old.registry != widget.registry || old.column != widget.column) {
      old.registry?.detach(old.column, _node);
      widget.registry?.attach(widget.column, _node);
    }
  }

  @override
  void dispose() {
    widget.registry?.detach(widget.column, _node);
    _node.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => FocusTraversalGroup(
    policy: ReadingOrderTraversalPolicy(),
    child: Focus(focusNode: _node, child: widget.child),
  );
}
