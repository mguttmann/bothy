// SPDX-License-Identifier: Apache-2.0

/// Access to the share inbox (15 6.1 `share_inbox`, 15 12): entries the extensions handed over,
/// kept by every migration and never touched by the cleanup.
library;

import 'dart:convert';

import 'package:domain/domain.dart';
import 'package:drift/drift.dart';

import '../database.dart';
import '../mappers/codec.dart';
import '../models.dart';

/// Data access object of the share inbox.
final class ShareInboxDao {
  /// Creates the DAO on a database; use `LocalDatabase.shareInboxDao`.
  ShareInboxDao(this._db);

  final LocalDatabase _db;

  /// Adds [entry]; an entry with the same ID (taken over twice) is kept as it is.
  Future<void> add(ShareInboxEntry entry) async {
    await _db
        .into(_db.shareInbox)
        .insert(
          ShareInboxCompanion.insert(
            id: entry.id,
            targetThreadId: Value(entry.targetThreadId?.value),
            payload: entry.payload,
            filePaths: jsonEncode(entry.filePaths),
            createdAt: toMillis(entry.createdAt),
          ),
          mode: InsertMode.insertOrIgnore,
        );
  }

  /// All entries, oldest first.
  Future<List<ShareInboxEntry>> all() => _all().get();

  /// [all] on every change.
  Stream<List<ShareInboxEntry>> watchAll() => _all().watch();

  /// Deletes entry [id] (sent through the outbox or discarded).
  Future<void> delete(String id) async {
    await (_db.delete(_db.shareInbox)..where((t) => t.id.equals(id))).go();
  }

  Selectable<ShareInboxEntry> _all() =>
      (_db.select(_db.shareInbox)..orderBy([
            (t) => OrderingTerm(expression: t.createdAt),
            (t) => OrderingTerm(expression: t.id),
          ]))
          .map(
            (row) => ShareInboxEntry(
              id: row.id,
              targetThreadId: optional(row.targetThreadId, ThreadId.parse),
              payload: row.payload,
              filePaths: decodeTexts(row.filePaths, (s) => s),
              createdAt: fromMillis(row.createdAt),
            ),
          );
}
