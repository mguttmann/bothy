// SPDX-License-Identifier: Apache-2.0

// The outline Material draws around its controls is visible (AUD-pkg-design-system-01, finding
// design-system-2; WCAG 1.4.11 "Non-text Contrast", 3:1). Material 3 takes
// `ColorScheme.outline` for edges that carry a control: the ring and the thumb of a switch that
// is off, the underline or the border of a text field that is not filled (also of a dropdown),
// the border of a segmented button. The theme mapped that role to the decoration token
// `color.border.default`, far below 3:1 on the surfaces.
//
// The tests render the real widgets with `buildDsTheme` and read the pixels of the line through
// their middle, so they hold whatever the widgets resolve the color from.

import 'dart:typed_data';

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_test/flutter_test.dart';

/// The smallest ratio of an edge that identifies a control (WCAG 1.4.11).
const double _minimum = 3;

/// Pixels per logical pixel of the pictures; a hairline is then two pixels wide.
const double _pixelRatio = 2;

/// The WCAG 2 contrast ratio of two opaque colors.
double contrast(Color a, Color b) {
  final la = a.computeLuminance();
  final lb = b.computeLuminance();
  final (hi, lo) = la > lb ? (la, lb) : (lb, la);
  return (hi + 0.05) / (lo + 0.05);
}

const _shot = ValueKey<String>('shot');

DsContext _context(DsThemeMode theme) => DsContext(
  theme: theme,
  density: DsDensityMode.comfortable,
  motion: DsMotionMode.reduced,
  accent: dsDefaultAccent,
);

