// SPDX-License-Identifier: Apache-2.0

/// The second shortcut table of docs/spec/01-product.md 7, "In the terminal", as data (brief
/// M4-36 Design note 4): one [TerminalShortcutRow] per row with its strokes on macOS (Cmd) and
/// on Windows (Ctrl+Shift), the activators the terminal view hands to the renderer, and the
/// pass-through region of M4-24 around the view.
///
/// Every stroke carries the platform modifier, so Ctrl+C, Ctrl+D, Ctrl+K, Ctrl+L, Alt and Esc
/// reach the shell (R-PR-10). `test/terminal/terminal_shortcuts_spec_test.dart` compares the
/// rows with the table of 01 7.
library;

import 'package:flutter/services.dart';
import 'package:flutter/widgets.dart';

import '../keyboard/app_shortcuts.dart' show KeyboardScope;
import '../keyboard/bindings.dart';

/// One action of the table "In the terminal".
enum TerminalShortcutAction {
  /// Copies the selection; the renderer of `terminal_ui` binds it.
  copy,

  /// Pastes through the paste guard; the renderer of `terminal_ui` binds it.
  paste,

  /// Opens the quick switcher.
  quickSwitcher,

  /// Opens a new session on the host of the view.
  newSession,

  /// Selects the next session tab.
  nextSession,

  /// Selects the previous session tab.
  previousSession,

  /// Detaches the selected session; it keeps running.
  detach,

  /// Searches in the scrollback of the selected session.
  search,

  /// Makes the font of the terminal larger.
  fontLarger,

  /// Makes the font of the terminal smaller.
  fontSmaller,
}

/// The intent of one [TerminalShortcutAction] inside the terminal.
final class TerminalShortcutIntent extends Intent {
  /// Creates the intent.
  const TerminalShortcutIntent(this.action);

  /// The action.
  final TerminalShortcutAction action;
}

/// One way to trigger an action: its strokes on macOS and on Windows; [shown] false marks a
/// further key of the same meaning that the table does not name (the plus key of another
/// keyboard layout).
@immutable
final class TerminalShortcutBinding {
  /// Creates the binding.
  const TerminalShortcutBinding(
    this.action, {
    required this.mac,
    required this.windows,
    this.shown = true,
  });

  /// The action.
  final TerminalShortcutAction action;

  /// The stroke on macOS.
  final KeyStroke mac;

  /// The stroke on Windows.
  final KeyStroke windows;

  /// Whether the table names this stroke.
  final bool shown;

  /// The stroke on [platform].
  KeyStroke on(ShortcutPlatform platform) =>
      platform == ShortcutPlatform.mac ? mac : windows;
}

/// One row of the table "In the terminal" with its English label as 01 7 writes it.
@immutable
final class TerminalShortcutRow {
  /// Creates the row.
  const TerminalShortcutRow(this.specLabel, this.bindings);

  /// The text of the column "Action in the terminal".
  final String specLabel;

  /// The bindings in the order of the cell.
  final List<TerminalShortcutBinding> bindings;

  /// The first action of the row (its label in the shortcut list).
  TerminalShortcutAction get action => bindings.first.action;
}

TerminalShortcutBinding _key(
  TerminalShortcutAction action,
  LogicalKeyboardKey key, {
  LogicalKeyboardKey? windowsKey,
  bool macShift = false,
  bool shown = true,
}) => TerminalShortcutBinding(
  action,
  mac: KeyStroke(key, meta: true, shift: macShift),
  windows: KeyStroke(windowsKey ?? key, control: true, shift: true),
  shown: shown,
);

