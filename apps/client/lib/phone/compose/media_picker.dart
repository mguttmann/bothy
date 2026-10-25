// SPDX-License-Identifier: Apache-2.0

/// Photos from the camera and the photo library of the phone (15 14; 12 7.1; ADR 0047; brief
/// M5-07 Design note 4): the [MediaPickerPort] opens the system's camera or photo picker, and
/// [MediaPicker] turns its result into a JPEG file in the app's temporary folder, which
/// [attachPhoto] hands to the composer's attachments: the size check against
/// `file_max_upload_bytes`, the upload entry in the outbox before the message, and the local
/// preview before sending all come from there.
///
/// Why JPEG: the server removes the metadata of JPEG and PNG losslessly but not of HEIC, so a
/// photo reaches it as JPEG and HEIC only as a deliberately sent file (12 7.1).
library;

import 'dart:io';
import 'dart:isolate';

import 'package:flutter/services.dart';
import 'package:image/image.dart' as img;
import 'package:image_picker/image_picker.dart' show ImagePicker, ImageSource;
import 'package:image_picker_android/image_picker_android.dart';
import 'package:image_picker_platform_interface/image_picker_platform_interface.dart'
    show ImagePickerPlatform;

import '../../files/attach_controller.dart';
import '../../files/file_ports.dart';

/// Where a photo comes from.
enum MediaSource {
  /// The camera takes a new photo.
  camera,

  /// The system's photo picker selects a photo of the library.
  library,
}

/// The file the system picker wrote: where it lies and the name it gave.
final class RawMedia {
  /// Creates the result at [path] named [name].
  const RawMedia({required this.path, required this.name});

  /// The file in the app's temporary or cache folder.
  final String path;

  /// The name of the file (for a photo of the library its name there).
  final String name;
}

/// The system refused the camera or the photo library ([source]).
final class MediaAccessDenied implements Exception {
  /// Creates the refusal.
  const MediaAccessDenied(this.source);

  /// What was refused.
  final MediaSource source;

  @override
  String toString() => 'MediaAccessDenied(${source.name})';
}

/// The camera and the photo picker of the system.
abstract interface class MediaPickerPort {
  /// Opens the camera or the photo picker ([source]); returns the picked photo, or null when
  /// the person cancelled.
  ///
  /// Throws [MediaAccessDenied] when the system refuses the access, and other exceptions when
  /// the picker fails.
  Future<RawMedia?> pick(MediaSource source);
}

/// The JPEG quality of a photo the app encodes (D-M5-07-4): high enough that a photo looks
/// unchanged, below 100 so the platform pickers re-encode a HEIC photo as JPEG.
const int photoJpegQuality = 90;

/// [MediaPickerPort] over package `image_picker` (BSD-3-Clause): `PHPickerViewController` on
/// iOS for the library and `UIImagePickerController` for the camera; on Android the system
/// photo picker and the camera app. Both re-encode a photo that is neither PNG nor GIF as JPEG
/// because a quality below 100 is requested; no full metadata is requested, so the library
/// needs no photo permission (D-M5-07-4).
final class ImagePickerPort implements MediaPickerPort {
  /// Creates the port; on Android it switches to the system photo picker, which needs no
  /// storage permission.
  ImagePickerPort() {
    final platform = ImagePickerPlatform.instance;
    if (platform is ImagePickerAndroid) platform.useAndroidPhotoPicker = true;
  }

  final ImagePicker _picker = ImagePicker();

  @override
  Future<RawMedia?> pick(MediaSource source) async {
    try {
      final file = await _picker.pickImage(
        source: source == MediaSource.camera
            ? ImageSource.camera
            : ImageSource.gallery,
        imageQuality: photoJpegQuality,
        requestFullMetadata: false,
      );
      if (file == null) return null;
      return RawMedia(path: file.path, name: file.name);
    } on PlatformException catch (e) {
      // The codes of image_picker for a refused camera or library.
      if (e.code == 'camera_access_denied' || e.code == 'photo_access_denied') {
        throw MediaAccessDenied(source);
      }
      rethrow;
    }
  }
}

/// What picking a photo gave.
sealed class PhotoOutcome {
  const PhotoOutcome();
}

/// A JPEG [file] in the temporary folder, owned by the app (deleted after its upload or its
/// refusal).
final class PhotoPicked extends PhotoOutcome {
  /// Creates the outcome.
  const PhotoPicked(this.file);

  /// The JPEG file.
  final PickedFile file;
}

/// The person cancelled the picker.
final class PhotoCancelled extends PhotoOutcome {
  /// Creates the outcome.
  const PhotoCancelled();
}

/// The system refused the access to [source]; the notice explains the system settings.
final class PhotoDenied extends PhotoOutcome {
  /// Creates the outcome.
  const PhotoDenied(this.source);

  /// What was refused.
  final MediaSource source;
}

