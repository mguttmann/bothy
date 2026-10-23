// SPDX-License-Identifier: Apache-2.0

// Widget-test support of files (brief M4-22 Tests): file metadata in every state of 12 2, a
// scripted HTTP adapter for the downloads (no socket: the widget tests fail any real
// request), the providers of the file views and an attachment builder.

import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:api_client/api_client.dart' show ApiClient;
import 'package:client/files/download_service.dart';
import 'package:client/files/file_cache.dart';
import 'package:client/files/file_ports.dart';
import 'package:client/files/providers.dart';
import 'package:client/files/ticket_player.dart';
import 'package:client/files/upload_executor.dart';
import 'package:client/voice/transcript_view.dart';
import 'package:crypto/crypto.dart';
import 'package:dio/dio.dart';
import 'package:domain/domain.dart';
import 'package:flutter_riverpod/misc.dart' show Override;
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';

import 'column_host.dart';
import 'structure.dart';
import 'thread_fixtures.dart';

/// The origin of the scripted server.
final Uri scriptedOrigin = Uri.parse('https://chat.example.com');

/// One answer of the [ScriptedAdapter].
final class ScriptedAnswer {
  /// An answer with [status], [bytes] and [headers].
  const ScriptedAnswer(this.status, this.bytes, [this.headers = const {}]);

  /// `200` with [bytes] and their `Repr-Digest` (a wrong one with [damaged]).
  factory ScriptedAnswer.file(List<int> bytes, {bool damaged = false}) {
    final digest = sha256.convert(damaged ? utf8.encode('other') : bytes);
    return ScriptedAnswer(200, bytes, {
      'repr-digest': ['sha-256=:${base64.encode(digest.bytes)}:'],
      'content-type': ['image/png'],
    });
  }

  /// A problem with [status] and [code] (and [reason]).
  factory ScriptedAnswer.problem(int status, String code, {String? reason}) =>
      ScriptedAnswer(
        status,
        utf8.encode(
          jsonEncode({
            'type': '/api/v1/problems/$code',
            'title': code,
            'status': status,
            'code': code,
            'reason': ?reason,
          }),
        ),
        {
          'content-type': ['application/problem+json'],
        },
      );

  /// A JSON answer [body] with [status].
  factory ScriptedAnswer.json(int status, Object body) =>
      ScriptedAnswer(status, utf8.encode(jsonEncode(body)), {
        'content-type': ['application/json'],
      });

  /// The HTTP status.
  final int status;

  /// The body.
  final List<int> bytes;

  /// The headers.
  final Map<String, List<String>> headers;
}

/// A Dio adapter that answers by path (below `/api/v1`) and records every request.
final class ScriptedAdapter implements HttpClientAdapter {
  /// The answers by path; the last one of a list repeats.
  final Map<String, List<ScriptedAnswer>> answers = {};

  /// The requested paths in order.
  final List<String> requested = [];

  /// Answers [path] with [answer] (after the earlier answers of the path).
  void on(String path, ScriptedAnswer answer) =>
      answers.putIfAbsent(path, () => []).add(answer);

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    final path = options.uri.path.replaceFirst('/api/v1', '');
    requested.add(path);
    final queue = answers[path];
    final answer = queue == null || queue.isEmpty
        ? ScriptedAnswer.problem(404, 'not_found')
        : (queue.length > 1 ? queue.removeAt(0) : queue.first);
    return ResponseBody(
      Stream.value(Uint8List.fromList(answer.bytes)),
      answer.status,
      headers: answer.headers,
    );
  }

  @override
  void close({bool force = false}) {}
}

/// A REST client over [adapter] (no network).
ApiClient scriptedApi(ScriptedAdapter adapter) => ApiClient(
  dio: Dio(BaseOptions(baseUrl: '${scriptedOrigin.origin}/api/v1'))
    ..httpClientAdapter = adapter,
);

/// The ID of file [n].
FileId fileId(int n) => FileId.parse(uuid(0x7000 + n));

