// SPDX-License-Identifier: Apache-2.0

/// The one send path of a share in the app: its files become upload entries of the outbox,
/// then the message or the new forum post follows with the share's `client_msg_id` (brief
/// M5-06 Design notes 5 and 6; 15 7.5: uploads before the message; 04 5.1).
library;

import 'dart:convert';
import 'dart:io';

import 'package:crypto/crypto.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

import '../files/file_cache.dart';
import '../files/upload_executor.dart';
import '../files/upload_planner.dart';
import '../sync/thread_ops.dart';
import '../thread/thread_actions.dart';
import 'share_target.dart';

/// Puts a [ShareDraft] into the outbox, idempotent per `client_msg_id`.
///
/// Every file without a completed upload is copied into the outbox's own folder first
/// ([outgoing], `<instance folder>/outgoing/<attachment key>/`, as `AttachController` does),
/// so the extension's App Group folder or the runner's cache copy can go right after; the
/// upload reads only the copy and deletes it when done. All copies are made before the first
/// entry is written, so the entries follow each other at once. The upload entries take IDs
/// derived from the `client_msg_id` ([uploadIdOf]) and the message the `client_msg_id`
/// itself; an entry that exists already is not written again, so a takeover that stopped
/// half way and runs again sends nothing twice. A completed upload of the extension is
/// attached by its file ID without a new upload.
///
/// Example:
/// ```dart
/// final shares = ShareOutbox(outbox: engine.outbox, entries: db.outboxDao,
///     outgoing: () async => outgoingFolder);
/// await shares.enqueue(draft);
/// ```
final class ShareOutbox {
  /// Creates the path into [outbox]; [entries] is the outbox table of the same database.
  ShareOutbox({
    required this._outbox,
    required this._entries,
    required this._outgoing,
  });

  final Outbox _outbox;
  final OutboxDao _entries;
  final Future<Directory> Function() _outgoing;

  /// Writes the upload entries and the message (or post) of [draft] and returns the paths of
  /// the outbox's own copies, in the order of the files that need an upload.
  ///
  /// Throws a [FileSystemException] when a file cannot be copied; then no entry is written and
  /// the caller keeps the share for a later attempt.
  Future<List<String>> enqueue(ShareDraft draft) async {
    final target = draft.target;
    final keys = <String>[];
    final fileIds = <String, String>{};
    final preview = <Map<String, Object?>>[];
    final copies = <String, UploadSource>{};
    final folder = await _outgoing();
    for (var i = 0; i < draft.files.length; i++) {
      final file = draft.files[i];
      final key = uploadIdOf(draft.clientMsgId, i);
      keys.add(key);
      preview.add({
        'key': key,
        'name': file.name,
        'size_bytes': file.sizeBytes,
      });
      final done = file.completedFileId;
      if (done != null) {
        fileIds[key] = done.value;
        continue;
      }
      final dir = Directory('${folder.path}${Platform.pathSeparator}$key');
      final copy = File(
        '${dir.path}${Platform.pathSeparator}${FileCache.safeFileName(file.name)}',
      );
      if (await _entries.byId(key) == null || !copy.existsSync()) {
        await dir.create(recursive: true);
        await File(file.path).copy(copy.path);
      }
      copies[key] = await prepareUpload(
        copy.path,
        name: file.name,
        mimeType: file.mimeType,
      );
    }
    for (final MapEntry(key: key, value: source) in copies.entries) {
      if (await _entries.byId(key) != null) continue;
      await _outbox.enqueue(
        UploadOps.request(
          threadId: target.threadId,
          channelId: target.channelId,
          source: source,
          key: key,
          owned: true,
          id: key,
        ),
      );
    }
    if (await _entries.byId(draft.clientMsgId) == null) {
      final attachments = keys.isEmpty
          ? null
          : MessageAttachments(
              keys: keys,
              uploads: [...copies.keys],
              fileIds: fileIds,
              preview: preview,
            );
      final thread = target.threadId;
      await _outbox.enqueue(
        thread == null
            ? ThreadOps.postRequest(
                forum: target.channelId,
                title: draft.title ?? '',
                contentMd: draft.comment,
                attachments: attachments,
                id: draft.clientMsgId,
              )
            // A share is not the draft the person types in that thread: the draft stays
            // (04 14).
            : ThreadActions.messageRequest(
                thread,
                draft.comment,
                attachments: attachments,
                id: draft.clientMsgId,
                clearDraft: false,
              ),
      );
    }
    return [for (final s in copies.values) s.path];
  }

  /// The ID of the upload entry of file [index] of the share [clientMsgId]: a UUIDv7 with the
  /// time of the `client_msg_id` and bits derived from it, so every takeover of the same
  /// share writes the same entries (and the same `Idempotency-Key` of `createUpload`).
  static String uploadIdOf(String clientMsgId, int index) {
    final hex = sha256
        .convert(utf8.encode('$clientMsgId/upload/$index'))
        .toString()
        .substring(0, 20);
    final time = clientMsgId.replaceAll('-', '').substring(0, 12);
    final variant = '89ab'[int.parse(hex[3], radix: 16) % 4];
    return '${time.substring(0, 8)}-${time.substring(8, 12)}-7${hex.substring(0, 3)}-'
        '$variant${hex.substring(4, 7)}-${hex.substring(8, 20)}';
  }
}
