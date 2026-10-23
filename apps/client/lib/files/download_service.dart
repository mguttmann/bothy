// SPDX-License-Identifier: Apache-2.0

/// Downloads through the server only (12 9; SEC-OUT-2, T-AI-06; brief M4-22 Design note 3):
/// the bytes of a file or a thumbnail stream into the file cache while their SHA-256 is
/// computed, and only bytes whose digest equals `Repr-Digest` are kept; `409 file_not_ready`
/// and `409 file_blocked` become their own results. Saving copies the checked bytes to a place
/// the person chooses, opening hands them to the system.
library;

import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:api_client/api_client.dart' hide File;
import 'package:crypto/crypto.dart';
import 'package:dio/dio.dart';
import 'package:domain/domain.dart';

import 'file_cache.dart';
import 'file_ports.dart';

/// The outcome of a download.
sealed class DownloadResult {
  const DownloadResult();
}

/// The checked bytes lie in [file].
final class Downloaded extends DownloadResult {
  /// Creates the result.
  const Downloaded(this.file);

  /// The cached file (or the saved copy of [DownloadService.saveAs]).
  final File file;
}

/// `409 file_not_ready`: the server still checks the file ("being checked", 12 2).
final class DownloadNotReady extends DownloadResult {
  /// Creates the result.
  const DownloadNotReady();
}

/// `409 file_blocked`: blocked or failed (12 2).
final class DownloadBlocked extends DownloadResult {
  /// Creates the result.
  const DownloadBlocked();
}

/// The received bytes did not match `Repr-Digest`; they were deleted (12 9.3).
final class DownloadDamaged extends DownloadResult {
  /// Creates the result.
  const DownloadDamaged();
}

/// The person cancelled the save dialog.
final class DownloadCancelled extends DownloadResult {
  /// Creates the result.
  const DownloadCancelled();
}

/// Any other failure: the problem [code] (`not_found`, `network`, ...) and its [reason].
final class DownloadFailed extends DownloadResult {
  /// Creates the result.
  const DownloadFailed(this.code, {this.reason});

  /// The problem code, or a client code.
  final String code;

  /// The `reason` member, if any.
  final String? reason;
}

/// Fetches files and thumbnails from the server into the [FileCache].
///
/// Example:
/// ```dart
/// final result = await downloads.fetch(file.id, variant: 'small');
/// if (result case Downloaded(:final file)) showImage(file);
/// ```
final class DownloadService {
  /// Creates the service on the REST client [api] (with the DPoP interceptors of `auth`) and
  /// [cache].
  DownloadService({required ApiClient api, required this.cache})
    : _api = api; // ignore: prefer_initializing_formals

  final ApiClient _api;

  /// Where the checked bytes go.
  final FileCache cache;

  final Map<String, Future<DownloadResult>> _running = {};

  /// The bytes of [variant] (`original`, `small`, `medium`, `large`) of [fileId]: from the
  /// cache, else from the server. A second call for the same variant while one runs joins it.
  Future<DownloadResult> fetch(
    FileId fileId, {
    String variant = 'original',
    String? name,
  }) {
    final key = '${fileId.value}/$variant';
    return _running[key] ??= _fetch(fileId, variant, name).whenComplete(() {
      // A block body: the removed future must not become the result of the callback.
      _running.remove(key);
    });
  }

  Future<DownloadResult> _fetch(
    FileId fileId,
    String variant,
    String? name,
  ) async {
    try {
      return await _download(fileId, variant, name);
    } on FileSystemException {
      // The cache folder cannot be written (a full disk): no bytes, and the caller shows why.
      return const DownloadFailed('cache_failed');
    }
  }

  Future<DownloadResult> _download(
    FileId fileId,
    String variant,
    String? name,
  ) async {
    final hit = await cache.lookup(fileId, variant);
    if (hit != null) return Downloaded(hit);
    final path = variant == 'original'
        ? '/files/${fileId.value}/content'
        : '/files/${fileId.value}/thumbnails/$variant';
    Response<ResponseBody> response;
    try {
      response = await _api.dio.get<ResponseBody>(
        path,
        options: Options(
          responseType: ResponseType.stream,
          // The security marker of the generated operations, so the auth interceptors of
          // M4-09 sign this request with DPoP (never a ticket, 12 9.2).
          extra: {
            'secure': [
              {'type': 'http', 'scheme': 'DPoP', 'name': 'dpopAuth'},
              {'type': 'http', 'scheme': 'bearer', 'name': 'bearerAuth'},
            ],
          },
        ),
      );
    } on DioException catch (e) {
      // Errors pass the interceptors of `auth` first (DPoP nonce, refresh, revocation); a
      // binary operation then still carries its problem as a stream.
      final failed = e.response;
      if (failed == null) return const DownloadFailed('network');
      final data = failed.data;
      final problem = data is ResponseBody
          ? await _problemOf(data)
          : const <String, Object?>{};
      final mapped = ApiProblem.of(e);
      return _failure(
        problem['code'] as String? ??
            mapped?.code ??
            'http_${failed.statusCode ?? 0}',
        problem['reason'] as String? ?? mapped?.reason,
      );
    }
    final body = response.data;
    if (response.statusCode != 200 || body == null) {
      return DownloadFailed('http_${response.statusCode ?? 0}');
    }
    final expected = _reprDigest(response.headers.value('repr-digest'));
    final partial = await cache.partial(fileId, variant);
    final sink = partial.openWrite();
    final digest = _DigestSink();
    final hasher = sha256.startChunkedConversion(digest);
    var failed = false;
    try {
      await for (final chunk in body.stream) {
        hasher.add(chunk);
        sink.add(chunk);
      }
      hasher.close();
    } on Object {
      failed = true;
    }
    // The sink is closed exactly once; a failed close (a full disk) fails the download too.
    try {
      await sink.close();
    } on Object {
      failed = true;
    }
    if (failed) {
      if (partial.existsSync()) await partial.delete();
      return const DownloadFailed('network');
    }
    if (expected == null || !_equal(expected, digest.value.bytes)) {
      // 12 9.3: the client checks the digest after the download; damaged bytes never stay.
      await partial.delete();
      return const DownloadDamaged();
    }
    return Downloaded(await cache.store(fileId, variant, partial, name: name));
  }