/// The rows of the table "In the terminal" of 01 7, in its order.
final List<TerminalShortcutRow> terminalShortcutRows = [
  TerminalShortcutRow('Copy / paste', [
    _key(TerminalShortcutAction.copy, LogicalKeyboardKey.keyC),
    _key(TerminalShortcutAction.paste, LogicalKeyboardKey.keyV),
  ]),
  TerminalShortcutRow('Quick switcher', [
    _key(TerminalShortcutAction.quickSwitcher, LogicalKeyboardKey.keyK),
  ]),
  TerminalShortcutRow('New session on this host', [
    _key(
      TerminalShortcutAction.newSession,
      LogicalKeyboardKey.keyT,
      windowsKey: LogicalKeyboardKey.keyN,
    ),
  ]),
  TerminalShortcutRow('Next or previous session', [
    _key(
      TerminalShortcutAction.nextSession,
      LogicalKeyboardKey.bracketRight,
      windowsKey: LogicalKeyboardKey.arrowRight,
      macShift: true,
    ),
    _key(
      TerminalShortcutAction.previousSession,
      LogicalKeyboardKey.bracketLeft,
      windowsKey: LogicalKeyboardKey.arrowLeft,
      macShift: true,
    ),
  ]),
  TerminalShortcutRow('Detach session (keeps running)', [
    _key(
      TerminalShortcutAction.detach,
      LogicalKeyboardKey.keyD,
      macShift: true,
    ),
  ]),
  TerminalShortcutRow('Search in the scrollback', [
    _key(TerminalShortcutAction.search, LogicalKeyboardKey.keyF),
  ]),
  TerminalShortcutRow('Font larger / smaller', [
    _key(TerminalShortcutAction.fontLarger, LogicalKeyboardKey.add),
    _key(TerminalShortcutAction.fontSmaller, LogicalKeyboardKey.minus),
    // The plus sits on the key of "=" on many layouts, with or without Shift, and on the
    // number pad; they mean the same (01 7 names the plus key).
    _key(
      TerminalShortcutAction.fontLarger,
      LogicalKeyboardKey.equal,
      shown: false,
    ),
    _key(
      TerminalShortcutAction.fontLarger,
      LogicalKeyboardKey.equal,
      macShift: true,
      shown: false,
    ),
    _key(
      TerminalShortcutAction.fontLarger,
      LogicalKeyboardKey.numpadAdd,
      shown: false,
    ),
    _key(
      TerminalShortcutAction.fontSmaller,
      LogicalKeyboardKey.numpadSubtract,
      shown: false,
    ),
  ]),
];

/// The actions the renderer of `terminal_ui` binds itself (copy, and paste through the guard).
const Set<TerminalShortcutAction> rendererActions = {
  TerminalShortcutAction.copy,
  TerminalShortcutAction.paste,
};

/// The activators the terminal view hands to the renderer on [platform]: every binding of
/// [terminalShortcutRows] except copy and paste, which `terminal_ui` binds with the guard.
///
/// Example: `TerminalPane(shortcuts: terminalActivators(ShortcutPlatform.current), ...)`.
Map<ShortcutActivator, Intent> terminalActivators(ShortcutPlatform platform) =>
    {
      for (final row in terminalShortcutRows)
        for (final b in row.bindings)
          if (!rendererActions.contains(b.action))
            b.on(platform).activator!: TerminalShortcutIntent(b.action),
    };

/// The words of a key: brackets as themselves, the plus and minus keys and the left and right
/// arrows by name, letters in capitals.
String _keyName(LogicalKeyboardKey key, TerminalKeyNames names) =>
    switch (key) {
      LogicalKeyboardKey.add ||
      LogicalKeyboardKey.equal ||
      LogicalKeyboardKey.numpadAdd => names.plus,
      LogicalKeyboardKey.minus ||
      LogicalKeyboardKey.numpadSubtract => names.minus,
      LogicalKeyboardKey.arrowLeft => '${names.base.arrow} ${names.left}',
      LogicalKeyboardKey.arrowRight => '${names.base.arrow} ${names.right}',
      _ => key.keyLabel.toUpperCase(),
    };

/// The word of the second key of a pair that 01 7 writes short ("] / [", "right / left").
String? _pairShort(LogicalKeyboardKey key, TerminalKeyNames names) =>
    switch (key) {
      LogicalKeyboardKey.bracketLeft ||
      LogicalKeyboardKey.bracketRight => key.keyLabel,
      LogicalKeyboardKey.arrowLeft => names.left,
      LogicalKeyboardKey.arrowRight => names.right,
      _ => null,
    };

