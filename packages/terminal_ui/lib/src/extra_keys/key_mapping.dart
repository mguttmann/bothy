// SPDX-License-Identifier: Apache-2.0

/// The bytes of the extra keys bar (docs/spec/01-product.md screen M8, R-PR-11; brief M5-12
/// Design note 2; docs/decisions.d/M5-12.md D-M5-12-1): what each key of the bar sends, and
/// what Ctrl does to a typed character.
library;

/// The keys of the bar besides the two modifiers, in the order of the bar.
enum ExtraKey {
  /// Esc: `ESC`.
  escape,

  /// Tab: `HT`.
  tab,

  /// Arrow up.
  up,

  /// Arrow down.
  down,

  /// Arrow left.
  left,

  /// Arrow right.
  right,

  /// `|`.
  pipe,

  /// `~`.
  tilde,

  /// `/`.
  slash,

  /// `-`.
  minus,
}

/// The sequence [key] sends.
///
/// The arrows send `ESC [ A` to `D`, or `ESC O A` to `D` when the program switched the
/// renderer to application cursor mode ([applicationCursor], DECCKM); the characters send
/// themselves; [alt] puts `ESC` in front of the sequence. Ctrl does not change a key of the bar
/// (only a letter or one of `@[\]^_` has a control byte, [controlByte]).
///
/// Example: `extraKeySequence(ExtraKey.up, applicationCursor: true)` is `'\x1bOA'`.
String extraKeySequence(
  ExtraKey key, {
  required bool applicationCursor,
  bool alt = false,
}) {
  final cursor = applicationCursor ? '\x1bO' : '\x1b[';
  final sequence = switch (key) {
    ExtraKey.escape => '\x1b',
    ExtraKey.tab => '\t',
    ExtraKey.up => '${cursor}A',
    ExtraKey.down => '${cursor}B',
    ExtraKey.right => '${cursor}C',
    ExtraKey.left => '${cursor}D',
    ExtraKey.pipe => '|',
    ExtraKey.tilde => '~',
    ExtraKey.slash => '/',
    ExtraKey.minus => '-',
  };
  return alt ? '\x1b$sequence' : sequence;
}

/// The control byte of the single character [c] with Ctrl: a letter (either case) or one of
/// `@[\]^_` becomes its C0 control (`c` gives `0x03`, `@` gives `0x00`); every other
/// character comes back unmodified, so Ctrl never turns a key into something the person did
/// not mean.
///
/// Example: `controlByte('c')` is `'\x03'`, `controlByte('1')` is `'1'`.
String controlByte(String c) {
  if (c.length != 1) return c;
  final code = c.codeUnitAt(0);
  final upper = code >= 0x61 && code <= 0x7a ? code - 0x20 : code;
  if (upper >= 0x40 && upper <= 0x5f) return String.fromCharCode(upper & 0x1f);
  return c;
}
