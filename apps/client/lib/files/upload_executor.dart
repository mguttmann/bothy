// SPDX-License-Identifier: Apache-2.0

/// The outbox executor `upload` (12 3; 15 7.5, 15 14; brief M4-22 Design notes 1 and 2) and
/// the attaching of its file to the message that waits for it.
///
/// An upload entry carries the local path, name, size, SHA-256 and declared type. Its
/// executor creates the session with `Idempotency-Key` = entry ID (a retry gets the same
/// session), reads the parts `getUpload` confirmed, sends the others, at most two at a time,
/// each with `Content-Digest` and read alone from disk, and completes the session. The entry
/// is confirmed when `completeUpload` answered `200` (12 3.4), which releases the message;
/// the file's states then follow `file.updated`.
library;

import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:api_client/api_client.dart' as wire show File;
import 'package:api_client/api_client.dart' hide File;
import 'package:crypto/crypto.dart';
import 'package:dio/dio.dart';
import 'package:domain/domain.dart';
import 'package:flutter/foundation.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

import 'upload_planner.dart';

/// The progress of the uploads of this app run, by attachment key: the bytes the server
/// confirmed and the file ID once the session exists. A tile listens to it; nothing here is
/// stored, the outbox entry is the record.
final class UploadProgress extends ChangeNotifier {
  final Map<String, int> _confirmed = {};
  final Map<String, int> _total = {};
  final Map<String, String> _fileIds = {};
  final Map<String, String> _completed = {};

  /// The share of confirmed bytes of [key] from 0 to 1, or null before its first attempt.
  double? fractionOf(String key) {
    final total = _total[key];
    if (total == null || total == 0) return null;
    return (_confirmed[key] ?? 0) / total;
  }

  /// The file ID of the completed upload of [key], or null.
  String? completedFileOf(String key) => _completed[key];

  /// The file ID of the session of [key], or null before the session exists.
  String? sessionFileOf(String key) => _fileIds[key];

  void _begin(String key, int total) {
    _total[key] = total;
    _confirmed[key] = 0;
    notifyListeners();
  }

  void _session(String key, String fileId, int confirmed) {
    _fileIds[key] = fileId;
    _confirmed[key] = confirmed;
    notifyListeners();
  }

  void _add(String key, int bytes) {
    _confirmed[key] = (_confirmed[key] ?? 0) + bytes;
    notifyListeners();
  }

  void _complete(String key, String fileId) {
    _completed[key] = fileId;
    _confirmed[key] = _total[key] ?? 0;
    notifyListeners();
  }
}

/// The upload operation of the outbox and the payload members of a message with attachments.
///
/// | Kind | Payload | Lane |
/// |---|---|---|
/// | `upload` | `channel_id`, `path`, `name`, `size_bytes`, `sha256`, `declared_mime_type`, `attachment_key` | `upload:<entry id>` |
///
/// A message with attachments names its uploads in `depends_on` and holds
/// `attachment_keys` (the keys in their order), `attachment_preview` (name and size for the
/// row before it is sent) and one slot `file:<key>` per attachment: the file ID, or null until
/// the upload is confirmed and the outbox fills it (D-M4-15-6). [AttachmentMessageOps]
/// turns them into `attachment_file_ids`.
///
/// Example: `UploadOps.register(executors, api, progress, baseUrl: instance.address);`
abstract final class UploadOps {
  /// The kind of an upload entry.
  static const String upload = 'upload';

  /// Parts in flight at the same time (12 3.2 item 4: the client chooses; D-M4-22-1).
  static const int parallelParts = 2;

  /// The payload member of a message that holds the file ID of attachment [key].
  static String slotOf(String key) => 'file:$key';

  /// The upload entry of [source] in [threadId] of [channelId] for the attachment [key]; the
  /// entry shows in the thread's outbox, but sends in its own lane so a long upload never
  /// holds the other writes of the thread. [owned] deletes the local file after the upload.
  static OutboxRequest request({
    required ThreadId threadId,
    required ChannelId channelId,
    required UploadSource source,
    required String key,
    bool owned = false,
  }) => OutboxRequest(
    kind: upload,
    threadId: threadId,
    payload: {
      'channel_id': channelId.value,
      'path': source.path,
      'name': source.name,
      'size_bytes': source.sizeBytes,
      'sha256': source.sha256,
      'declared_mime_type': source.mimeType,
      'attachment_key': key,
      // A file the app wrote (a recording, a pasted image) goes after its upload.
      if (owned) 'delete_after': true,
    },
  );

