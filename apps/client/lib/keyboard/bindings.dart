// SPDX-License-Identifier: Apache-2.0

/// The first shortcut table of docs/spec/01-product.md 7 as data (brief M4-24 Design note 1):
/// one [ShortcutRow] per row of the table, each with its [ShortcutBinding]s and their key
/// strokes on macOS and on Windows.
///
/// `test/keyboard/bindings_spec_test.dart` renders every row with [specKeyNames] and compares
/// it with the Markdown table of 01 7, so a change of the specification without the code (or
/// the other way round) fails. The shortcut list (Cmd+/) renders the same rows in the language
/// of the app.
library;

import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';
import 'package:flutter/widgets.dart';

import 'intents.dart';

/// The keyboard family a binding is written for: macOS (Cmd and Option) or Windows (Ctrl and
/// Alt).
enum ShortcutPlatform {
  /// macOS.
  mac,

  /// Windows; every other desktop platform uses the same keys.
  windows;

  /// The family of the running platform: macOS (and iOS with a keyboard) use [mac], every
  /// other platform [windows].
  static ShortcutPlatform get current => switch (defaultTargetPlatform) {
    TargetPlatform.macOS || TargetPlatform.iOS => ShortcutPlatform.mac,
    _ => ShortcutPlatform.windows,
  };
}

/// Who handles a binding.
enum ShortcutKind {
  /// The app layer (`AppShortcuts`) maps the stroke to its [AppIntent].
  app,

  /// The history column handles the key itself while the focus is in it: Esc marks read,
  /// "Esc then Tab" focuses the input (01 7). Outside the history the key stays free.
  history,

  /// The focused input handles the key by the composer rules (01 7, `composer.enter_sends`);
  /// the app layer binds nothing, so the text field keeps the key.
  composer,

  /// A pointer gesture with a modifier on a message (Option+Click, Alt+Click).
  pointer,
}

/// One key press with its modifiers, or a click when [key] is null.
///
/// Example: `KeyStroke(LogicalKeyboardKey.keyK, meta: true)` is Cmd+K.
@immutable
final class KeyStroke {
  /// Creates the stroke.
  const KeyStroke(
    this.key, {
    this.meta = false,
    this.control = false,
    this.alt = false,
    this.shift = false,
  });

  /// A click of the primary pointer button with the given modifiers.
  const KeyStroke.click({this.alt = false})
    : key = null,
      meta = false,
      control = false,
      shift = false;

  /// The key, or null for a click.
  final LogicalKeyboardKey? key;

  /// Cmd on macOS (the Windows key on Windows, which no binding uses).
  final bool meta;

  /// Ctrl.
  final bool control;

  /// Option on macOS, Alt on Windows.
  final bool alt;

  /// Shift.
  final bool shift;

  /// Whether the stroke is a click.
  bool get isClick => key == null;

  /// The activator of the stroke, or null for a click.
  SingleActivator? get activator => key == null
      ? null
      : SingleActivator(
          key!,
          meta: meta,
          control: control,
          alt: alt,
          shift: shift,
        );

  @override
  bool operator ==(Object other) =>
      other is KeyStroke &&
      other.key == key &&
      other.meta == meta &&
      other.control == control &&
      other.alt == alt &&
      other.shift == shift;

  @override
  int get hashCode => Object.hash(key, meta, control, alt, shift);

  @override
  String toString() => describeStrokes([this], specKeyNames);
}

/// One way to trigger [action] on both platforms: each entry of [mac] and [windows] is one
/// alternative, a sequence of strokes ("Esc then Tab" has two).
@immutable
final class ShortcutBinding {
  /// Creates the binding.
  const ShortcutBinding(
    this.action, {
    required this.kind,
    required this.mac,
    required this.windows,
  });

  /// The action.
  final ShortcutAction action;

  /// Who handles it.
  final ShortcutKind kind;

  /// The alternatives on macOS.
  final List<List<KeyStroke>> mac;

  /// The alternatives on Windows.
  final List<List<KeyStroke>> windows;

  /// The alternatives on [platform].
  List<List<KeyStroke>> on(ShortcutPlatform platform) =>
      platform == ShortcutPlatform.mac ? mac : windows;
}

/// A remark of a row that the table writes after the keys.
enum ShortcutQualifier {
  /// "in the empty field" (arrow up edits the last own message).
  inEmptyField,