/// Pumps [control] on the canvas of [theme], with canvas around it.
Future<void> _pump(
  WidgetTester tester,
  DsThemeMode theme,
  Widget control,
) async {
  await tester.pumpWidget(
    MaterialApp(
      theme: buildDsTheme(_context(theme)),
      home: Scaffold(
        body: Align(
          alignment: Alignment.topLeft,
          child: RepaintBoundary(
            key: _shot,
            child: Builder(
              builder: (context) => ColoredBox(
                color: Theme.of(context).colorScheme.surface,
                child: Padding(
                  padding: const EdgeInsets.all(DsSpace.s12),
                  child: SizedBox(width: 240, child: control),
                ),
              ),
            ),
          ),
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
}

/// The colors of the pixel line through the middle of the picture, as runs of one color at
/// least two pixels long; the blended pixels of an anti-aliased edge drop out. The line is the
/// row from the left, or with [down] the column from the top.
Future<List<Color>> _runs(WidgetTester tester, {bool down = false}) async {
  final boundary = tester.renderObject<RenderRepaintBoundary>(
    find.byKey(_shot),
  );
  late int width;
  late int height;
  late ByteData bytes;
  await tester.runAsync(() async {
    final image = await boundary.toImage(pixelRatio: _pixelRatio);
    width = image.width;
    height = image.height;
    bytes = (await image.toByteData())!;
    image.dispose();
  });
  final length = down ? height : width;
  Color at(int n) {
    final i = down ? (n * width + width ~/ 2) : ((height ~/ 2) * width + n);
    return Color.fromARGB(
      bytes.getUint8(i * 4 + 3),
      bytes.getUint8(i * 4),
      bytes.getUint8(i * 4 + 1),
      bytes.getUint8(i * 4 + 2),
    );
  }

  final runs = <Color>[];
  var start = 0;
  for (var n = 1; n <= length; n++) {
    if (n < length && at(n) == at(start)) continue;
    if (n - start >= 2 && (runs.isEmpty || runs.last != at(start))) {
      runs.add(at(start));
    }
    start = n;
  }
  return runs;
}

void main() {
  test('the ratio matches known values', () {
    expect(contrast(Colors.black, Colors.white), closeTo(21, 0.01));
    expect(
      contrast(const Color(0xFF767676), Colors.white),
      closeTo(4.54, 0.01),
    );
  });

  for (final mode in DsThemeMode.values) {
    group('theme ${mode.name}', () {
      final scheme = buildDsTheme(_context(mode)).colorScheme;

      test(
        'the outline role is the control border, the variant stays decoration',
        () {
          final colors = mode == DsThemeMode.dark
              ? DsColors.dark
              : DsColors.light;
          expect(scheme.outline, colors.borderControl);
          expect(scheme.outlineVariant, colors.borderSubtle);
          // The surfaces Material draws the outline on: the canvas, and the track of a switch.
          for (final surface in [
            scheme.surface,
            scheme.surfaceContainerHighest,
          ]) {
            expect(
              contrast(scheme.outline, surface),
              greaterThanOrEqualTo(_minimum),
            );
          }
        },
      );

      testWidgets('a switch that is off shows its ring and its thumb', (
        tester,
      ) async {
        await _pump(
          tester,
          mode,
          Align(
            alignment: Alignment.centerLeft,
            child: Switch(value: false, onChanged: (_) {}),
          ),
        );
        // From the left: canvas, ring, track, thumb, track, ring, canvas.
        final runs = await _runs(tester);
        expect(runs.length, greaterThanOrEqualTo(4), reason: '$runs');
        final [canvas, ring, track, thumb, ...] = runs;
        expect(canvas, scheme.surface);
        expect(
          contrast(ring, canvas),
          greaterThanOrEqualTo(_minimum),
          reason: 'ring $ring on canvas $canvas',
        );
        expect(
          contrast(thumb, track),
          greaterThanOrEqualTo(_minimum),
          reason: 'thumb $thumb on track $track',
        );
      });

      testWidgets('a text field shows its underline', (tester) async {
        // The decoration of nearly every field of the apps: not filled, no border named.
        await _pump(tester, mode, const TextField());
        // From the top: canvas, underline, canvas.
        final runs = await _runs(tester, down: true);
        expect(runs.length, greaterThanOrEqualTo(2), reason: '$runs');
        final [canvas, underline, ...] = runs;
        expect(canvas, scheme.surface);
        expect(
          contrast(underline, canvas),
          greaterThanOrEqualTo(_minimum),
          reason: 'underline $underline on canvas $canvas',
        );
      });

      testWidgets('an outlined text field shows its border', (tester) async {
        await _pump(
          tester,
          mode,
          const TextField(
            decoration: InputDecoration(border: OutlineInputBorder()),
          ),
        );
        final runs = await _runs(tester);
        expect(runs.length, greaterThanOrEqualTo(2), reason: '$runs');
        final [canvas, border, ...] = runs;
        expect(canvas, scheme.surface);
        expect(
          contrast(border, canvas),
          greaterThanOrEqualTo(_minimum),
          reason: 'border $border on canvas $canvas',
        );
      });

      testWidgets('the dropdown field shows its border', (tester) async {
        await _pump(
          tester,
          mode,
          DsDropdownField<int>(
            label: 'Level',
            entries: const [
              DropdownMenuEntry(value: 1, label: 'Low'),
              DropdownMenuEntry(value: 2, label: 'High'),
            ],
            onSelected: (_) {},
          ),
        );
        final runs = await _runs(tester);
        expect(runs.length, greaterThanOrEqualTo(2), reason: '$runs');
        final [canvas, border, ...] = runs;
        expect(canvas, scheme.surface);
        expect(
          contrast(border, canvas),
          greaterThanOrEqualTo(_minimum),
          reason: 'border $border on canvas $canvas',
        );
      });

      testWidgets('a segmented button shows its border', (tester) async {
        await _pump(
          tester,
          mode,
          Align(
            alignment: Alignment.centerLeft,
            child: SegmentedButton<int>(
              showSelectedIcon: false,
              segments: const [
                ButtonSegment(value: 1, label: SizedBox(width: DsSpace.s24)),
                ButtonSegment(value: 2, label: SizedBox(width: DsSpace.s24)),
              ],
              selected: const {2},
              onSelectionChanged: (_) {},
            ),
          ),
        );
        final runs = await _runs(tester);
        expect(runs.length, greaterThanOrEqualTo(2), reason: '$runs');
        final [canvas, border, ...] = runs;
        expect(canvas, scheme.surface);
        expect(
          contrast(border, canvas),
          greaterThanOrEqualTo(_minimum),
          reason: 'border $border on canvas $canvas',
        );
      });
    });
  }
}
