// SPDX-License-Identifier: Apache-2.0

/// One mouse selection over the contents of the rows of a scrolling list (01 6.15, R-PR-29;
/// ticket FIX-drag-select-autoscroll): the history of a chat on the desktop.
library;

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart'
    show
        MatrixUtils,
        SelectParagraphSelectionEvent,
        SelectWordSelectionEvent,
        SelectedContent,
        SelectionRegistrar,
        SelectionResult;

/// A selection over the contents of the rows of the scroll view in [child].
///
/// Put it above the scroll view, and wrap every row in a [DsSelectableRow]; a [DsMessage]
/// with `selectable` set joins the selection with its content only, everything else of a row
/// stays out of it. Because the selection sits above the scroll view, a drag held beyond the
/// top or bottom edge of the view scrolls it, and the selection grows over the rows that come
/// into view. The copy (Cmd+C or Ctrl+C) is the selected content of every row in order, one
/// row apart from the next by an empty line.
///
/// A row's selection exists only while the row is built: the list must keep the rows of a
/// selection built while it lasts (the history widens its cache extent, see `MessageList`).
///
/// The selection draws no menu of its own; [onSelectionChanged] hears the selected text,
/// empty without a selection, for the app's own menu of a right click.
///
/// ```dart
/// DsListSelection(
///   onSelectionChanged: (text) => selected.value = text,
///   child: ListView.builder(
///     itemBuilder: (_, i) => DsSelectableRow(child: DsMessage(..., selectable: true)),
///   ),
/// );
/// ```
class DsListSelection extends StatefulWidget {
  /// Creates the selection over [child].
  const DsListSelection({
    super.key,
    required this.child,
    this.onSelectionChanged,
  });

  /// The scroll view whose rows are selectable.
  final Widget child;

  /// Hears the selected text, empty when nothing is selected.
  final ValueChanged<String>? onSelectionChanged;

  @override
  State<DsListSelection> createState() => _DsListSelectionState();
}

class _DsListSelectionState extends State<DsListSelection> {
  final _ContentGroup _group = _ContentGroup();

  @override
  Widget build(BuildContext context) => SelectionArea(
    // The app's menu is the menu of a right click; the selection draws none of its own.
    contextMenuBuilder: (_, _) => const SizedBox.shrink(),
    onSelectionChanged: (content) =>
        widget.onSelectionChanged?.call(content?.plainText ?? ''),
    child: _GroupScope(group: _group, child: widget.child),
  );
}

/// A row of the list below a [DsListSelection]: nothing of [child] is selectable except the
/// content of a selectable [DsMessage] in it.
///
/// Without a [DsListSelection] above, [child] is returned as it is.
class DsSelectableRow extends StatelessWidget {
  /// Creates the row.
  const DsSelectableRow({super.key, required this.child});

  /// The row.
  final Widget child;

  @override
  Widget build(BuildContext context) {
    final group = context
        .dependOnInheritedWidgetOfExactType<_GroupScope>()
        ?.group;
    final registrar = SelectionContainer.maybeOf(context);
    if (group == null || registrar == null) return child;
    return _RowScope(
      group: group,
      registrar: registrar,
      child: SelectionContainer.disabled(child: child),
    );
  }
}

/// The content of a row that joins the selection of the list above it (used by
/// [DsMessage]); without a [DsSelectableRow] above, [child] is returned as it is.
class DsSelectableContent extends StatefulWidget {
  /// Creates the content.
  const DsSelectableContent({super.key, required this.child});

  /// The selectable content.
  final Widget child;

  /// Whether content below [context] can join a list selection.
  static bool availableIn(BuildContext context) =>
      context.dependOnInheritedWidgetOfExactType<_RowScope>() != null;

  @override
  State<DsSelectableContent> createState() => _DsSelectableContentState();
}

class _DsSelectableContentState extends State<DsSelectableContent> {
  _ContentDelegate? _delegate;

  @override
  void dispose() {
    _delegate?.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final scope = context.dependOnInheritedWidgetOfExactType<_RowScope>();
    if (scope == null) return widget.child;
    final delegate = _delegate ??= _ContentDelegate(scope.group);
    return SelectionContainer(
      registrar: scope.registrar,
      delegate: delegate,
      child: widget.child,
    );
  }
}

class _GroupScope extends InheritedWidget {
  const _GroupScope({required this.group, required super.child});

  final _ContentGroup group;

  @override
  bool updateShouldNotify(_GroupScope oldWidget) => group != oldWidget.group;
}

class _RowScope extends InheritedWidget {
  const _RowScope({
    required this.group,
    required this.registrar,
    required super.child,
  });

  final _ContentGroup group;
  final SelectionRegistrar registrar;

  @override
  bool updateShouldNotify(_RowScope oldWidget) =>
      group != oldWidget.group || registrar != oldWidget.registrar;
}

/// The contents of the rows of one list selection, for the separators between them.
class _ContentGroup {
  final Set<_ContentDelegate> members = {};
}

/// The selection container of one row's content. Its copy ends with an empty line when a
/// later row (lower on the screen) has selected content as well, so the rows of a copy stay
/// apart; Flutter would join them with nothing.
class _ContentDelegate extends StaticSelectionContainerDelegate {
  _ContentDelegate(this._group) {
    _group.members.add(this);
  }

  final _ContentGroup _group;

  static const String separator = '\n\n';

  @override
  void dispose() {
    _group.members.remove(this);
    super.dispose();
  }

  /// A word or paragraph select (a double or triple click, the right click of macOS) outside
  /// the content, on the avatar, the name or the space between rows, selects nothing; Flutter
  /// would select the nearest word of the list.
  @override
  SelectionResult handleSelectWord(SelectWordSelectionEvent event) =>
      _hits(event.globalPosition)
      ? super.handleSelectWord(event)
      : SelectionResult.end;

  @override
  SelectionResult handleSelectParagraph(SelectParagraphSelectionEvent event) =>
      _hits(event.globalPosition)
      ? super.handleSelectParagraph(event)
      : SelectionResult.end;

  bool _hits(Offset global) {
    for (final selectable in selectables) {
      final transform = selectable.getTransformTo(null);
      for (final rect in selectable.boundingBoxes) {
        if (MatrixUtils.transformRect(transform, rect).contains(global)) {
          return true;
        }
      }
    }
    return false;
  }

  /// The selected text of the content itself, or null when none of it is selected.
  String? _own() {
    final parts = [
      for (final selectable in selectables)
        if (selectable.getSelectedContent() case final SelectedContent data)
          data.plainText,
    ];
    final text = parts.join();
    return text.isEmpty ? null : text;
  }

  double? _top() {
    if (selectables.isEmpty) return null;
    return MatrixUtils.transformPoint(
      selectables.first.getTransformTo(null),
      Offset.zero,
    ).dy;
  }

  @override
  SelectedContent? getSelectedContent() {
    final own = _own();
    if (own == null) return null;
    final top = _top();
    final later =
        top != null &&
        _group.members.any(
          (other) =>
              !identical(other, this) &&
              (other._top() ?? double.negativeInfinity) > top &&
              other._own() != null,
        );
    return SelectedContent(plainText: later ? '$own$separator' : own);
  }
}
