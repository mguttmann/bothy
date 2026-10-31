// SPDX-License-Identifier: Apache-2.0

/// Pasted screenshots go out small (FIX-scan-state-stale; 15 14): the bitmap of the clipboard,
/// which the app writes as a PNG, is sent as a JPEG of [photoJpegQuality] when that is at most
/// [pastedJpegMaxShare] of the PNG's size. A screenshot of a desktop is often several megabytes
/// as a PNG and a fraction of that as a JPEG, which uploads, scans and loads much faster; a flat
/// image whose JPEG would not be much smaller stays a PNG. Only a bitmap the app wrote itself is
/// changed: a file the person copied, dropped or picked is sent as it is, and the personal setting
/// `composer.paste_originals` keeps every pasted bitmap as it is.
library;

import 'dart:io';
import 'dart:isolate';
import 'dart:typed_data';

import '../phone/compose/media_picker.dart';
import 'file_ports.dart';

/// The largest share of the PNG's size at which the JPEG of a pasted bitmap is sent instead.
const double pastedJpegMaxShare = 0.7;

/// [files] of a paste with every bitmap the app wrote ([PickedFile.owned]) replaced by its JPEG
/// when that is at most [pastedJpegMaxShare] of its size; the PNG is deleted then. [originals]
/// (`composer.paste_originals`) keeps every file as it is. A bitmap the encoder cannot read, or
/// a write that fails, keeps the PNG.
///
/// Example:
/// ```dart
/// final files = await compactPastedImages(await paste.read(scratch), originals: false);
/// ```
Future<List<PickedFile>> compactPastedImages(
  List<PickedFile> files, {
  required bool originals,
}) async {
  if (originals) return files;
  return [for (final f in files) await _compact(f)];
}

Future<PickedFile> _compact(PickedFile f) async {
  if (!f.owned || !f.name.toLowerCase().endsWith('.png')) return f;
  try {
    final png = await File(f.path).readAsBytes();
    final jpeg = await Isolate.run(
      () => encodeAsJpeg(png),
      debugName: 'paste-jpeg',
    );
    if (jpeg == null || jpeg.length > png.length * pastedJpegMaxShare) {
      return f;
    }
    final name = jpegName(f.name);
    final out = File(
      '${File(f.path).parent.path}${Platform.pathSeparator}$name',
    );
    await out.writeAsBytes(Uint8List.fromList(jpeg), flush: true);
    await File(f.path).delete();
    return PickedFile(path: out.path, name: name, owned: true);
  } on FileSystemException {
    return f;
  }
}
