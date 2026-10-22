// SPDX-License-Identifier: Apache-2.0

/// The renderer theme of the terminal view from the design tokens (docs/design/components.md
/// section 12 "Terminal view", roles `color.terminal.*`).
library;

import 'package:design_system/design_system.dart';
import 'package:terminal_ui/src/render/terminal_marks.dart';
import 'package:xterm/xterm.dart' show TerminalTheme;

/// The xterm [TerminalTheme] of the terminal screen, every field taken from a token of
/// [colors] (components.md 12), so the screen follows light and dark mode without a colour of
/// its own.
///
/// - Screen: `background` is `color.terminal.bg`, `foreground` is `color.terminal.fg`.
/// - Cursor and selection: transparent. xterm 4.0.0 paints both over the text (the block
///   covers the character, the underline and the bar ignore the cursor row), so the view
///   paints them itself, opaque and with the pairings of the tokens (`TerminalMarksPainter`,
///   docs/decisions.d/M4-35.md D-M4-35-18).
/// - ANSI colours: SGR 30 to 37 map to `color.terminal.ansi.<name>` and SGR 90 to 97 to
///   `color.terminal.ansi.bright-<name>` (components.md 12); 256-colour and truecolor values
///   are painted unchanged, because the program chooses them. xterm 4.0.0 resolves palette
///   index 15 (SGR 97) to `white` instead of `brightWhite` (`PaletteBuilder.paletteColor`), so
///   bright white shows in `color.terminal.ansi.white` until the renderer fixes that.
/// - Search hits: there is no search role in the tokens, so the closest roles are used:
///   `color.terminal.selection-bg` for both hit backgrounds (current and other) and
///   `color.terminal.fg` for the hit foreground. xterm 4.0.0 does not read these fields
///   while painting (highlights carry their own colour), they are set because the
///   constructor requires them.
///
/// [colors] is usually [DsColors.light] or [DsColors.dark], or the extension of the current
/// theme. Never throws.
///
/// ```dart
/// TerminalView(terminal, theme: buildTerminalTheme(DsColors.dark));
/// ```
TerminalTheme buildTerminalTheme(DsColors colors) => TerminalTheme(
  cursor: colors.terminalCursor.withValues(alpha: 0),
  selection: colors.terminalSelectionBg.withValues(alpha: 0),
  foreground: colors.terminalFg,
  background: colors.terminalBg,
  black: colors.terminalAnsiBlack,
  red: colors.terminalAnsiRed,
  green: colors.terminalAnsiGreen,
  yellow: colors.terminalAnsiYellow,
  blue: colors.terminalAnsiBlue,
  magenta: colors.terminalAnsiMagenta,
  cyan: colors.terminalAnsiCyan,
  white: colors.terminalAnsiWhite,
  brightBlack: colors.terminalAnsiBrightBlack,
  brightRed: colors.terminalAnsiBrightRed,
  brightGreen: colors.terminalAnsiBrightGreen,
  brightYellow: colors.terminalAnsiBrightYellow,
  brightBlue: colors.terminalAnsiBrightBlue,
  brightMagenta: colors.terminalAnsiBrightMagenta,
  brightCyan: colors.terminalAnsiBrightCyan,
  brightWhite: colors.terminalAnsiBrightWhite,
  searchHitBackground: colors.terminalSelectionBg,
  searchHitBackgroundCurrent: colors.terminalSelectionBg,
  searchHitForeground: colors.terminalFg,
);

/// The colours of the cursor and the selection the view paints itself (components.md 12):
/// the cursor cell in `color.terminal.cursor` with its character in
/// `color.terminal.cursor-text`, a selected cell in `color.terminal.selection-bg` with its
/// character in `color.terminal.fg`. Never throws.
TerminalMarkColors terminalMarkColors(DsColors colors) => TerminalMarkColors(
  cursor: colors.terminalCursor,
  cursorText: colors.terminalCursorText,
  selection: colors.terminalSelectionBg,
  selectionText: colors.terminalFg,
);
