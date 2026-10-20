// SPDX-License-Identifier: Apache-2.0

// The painted cursor and selection (components.md 12, D-M4-35-18), measured on real pixels
// over the whole table of light and dark, device pixel ratios 1, 1.25 and 2, cursor cells at
// the top left, in the middle and at the bottom right, focused and unfocused, and a plain, a
// coloured and an empty cell under the cursor:
//
// - the cursor is painted in its own cell and nowhere else (red team N1: an underline in row
//   0 instead of the cursor row);
// - the focused cursor fills its cell opaque in `cursor`, reaches `ui` (3:1) against `bg`, and
//   its character is in `cursor-text` at `text` (4.5:1) (red team M7 and N4);
// - the unfocused cursor is a frame in `cursor` with at least one pixel at full coverage;
// - a selected cell is opaque `selection-bg` with its character in `fg` at 4.5:1 (red team N4,
//   3.28:1 in light).

import 'dart:convert';
import 'dart:math' as math;
import 'dart:typed_data';
import 'dart:ui' as ui;

import 'package:design_system/design_system.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';
import 'package:terminal_ui/terminal_ui.dart';
import 'package:xterm/xterm.dart'
    show CellOffset, TerminalView, TerminalViewState;

import 'support/fakes.dart';

const Size _logical = Size(640, 400);

double _lum(Color c) {
  double ch(double v) =>
      v <= 0.03928 ? v / 12.92 : math.pow((v + 0.055) / 1.055, 2.4).toDouble();
  return 0.2126 * ch(c.r) + 0.7152 * ch(c.g) + 0.0722 * ch(c.b);
}

double _contrast(Color a, Color b) {
  final x = _lum(a);
  final y = _lum(b);
  return (math.max(x, y) + 0.05) / (math.min(x, y) + 0.05);
}

bool _near(Color a, Color b, [int tolerance = 2]) =>
    ((a.r - b.r) * 255).abs() <= tolerance &&
    ((a.g - b.g) * 255).abs() <= tolerance &&
    ((a.b - b.b) * 255).abs() <= tolerance;

final class _Shot {
  _Shot(this.bytes, this.width, this.height);

  final ByteData bytes;
  final int width;
  final int height;

  Color at(int x, int y) {
    final i = (y * width + x) * 4;
    return Color.fromARGB(
      255,
      bytes.getUint8(i),
      bytes.getUint8(i + 1),
      bytes.getUint8(i + 2),
    );
  }
}

final class _Scene {
  _Scene(this.tester, this.h, this.dpr);

  final WidgetTester tester;
  final Harness h;
  final double dpr;
  final GlobalKey boundary = GlobalKey();

  Future<_Shot> shot() async {
    await tester.pump();
    final render =
        boundary.currentContext!.findRenderObject()! as RenderRepaintBoundary;
    final image = (await tester.runAsync(
      () => render.toImage(pixelRatio: dpr),
    ))!;
    final data = (await tester.runAsync(
      () => image.toByteData(format: ui.ImageByteFormat.rawRgba),
    ))!;
    final shot = _Shot(data, image.width, image.height);
    image.dispose();
    return shot;
  }

  /// The cell [x], [y] (screen row) in device pixels of the shot.
  Rect cell(int x, int y) {
    final rt = tester
        .state<TerminalViewState>(find.byType(TerminalView))
        .renderTerminal;
    final origin =
        rt.localToGlobal(Offset.zero) -
        (boundary.currentContext!.findRenderObject()! as RenderBox)
            .localToGlobal(Offset.zero);
    final abs = h.controller.screen.terminal.buffer.scrollBack + y;
    final o = rt.getOffset(CellOffset(x, abs)) + origin;
    final top = o.dy.truncateToDouble();
    return Rect.fromLTWH(
      o.dx * dpr,
      top * dpr,
      rt.cellSize.width * dpr,
      rt.cellSize.height * dpr,
    );
  }

