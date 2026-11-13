// SPDX-License-Identifier: Apache-2.0

/// "Try again", "Upload again" and "Discard" of a new post that was not created (01 6.6; 04 13.2
/// item 5; FEAT-NEW-CHAT-ATTACH, ruling FNCA/4): the post of the new chat view, of a share or of a
/// refused post in the post list, with the uploads of its files.
///
/// The uploads of a new post belong to it alone, so its actions take them along: "Discard"
/// removes the post and every upload it waits for, deletes their copies on this device and
/// releases on the server what an upload created there (`UploadOps.releaseUpload`: an announced
/// session or a completed file), and "Try again" of a post that waits for a failed upload sends
/// that upload again after releasing the session it ended with. Nothing of a discarded post
/// stays in the outbox or on the server.
library;

import 'dart:io';

import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

import '../files/upload_executor.dart';

/// "Try again" of the post [entryId]: the refused post goes again with a new ID (04 13.2 item
/// 5); a post that waits for a failed upload sends each failed upload again ([retryUpload]),
/// and the post follows once they are confirmed. Does nothing for an unknown entry.
Future<void> retryPost(Outbox outbox, OutboxDao dao, String entryId) async {
  final post = await dao.byId(entryId);
  if (post == null) return;
  if (post.state == OutboxState.failed) {
    await outbox.retry(entryId);
    return;
  }
  for (final up in await _uploadsOf(dao, post)) {
    if (up.state == OutboxState.failed) await retryUpload(outbox, dao, up.id);
  }
}

/// "Upload again" of the failed upload [entryId] of a new post: the session it ended with is
/// released on the server, then the upload goes again with a new entry and a new session (12
/// 3.8 item 1), and the post waits for that one.
Future<void> retryUpload(Outbox outbox, OutboxDao dao, String entryId) async {
  final up = await dao.byId(entryId);
  if (up == null || up.state != OutboxState.failed) return;
  final p = decodePayload(up.payload);
  final fileId = _sessionOf(up, p);
  if (fileId != null) {
    await outbox.enqueue(
      UploadOps.releaseRequest(
        channelId: p['channel_id']! as String,
        key: p['attachment_key']! as String,
        fileId: fileId,
      ),
    );
  }
  await outbox.retry(entryId);
}

/// "Discard" of the post [entryId] before the server created it: the post leaves the outbox,
/// then each upload it waits for, with its copy on this device; for every upload that may have
/// created something on the server (confirmed, attempted, or under way) a `releaseUpload` entry
/// deletes it there. Does nothing for an unknown entry.
Future<void> discardPost(Outbox outbox, OutboxDao dao, String entryId) async {
  final post = await dao.byId(entryId);
  if (post == null) return;
  final p = decodePayload(post.payload);
  final channel = p['channel_id'];
  final keys = [...?(p['attachment_keys'] as List?)?.cast<String>()];
  final uploads = {
    for (final up in await _uploadsOf(dao, post))
      decodePayload(up.payload)['attachment_key']: up,
  };
  // The post first, so nothing is sent for it while its uploads go.
  await outbox.discard(entryId);
  if (channel is! String) return;
  for (final key in keys) {
    // A confirmed upload left its file ID in the post's slot.
    var fileId = p[UploadOps.slotOf(key)] as String?;
    var server = fileId != null;
    final up = uploads[key];
    if (up != null) {
      final payload = decodePayload(up.payload);
      fileId ??= _sessionOf(up, payload);
      // An attempt may have announced a session that the entry has not kept yet; the release
      // runs after it in the uploads' lane and asks the progress of this run.
      server =
          fileId != null || up.attempts > 0 || up.state == OutboxState.sending;
      await outbox.discard(up.id);
      await _deleteCopy(payload['path']);
      // A session an earlier failed entry of this file ended with was released by "Upload
      // again" (retryUpload).
    }
    if (server) {
      await outbox.enqueue(
        UploadOps.releaseRequest(channelId: channel, key: key, fileId: fileId),
      );
    }
  }
}

/// The upload entries [post] waits for.
Future<List<OutboxEntry>> _uploadsOf(OutboxDao dao, OutboxEntry post) async {
  final ids = post.dependsOn?.split(',') ?? const <String>[];
  return [
    for (final id in ids)
      if (await dao.byId(id) case final up? when up.kind == UploadOps.upload)
        up,
  ];
}

/// The file ID of the session [up] kept for itself ([UploadOps.sessionMember]), or null; a
/// session copied from the failed entry it replaced is that entry's, not its own.
String? _sessionOf(OutboxEntry up, Map<String, Object?> payload) {
  final kept = payload[UploadOps.sessionMember];
  if (kept is Map && kept['entry'] == up.id && kept['file_id'] is String) {
    return kept['file_id']! as String;
  }
  return null;
}

/// Deletes the copy at [path] that an upload would have read, with its folder of the attachment
/// key (`outgoing/<key>/`, D-M4-22-5).
Future<void> _deleteCopy(Object? path) async {
  if (path is! String) return;
  final file = File(path);
  try {
    if (file.existsSync()) await file.delete();
    final dir = file.parent;
    if (dir.existsSync() && dir.listSync().isEmpty) await dir.delete();
  } on FileSystemException {
    // A copy that cannot be deleted now goes with the instance folder (D-M4-22-5).
  }
}
