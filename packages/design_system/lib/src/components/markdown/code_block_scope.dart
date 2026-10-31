// SPDX-License-Identifier: Apache-2.0

/// How the code blocks of `DsMarkdown` show on this device (01 6.14, R-PR-28; brief M4-49).
library;

import 'package:flutter/widgets.dart';

/// Hands the code blocks below it the device's choice whether long lines wrap, the callback
/// that stores a new choice, and the height from which a long block shows collapsed.
///
/// The design system stores nothing: the app provides the scope at its root, reads [wrap] from
/// the local view state of the device and stores what [onWrapChanged] reports, so every block
/// on the device follows one choice and the choice survives a restart (D-M4-49-2). Without a
/// scope a block wraps, a toggle switches only that block, and the limit is
/// [defaultCollapseLines].
///
/// ```dart
/// DsCodeBlockScope(
///   wrap: !unwrapped,
///   collapseLines: config.codeBlockCollapseLines,
///   onWrapChanged: (wrap) => dao.setCodeBlocksUnwrapped(unwrapped: !wrap),
///   child: app,
/// );
/// ```
class DsCodeBlockScope extends InheritedWidget {
  /// Creates the scope.
  const DsCodeBlockScope({
    super.key,
    required this.wrap,
    this.onWrapChanged,
    this.collapseLines = defaultCollapseLines,
    required super.child,
  }) : assert(collapseLines > 0, 'collapseLines must be positive');

  /// The limit of a block without a scope: the default of `ui.code_block_collapse_lines`.
  static const int defaultCollapseLines = 20;

  /// Whether long lines of a code block wrap (true, the default of 01 6.14 item 1) or every
  /// line stays one line with a horizontal scroll (false).
  final bool wrap;

  /// Called with the new choice when the person switches the toggle "Wrap lines" of any
  /// block; null when nothing stores it, then the toggle switches only its own block.
  final ValueChanged<bool>? onWrapChanged;

  /// A block whose content is higher than this many lines of `type.code` shows collapsed to
  /// that height with a fade and "Show all" (01 6.14 item 2); at least 1.
  final int collapseLines;

  /// The nearest scope above [context], or null.
  static DsCodeBlockScope? maybeOf(BuildContext context) =>
      context.dependOnInheritedWidgetOfExactType<DsCodeBlockScope>();

  @override
  bool updateShouldNotify(DsCodeBlockScope oldWidget) =>
      wrap != oldWidget.wrap ||
      collapseLines != oldWidget.collapseLines ||
      onWrapChanged != oldWidget.onWrapChanged;
}
