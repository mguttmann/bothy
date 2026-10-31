// SPDX-License-Identifier: Apache-2.0

// A pasted screenshot goes out small (FIX-scan-state-stale): the bitmap of the clipboard, which
// the app writes as a PNG, is sent as a high quality JPEG when that is much smaller; a flat image
// whose JPEG would not be much smaller stays a PNG, a file the person copied or attached as a file
// is never changed, and `composer.paste_originals` keeps every pasted image as it is.

import 'dart:io';
import 'dart:math';
import 'dart:typed_data';

import 'package:client/files/file_ports.dart';
import 'package:client/files/pasted_image.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:image/image.dart' as img;

void main() {
  late Directory dir;
  setUp(() => dir = Directory.systemTemp.createTempSync('pasted-image-test'));
  tearDown(() => dir.deleteSync(recursive: true));

  /// A screenshot-like bitmap: a photo in a window, which PNG compresses poorly.
  Uint8List busyPng() {
    final random = Random(7);
    final image = img.Image(width: 320, height: 240);
    for (final p in image) {
      final shade = (p.x * 255 ~/ 320 + random.nextInt(48)) % 256;
      p
        ..r = shade
        ..g = (shade + p.y) % 256
        ..b = (255 - shade + random.nextInt(32)) % 256;
    }
    return img.encodePng(image);
  }

  /// A flat bitmap, which PNG compresses better than JPEG.
  Uint8List flatPng() => img.encodePng(
    img.Image(width: 64, height: 64)..clear(img.ColorRgb8(240, 240, 240)),
  );

  PickedFile write(String name, Uint8List bytes, {bool owned = true}) {
    final f = File('${dir.path}/$name')..writeAsBytesSync(bytes);
    return PickedFile(path: f.path, name: name, owned: owned);
  }

  test('a pasted bitmap that is much smaller as JPEG goes as JPEG', () async {
    final png = busyPng();
    final pasted = write('pasted-1.png', png);
    final out = await compactPastedImages([pasted], originals: false);
    expect(out.single.name, 'pasted-1.jpg');
    expect(out.single.owned, isTrue);
    final jpeg = File(out.single.path).readAsBytesSync();
    expect(jpeg.sublist(0, 3), [0xFF, 0xD8, 0xFF]);
    expect(jpeg.length, lessThanOrEqualTo(png.length * pastedJpegMaxShare));
    // The PNG the app wrote is gone; only the JPEG is uploaded.
    expect(File(pasted.path).existsSync(), isFalse);
  });

  test('a flat bitmap stays a PNG', () async {
    final pasted = write('pasted-2.png', flatPng());
    final out = await compactPastedImages([pasted], originals: false);
    expect(out.single.path, pasted.path);
    expect(out.single.name, 'pasted-2.png');
  });

  test('a copied file is never changed', () async {
    final copied = write('shot.png', busyPng(), owned: false);
    final out = await compactPastedImages([copied], originals: false);
    expect(out.single.path, copied.path);
    expect(File(copied.path).existsSync(), isTrue);
  });

  test('composer.paste_originals keeps the pasted bitmap', () async {
    final pasted = write('pasted-3.png', busyPng());
    final out = await compactPastedImages([pasted], originals: true);
    expect(out.single.path, pasted.path);
    expect(out.single.name, 'pasted-3.png');
  });

  test('a bitmap the encoder cannot read stays as it is', () async {
    final pasted = write(
      'pasted-4.png',
      Uint8List.fromList(List.filled(64, 0x41)),
    );
    final out = await compactPastedImages([pasted], originals: false);
    expect(out.single.path, pasted.path);
  });
}
