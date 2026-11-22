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
  /// connection loads a whole snapshot (04 7.3). Without a `sync_state` row nothing changes: the
  /// first connection loads the snapshot anyway. Alone it does not refill a new mirror column of
  /// a kept row: the snapshot brings the row at the version the device already holds, and the
  /// version rule skips it; such a step uses [reloadRows] (FEAT-PIN-CHANNELS).
  static Future<void> resetCursor(Migrator migrator) =>
      migrator.database.customStatement(
        'UPDATE sync_state SET cursor = NULL, sync_epoch = NULL',
      );

  /// Sets the version of every row of the mirror table [tableName] to 0 and clears the cursor
  /// ([resetCursor]), so the next snapshot replaces each kept row by the version rule (04 9 item
  /// 3) and fills a mirror column the step adds (FEAT-PIN-CHANNELS: a row an older app stored at
  /// version 3 without `pinned_at` gets the pin of the snapshot at version 3). [tableName] must be
  /// a mirror table with a `version` column, never a preserved one.
  static Future<void> reloadRows(Migrator migrator, String tableName) async {
    if (preservedTables.contains(tableName)) {
      throw ArgumentError(
        '$tableName keeps its rows on every migration (15 6.2)',
      );
    }
    await migrator.database.customStatement(
      'UPDATE $tableName SET version = 0',
    );
    await resetCursor(migrator);
  }

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
