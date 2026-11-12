// SPDX-License-Identifier: Apache-2.0

/// The writes of the search index (`search_index` in `search.drift` with its companion
/// `search_rows`, 15 6.1). Not exported: callers use `SearchDao`, `removeTargets`, the cleanup
/// and `SyncStateDao.clearMirror`.
///
/// FTS5 reaches a row without a scan only through a match or its `rowid`; a condition on a
/// locating column (`kind`, `target_id`, `thread_id`, `channel_id`, `space_id`) reads the whole
/// index table. So every row has a twin in the normal table `search_rows` with the same
/// `rowid` and indexes on those columns, and a delete finds the rows there.
library;

import 'package:drift/drift.dart';

import 'database.dart';

/// Adds a row to the search index: the locating columns to `search_rows`, the same values and
/// [body] to `search_index` under the same `rowid`. Runs in the caller's transaction.
Future<void> insertSearchRow(
  LocalDatabase db, {
  required String kind,
  required String targetId,
  required String threadId,
  required String channelId,
  required String spaceId,
  required String body,
}) async {
  final id = await db
      .into(db.searchRows)
      .insert(
        SearchRowsCompanion.insert(
          kind: kind,
          targetId: targetId,
          threadId: threadId,
          channelId: channelId,
          spaceId: spaceId,
        ),
      );
  // OR REPLACE: `search_rows` hands out the rowid, so a row the index should still hold under
  // it is stale by definition and must not fail the write of a message.
  await db.customInsert(
    'INSERT OR REPLACE INTO search_index '
    '(rowid, kind, target_id, thread_id, channel_id, space_id, body) '
    'VALUES (?, ?, ?, ?, ?, ?, ?)',
    variables: [
      Variable<int>(id),
      Variable<String>(kind),
      Variable<String>(targetId),
      Variable<String>(threadId),
      Variable<String>(channelId),
      Variable<String>(spaceId),
      Variable<String>(body),
    ],
    updates: {db.searchIndex},
  );
}

/// Deletes the rows of the search index that [where], a condition over the columns of
/// `search_rows`, selects with [variables]; returns how many went. Runs in the caller's
/// transaction: both tables must change together.
Future<int> deleteSearchRows(
  LocalDatabase db,
  String where,
  List<Variable<Object>> variables,
) async {
  await db.customUpdate(
    'DELETE FROM search_index WHERE rowid IN '
    '(SELECT id FROM search_rows WHERE $where)',
    variables: variables,
    updates: {db.searchIndex},
    updateKind: UpdateKind.delete,
  );
  return db.customUpdate(
    'DELETE FROM search_rows WHERE $where',
    variables: variables,
    updates: {db.searchRows},
    updateKind: UpdateKind.delete,
  );
}

/// Empties the search index. Runs in the caller's transaction.
Future<void> clearSearchRows(LocalDatabase db) async {
  await db.delete(db.searchIndex).go();
  await db.delete(db.searchRows).go();
}
