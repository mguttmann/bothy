// SPDX-License-Identifier: Apache-2.0

// The WCAG AA contrast of every text and icon color pair of the light and the dark theme, on
// the colors the app renders (FIX-light-theme-contrast): the pairs of
// design/design-rules.json (`text` 4.5:1, WCAG 1.4.3; `large-text` and `ui` 3:1, WCAG 1.4.11)
// over the `DsColors` constants compiled into the app and over every accent of `DsAccent`.
// The generator checks the same pairs on the token sources (`make tokens`); this test holds
// the generated Dart that the widgets read, and fails when a rule names a role the Dart
// theme does not have.

import 'dart:convert';
import 'dart:io';

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

/// The rules file of the design system; `flutter test` runs in the package folder.
final File _rules = File('../../design/design-rules.json');

/// The generated tokens the package compiles.
final File _generated = File('lib/src/generated/design_tokens.g.dart');

/// The minimum ratio of a pair kind.
const Map<String, double> _minimum = {'text': 4.5, 'large-text': 3, 'ui': 3};

/// The Dart field of a color token: `color.text.on-sidebar` is `textOnSidebar`, as the
/// generator names it (tools/design-tokens/tokenkit/emit_dart.py, `identifier`).
String fieldOf(String token) {
  final parts = token
      .replaceFirst('color.', '')
      .split(RegExp('[.-]'))
      .where((p) => p.isNotEmpty)
      .toList();
  return [
    parts.first,
    for (final p in parts.skip(1)) p[0].toUpperCase() + p.substring(1),
  ].join();
}

/// The colors of the constant `DsColors.<theme>` in the generated source, by field.
Map<String, Color> colorsIn(String source, String theme) {
  final start = source.indexOf('static const DsColors $theme = DsColors(');
  if (start < 0) throw StateError('DsColors.$theme not in the generated file');
  final end = source.indexOf('\n  );', start);
  final block = source.substring(start, end);
  return {
    for (final m in RegExp(
      r'(\w+): Color\(0x([0-9A-Fa-f]{8})\)',
    ).allMatches(block))
      m.group(1)!: Color(int.parse(m.group(2)!, radix: 16)),
  };
}

/// The accent roles of [accent] by field; the other roles come from `DsColors`.
Map<String, Color> accentRoles(DsAccent accent) => {
  'accentBg': accent.accentBg,
  'accentBgHover': accent.accentBgHover,
  'accentBgPressed': accent.accentBgPressed,
  'accentFg': accent.accentFg,
  'accentSubtle': accent.accentSubtle,
  'textLink': accent.textLink,
  'focusRing': accent.focusRing,
  'runRunning': accent.runRunning,
};

/// The WCAG 2 contrast ratio of two opaque colors.
double contrast(Color a, Color b) {
  final la = a.computeLuminance();
  final lb = b.computeLuminance();
  final (hi, lo) = la > lb ? (la, lb) : (lb, la);
  return (hi + 0.05) / (lo + 0.05);
}

List<String> _list(Object? value) =>
    value is List ? value.cast<String>() : [value! as String];