/// File [n] named [name] of [mime] in [upload] and [scan] state; an image gets the
/// thumbnails `small` and `medium` unless [thumbnails] is false.
FileMeta fileOf(
  int n, {
  String name = 'report.pdf',
  String mime = 'application/pdf',
  int size = 48213,
  UploadState upload = UploadState.complete,
  ScanStatus scan = ScanStatus.clean,
  bool thumbnails = true,
  TranscriptStatus transcript = TranscriptStatus.none,
  int? durationMs,
  PrincipalId? uploadedBy,
}) => FileMeta(
  id: fileId(n),
  sizeBytes: size,
  mimeType: mime,
  originalName: name,
  uploadedBy: uploadedBy ?? sam,
  purpose: FilePurpose.attachment,
  scopeChannelId: channelId(1),
  uploadState: upload,
  scanStatus: scan,
  transcriptStatus: transcript,
  durationMs: durationMs,
  thumbnails: [
    if (thumbnails && mime.startsWith('image/'))
      for (final v in [ThumbnailVariant.small, ThumbnailVariant.medium])
        Thumbnail(variant: v, mimeType: mime, width: 1, height: 1),
  ],
  createdAt: t0,
);

/// The tombstone of a deleted file [n] (04 2.2 rule 5).
FileMeta deletedFile(int n) => FileMeta(
  id: fileId(n),
  sizeBytes: 0,
  mimeType: 'application/octet-stream',
  originalName: '',
  uploadedBy: sam,
  purpose: FilePurpose.attachment,
  uploadState: UploadState.complete,
  scanStatus: ScanStatus.clean,
  transcriptStatus: TranscriptStatus.none,
  thumbnails: const [],
  createdAt: t0,
);

/// Stores [files] and attaches them to message [seq] of thread 1.
Future<void> attachFiles(
  LocalDatabase db,
  int seq,
  List<FileMeta> files,
) async {
  for (final f in files) {
    await db.messagesDao.putFile(f);
  }
  await db.messagesDao.putAttachments(messageId(seq), [
    for (var i = 0; i < files.length; i++)
      MessageAttachment(
        messageId: messageId(seq),
        fileId: files[i].id,
        position: i + 1,
        threadId: threadId(1),
        channelId: channelId(1),
      ),
  ]);
}

/// A 1 by 1 PNG.
final Uint8List onePixelPng = base64.decode(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==',
);

/// The file services of a widget test over [adapter] with the cache in [root].
final class FileTestServices {
  /// Creates the services.
  FileTestServices(LocalDatabase db, Directory root, this.ports)
    : adapter = ScriptedAdapter() {
    cache = FileCache(db: db, root: Directory('${root.path}/files'));
    downloads = DownloadService(api: scriptedApi(adapter), cache: cache);
    tickets = FileTickets(scriptedApi(adapter), scriptedOrigin);
    transcripts = FileTranscripts(scriptedApi(adapter));
  }

  /// The scripted server.
  final ScriptedAdapter adapter;

  /// The cache.
  late final FileCache cache;

  /// The downloads.
  late final DownloadService downloads;

  /// The tickets.
  late final FileTickets tickets;

  /// The transcripts.
  late final FileTranscripts transcripts;

  /// The ports.
  final FilePorts ports;

  /// The upload progress.
  final UploadProgress progress = UploadProgress();

  /// The provider overrides of the file views.
  List<Override> get overrides => [
    fileCacheProvider.overrideWithValue(cache),
    downloadServiceProvider.overrideWithValue(downloads),
    fileTicketsProvider.overrideWithValue(tickets),
    fileTranscriptsProvider.overrideWithValue(transcripts),
    filePortsProvider.overrideWithValue(ports),
    uploadProgressProvider.overrideWithValue(progress),
    attachmentKeysProvider.overrideWithValue(() => 'attach-${_keys++}'),
  ];

  int _keys = 0;
}

/// Settles [host] until [finder] finds something, at most twenty rounds: the file views wait
/// for the database stream, the provider and the scripted answer in turn.
Future<void> settleUntil(
  WidgetTester tester,
  ColumnHost host,
  Finder finder,
) async {
  for (var i = 0; i < 20 && finder.evaluate().isEmpty; i++) {
    // Dio waits on a timer of the fake clock, so the fake time must move as well.
    await tester.pump(const Duration(milliseconds: 10));
    await host.settle(tester);
  }
}
