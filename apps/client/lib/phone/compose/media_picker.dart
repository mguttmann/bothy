// SPDX-License-Identifier: Apache-2.0

/// Photos from the camera and the photo library of the phone (15 14; 12 7.1; ADR 0047; brief
/// M5-07 Design note 4; brief M5-16): the [MediaPickerPort] opens the system's camera (one photo)
/// or photo picker (several photos at once, up to the free attachment slots), and [MediaPicker]
/// turns the result into JPEG files in the app's temporary folder, which [attachPhoto] hands to
/// the composer's attachments in one call: the size and count checks against
/// `file_max_upload_bytes` and `message_max_attachments`, the upload entries in the outbox before
/// the message, and the local preview before sending all come from there.
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

  /// The system's photo picker selects one or more photos of the library.
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
  /// Opens the camera; returns the photo it took, or null when the person cancelled.
  ///
  /// Throws [MediaAccessDenied] when the system refuses the camera, and other exceptions when
  /// it fails.
  Future<RawMedia?> camera();

  /// Opens the photo picker for several photos; returns the picked photos in the picker's
  /// order, empty when the person cancelled. [limit] (at least 1) caps the selection where the
  /// platform supports it; null leaves it to the platform.
  ///
  /// Throws [MediaAccessDenied] when the system refuses the library, and other exceptions when
  /// the picker fails.
  Future<List<RawMedia>> library({int? limit});
}

/// The JPEG quality of a photo the app encodes (D-M5-07-4): high enough that a photo looks
/// unchanged, below 100 so the platform pickers re-encode a HEIC photo as JPEG.
const int photoJpegQuality = 90;