  void write(String text) =>
      h.controller.screen.write(Uint8List.fromList(utf8.encode(text)));
}

Future<_Scene> _pump(
  WidgetTester tester, {
  required DsThemeMode theme,
  required double dpr,
  required bool focused,
}) async {
  tester.view
    ..physicalSize = _logical * dpr
    ..devicePixelRatio = dpr;
  addTearDown(tester.view.reset);
  final h = (await tester.runAsync(Harness.create))!;
  final scene = _Scene(tester, h, dpr);
  final ds = DsContext(
    theme: theme,
    density: DsDensityMode.comfortable,
    motion: DsMotionMode.reduced,
    accent: dsDefaultAccent,
  );
  await tester.pumpWidget(
    MaterialApp(
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      theme: buildDsTheme(ds),
      builder: (context, app) => DsThemeScope(dsContext: ds, child: app!),
      home: Scaffold(
        body: RepaintBoundary(
          key: scene.boundary,
          child: TerminalScreenView(
            controller: h.controller,
            fontSize: 14,
            autofocus: focused,
          ),
        ),
      ),
    ),
  );
  await tester.pump();
  return scene;
}

// Device pixel rect of [r], grown by one pixel for anti-aliasing at the edges.
(int, int, int, int) _box(Rect r, _Shot s, [int grow = 0]) => (
  math.max(0, r.left.floor() - grow),
  math.max(0, r.top.floor() - grow),
  math.min(s.width, r.right.ceil() + grow),
  math.min(s.height, r.bottom.ceil() + grow),
);