  /// Registers `upload` on [registry] with the REST client [api]; [progress] receives the
  /// confirmed bytes. Parts of a `presigned` session go to their URLs (resolved against
  /// [baseUrl]) through [objectStore], a client without the authentication of [api], because
  /// the object store must never see the token (12 3.3).
  static void register(
    OutboxExecutors registry,
    ApiClient api,
    UploadProgress progress, {
    required Uri baseUrl,
    Dio? objectStore,
  }) {
    final store = objectStore ?? Dio();
    registry.register(
      upload,
      (e, ctx) => _send(api, store, baseUrl, progress, e, ctx),
      lane: (e) => 'upload:${e.id}',
    );
  }

  static Future<OutboxResponse> _send(
    ApiClient api,
    Dio store,
    Uri baseUrl,
    UploadProgress progress,
    OutboxEntry e,
    ApiCallContext ctx,
  ) async {
    final p = decodePayload(e.payload);
    final key = p['attachment_key']! as String;
    final path = p['path']! as String;
    final size = p['size_bytes']! as int;
    final local = File(path);
    final files = api.getFilesApi();
    progress._begin(key, size);
    final session = (await files.createUpload(
      channelId: p['channel_id']! as String,
      idempotencyKey: ctx.idempotencyKey,
      uploadCreate: UploadCreate(
        (b) => b
          ..name = p['name']! as String
          ..sizeBytes = size
          ..sha256 = p['sha256']! as String
          ..declaredMimeType = p['declared_mime_type'] as String?,
      ),
    )).data!;
    final fileId = session.fileId;
    // 12 3.2 item 5: resume from what the server confirmed after an interruption.
    final state = (await files.getUpload(fileId: fileId)).data!;
    final etags = <int, String>{
      for (final c in state.completedParts) c.partNumber: c.etag,
    };
    final plan = planParts(size, session.partSizeBytes);
    progress._session(
      key,
      fileId,
      [
        for (final part in plan)
          if (etags.containsKey(part.number)) part.length,
      ].fold(0, (a, b) => a + b),
    );
    if (state.uploadState != UploadStateUploadStateEnum.complete) {
      final todo = [
        for (final part in plan)
          if (!etags.containsKey(part.number)) part,
      ];
      // The local file matters only while parts are missing: a session the server completed
      // (the app quit after `completeUpload`, which deleted an owned copy) completes again.
      try {
        if (todo.isNotEmpty &&
            (!local.existsSync() || local.lengthSync() != size)) {
          throw const FileSystemException('changed since it was attached');
        }
      } on FileSystemException {
        // Nothing to resume from: a final failure, never a retry (a file error is an
        // IOException, which the outbox would retry).
        throw _unreadable;
      }
      final targets = {for (final t in session.parts) t.partNumber: t};
      var next = 0;
      var stopped = false;
      Future<void> worker() async {
        while (!stopped && next < todo.length) {
          final part = todo[next++];
          final List<int> bytes;
          try {
            bytes = await readPart(path, part);
          } on FileSystemException {
            throw _unreadable;
          }
          final digest = contentDigestOf(bytes);
          etags[part.number] = session.mode == UploadSessionModeEnum.presigned
              ? await _putPresigned(
                  store,
                  baseUrl,
                  targets[part.number]!,
                  bytes,
                )
              : (await files.uploadPart(
                  fileId: fileId,
                  partNumber: part.number,
                  contentDigest: digest,
                  body: MultipartFile.fromBytes(bytes),
                  headers: {Headers.contentLengthHeader: bytes.length},
                )).data!.etag;
          progress._add(key, part.length);
        }
      }

      Future<void> guarded() =>
          worker().catchError((Object error, StackTrace stack) {
            // The other worker stops after its part; the outbox retries the entry and
            // resumes from the parts the server confirmed.
            stopped = true;
            Error.throwWithStackTrace(error, stack);
          });

      await Future.wait([
        for (var i = 0; i < UploadOps.parallelParts && i < todo.length; i++)
          guarded(),
      ]);
    }
    final Response<wire.File> done;
    if (state.uploadState == UploadStateUploadStateEnum.complete) {
      // Completed already (an earlier attempt's answer was lost): the file as it is now. A
      // second completion would send other parts under the same key (422).
      done = await files.getFile(fileId: fileId);
    } else {
      final numbers = etags.keys.toList()..sort();
      done = await files.completeUpload(
        fileId: fileId,
        idempotencyKey: completionKey(ctx.idempotencyKey),
        uploadComplete: UploadComplete(
          (b) => b.parts.replace([
            for (final n in numbers)
              UploadPartResult(
                (r) => r
                  ..partNumber = n
                  ..etag = etags[n],
              ),
          ]),
        ),
      );
    }
    progress._complete(key, fileId);
    if (p['delete_after'] == true) {
      try {
        await local.delete();
      } on FileSystemException {
        // The wipe of the instance folder takes it later.
      }
    }
    return OutboxResponse(
      status: done.statusCode,
      body: done.data,
      result: {slotOf(key): fileId},
    );
  }