/// The key names of the terminal table: those of the first table plus plus, minus, left and
/// right.
final class TerminalKeyNames {
  /// Creates the names over [base].
  const TerminalKeyNames(
    this.base, {
    required this.plus,
    required this.minus,
    required this.left,
    required this.right,
  });

  /// The names of the first table.
  final KeyNames base;

  /// The plus key.
  final String plus;

  /// The minus key.
  final String minus;

  /// The direction left of an arrow.
  final String left;

  /// The direction right of an arrow.
  final String right;
}

/// The English names of 01 7.
const TerminalKeyNames specTerminalKeyNames = TerminalKeyNames(
  specKeyNames,
  plus: 'Plus',
  minus: 'Minus',
  left: 'left',
  right: 'right',
);

String _stroke(KeyStroke s, TerminalKeyNames names) => [
  if (s.meta) names.base.cmd,
  if (s.control) names.base.ctrl,
  if (s.alt) names.base.alt,
  if (s.shift) names.base.shift,
  _keyName(s.key!, names),
].join('+');

/// The keys of [row] on [platform] as one cell, written as 01 7 writes them: the shown
/// alternatives joined with " / ", the second key of a pair of brackets or arrows with the same
/// modifiers written short ("Cmd+Shift+] / [").
///
/// Example: `describeTerminalRow(row, ShortcutPlatform.windows, specTerminalKeyNames)` gives
/// "Ctrl+Shift+C / Ctrl+Shift+V" for the row "Copy / paste".
String describeTerminalRow(
  TerminalShortcutRow row,
  ShortcutPlatform platform,
  TerminalKeyNames names,
) {
  final parts = <String>[];
  KeyStroke? previous;
  for (final b in row.bindings.where((b) => b.shown)) {
    final s = b.on(platform);
    final last = previous;
    final short =
        last != null &&
        _pairShort(last.key!, names) != null &&
        _pairShort(s.key!, names) != null &&
        last.meta == s.meta &&
        last.control == s.control &&
        last.shift == s.shift;
    parts.add(short ? _pairShort(s.key!, names)! : _stroke(s, names));
    previous = s;
  }
  return parts.join(' / ');
}

/// Runs a [TerminalShortcutIntent] through [handlers]; an action without an enabled handler is
/// not consumed, so its key goes to the shell.
final class TerminalShortcutHandler extends Action<TerminalShortcutIntent> {
  /// Creates the action over [handlers].
  TerminalShortcutHandler(this.handlers);

  /// The handlers by action; null or missing disables the action.
  final Map<TerminalShortcutAction, VoidCallback?> handlers;

  @override
  bool isEnabled(TerminalShortcutIntent intent) =>
      handlers[intent.action] != null;

  @override
  bool consumesKey(TerminalShortcutIntent intent) => isEnabled(intent);

  @override
  Object? invoke(TerminalShortcutIntent intent) {
    handlers[intent.action]?.call();
    return null;
  }
}

/// The terminal region: the pass-through layer of M4-24 (only strokes with the platform
/// modifier reach the app) and the handlers of the table "In the terminal".
///
/// Example:
/// ```dart
/// TerminalShortcutScope(handlers: {TerminalShortcutAction.detach: tab.detach}, child: pane)
/// ```
class TerminalShortcutScope extends StatelessWidget {
  /// Creates the region.
  const TerminalShortcutScope({
    super.key,
    required this.handlers,
    required this.child,
  });

  /// The handlers by action.
  final Map<TerminalShortcutAction, VoidCallback?> handlers;

  /// The terminal view.
  final Widget child;

  @override
  Widget build(BuildContext context) => KeyboardScope.passThrough(
    child: Actions(
      actions: {TerminalShortcutIntent: TerminalShortcutHandler(handlers)},
      child: child,
    ),
  );
}