  /// "(reversible)" (Enter and Shift+Enter swap with `composer.enter_sends`).
  reversible,
}

/// One row of the table of 01 7: its English label as the specification writes it, its
/// bindings in the order of the cell, how the alternatives are joined and the remark after
/// them.
@immutable
final class ShortcutRow {
  /// Creates the row.
  const ShortcutRow(
    this.specLabel,
    this.bindings, {
    this.orJoined = false,
    this.qualifier,
  });

  /// The text of the column "Action" in 01 7.
  final String specLabel;

  /// The bindings in the order of the cell.
  final List<ShortcutBinding> bindings;

  /// Whether the alternatives are joined with ", or " (true) or " / " (false).
  final bool orJoined;

  /// The remark after the keys, or null.
  final ShortcutQualifier? qualifier;

  /// The actions of the row in order, without repetition.
  List<ShortcutAction> get actions => [
    for (final b in bindings)
      if (!bindings
          .takeWhile((x) => !identical(x, b))
          .any((x) => x.action == b.action))
        b.action,
  ];
}

const LogicalKeyboardKey _up = LogicalKeyboardKey.arrowUp;
const LogicalKeyboardKey _down = LogicalKeyboardKey.arrowDown;

/// An app binding with the platform modifier: Cmd on macOS, Ctrl on Windows.
ShortcutBinding _primary(
  ShortcutAction action,
  LogicalKeyboardKey key, {
  bool shift = false,
}) => ShortcutBinding(
  action,
  kind: ShortcutKind.app,
  mac: [
    [KeyStroke(key, meta: true, shift: shift)],
  ],
  windows: [
    [KeyStroke(key, control: true, shift: shift)],
  ],
);

/// An app binding with Option on macOS and Alt on Windows.
ShortcutBinding _option(
  ShortcutAction action,
  LogicalKeyboardKey key, {
  bool shift = false,
}) => ShortcutBinding(
  action,
  kind: ShortcutKind.app,
  mac: [
    [KeyStroke(key, alt: true, shift: shift)],
  ],
  windows: [
    [KeyStroke(key, alt: true, shift: shift)],
  ],
);

/// A binding with the same strokes on both platforms.
ShortcutBinding _same(
  ShortcutAction action,
  ShortcutKind kind,
  List<KeyStroke> strokes,
) => ShortcutBinding(action, kind: kind, mac: [strokes], windows: [strokes]);