void main() {
  final rules = jsonDecode(_rules.readAsStringSync()) as Map<String, dynamic>;
  final pairs = (rules['contrast_pairs'] as List).cast<Map<String, dynamic>>();
  final variants = rules['accent_variants'] as Map<String, dynamic>;
  final accents = (variants['accents'] as List).cast<String>();
  final accentTokens = (variants['roles'] as List).cast<String>();
  final source = _generated.readAsStringSync();

  test('the field names follow the generator', () {
    expect(fieldOf('color.text.on-sidebar'), 'textOnSidebar');
    expect(
      fieldOf('color.category.spruce.on-sidebar'),
      'categorySpruceOnSidebar',
    );
    expect(
      fieldOf('color.terminal.ansi.bright-white'),
      'terminalAnsiBrightWhite',
    );
    expect(fieldOf('color.viz.series.1'), 'vizSeries1');
  });

  test('the WCAG ratio matches known values', () {
    expect(contrast(Colors.black, Colors.white), closeTo(21, 0.01));
    expect(contrast(Colors.white, Colors.white), closeTo(1, 0.0001));
    // #767676 on white is the classic 4.54:1.
    expect(
      contrast(const Color(0xFF767676), Colors.white),
      closeTo(4.54, 0.01),
    );
  });

  test('every accent role of the rules is a role of DsAccent', () {
    expect({
      for (final t in accentTokens) fieldOf(t),
    }, accentRoles(DsAccent.of('spruce', Brightness.light)).keys.toSet());
  });

  test('the sidebar roles fail on the light content surface', () {
    // Why the sidebar view paints `surface.sidebar` itself: on `surface.canvas` of the light
    // theme its texts and icons fall far below AA (the phone report of 2026-10-31).
    const c = DsColors.light;
    for (final fg in [
      c.textOnSidebar,
      c.textOnSidebarSecondary,
      c.textOnSidebarMuted,
      c.categorySpruceOnSidebar,
    ]) {
      expect(contrast(fg, c.surfaceCanvas), lessThan(3));
    }
  });

  for (final (theme, brightness, runtime) in [
    ('light', Brightness.light, DsColors.light),
    ('dark', Brightness.dark, DsColors.dark),
  ]) {
    group('theme $theme', () {
      final parsed = colorsIn(source, theme);

      test('the parsed constants are the ones the app renders', () {
        // Every color field of the constant, and a sample checked at runtime.
        expect(parsed.length, greaterThan(100));
        expect(parsed['textPrimary'], runtime.textPrimary);
        expect(parsed['surfaceCanvas'], runtime.surfaceCanvas);
        expect(parsed['surfaceSidebar'], runtime.surfaceSidebar);
        expect(parsed['textOnSidebar'], runtime.textOnSidebar);
        expect(parsed['textOnSidebarMuted'], runtime.textOnSidebarMuted);
        expect(
          parsed['categoryGraniteOnSidebar'],
          runtime.categoryGraniteOnSidebar,
        );
        expect(parsed['terminalAnsiWhite'], runtime.terminalAnsiWhite);
      });

      test('every role of the rules exists in DsColors', () {
        final missing = <String>{
          for (final pair in pairs)
            for (final token in [..._list(pair['fg']), ..._list(pair['bg'])])
              if (!parsed.containsKey(fieldOf(token))) token,
        };
        expect(missing, isEmpty);
      });

      test('every text role of DsColors is in a pair', () {
        final used = {
          for (final pair in pairs)
            for (final token in _list(pair['fg'])) fieldOf(token),
        };
        final texts = parsed.keys.where((f) => f.startsWith('text'));
        expect(texts, isNotEmpty);
        expect(texts.where((f) => !used.contains(f)), isEmpty);
      });

      for (final accent in accents) {
        test('every pair meets WCAG AA with the accent $accent', () {
          final colors = {
            ...parsed,
            ...accentRoles(DsAccent.of(accent, brightness)),
          };
          final failures = <String>[];
          var checked = 0;
          for (final pair in pairs) {
            final kind = pair['kind'] as String;
            final min = _minimum[kind];
            expect(min, isNotNull, reason: 'unknown kind $kind');
            for (final fg in _list(pair['fg'])) {
              for (final bg in _list(pair['bg'])) {
                final ratio = contrast(
                  colors[fieldOf(fg)]!,
                  colors[fieldOf(bg)]!,
                );
                checked++;
                if (ratio < min!) {
                  failures.add(
                    '$fg on $bg ($kind): ${ratio.toStringAsFixed(2)} < $min '
                    '(${pair['usage']})',
                  );
                }
              }
            }
          }
          expect(checked, greaterThan(100));
          expect(failures, isEmpty);
        });
      }
    });
  }
}
