// SPDX-License-Identifier: Apache-2.0

// The golden comparator of the design system (docs/DECISIONS.md PL/5): small
// anti-aliasing noise between hosts passes in a CI golden, a changed line or size does not,
// and a platform golden matches exactly.

import 'dart:async';
import 'dart:io';
import 'dart:typed_data';
import 'dart:ui' as ui;

import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:flutter_test/flutter_test.dart';

const _size = 100;

/// A PNG of [_size] x [_size] grey pixels; [paint] may change single pixels (RGBA bytes).
Future<Uint8List> _png([
  void Function(Uint8List rgba, int width)? paint,
  int width = _size,
]) async {
  final rgba = Uint8List(width * _size * 4);
  for (var i = 0; i < rgba.length; i += 4) {
    rgba
      ..[i] = 120
      ..[i + 1] = 120
      ..[i + 2] = 120
      ..[i + 3] = 255;
  }
  paint?.call(rgba, width);
  final completer = Completer<ui.Image>();
  ui.decodeImageFromPixels(
    rgba,
    width,
    _size,
    ui.PixelFormat.rgba8888,
    completer.complete,
  );
  final image = await completer.future;
  final png = await image.toByteData(format: ui.ImageByteFormat.png);
  return png!.buffer.asUint8List();
}

void main() {
  late Directory dir;
  late DsGoldenFileComparator comparator;

  setUp(() {
    dir = Directory.systemTemp.createTempSync('ds_golden_comparator');
    comparator = DsGoldenFileComparator(dir.uri.resolve('a_test.dart'));
  });

  tearDown(() => dir.deleteSync(recursive: true));

  Future<bool> compare(Uint8List image, [String golden = 'ci/g.png']) =>
      comparator.compare(image, Uri.parse(golden));

  Future<void> writeGolden(Uint8List golden, [String path = 'ci/g.png']) async {
    final file = File('${dir.path}/$path');
    await file.parent.create(recursive: true);
    await file.writeAsBytes(golden);
  }

  testWidgets('anti-aliasing noise on many pixels passes', (tester) async {
    await tester.runAsync(() async {
      await writeGolden(await _png());
      // Every pixel of every second row off by the channel tolerance: 50 % of the image.
      final noisy = await _png((rgba, width) {
        for (var y = 0; y < _size; y += 2) {
          for (var x = 0; x < width; x++) {
            rgba[(y * width + x) * 4] += dsCiGoldenChannelTolerance;
          }
        }
      });
      expect(await compare(noisy), isTrue);
    });
  });

  testWidgets('a changed one-pixel line fails', (tester) async {
    await tester.runAsync(() async {
      await writeGolden(await _png());
      final line = await _png((rgba, width) {
        for (var x = 0; x < width; x++) {
          rgba[(50 * width + x) * 4] = 0;
        }
      });
      expect(await compare(line), isFalse);
    });
  });

  testWidgets('a difference just above the tolerance counts', (tester) async {
    await tester.runAsync(() async {
      await writeGolden(await _png());
      final shifted = await _png((rgba, width) {
        for (var i = 0; i < rgba.length; i += 4) {
          rgba[i] += dsCiGoldenChannelTolerance + 1;
        }
      });
      expect(await compare(shifted), isFalse);
    });
  });

  test('a subtle dark border on a raised surface differs by more than the tolerance', () {
    int channel(ui.Color c, double Function(ui.Color) f) =>
        (f(c) * 255).round();
    const dark = DsColors.dark;
    final steps = [
      for (final f in <double Function(ui.Color)>[
        (c) => c.r,
        (c) => c.g,
        (c) => c.b,
      ])
        (channel(dark.borderSubtle, f) - channel(dark.surfaceRaised, f)).abs(),
    ];
    expect(steps.reduce((a, b) => a > b ? a : b), 9);
    expect(steps.any((s) => s > dsCiGoldenChannelTolerance), isTrue);
  });

  testWidgets('a platform golden still matches exactly', (tester) async {
    await tester.runAsync(() async {
      await writeGolden(await _png(), 'macos/g.png');
      final pixel = await _png((rgba, width) => rgba[0] += 1);
      expect(await compare(pixel, 'macos/g.png'), isFalse);
    });
  });

  testWidgets('a different size fails', (tester) async {
    await tester.runAsync(() async {
      await writeGolden(await _png());
      expect(await compare(await _png(null, _size + 1)), isFalse);
    });
  });
}
