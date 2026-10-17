// SPDX-License-Identifier: Apache-2.0

// The golden comparator of the design system (docs/DECISIONS.md PL/5): small
// anti-aliasing noise between hosts passes, a changed line or size does not.

import 'dart:async';
import 'dart:io';
import 'dart:typed_data';
import 'dart:ui' as ui;

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

  Future<bool> compare(Uint8List image) =>
      comparator.compare(image, Uri.parse('g.png'));

  Future<void> writeGolden(Uint8List golden) =>
      File('${dir.path}/g.png').writeAsBytes(golden);

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

  testWidgets('a different size fails', (tester) async {
    await tester.runAsync(() async {
      await writeGolden(await _png());
      expect(await compare(await _png(null, _size + 1)), isFalse);
    });
  });
}
