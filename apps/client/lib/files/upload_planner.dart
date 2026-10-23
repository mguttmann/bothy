// SPDX-License-Identifier: Apache-2.0

/// What an upload needs before its entry is written (12 3.1, 3.2; brief M4-22 Design note 1):
/// the SHA-256 of the whole file, computed in a background isolate while the file streams
/// (never the whole file in memory), the declared type, the part plan of the session and the
/// `Content-Digest` of one part (RFC 9530).
library;

import 'dart:convert';
import 'dart:io';
import 'dart:isolate';

import 'package:crypto/crypto.dart';

/// A local file prepared for an upload entry.
final class UploadSource {
  /// Creates the source.
  const UploadSource({
    required this.path,
    required this.name,
    required this.sizeBytes,
    required this.sha256,
    required this.mimeType,
  });

  /// The file on disk; the executor reads the parts from here.
  final String path;

  /// The name to declare (12 3.1 `name`).
  final String name;

  /// The size in bytes (12 3.1 `size_bytes`).
  final int sizeBytes;

  /// The lowercase hex SHA-256 of the whole file (`UploadCreate.sha256`).
  final String sha256;

  /// The declared type, a hint only (12 6.1).
  final String mimeType;
}

/// Reads [path] and returns its [UploadSource] named [name] (default the last path segment),
/// with the type [mimeType] or one guessed from the extension.
///
/// The SHA-256 runs in a background isolate over a stream of the file ([sha256OfFile]).
/// Throws a [FileSystemException] when the file cannot be read.
Future<UploadSource> prepareUpload(
  String path, {
  String? name,
  String? mimeType,
}) async {
  final file = File(path);
  final size = await file.length();
  final fileName = name ?? path.split(Platform.pathSeparator).last;
  return UploadSource(
    path: path,
    name: fileName,
    sizeBytes: size,
    sha256: await sha256OfFile(path),
    mimeType: mimeType ?? guessMimeType(fileName),
  );
}

/// The lowercase hex SHA-256 of the file at [path], computed in a background isolate that
/// streams the file in chunks (brief M4-22 Design note 1).
Future<String> sha256OfFile(String path) =>
    Isolate.run(() => _hashFile(path), debugName: 'sha256');

Future<String> _hashFile(String path) async {
  final out = _DigestSink();
  final input = sha256.startChunkedConversion(out);
  await for (final chunk in File(path).openRead()) {
    input.add(chunk);
  }
  input.close();
  return out.value.toString();
}

final class _DigestSink implements Sink<Digest> {
  late Digest value;

  @override
  void add(Digest data) => value = data;

  @override
  void close() {}
}

/// One part of the plan of a session (12 3.1: `part_count` parts of `part_size_bytes`, the
/// last may be smaller).
final class PartPlan {
  /// Creates the part.
  const PartPlan({
    required this.number,
    required this.offset,
    required this.length,
  });

  /// The part number, from 1.
  final int number;

  /// Its first byte in the file.
  final int offset;

  /// Its length.
  final int length;

  @override
  bool operator ==(Object other) =>
      other is PartPlan &&
      other.number == number &&
      other.offset == offset &&
      other.length == length;

  @override
  int get hashCode => Object.hash(number, offset, length);
}

/// The parts of a file of [sizeBytes] with the session's [partBytes].
///
/// Throws an [ArgumentError] for a part size below 1.
List<PartPlan> planParts(int sizeBytes, int partBytes) {
  if (partBytes < 1) throw ArgumentError.value(partBytes, 'partBytes');
  return [
    for (var offset = 0, n = 1; offset < sizeBytes; offset += partBytes, n++)
      PartPlan(
        number: n,
        offset: offset,
        length: offset + partBytes > sizeBytes ? sizeBytes - offset : partBytes,
      ),
  ];
}

/// Reads exactly [part] of the file at [path] (only that range is read from disk).
Future<List<int>> readPart(String path, PartPlan part) async {
  final raf = await File(path).open();
  try {
    await raf.setPosition(part.offset);
    final bytes = await raf.read(part.length);
    if (bytes.length != part.length) {
      throw FileSystemException('the file is shorter than declared', path);
    }
    return bytes;
  } finally {
    await raf.close();
  }
}

/// The `Content-Digest` of [bytes]: `sha-256=:<base64>:` (RFC 9530, 12 3.2).
String contentDigestOf(List<int> bytes) =>
    'sha-256=:${base64.encode(sha256.convert(bytes).bytes)}:';

/// The type the extension of [name] suggests; the server detects the type itself (12 6.1),
/// so this is a hint only.
String guessMimeType(String name) {
  final dot = name.lastIndexOf('.');
  final ext = dot < 0 ? '' : name.substring(dot + 1).toLowerCase();
  return switch (ext) {
    'png' => 'image/png',
    'jpg' || 'jpeg' => 'image/jpeg',
    'gif' => 'image/gif',
    'webp' => 'image/webp',
    'heic' => 'image/heic',
    'm4a' => 'audio/mp4',
    'mp3' => 'audio/mpeg',
    'mp4' => 'video/mp4',
    'mov' => 'video/quicktime',
    'pdf' => 'application/pdf',
    'txt' || 'log' => 'text/plain',
    'md' => 'text/markdown',
    'json' => 'application/json',
    'csv' => 'text/csv',
    'zip' => 'application/zip',
    _ => 'application/octet-stream',
  };
}