  static const ApiProblem _unreadable = ApiProblem(
    status: 400,
    code: 'file_unreadable',
  );

  static Future<String> _putPresigned(
    Dio store,
    Uri baseUrl,
    UploadPart target,
    List<int> bytes,
  ) async {
    final response = await store.putUri<void>(
      baseUrl.resolve(target.url),
      data: Stream<List<int>>.value(bytes),
      options: Options(
        headers: {
          for (final h in target.headers) h.name: h.value,
          Headers.contentLengthHeader: bytes.length,
        },
      ),
    );
    final etag = response.headers.value('etag');
    if (etag == null) {
      throw DioException.badResponse(
        statusCode: response.statusCode ?? 0,
        requestOptions: response.requestOptions,
        response: response,
      );
    }
    return etag;
  }

  /// The `Idempotency-Key` of the completion of the upload entry [entryId]: a UUIDv7 with the
  /// entry's time and bits derived from it, so every attempt of the entry repeats the same
  /// completion while it differs from the key of `createUpload` (04 5: one key per request).
  static String completionKey(String entryId) {
    final hex = sha256
        .convert(utf8.encode('$entryId/complete'))
        .toString()
        .substring(0, 20);
    final time = entryId.replaceAll('-', '').substring(0, 12);
    final variant = '89ab'[int.parse(hex[3], radix: 16) % 4];
    return '${time.substring(0, 8)}-${time.substring(8, 12)}-7${hex.substring(0, 3)}-'
        '$variant${hex.substring(4, 7)}-${hex.substring(8, 20)}';
  }
}

/// The attachments a message takes along: its upload entries and the payload members of
/// [UploadOps].
final class MessageAttachments {
  /// Creates the attachments of the upload entries [uploads] for the keys [keys] with the
  /// file IDs known now ([fileIds], by key) and the preview [preview] (name and size by key).
  const MessageAttachments({
    required this.keys,
    required this.uploads,
    this.fileIds = const {},
    this.preview = const [],
  });

  /// The attachment keys in their order.
  final List<String> keys;

  /// The upload entries the message waits for.
  final List<String> uploads;

  /// The file IDs of uploads confirmed already, by key.
  final Map<String, String> fileIds;

  /// Name and size of each attachment, for the row while the message waits.
  final List<Map<String, Object?>> preview;

  /// The payload members of a message with these attachments.
  Map<String, Object?> get payload => {
    'attachment_keys': keys,
    'attachment_preview': preview,
    for (final k in keys) UploadOps.slotOf(k): fileIds[k],
  };
}

/// Wraps the executor `createMessage` so a message with attachments sends
/// `attachment_file_ids` (12 10) built from its slots, and none of the local members.
///
/// A slot still null when the message is sent means its upload was confirmed before the
/// message was written; the file ID then comes from [UploadProgress], else the message fails
/// with `executor_error` and the person sends it again.
abstract final class AttachmentMessageOps {
  /// Replaces `createMessage` of [registry] (registered by `MessageOps`) with the wrapped one.
  static void wrap(OutboxExecutors registry, UploadProgress progress) {
    final inner = registry.lookup(MessageOps.createMessage);
    if (inner == null) throw StateError('createMessage is not registered');
    registry.register(
      MessageOps.createMessage,
      (e, ctx) => inner.send(withFileIds(e, progress), ctx),
      lane: inner.lane,
      followUpEdit: inner.followUpEdit,
      followUpRemove: inner.followUpRemove,
    );
  }

  /// [entry] with `attachment_file_ids` from its slots and without the local members.
  ///
  /// Throws a [StateError] for a slot without a file ID.
  static OutboxEntry withFileIds(OutboxEntry entry, UploadProgress progress) {
    final p = decodePayload(entry.payload);
    final keys = (p.remove('attachment_keys') as List?)?.cast<String>();
    p.remove('attachment_preview');
    if (keys == null) return entry;
    p['attachment_file_ids'] = [
      for (final k in keys)
        (p.remove(UploadOps.slotOf(k)) as String?) ??
            progress.completedFileOf(k) ??
            (throw StateError('the upload of attachment $k has no file')),
    ];
    return OutboxEntry(
      id: entry.id,
      kind: entry.kind,
      threadId: entry.threadId,
      dependsOn: entry.dependsOn,
      payload: jsonEncode(p),
      expectedVersion: entry.expectedVersion,
      state: entry.state,
      attempts: entry.attempts,
      nextAttemptAt: entry.nextAttemptAt,
      lastErrorCode: entry.lastErrorCode,
      lastErrorReason: entry.lastErrorReason,
      conflictCurrent: entry.conflictCurrent,
      createdAt: entry.createdAt,
      stateChangedAt: entry.stateChangedAt,
    );
  }
}