/// The photo [name] could not become a JPEG (a format the app cannot decode); nothing was
/// kept.
final class PhotoUnsupported extends PhotoOutcome {
  /// Creates the outcome.
  const PhotoUnsupported(this.name);

  /// The name of the photo (a foreign name).
  final String name;
}

/// The camera or the picker failed.
final class PhotoFailed extends PhotoOutcome {
  /// Creates the outcome.
  const PhotoFailed();
}

/// Turns what the [MediaPickerPort] gives into a JPEG file in the temporary folder.
///
/// A JPEG stays as it is (only its name gets `.jpg`); any other image the app can decode
/// (PNG, GIF, WebP, BMP, TIFF) is decoded and encoded as JPEG at [photoJpegQuality], with
/// transparency on white; a file the app cannot decode (a HEIC that the platform did not
/// convert) is refused with [PhotoUnsupported]. The picker's own file is deleted in every
/// case, so no photo stays outside the app's outgoing folder (requirement 16).
///
/// Example:
/// ```dart
/// final picker = MediaPicker(port: ImagePickerPort(), scratch: temporaryFolder);
/// final outcome = await picker.pick(MediaSource.library);
/// ```
final class MediaPicker {
  /// Creates the picker over [port], writing into [scratch].
  MediaPicker({required this._port, required this._scratch});

  final MediaPickerPort _port;
  final Future<Directory> Function() _scratch;

  /// Opens [source] and returns the JPEG of the picked photo, or why there is none.
  Future<PhotoOutcome> pick(MediaSource source) async {
    final RawMedia? raw;
    try {
      raw = await _port.pick(source);
    } on MediaAccessDenied catch (e) {
      return PhotoDenied(e.source);
    } on Object {
      return const PhotoFailed();
    }
    if (raw == null) return const PhotoCancelled();
    try {
      final bytes = await File(raw.path).readAsBytes();
      final jpeg = isJpeg(bytes)
          ? bytes
          : await Isolate.run(
              () => encodeAsJpeg(bytes),
              debugName: 'photo-jpeg',
            );
      if (jpeg == null) return PhotoUnsupported(raw.name);
      final dir = await _scratch();
      await dir.create(recursive: true);
      final name = jpegName(raw.name);
      final out = File(
        '${dir.path}${Platform.pathSeparator}'
        '${DateTime.now().toUtc().microsecondsSinceEpoch}-$name',
      );
      await out.writeAsBytes(jpeg, flush: true);
      return PhotoPicked(PickedFile(path: out.path, name: name, owned: true));
    } on Object {
      // An unreadable file, a full disk or an encoder error: the photo is not attached.
      return const PhotoFailed();
    } finally {
      await _delete(raw.path);
    }
  }

  static Future<void> _delete(String path) async {
    try {
      final f = File(path);
      if (f.existsSync()) await f.delete();
    } on FileSystemException {
      // The picker's temporary folder, which the system empties.
    }
  }
}

/// Whether [bytes] start like a JPEG file (`SOI` followed by a marker).
bool isJpeg(Uint8List bytes) =>
    bytes.length > 3 &&
    bytes[0] == 0xFF &&
    bytes[1] == 0xD8 &&
    bytes[2] == 0xFF;

/// [name] with the extension `.jpg` (`IMG_0042.HEIC` becomes `IMG_0042.jpg`); `photo.jpg`
/// for an empty name.
String jpegName(String name) {
  final dot = name.lastIndexOf('.');
  final stem = dot > 0 ? name.substring(0, dot) : name;
  return '${stem.isEmpty ? 'photo' : stem}.jpg';
}

/// The JPEG encoding of the image [bytes] at [photoJpegQuality] (the first frame of an
/// animation, transparency on white), or null when package `image` cannot decode them.
///
/// Pure Dart, run in a background isolate by [MediaPicker].
Uint8List? encodeAsJpeg(Uint8List bytes) {
  final img.Image? decoded;
  try {
    decoded = img.decodeImage(bytes);
  } on Object {
    return null;
  }
  if (decoded == null) return null;
  var image = decoded;
  if (image.hasAlpha) {
    // JPEG has no transparency: put the image on white, as viewers show it.
    final white = img.Image(width: image.width, height: image.height)
      ..clear(img.ColorRgb8(255, 255, 255));
    image = img.compositeImage(white, image);
  }
  return img.encodeJpg(image, quality: photoJpegQuality);
}

/// Picks a photo from [source] with [picker] and attaches it to [attach]: the size check
/// against `file_max_upload_bytes` and the upload entry happen there (a refused photo is
/// deleted and the reason shows as `attach.refusal`). Returns the outcome of the picker.
///
/// Example: `final outcome = await attachPhoto(picker, attach, MediaSource.camera);`
Future<PhotoOutcome> attachPhoto(
  MediaPicker picker,
  AttachController attach,
  MediaSource source,
) async {
  final outcome = await picker.pick(source);
  if (outcome is PhotoPicked) await attach.add([outcome.file]);
  return outcome;
}