void main() {
  const cells = {'plain': 'W', 'coloured': '\x1b[31mW\x1b[0m', 'empty': ' '};
  for (final theme in [DsThemeMode.light, DsThemeMode.dark]) {
    final colors = theme == DsThemeMode.light ? DsColors.light : DsColors.dark;
    for (final dpr in [1.0, 1.25, 2.0]) {
      for (final focused in [true, false]) {
        testWidgets(
          'cursor: ${theme.name}, dpr $dpr, ${focused ? 'focused' : 'unfocused'}: '
          'every position and cell',
          (tester) async {
            final scene = await _pump(
              tester,
              theme: theme,
              dpr: dpr,
              focused: focused,
            );
            final t = scene.h.controller.screen.terminal;
            final positions = [
              (0, 0),
              (10, 5),
              (t.viewWidth - 1, t.viewHeight - 1),
            ];
            for (final (cx, cy) in positions) {
              for (final MapEntry(key: kind, value: content) in cells.entries) {
                final why =
                    '${theme.name} dpr $dpr focused $focused '
                    'cell ($cx, $cy) $kind';
                final at = '\x1b[${cy + 1};${cx + 1}H';
                scene.write('\x1b[2J\x1b[H$at$content$at\x1b[?25l');
                final hidden = await scene.shot();
                scene.write('\x1b[?25h');
                final shown = await scene.shot();
                final rect = scene.cell(cx, cy);
                final (l, tp, r, b) = _box(rect, shown, 1);
                // Painted in its own cell and nowhere else.
                var changed = 0;
                for (var y = 0; y < shown.height; y++) {
                  for (var x = 0; x < shown.width; x++) {
                    if (shown.at(x, y) == hidden.at(x, y)) continue;
                    changed++;
                    expect(
                      x >= l && x < r && y >= tp && y < b,
                      isTrue,
                      reason: '$why: pixel ($x, $y) outside the cell $rect',
                    );
                  }
                }
                expect(changed, greaterThan(0), reason: '$why: no cursor');
                final (il, it, ir, ib) = _box(rect.deflate(0.01), shown);
                if (focused) {
                  // Opaque cursor fill, contrast `ui` against bg.
                  var fill = 0;
                  var best = 1.0;
                  for (var y = it; y < ib; y++) {
                    for (var x = il; x < ir; x++) {
                      final p = shown.at(x, y);
                      if (_near(p, colors.terminalCursor)) fill++;
                      best = math.max(
                        best,
                        _contrast(p, colors.terminalCursor),
                      );
                      // Only the two colours of the pairing (and their blends).
                      final d1 = _contrast(p, colors.terminalCursor);
                      final d2 = _contrast(p, colors.terminalCursorText);
                      expect(
                        d1 * d2 <=
                            _contrast(
                                  colors.terminalCursor,
                                  colors.terminalCursorText,
                                ) *
                                1.05,
                        isTrue,
                        reason: '$why: pixel ($x, $y) $p is not of the pairing',
                      );
                    }
                  }
                  // The test font draws every glyph as a box, so only an empty cell shows
                  // the fill on most of its pixels; a glyph cell shows both colours.
                  expect(
                    fill,
                    kind == 'empty'
                        ? greaterThan((ir - il) * (ib - it) * 9 ~/ 10)
                        : greaterThan(0),
                    reason: '$why: the cell is not filled with the cursor',
                  );
                  expect(
                    _contrast(colors.terminalCursor, colors.terminalBg),
                    greaterThanOrEqualTo(3.0),
                    reason: why,
                  );
                  if (kind != 'empty') {
                    expect(best, greaterThanOrEqualTo(4.5), reason: why);
                  }
                } else {
                  // A frame with at least one pixel of the full cursor colour on each side.
                  bool row(int y) =>
                      [for (var x = il; x < ir; x++) shown.at(x, y)]
                          .any((p) => _near(p, colors.terminalCursor));
                  bool col(int x) =>
                      [for (var y = it; y < ib; y++) shown.at(x, y)]
                          .any((p) => _near(p, colors.terminalCursor));
                  expect(row(it) && row(ib - 1), isTrue, reason: why);
                  expect(col(il) && col(ir - 1), isTrue, reason: why);
                }
              }
            }
            scene.h.controller.dispose();
          },
        );
      }
    }

    for (final dpr in [1.0, 1.25, 2.0]) {
      testWidgets(
        'selection: ${theme.name}, dpr $dpr: opaque with fg at 4.5:1',
        (tester) async {
          final scene = await _pump(
            tester,
            theme: theme,
            dpr: dpr,
            focused: true,
          );
          scene.write(
            '\x1b[2J\x1b[H\x1b[3;1HWWWW \x1b[32mWWWW\x1b[0m\x1b[10;1H',
          );
          await tester.pump();
          // Select row 2 (zero-based) columns 0 to 8 with the mouse.
          final from = scene.cell(0, 2).center / dpr;
          final to = scene.cell(8, 2).center / dpr;
          final box =
              scene.boundary.currentContext!.findRenderObject()! as RenderBox;
          final g = await tester.startGesture(
            box.localToGlobal(from),
            kind: PointerDeviceKind.mouse,
          );
          await tester.pump();
          await g.moveTo(box.localToGlobal(from + const Offset(5, 0)));
          await g.moveTo(box.localToGlobal(to));
          await g.up();
          await tester.pump();
          final shot = await scene.shot();
          for (final x in [0, 1, 2, 3, 5, 6, 7]) {
            final rect = scene.cell(x, 2);
            final (l, t, r, b) = _box(rect.deflate(0.01), shot);
            var fill = 0;
            var best = 1.0;
            for (var y = t; y < b; y++) {
              for (var px = l; px < r; px++) {
                final p = shot.at(px, y);
                if (_near(p, colors.terminalSelectionBg)) fill++;
                best = math.max(best, _contrast(p, colors.terminalSelectionBg));
              }
            }
            final why = '${theme.name} dpr $dpr cell $x';
            expect(fill, greaterThan(0), reason: why);
            expect(best, greaterThanOrEqualTo(4.5), reason: why);
          }
          scene.h.controller.dispose();
        },
      );
    }
  }
}
