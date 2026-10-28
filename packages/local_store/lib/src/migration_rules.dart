// SPDX-License-Identifier: Apache-2.0

/// The migration rule of docs/spec/15-client-architecture.md 6.2 as a helper for the steps of
/// `stepByStep`: the cache may be discarded and reloaded on any step, unsent work never.
library;

import 'package:drift/drift.dart';

/// Building blocks for a migration step of the local database (docs/DECISIONS.md D-M4-12-6).
///
/// A step either changes a mirror table by [discardCache] (drop, recreate, reset the cursor so
/// the next connection does a resync) or a preserved table by [keepRows] (drift's table
/// migration copies every row into the new layout). `test/drift/survival_test.dart` checks
/// every pair of stored schema versions for loss in [preservedTables].
abstract final class MigrationRules {
  /// Tables whose rows survive every migration: unsent work (`outbox`, `drafts`),
  /// handed-over shares (`share_inbox`) and the pinned host keys (`host_key_pins`, M4-36), which
  /// are trust anchors: a lost pin would prevent the warning on a changed key (15 6.2).
  static const Set<String> preservedTables = {
    'outbox',
    'drafts',
    'share_inbox',
    'host_key_pins',
  };

  /// Drops and recreates [cacheTables] and clears `sync_state.cursor` and `sync_epoch`, so the
  /// next connection loads a snapshot (04 7.3). Throws an [ArgumentError] when a preserved
  /// table is among [cacheTables].
  static Future<void> discardCache(
    Migrator migrator,
    Iterable<TableInfo<Table, Object?>> cacheTables,
  ) async {
    for (final table in cacheTables) {
      if (preservedTables.contains(table.actualTableName)) {
        throw ArgumentError(
          '${table.actualTableName} keeps its rows on every migration (15 6.2)',
        );
      }
      await migrator.deleteTable(table.actualTableName);
      await migrator.createTable(table);
    }
    await resetCursor(migrator);
  }

  /// Clears `sync_state.cursor` and `sync_epoch` and keeps every other row, so the next
  /// connection loads a whole snapshot (04 7.3); a step uses it alone when it adds a mirror
  /// column that only the snapshot fills for the rows it keeps (M4-40: `channels.project_bound`).
  /// Without a `sync_state` row nothing changes: the first connection loads the snapshot anyway.
  static Future<void> resetCursor(Migrator migrator) =>
      migrator.database.customStatement(
        'UPDATE sync_state SET cursor = NULL, sync_epoch = NULL',
      );

  /// Migrates [tables] row by row into their new layout (`Migrator.alterTable`); new columns
  /// take their defaults.
  static Future<void> keepRows(
    Migrator migrator,
    Iterable<TableInfo<Table, Object?>> tables,
  ) async {
    for (final table in tables) {
      await migrator.alterTable(TableMigration(table));
    }
  }
}
