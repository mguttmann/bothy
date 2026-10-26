// SPDX-License-Identifier: Apache-2.0

/// The takeover of the share inbox of the App Group into the app's outbox (brief M5-06 Design
/// note 5; 15 6.1 `share_inbox`, 7.5; 04 13.2).
library;

import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';

import '../src/generated/share_apple_api.g.dart';
import 'share_outbox.dart';
import 'share_target.dart';

/// Takes the entries the Share Extension could not send into the outbox, each with the
/// extension's `client_msg_id`.
///
/// [run] reads the inbox of the App Group and, for every entry of the signed-in instance:
/// when `share_inbox` has no row of it, hands it to [ShareOutbox] (the files are copied into
/// the outbox's own folder, the uploads and the message or post are written with IDs that
/// follow from the `client_msg_id`), writes the `share_inbox` row and then deletes the entry's
/// folder in the App Group. A row that exists already means the outbox took the files before
/// (the row is written only after the entries), so only the folder goes. Entries of another
/// instance, malformed entries and entries whose files cannot be copied stay for a later run.
/// A replay the server answers with the original message confirms the entry like a first
/// send (`2xx`, 04 5.1).
///
/// [prune] deletes the rows whose outbox entry is gone: confirmed, or discarded by the person.
/// A failed entry keeps its row until the person sends it again or discards it.
///
/// Example: `await ShareTakeover(host: host, instanceId: id, outbox: shares, db: db).run();`
final class ShareTakeover {
  /// Creates the takeover of [instanceId] from [host] into [outbox] and the `share_inbox` of
  /// [db].
  ShareTakeover({
    required this._host,
    required this._instanceId,
    required this._outbox,
    required this._db,
  });

  final ShareAppleHostApi _host;
  final String _instanceId;
  final ShareOutbox _outbox;
  final LocalDatabase _db;
  Future<int>? _running;

  /// Takes every entry of the inbox over and returns how many went into the outbox now; a
  /// call while one runs waits for it. A failing host gives 0.
  Future<int> run() => _running ??= _run().whenComplete(() => _running = null);

  Future<int> _run() async {
    final List<ShareInboxEntryRecord> entries;
    try {
      entries = await _host.readInbox();
    } on Object {
      // A build without the App Group, or a broken channel: nothing to take now.
      return 0;
    }
    final taken = {for (final row in await _db.shareInboxDao.all()) row.id};
    var count = 0;
    for (final entry in entries) {
      if (entry.instanceId != _instanceId) continue;
      final ShareDraft draft;
      try {
        draft = draftOf(entry);
      } on FormatException {
        // Nothing the app could send; the entry stays where the extension wrote it.
        continue;
      }
      if (!taken.contains(draft.clientMsgId)) {
        final List<String> copies;
        try {
          copies = await _outbox.enqueue(draft);
        } on FileSystemException {
          // The files stay in the App Group; the next start tries again.
          continue;
        }
        await _db.shareInboxDao.add(
          ShareInboxEntry(
            id: draft.clientMsgId,
            targetThreadId: draft.target.threadId,
            payload: jsonEncode({
              ...draft.target.toJson(),
              'comment': draft.comment,
              'title': ?draft.title,
            }),
            filePaths: copies,
            createdAt: DateTime.fromMillisecondsSinceEpoch(
              entry.createdAtMillis,
              isUtc: true,
            ),
          ),
        );
        count++;
      }
      try {
        await _host.deleteInboxEntry(draft.clientMsgId);
      } on Object {
        // The row exists, so the next run only deletes the folder.
      }
    }
    return count;
  }

  /// Deletes the `share_inbox` rows whose outbox entry is confirmed or discarded.
  Future<void> prune() async {
    for (final row in await _db.shareInboxDao.all()) {
      if (await _db.outboxDao.byId(row.id) == null) {
        await _db.shareInboxDao.delete(row.id);
      }
    }
  }

  /// The draft of the inbox entry [record]; throws a [FormatException] for an ID that is no
  /// UUID or a target that does not fit its kind.
  static ShareDraft draftOf(ShareInboxEntryRecord record) {
    final target = ShareTarget.fromRecord(record.target);
    return ShareDraft(
      clientMsgId: MessageId.parse(record.clientMsgId).value,
      target: target,
      comment: record.comment,
      title: target.kind == ShareTargetKind.newPost ? record.title : null,
      files: [
        for (final f in record.files)
          ShareFile(
            path: f.path,
            name: f.name,
            mimeType: f.mimeType,
            sizeBytes: f.sizeBytes,
            completedFileId: switch (f.completedFileId) {
              final String id => FileId.parse(id),
              null => null,
            },
          ),
      ],
    );
  }
}
