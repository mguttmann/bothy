// SPDX-License-Identifier: Apache-2.0

/// The renderer theme of the terminal view from the design tokens (docs/design/components.md
/// section 12 "Terminal view", roles `color.terminal.*`).
library;

import 'package:design_system/design_system.dart';
import 'package:xterm/xterm.dart' show TerminalTheme;

/// The opacity of the selection, which xterm paints over the text: the token colour stays,
/// only its alpha drops so the selected characters remain readable (D-M4-35-9).
const double terminalOverlayAlpha = 0.5;

/// The xterm [TerminalTheme] of the terminal screen, every field taken from a token of
/// [colors] (components.md 12), so the screen follows light and dark mode without a colour of
/// its own.
///
/// - Screen: `background` is `color.terminal.bg`, `foreground` is `color.terminal.fg`.
/// - Cursor: `cursor` is `color.terminal.cursor`, opaque. xterm 4.0.0 has no colour for the
///   character under the cursor: `TerminalPainter.paintCursor` fills a block cell with
///   `cursor` after the text is painted, so the view draws the cursor as an underline, which
///   keeps the character at full contrast and the cursor at the contrast of its pairing.
///   `color.terminal.cursor-text` ([DsColors.terminalCursorText]) therefore has no field here
///   (docs/DECISIONS.md D-M4-35-9).
/// - Selection: `selection` is `color.terminal.selection-bg` at [terminalOverlayAlpha]. The
///   design pairs it as the background under `color.terminal.fg` (design/design-rules.json),
///   but xterm 4.0.0 paints the selection as a filled rectangle after the text, so the opaque
///   token would cover the selected characters (D-M4-35-9).
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
  cursor: colors.terminalCursor,
  selection: colors.terminalSelectionBg.withValues(alpha: terminalOverlayAlpha),
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