/// The rows of the first table of 01 7, in the order of the table.
final List<ShortcutRow> shortcutRows = [
  ShortcutRow('Quick switcher', [
    _primary(ShortcutAction.quickSwitcher, LogicalKeyboardKey.keyK),
  ]),
  ShortcutRow('Search', [
    _primary(ShortcutAction.search, LogicalKeyboardKey.keyF, shift: true),
  ]),
  ShortcutRow('Search in thread', [
    _primary(ShortcutAction.searchInThread, LogicalKeyboardKey.keyF),
  ]),
  ShortcutRow('New post or new chat in the current forum', [
    _primary(ShortcutAction.newPost, LogicalKeyboardKey.keyN),
  ]),
  ShortcutRow('Focus the input', [
    _same(ShortcutAction.focusInput, ShortcutKind.history, const [
      KeyStroke(LogicalKeyboardKey.escape),
      KeyStroke(LogicalKeyboardKey.tab),
    ]),
    _primary(ShortcutAction.focusInput, LogicalKeyboardKey.keyL),
  ], orJoined: true),
  ShortcutRow('Next or previous channel or thread row in the sidebar', [
    _option(ShortcutAction.nextChannel, _down),
    _option(ShortcutAction.previousChannel, _up),
  ]),
  ShortcutRow('Next or previous unread channel or thread row', [
    _option(ShortcutAction.nextUnreadChannel, _down, shift: true),
    _option(ShortcutAction.previousUnreadChannel, _up, shift: true),
  ]),
  ShortcutRow('Pin or unpin the forum overview as a column', [
    _primary(ShortcutAction.pinOverview, LogicalKeyboardKey.keyO, shift: true),
  ]),
  ShortcutRow('Switch column (sidebar, pinned overview, history, session)', [
    _same(ShortcutAction.nextColumn, ShortcutKind.app, const [
      KeyStroke(LogicalKeyboardKey.f6),
    ]),
    _same(ShortcutAction.previousColumn, ShortcutKind.app, const [
      KeyStroke(LogicalKeyboardKey.f6, shift: true),
    ]),
  ]),
  ShortcutRow('Mark thread as read', [
    _same(ShortcutAction.markRead, ShortcutKind.history, const [
      KeyStroke(LogicalKeyboardKey.escape),
    ]),
  ]),
  ShortcutRow('Mark as unread (from a message)', [
    _same(ShortcutAction.markUnread, ShortcutKind.pointer, const [
      KeyStroke.click(alt: true),
    ]),
  ]),
  ShortcutRow('Mark thread as done or reopen it', [
    _primary(ShortcutAction.toggleDone, LogicalKeyboardKey.keyE, shift: true),
  ]),
  ShortcutRow('Stop the run in the current thread', [
    _primary(ShortcutAction.stopRun, LogicalKeyboardKey.period),
  ]),
  ShortcutRow('Expand and collapse the live box', [
    _primary(
      ShortcutAction.toggleLiveBox,
      LogicalKeyboardKey.keyL,
      shift: true,
    ),
  ]),
  ShortcutRow('Show and hide the session column', [
    _primary(
      ShortcutAction.toggleSessionColumn,
      LogicalKeyboardKey.keyI,
      shift: true,
    ),
  ]),
  ShortcutRow('Edit the last own message', [
    _same(ShortcutAction.editLast, ShortcutKind.composer, const [
      KeyStroke(_up),
    ]),
  ], qualifier: ShortcutQualifier.inEmptyField),
  ShortcutRow('Send / new line', [
    _same(ShortcutAction.send, ShortcutKind.composer, const [
      KeyStroke(LogicalKeyboardKey.enter),
    ]),
    _same(ShortcutAction.newLine, ShortcutKind.composer, const [
      KeyStroke(LogicalKeyboardKey.enter, shift: true),
    ]),
  ], qualifier: ShortcutQualifier.reversible),
  ShortcutRow('Running now', [
    _primary(ShortcutAction.runningNow, LogicalKeyboardKey.keyR, shift: true),
  ]),
  ShortcutRow('Mentions', [
    _primary(ShortcutAction.mentions, LogicalKeyboardKey.keyM, shift: true),
  ]),
  ShortcutRow('Settings', [
    _primary(ShortcutAction.settings, LogicalKeyboardKey.comma),
  ]),
  ShortcutRow('List of shortcuts', [
    _primary(ShortcutAction.shortcutList, LogicalKeyboardKey.slash),
  ]),
  ShortcutRow('Open hosts and terminal', [
    _primary(
      ShortcutAction.hostsTerminal,
      LogicalKeyboardKey.keyT,
      shift: true,
    ),
  ]),
];

/// Every binding of [shortcutRows].
Iterable<ShortcutBinding> get allShortcutBindings =>
    shortcutRows.expand((r) => r.bindings);

/// The activators of the app layer on [platform] with their intents: every single-stroke
/// alternative of a binding of kind [ShortcutKind.app].
///
/// Example: `Shortcuts(shortcuts: appActivators(ShortcutPlatform.current), child: ...)`.
Map<ShortcutActivator, Intent> appActivators(ShortcutPlatform platform) => {
  for (final b in allShortcutBindings)
    if (b.kind == ShortcutKind.app)
      for (final strokes in b.on(platform))
        if (strokes.length == 1 && strokes.single.activator != null)
          strokes.single.activator!: intentOf(b.action),
};

/// The words a description of keys uses: the English names of 01 7 ([specKeyNames]) or the
/// language of the app.
abstract interface class KeyNames {
  /// Cmd.
  String get cmd;

  /// Ctrl.
  String get ctrl;

  /// Option (macOS).
  String get option;

  /// Alt (Windows).
  String get alt;

  /// Shift.
  String get shift;

  /// The word before the direction of an arrow key ("Arrow").
  String get arrow;

  /// The direction "up".
  String get up;

  /// The direction "down".
  String get down;

  /// Esc.
  String get escape;

  /// Tab.
  String get tab;

  /// Enter.
  String get enter;

  /// A click.
  String get click;

  /// The word between the strokes of a sequence ("then").
  String get then;

  /// The word that joins alternatives in the cell "Focus the input" ("or").
  String get or;

