// SPDX-License-Identifier: Apache-2.0

/// Handlers of the file operations (12 3, 8, 9; brief M4-22 Scope): `createUpload`,
/// `uploadPart`, `completeUpload`, `getUpload`, `abortUpload`, `getFile`, `downloadFile`,
/// `downloadThumbnail`, `createFileTicket`, `getTranscript`, `requestTranscription` and
/// `deleteFile`, over `FilesWorld`.
library;

import 'dart:convert';
import 'dart:typed_data';

import 'package:crypto/crypto.dart';

import '../rest_fake.dart';
import '../world.dart';

/// The types the server serves inline (12 9.3); everything else is an attachment.
const Set<String> _inline = {
  'image/jpeg',
  'image/png',
  'image/gif',
  'audio/mp4',
  'audio/mpeg',
  'video/mp4',
};

/// Registers the file handlers on [rest].
///
/// Downloads answer the headers of 12 9.3 with `ETag` and `Repr-Digest` of the whole variant
/// (a wrong digest for the files of `FakeFileSettings.wrongDigest`) and exactly one byte range
/// with `206`; a ticket download carries `Cache-Control: private, no-store`.
void registerFileHandlers(RestFake rest) {
  final world = rest.world;
  rest
    ..handle(
      'createUpload',
      (r) => FakeResponse(
        201,
        world.createUploadSession(r.person.id, r.param('channel_id'), r.json),
      ),
    )
    ..handle('uploadPart', (r) {
      final bytes = r.bytes ?? Uint8List(0);
      return FakeResponse(
        200,
        world.putPart(
          r.person.id,
          r.param('file_id'),
          int.parse(r.param('part_number')),
          r.headers['content-digest'],
          bytes,
        ),
      );
    })
    ..handle(
      'completeUpload',
      (r) => FakeResponse(
        200,
        world.completeUpload(r.person.id, r.param('file_id'), r.json),
      ),
    )
    ..handle(
      'getUpload',
      (r) =>
          FakeResponse(200, world.uploadState(r.person.id, r.param('file_id'))),
    )
    ..handle('abortUpload', (r) {
      world.abortUpload(r.person.id, r.param('file_id'));
      return const FakeResponse.noContent();
    })
    ..handle(
      'getFile',
      (r) => FakeResponse(
        200,
        world.visibleFile(r.person.id, r.param('file_id')).json,
      ),
    )
    ..handle('downloadFile', (r) => _download(r, world, 'original'))
    ..handle(
      'downloadThumbnail',
      (r) => _download(r, world, r.param('variant')),
    )
    ..handle(
      'createFileTicket',
      (r) => FakeResponse(
        201,
        world.createTicket(
          r.person.id,
          r.session!.id,
          r.param('file_id'),
          r.json['variant']! as String,
        ),
      ),
    )
    ..handle(
      'getTranscript',
      (r) => FakeResponse(
        200,
        world.transcriptOf(r.person.id, r.param('file_id')),
      ),
    )
    ..handle(
      'requestTranscription',
      (r) => FakeResponse(
        202,
        world.requestTranscription(r.person.id, r.param('file_id')),
      ),
    )
    ..handle('deleteFile', (r) {
      world.deleteOwnFile(r.person.id, r.param('file_id'));
      return const FakeResponse.noContent();
    });
}

FakeResponse _download(FakeRequest r, FakeWorld world, String variant) {
  final fileId = r.param('file_id');
  final (bytes, mime) = world.content(r.person.id, fileId, variant);
  final inline = _inline.contains(mime);
  final digest = world.fileSettings.wrongDigest.contains(fileId)
      ? sha256.convert(utf8.encode('damaged')).bytes
      : sha256.convert(bytes).bytes;
  final name = world.files[fileId]!.json['original_name']! as String;
  final headers = <String, String>{
    'Content-Disposition':
        '${inline ? 'inline' : 'attachment'}; filename="${_ascii(name)}"; '
        "filename*=UTF-8''${Uri.encodeComponent(name)}",
    'X-Content-Type-Options': 'nosniff',
    'Content-Security-Policy': "default-src 'none'; sandbox",
    'Cross-Origin-Resource-Policy': 'same-origin',
    'Cache-Control': r.query('ticket') != null
        ? 'private, no-store'
        : 'private, max-age=86400, immutable',
    'ETag': '"${sha256.convert(bytes)}"',
    'Repr-Digest': 'sha-256=:${base64.encode(digest)}:',
    'Accept-Ranges': 'bytes',
  };
  final range = RegExp(r'^bytes=(\d*)-(\d*)$')
      .firstMatch(r.headers['range'] ?? '');
  if (range != null && (range[1]!.isNotEmpty || range[2]!.isNotEmpty)) {
    final total = bytes.length;
    var start = int.tryParse(range[1]!) ?? -1;
    var end = int.tryParse(range[2]!) ?? total - 1;
    if (start < 0) {
      start = total - end;
      end = total - 1;
    }
    if (start >= total) throw const FakeProblem(416, 'range_not_satisfiable');
    if (end >= total) end = total - 1;
    return FakeResponse.bytes(
      bytes.sublist(start, end + 1),
      contentType: inline ? mime : 'application/octet-stream',
      status: 206,
      headers: {...headers, 'Content-Range': 'bytes $start-$end/$total'},
    );
  }
  return FakeResponse.bytes(
    bytes,
    contentType: inline ? mime : 'application/octet-stream',
    headers: headers,
  );
}

/// The ASCII fallback of a file name in `filename=` (12 9.3).
String _ascii(String name) => String.fromCharCodes([
  for (final c in name.runes)
    if (c >= 0x20 && c < 0x7f && c != 0x22 && c != 0x5c) c else 0x5f,
]);