  /// Fetches [file] and copies it to the place the person chooses in [picker].
  Future<DownloadResult> saveAs(FileMeta file, FilePicker picker) async {
    final target = await picker.saveLocation(file.originalName);
    if (target == null) return const DownloadCancelled();
    final result = await fetch(file.id, name: file.originalName);
    if (result is! Downloaded) return result;
    try {
      return Downloaded(await result.file.copy(target));
    } on FileSystemException {
      // A folder that went away, a locked file, a full disk or a read-only volume.
      return const DownloadFailed('save_failed');
    }
  }

  /// Fetches [file] and opens it with the system through [opener]; a program or a script
  /// (by the extension of its name) is refused with `open_refused` and never fetched or
  /// launched, Save as stays (D-M4-22-14).
  Future<DownloadResult> open(FileMeta file, FileOpener opener) async {
    if (isExecutableName(file.originalName)) {
      return const DownloadFailed('open_refused');
    }
    final result = await fetch(file.id, name: file.originalName);
    if (result is! Downloaded) return result;
    try {
      if (!await opener.open(result.file.path)) {
        return const DownloadFailed('open_failed');
      }
    } on Object {
      return const DownloadFailed('open_failed');
    }
    return result;
  }

  static DownloadResult _failure(String code, String? reason) => switch (code) {
    'file_not_ready' => const DownloadNotReady(),
    'file_blocked' => const DownloadBlocked(),
    _ => DownloadFailed(code, reason: reason),
  };

  /// The problem of an error response; a binary operation sends it as bytes.
  static Future<Map<String, Object?>> _problemOf(ResponseBody? body) async {
    if (body == null) return const {};
    try {
      final bytes = await body.stream.fold<List<int>>(
        [],
        (all, chunk) => all..addAll(chunk),
      );
      final json = jsonDecode(utf8.decode(bytes));
      return json is Map<String, Object?> ? json : const {};
    } on Object {
      return const {};
    }
  }

  /// The `sha-256` member of a `Repr-Digest` header (RFC 9530), or null.
  static List<int>? _reprDigest(String? header) {
    if (header == null) return null;
    final match = RegExp(r'sha-256=:([A-Za-z0-9+/=]+):').firstMatch(header);
    if (match == null) return null;
    try {
      return base64.decode(match[1]!);
    } on FormatException {
      return null;
    }
  }

  static bool _equal(List<int> a, List<int> b) {
    if (a.length != b.length) return false;
    var diff = 0;
    for (var i = 0; i < a.length; i++) {
      diff |= a[i] ^ b[i];
    }
    return diff == 0;
  }
}

final class _DigestSink implements Sink<Digest> {
  late Digest value;

  @override
  void add(Digest data) => value = data;

  @override
  void close() {}
}

/// The extensions the desktop systems run as a program or a script when a file is opened.
const Set<String> executableExtensions = {
  'app',
  'appimage',
  'apk',
  'application',
  'appref-ms',
  'chm',
  'bat',
  'cmd',
  'com',
  'command',
  'cpl',
  'csh',
  'deb',
  'desktop',
  'dmg',
  'exe',
  'gadget',
  'hta',
  'inf',
  'ins',
  'jar',
  'js',
  'jse',
  'ksh',
  'lnk',
  'msc',
  'msi',
  'msix',
  'msp',
  'pif',
  'pkg',
  'ps1',
  'psm1',
  'py',
  'rb',
  'reg',
  'rpm',
  'run',
  'scf',
  'scr',
  'sct',
  'settingcontent-ms',
  'sh',
  'shb',
  'terminal',
  'tool',
  'url',
  'vb',
  'vbe',
  'vbs',
  'website',
  'workflow',
  'ws',
  'wsc',
  'wsf',
  'wsh',
  'xll',
  'zsh',
};

/// Whether [name] ends with one of [executableExtensions], in any case, after the trailing
/// dots and spaces that the cache and the systems drop ([FileCache.safeFileName]).
bool isExecutableName(String name) {
  final cleaned = FileCache.safeFileName(name);
  final dot = cleaned.lastIndexOf('.');
  if (dot < 0) return false;
  return executableExtensions.contains(
    cleaned.substring(dot + 1).toLowerCase(),
  );
}