/// [MediaPickerPort] over package `image_picker` (BSD-3-Clause): `PHPickerViewController` with a
/// selection limit on iOS for the library and `UIImagePickerController` for the camera; on
/// Android the system photo picker (`EXTRA_PICK_IMAGES_MAX`) and the camera app. Both re-encode a photo that is neither PNG nor GIF as JPEG
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
  Future<RawMedia?> camera() => _denied(MediaSource.camera, () async {
    final file = await _picker.pickImage(
      source: ImageSource.camera,
      imageQuality: photoJpegQuality,
      requestFullMetadata: false,
    );
    return file == null ? null : RawMedia(path: file.path, name: file.name);
  });

  @override
  Future<List<RawMedia>> library({int? limit}) =>
      _denied(MediaSource.library, () async {
        // image_picker opens the single photo picker for a limit of 1 itself.
        final files = await _picker.pickMultiImage(
          imageQuality: photoJpegQuality,
          requestFullMetadata: false,
          limit: limit,
        );
        return [for (final f in files) RawMedia(path: f.path, name: f.name)];
      });

  static Future<T> _denied<T>(
    MediaSource source,
    Future<T> Function() open,
  ) async {
    try {
      return await open();
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

/// The JPEG [files] in the temporary folder in the picker's order (one of the camera, one or
/// more of the library), owned by the app (deleted after their upload or their refusal).
final class PhotoPicked extends PhotoOutcome {
  /// Creates the outcome.
  const PhotoPicked(this.files);

  /// The JPEG files.
  final List<PickedFile> files;
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

/// The photo [name] could not become a JPEG (a format the app cannot decode); nothing of the
/// pick was kept, also not the other photos picked with it.
final class PhotoUnsupported extends PhotoOutcome {
  /// Creates the outcome.
  const PhotoUnsupported(this.name);

  /// The name of the photo (a foreign name).
  final String name;
}

/// The camera or the picker failed, or a picked photo could not be read or written; nothing of
/// the pick was kept.
final class PhotoFailed extends PhotoOutcome {
  /// Creates the outcome.
  const PhotoFailed();
}

/// No attachment slot of `message_max_attachments` is free: the picker did not open and the
/// composer shows the attachments' refusal.
final class PhotoNoRoom extends PhotoOutcome {
  /// Creates the outcome.
  const PhotoNoRoom();
}

/// Turns what the [MediaPickerPort] gives into a JPEG file in the temporary folder.
///
/// A JPEG stays as it is (only its name gets `.jpg`); any other image the app can decode
/// (PNG, GIF, WebP, BMP, TIFF) is decoded and encoded as JPEG at [photoJpegQuality], with
/// transparency on white; a file the app cannot decode (a HEIC that the platform did not
/// convert) is refused with [PhotoUnsupported]. Several photos of the library are converted in
/// the picker's order and kept all or none: one that cannot be converted refuses the whole
/// pick and deletes the JPEGs already written. The picker's own files are deleted in every
/// case, so no photo stays outside the app's outgoing folder (requirement 16).
///
/// Example:
/// ```dart
/// final picker = MediaPicker(port: ImagePickerPort(), scratch: temporaryFolder);
/// final outcome = await picker.pick(MediaSource.library, limit: 4);
/// ```
final class MediaPicker {
  /// Creates the picker over [port], writing into [scratch].
  MediaPicker({required this._port, required this._scratch});

  final MediaPickerPort _port;
  final Future<Directory> Function() _scratch;

  /// Opens [source] and returns the JPEGs of the picked photos, or why there are none.
  /// [limit] caps the photos of the library (see [MediaPickerPort.library]); the camera takes
  /// one photo and ignores it.
  Future<PhotoOutcome> pick(MediaSource source, {int? limit}) async {
    final List<RawMedia> raws;
    try {
      raws = switch (source) {
        MediaSource.camera => [?await _port.camera()],
        MediaSource.library => await _port.library(limit: limit),
      };
    } on MediaAccessDenied catch (e) {
      return PhotoDenied(e.source);
    } on Object {
      return const PhotoFailed();
    }
    if (raws.isEmpty) return const PhotoCancelled();
    final written = <String>[];
    var kept = false;
    try {
      final dir = await _scratch();
      await dir.create(recursive: true);
      final stamp = DateTime.now().toUtc().microsecondsSinceEpoch;
      final files = <PickedFile>[];
      for (final (i, raw) in raws.indexed) {
        final bytes = await File(raw.path).readAsBytes();
        final jpeg = isJpeg(bytes)
            ? bytes
            : await Isolate.run(
                () => encodeAsJpeg(bytes),
                debugName: 'photo-jpeg',
              );
        if (jpeg == null) return PhotoUnsupported(raw.name);
        final name = jpegName(raw.name);
        // The index keeps two photos of one pick with the same name apart.
        final out = File('${dir.path}${Platform.pathSeparator}$stamp-$i-$name');
        written.add(out.path);
        await out.writeAsBytes(jpeg, flush: true);
        files.add(PickedFile(path: out.path, name: name, owned: true));
      }
      kept = true;
      return PhotoPicked(files);
    } on Object {
      // An unreadable file, a full disk or an encoder error: no photo of the pick is attached.
      return const PhotoFailed();
    } finally {
      for (final raw in raws) {
        await _delete(raw.path);
      }
      if (!kept) {
        for (final path in written) {
          await _delete(path);
        }
      }
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

/// Picks photos from [source] with [picker] and attaches them to [attach] in one call: the
/// size and count checks against `file_max_upload_bytes` and `message_max_attachments` and the
/// upload entries happen there (a refused set is deleted and the reason shows as
/// `attach.refusal`). The library is asked for at most the free attachment slots; with none
/// free nothing opens, `attach.refusal` says why and the outcome is [PhotoNoRoom]. Returns the
/// outcome of the picker.
///
/// Example: `final outcome = await attachPhoto(picker, attach, MediaSource.library);`
Future<PhotoOutcome> attachPhoto(
  MediaPicker picker,
  AttachController attach,
  MediaSource source,
) async {
  if (!attach.hasRoom()) return const PhotoNoRoom();
  final outcome = await picker.pick(source, limit: attach.freeSlots);
  if (outcome is PhotoPicked) await attach.add(outcome.files);
  return outcome;
}