  /// The remark [qualifier].
  String qualifier(ShortcutQualifier qualifier);
}

/// The English names of the table of 01 7.
const KeyNames specKeyNames = _SpecKeyNames();

final class _SpecKeyNames implements KeyNames {
  const _SpecKeyNames();

  @override
  String get cmd => 'Cmd';
  @override
  String get ctrl => 'Ctrl';
  @override
  String get option => 'Option';
  @override
  String get alt => 'Alt';
  @override
  String get shift => 'Shift';
  @override
  String get arrow => 'Arrow';
  @override
  String get up => 'up';
  @override
  String get down => 'down';
  @override
  String get escape => 'Esc';
  @override
  String get tab => 'Tab';
  @override
  String get enter => 'Enter';
  @override
  String get click => 'Click';
  @override
  String get then => 'then';
  @override
  String get or => 'or';
  @override
  String qualifier(ShortcutQualifier qualifier) => switch (qualifier) {
    ShortcutQualifier.inEmptyField => 'in the empty field',
    ShortcutQualifier.reversible => '(reversible)',
  };
}

/// The modifiers of [stroke] with the words of [names] for [platform], in the order of the
/// table (platform modifier, Option or Alt, Shift).
List<String> _modifiers(
  KeyStroke stroke,
  KeyNames names,
  ShortcutPlatform platform,
) => [
  if (stroke.meta) names.cmd,
  if (stroke.control) names.ctrl,
  if (stroke.alt) platform == ShortcutPlatform.mac ? names.option : names.alt,
  if (stroke.shift) names.shift,
];

String? _direction(LogicalKeyboardKey? key, KeyNames names) => switch (key) {
  LogicalKeyboardKey.arrowUp => names.up,
  LogicalKeyboardKey.arrowDown => names.down,
  _ => null,
};

String _keyName(KeyStroke stroke, KeyNames names) {
  final key = stroke.key;
  if (key == null) return names.click;
  final direction = _direction(key, names);
  if (direction != null) return '${names.arrow} $direction';
  return switch (key) {
    LogicalKeyboardKey.escape => names.escape,
    LogicalKeyboardKey.tab => names.tab,
    LogicalKeyboardKey.enter => names.enter,
    _ => key.keyLabel.toUpperCase(),
  };
}

String _stroke(KeyStroke stroke, KeyNames names, ShortcutPlatform platform) =>
    [..._modifiers(stroke, names, platform), _keyName(stroke, names)].join('+');

/// [strokes] as text with [names], for example "Esc then Tab" or "Cmd+Shift+F"; [platform]
/// decides between Option and Alt.
String describeStrokes(
  List<KeyStroke> strokes,
  KeyNames names, {
  ShortcutPlatform platform = ShortcutPlatform.mac,
}) => strokes.map((s) => _stroke(s, names, platform)).join(' ${names.then} ');

/// The keys of [row] on [platform] as one cell, written as 01 7 writes them: alternatives
/// joined with " / " (or ", or "), an arrow that follows an arrow with the same modifiers
/// written as its direction only ("Option+Arrow down / up"), and the row's remark after the
/// keys.
///
/// Example: `describeRow(row, ShortcutPlatform.windows, specKeyNames)` gives
/// "Alt+Arrow down / up" for the row "Next or previous channel".
String describeRow(ShortcutRow row, ShortcutPlatform platform, KeyNames names) {
  final parts = <String>[];
  List<KeyStroke>? previous;
  for (final binding in row.bindings) {
    for (final strokes in binding.on(platform)) {
      final last = previous;
      final short =
          last != null &&
          last.length == 1 &&
          strokes.length == 1 &&
          _direction(last.single.key, names) != null &&
          _direction(strokes.single.key, names) != null &&
          last.single.meta == strokes.single.meta &&
          last.single.control == strokes.single.control &&
          last.single.alt == strokes.single.alt &&
          last.single.shift == strokes.single.shift;
      parts.add(
        short
            ? _direction(strokes.single.key, names)!
            : describeStrokes(strokes, names, platform: platform),
      );
      previous = strokes;
    }
  }
  final joined = parts.join(row.orJoined ? ', ${names.or} ' : ' / ');
  final qualifier = row.qualifier;
  return qualifier == null ? joined : '$joined ${names.qualifier(qualifier)}';
}
