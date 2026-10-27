// SPDX-License-Identifier: Apache-2.0

// The capture of one showcase image (brief M7-28 Design note 1): the layer tree of the app's
// view rendered to a PNG through Flutter's own image API, the screenshot API of the test framework
// that works the same on every platform. Only the app's pixels are captured: no window frame, no
// system bar and nothing of the host's desktop can appear (brief requirement 8).

import 'dart:typed_data';
import 'dart:ui' as ui;

import 'package:flutter/rendering.dart';
import 'package:flutter_test/flutter_test.dart';

/// The pixel ratio that renders [physicalWidth] pixels of the view as [width] pixels.
///
/// Throws an [ArgumentError] for a width that is not positive.
///
/// Example: `captureRatio(1080, 720)` is `0.666...`.
double captureRatio(double physicalWidth, int width) {
  if (physicalWidth <= 0 || width <= 0) {
    throw ArgumentError('widths must be positive: $physicalWidth, $width');
  }
  return width / physicalWidth;
}

/// Renders the first view of [tester]'s binding to a PNG [width] pixels wide.
///
/// Pumps [settleFrames] frames first, so a tap indicator of the live test binding has faded
/// and the frame is painted. The PNG comes from the engine's encoder; tools/showcase strips
/// whatever ancillary chunks it may carry before the image is committed.
Future<Uint8List> capturePng(
  WidgetTester tester, {
  required int width,
  int settleFrames = 30,
}) async {
  for (var i = 0; i < settleFrames; i++) {
    await tester.pump(const Duration(milliseconds: 16));
  }
  final view = tester.binding.renderViews.first;
  // The root layer exists in a debug build, the mode `flutter drive` runs the showcase in.
  final layer = view.debugLayer! as OffsetLayer;
  final bounds = view.paintBounds;
  final bytes = await tester.runAsync(() async {
    final image = await layer.toImage(
      bounds,
      pixelRatio: captureRatio(bounds.width, width),
    );
    try {
      final data = await image.toByteData(format: ui.ImageByteFormat.png);
      return data!.buffer.asUint8List();
    } finally {
      image.dispose();
    }
  });
  return bytes!;
}
