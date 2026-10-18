// SPDX-License-Identifier: Apache-2.0

/// Deletion of threads with everything below them, shared by the removal of 04 8.3 and the
/// cleanup of 15 6.3. Not exported: callers use `removeTargets` and `CacheCleaner`.
library;

import 'package:drift/drift.dart';

import 'daos/upsert.dart';
import 'database.dart';

/// Deletes the threads [threadIds] with their messages, reactions, mentions, components,
/// attachments, runs, tags, read states, loaded ranges, thread notification preferences and
/// search rows, the metadata of files no other message attaches, and their file cache rows;
/// drafts of the threads only with [withDrafts]. Runs in the caller's transaction and returns
/// the paths of the deleted file cache rows.
Future<List<String>> deleteThreads(
  LocalDatabase db,
  List<String> threadIds, {
  required bool withDrafts,
}) async {
  if (threadIds.isEmpty) return const [];
  final ids = placeholders(threadIds.length);
  final vars = textVariables(threadIds);
  Future<void> run(String sql, Set<TableInfo<Table, Object?>> tables) =>
      db.customUpdate(
        sql,
        variables: vars,
        updates: tables,
        updateKind: UpdateKind.delete,
      );

  final fileRows = await db
      .customSelect(
        'SELECT DISTINCT file_id FROM message_attachments WHERE thread_id IN $ids',
        variables: vars,
      )
      .get();
  final fileIds = [for (final r in fileRows) r.read<String>('file_id')];

  const messageIds = 'SELECT id FROM messages WHERE thread_id IN';
  await run(
    'DELETE FROM reactions_summary WHERE message_id IN ($messageIds $ids)',
    {db.reactionsSummary},
  );
  await run(
    'DELETE FROM message_components WHERE message_id IN ($messageIds $ids)',
    {db.messageComponents},
  );
  await run('DELETE FROM my_mentions WHERE thread_id IN $ids', {db.myMentions});
  await run('DELETE FROM message_attachments WHERE thread_id IN $ids', {
    db.messageAttachments,
  });
  await run('DELETE FROM runs WHERE thread_id IN $ids', {db.runs});
  await run('DELETE FROM messages WHERE thread_id IN $ids', {db.messages});
  await run('DELETE FROM thread_tags WHERE thread_id IN $ids', {db.threadTags});
  await run('DELETE FROM read_states WHERE thread_id IN $ids', {db.readStates});
  await run('DELETE FROM loaded_ranges WHERE thread_id IN $ids', {
    db.loadedRangesTable,
  });
  await run(
    "DELETE FROM notification_preferences WHERE target_kind = 'thread' AND target_id IN $ids",
    {db.notificationPreferences},
  );
  await run('DELETE FROM search_index WHERE thread_id IN $ids', {
    db.searchIndex,
  });
  if (withDrafts) {
    await run('DELETE FROM drafts WHERE thread_id IN $ids', {db.drafts});
  }
  await run('DELETE FROM threads WHERE id IN $ids', {db.threads});
  return deleteOrphanFiles(db, fileIds);
}

/// Deletes the metadata and the file cache rows of those [fileIds] that no remaining message
/// attaches, and returns the paths of the deleted file cache rows.
Future<List<String>> deleteOrphanFiles(
  LocalDatabase db,
  List<String> fileIds,
) async {
  if (fileIds.isEmpty) return const [];
  final ids = placeholders(fileIds.length);
  const stillAttached = 'SELECT file_id FROM message_attachments';
  final pathRows = await db
      .customSelect(
        'SELECT path FROM file_cache WHERE file_id IN $ids '
        'AND file_id NOT IN ($stillAttached)',
        variables: textVariables(fileIds),
      )
      .get();
  await db.customUpdate(
    'DELETE FROM file_cache WHERE file_id IN $ids AND file_id NOT IN ($stillAttached)',
    variables: textVariables(fileIds),
    updates: {db.fileCache},
    updateKind: UpdateKind.delete,
  );
  await db.customUpdate(
    'DELETE FROM files WHERE id IN $ids AND id NOT IN ($stillAttached)',
    variables: textVariables(fileIds),
    updates: {db.files},
    updateKind: UpdateKind.delete,
  );
  return [for (final r in pathRows) r.read<String>('path')];
}
