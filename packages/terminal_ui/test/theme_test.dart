// SPDX-License-Identifier: Apache-2.0

import 'dart:io';

import 'package:design_system/design_system.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:terminal_ui/src/render/terminal_theme.dart';
import 'package:xterm/xterm.dart' show TerminalTheme;

/// Every field of xterm's [TerminalTheme] with the token it must carry (components.md 12
/// "Terminal view": screen, cursor, selection, ANSI colours).
Map<String, (Color, Color)> pairs(TerminalTheme t, DsColors c) => {
  // Cursor and selection keep their token colour at terminalOverlayAlpha (D-M4-35-9).
  'cursor': (
    t.cursor,
    c.terminalCursor.withValues(alpha: terminalOverlayAlpha),
  ),
  'selection': (
    t.selection,
    c.terminalSelectionBg.withValues(alpha: terminalOverlayAlpha),
  ),
  'foreground': (t.foreground, c.terminalFg),
  'background': (t.background, c.terminalBg),
  'black': (t.black, c.terminalAnsiBlack),
  'red': (t.red, c.terminalAnsiRed),
  'green': (t.green, c.terminalAnsiGreen),
  'yellow': (t.yellow, c.terminalAnsiYellow),
  'blue': (t.blue, c.terminalAnsiBlue),
  'magenta': (t.magenta, c.terminalAnsiMagenta),
  'cyan': (t.cyan, c.terminalAnsiCyan),
  'white': (t.white, c.terminalAnsiWhite),
  'brightBlack': (t.brightBlack, c.terminalAnsiBrightBlack),
  'brightRed': (t.brightRed, c.terminalAnsiBrightRed),
  'brightGreen': (t.brightGreen, c.terminalAnsiBrightGreen),
  'brightYellow': (t.brightYellow, c.terminalAnsiBrightYellow),
  'brightBlue': (t.brightBlue, c.terminalAnsiBrightBlue),
  'brightMagenta': (t.brightMagenta, c.terminalAnsiBrightMagenta),
  'brightCyan': (t.brightCyan, c.terminalAnsiBrightCyan),
  'brightWhite': (t.brightWhite, c.terminalAnsiBrightWhite),
  'searchHitBackground': (t.searchHitBackground, c.terminalSelectionBg),
  'searchHitBackgroundCurrent': (
    t.searchHitBackgroundCurrent,
    c.terminalSelectionBg,
  ),
  'searchHitForeground': (t.searchHitForeground, c.terminalFg),
};

/// The colour literals no file under `lib/` may hold: colours come from the tokens only.
final List<(String, RegExp)> colourLiterals = [
  ('Color(0x', RegExp(r'Color\(\s*0[xX]')),
  ('Color(<digit>', RegExp(r'Color\(\s*(?!0[xX])\d')),
  ('Color.fromARGB', RegExp(r'Color\.fromARGB\b')),
  ('Color.fromRGBO', RegExp(r'Color\.fromRGBO\b')),
  ('Colors.', RegExp(r'\bColors\.')),
];

/// The colour literals in [source], as "line: pattern" entries.
List<String> findColourLiterals(String source) => [
  for (final (i, line) in source.split('\n').indexed)
    for (final (name, pattern) in colourLiterals)
      if (pattern.hasMatch(line)) '${i + 1}: $name',
];

void main() {
  for (final (name, colors) in [
    ('light', DsColors.light),
    ('dark', DsColors.dark),
  ]) {
    test('$name: every field of the theme is its token', () {
      final theme = buildTerminalTheme(colors);
      final p = pairs(theme, colors);
      // 23 fields of xterm 4.0.0 TerminalTheme; a new field would go unmapped silently.
      expect(p, hasLength(23));
      for (final MapEntry(key: field, value: (actual, token)) in p.entries) {
        expect(actual, token, reason: '$name.$field');
      }
    });
  }

  test('light and dark themes differ, so the tokens are really read', () {
    expect(
      buildTerminalTheme(DsColors.light).background,
      isNot(buildTerminalTheme(DsColors.dark).background),
    );
  });

  group('colour literal scan', () {
    test('finds every forbidden form', () {
      expect(findColourLiterals('const a = Color(0xFF000000);'), [
        '1: Color(0x',
      ]);
      expect(findColourLiterals('Color( 0XFF000000)'), ['1: Color(0x']);
      expect(findColourLiterals('Color(4278190080)'), ['1: Color(<digit>']);
      expect(findColourLiterals('Color.fromARGB(255, 0, 0, 0)'), [
        '1: Color.fromARGB',
      ]);
      expect(findColourLiterals('x\nColor.fromRGBO(0, 0, 0, 1)'), [
        '2: Color.fromRGBO',
      ]);
      expect(findColourLiterals('Colors.red'), ['1: Colors.']);
    });

    test('allows token access', () {
      expect(findColourLiterals('colors.terminalBg; DsColors.light'), isEmpty);
      expect(findColourLiterals('final Color c = colors.terminalFg;'), isEmpty);
    });

    test('no file under lib/ holds a colour literal', () {
      // The test runs with the package as current directory (flutter test).
      final lib = Directory('lib');
      expect(lib.existsSync(), isTrue, reason: 'run from packages/terminal_ui');
      final files = lib
          .listSync(recursive: true)
          .whereType<File>()
          .where((f) => f.path.endsWith('.dart'))
          .toList();
      expect(files, isNotEmpty);
      final hits = [
        for (final f in files)
          for (final hit in findColourLiterals(f.readAsStringSync()))
            '${f.path}:$hit',
      ];
      expect(hits, isEmpty);
    });
  });
}
