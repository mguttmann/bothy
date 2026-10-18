// SPDX-License-Identifier: Apache-2.0

/// Access to the sync state (04 7.1, 7.3, 7.5; 15 7.3): a page and its cursor commit in one
/// transaction, the session fields of the gateway, and the emptying of the mirror on a resync.
library;

import 'package:domain/domain.dart';
import 'package:drift/drift.dart';

import '../database.dart';
import '../mappers/codec.dart';
import '../models.dart';

/// The open transaction of [SyncStateDao.commitPage]: every DAO call inside the `apply`
/// callback goes through [db] and so into the same transaction.
final class LocalTxn {
  LocalTxn._(this.db);

  /// The database; its DAOs write inside the transaction while the callback runs.
  final LocalDatabase db;
}

/// Data access object of the one row of `sync_state`.
final class SyncStateDao {
  /// Creates the DAO on a database; use `LocalDatabase.syncStateDao`.
  SyncStateDao(this._db);

  final LocalDatabase _db;

  /// The stored sync state; [SyncState.empty] before the first page.
  Future<SyncState> read() async {
    final row = await _db.select(_db.syncStateTable).getSingleOrNull();
    return row == null ? SyncState.empty : _fromRow(row);
  }

  /// The sync state on every change.
  Stream<SyncState> watch() => _db
      .select(_db.syncStateTable)
      .watchSingleOrNull()
      .map((row) => row == null ? SyncState.empty : _fromRow(row));

  /// Runs [apply] and stores [cursor] (and [syncEpoch] when given) in one transaction (15 7.3
  /// item 1, 04 7.5): when [apply] throws, neither its rows nor the cursor change, and the
  /// error propagates.
  Future<void> commitPage(
    Future<void> Function(LocalTxn txn) apply, {
    required GlobalSeq cursor,
    String? syncEpoch,
  }) => _db.transaction(() async {
    await apply(LocalTxn._(_db));
    await _write(
      SyncStateTableCompanion(
        cursor: Value(cursor.value),
        syncEpoch: syncEpoch == null ? const Value.absent() : Value(syncEpoch),
      ),
    );
  });

  /// Stores the session fields of `ready` (04 6.3) in their own transaction; the cursor and
  /// the epoch stay.
  Future<void> saveSession({
    required String gatewaySessionId,
    required PermissionVersion permissionVersion,
    required GlobalSeq headSeq,
  }) => _write(
    SyncStateTableCompanion(
      gatewaySessionId: Value(gatewaySessionId),
      permissionVersion: Value(permissionVersion.value),
      headSeq: Value(headSeq.value),
    ),
  );

  /// Stores the time the connection was lost; the cursor and the epoch stay.
  Future<void> markDisconnected(DateTime at) =>
      _write(SyncStateTableCompanion(disconnectedAt: Value(toMillis(at))));

  /// Stores the time of the last snapshot (04 7.3).
  Future<void> markSnapshot(DateTime at) =>
      _write(SyncStateTableCompanion(lastSnapshotAt: Value(toMillis(at))));

  /// Removes the cursor and the epoch, so the next connection starts with a resync (15 6.2).
  Future<void> resetCursor() => _write(
    const SyncStateTableCompanion(cursor: Value(null), syncEpoch: Value(null)),
  );

  /// Empties every mirror table, `loaded_ranges` and the search index inside [txn] for the
  /// resync of 04 7.3; keeps `outbox`, `drafts`, `share_inbox`, `sync_state` and the file cache
  /// rows (drafts are personal data the snapshot does not replace).
  Future<void> clearMirror(LocalTxn txn) async {
    for (final table in txn.db.mirrorTables) {
      await txn.db.delete(table).go();
    }
    await txn.db.delete(txn.db.loadedRangesTable).go();
    await txn.db.delete(txn.db.searchIndex).go();
  }

  Future<void> _write(SyncStateTableCompanion values) async {
    await _db.transaction(() async {
      await _db
          .into(_db.syncStateTable)
          .insert(
            const SyncStateTableCompanion(id: Value(1)),
            mode: InsertMode.insertOrIgnore,
          );
      await (_db.update(
        _db.syncStateTable,
      )..where((t) => t.id.equals(1))).write(values);
    });
  }

  static SyncState _fromRow(SyncStateRow row) => SyncState(
    cursor: row.cursor == null ? null : GlobalSeq(row.cursor!),
    syncEpoch: row.syncEpoch,
    headSeq: row.headSeq == null ? null : GlobalSeq(row.headSeq!),
    permissionVersion: row.permissionVersion == null
        ? null
        : PermissionVersion(row.permissionVersion!),
    gatewaySessionId: row.gatewaySessionId,
    disconnectedAt: fromMillisOrNull(row.disconnectedAt),
    lastSnapshotAt: fromMillisOrNull(row.lastSnapshotAt),
  );
}
