// SPDX-License-Identifier: Apache-2.0

/// Removal of targets on `access.changed` with `removed` and on `thread.moved_out` (04 8.3,
/// 15 6.3), and the wipe on `4012` (R-SY-14).
library;

import 'dart:io';

import 'package:drift/drift.dart';

import 'cascade.dart';
import 'daos/upsert.dart';
import 'database.dart';
import 'models.dart';

/// Removal and wipe on the [LocalDatabase].
extension LocalStoreRemoval on LocalDatabase {
  /// Deletes in one transaction everything of [removed] (04 8.3 item 3): a space with its
  /// channels, a channel with its threads, a thread with its messages, attachments, reaction
  /// summaries, mentions, components, runs, drafts, read states, loaded ranges, search rows and
  /// file cache rows, plus the sidebar state, operator setting, forum tags, commands and
  /// notification preferences of the targets. Outbox entries stay: they fail with `404` and
  /// keep their text (04 8.3 item 4). Host targets are ignored (M4-36).
  ///
  /// Returns the paths of the deleted file cache rows; the caller deletes the bytes (M4-22).
  Future<List<String>> removeTargets(
    List<TargetRef> removed,
  ) => transaction(() async {
    List<String> idsOf(RemovedTargetKind kind) => [
      for (final t in removed)
        if (t.kind == kind) t.id,
    ];
    final spaceIds = idsOf(RemovedTargetKind.space);
    final channelIds = {...idsOf(RemovedTargetKind.channel)};
    if (spaceIds.isNotEmpty) {
      final rows = await customSelect(
        'SELECT id FROM channels WHERE space_id IN ${placeholders(spaceIds.length)}',
        variables: textVariables(spaceIds),
      ).get();
      channelIds.addAll(rows.map((r) => r.read<String>('id')));
    }
    final threadIds = {...idsOf(RemovedTargetKind.thread)};
    if (channelIds.isNotEmpty) {
      // Rows below a thread may outlive its thread row (the cleanup dropped the thread, a
      // later event stored a message again), so the threads come from every table that
      // carries the channel.
      final ids = placeholders(channelIds.length);
      final rows = await customSelect(
        'SELECT id FROM threads WHERE channel_id IN $ids '
        'UNION SELECT thread_id FROM messages WHERE channel_id IN $ids '
        'UNION SELECT thread_id FROM message_attachments WHERE channel_id IN $ids '
        'UNION SELECT thread_id FROM runs WHERE channel_id IN $ids '
        'UNION SELECT thread_id FROM read_states WHERE channel_id IN $ids',
        variables: [for (var i = 0; i < 5; i++) ...textVariables(channelIds)],
      ).get();
      threadIds.addAll(rows.map((r) => r.read<String>('id')));
    }
    final paths = [
      ...await deleteThreads(this, threadIds.toList(), withDrafts: true),
    ];
    final channels = channelIds.toList();
    if (channels.isNotEmpty) {
      // Files of the channels that no message attaches any more (an edit dropped the
      // attachment) still hold cached content of the target (04 8.3 item 3, T-DV-01).
      final scoped = await customSelect(
        'SELECT id FROM files WHERE scope_channel_id IN ${placeholders(channels.length)}',
        variables: textVariables(channels),
      ).get();
      paths.addAll(
        await deleteOrphanFiles(this, [
          for (final r in scoped) r.read<String>('id'),
        ]),
      );
    }
    await _deleteWhereIn(channels, 'drafts', 'channel_id', {drafts});
    await _deleteWhereIn(channels, 'forum_tags', 'channel_id', {forumTags});
    await _deleteWhereIn(channels, 'channel_operators', 'channel_id', {
      channelOperators,
    });
    await _deleteWhereIn(channels, 'search_index', 'channel_id', {searchIndex});
    await _deleteWhereIn(channels, 'channels', 'id', {this.channels});
    await _deleteWhereIn(spaceIds, 'user_space_states', 'space_id', {
      userSpaceStates,
    });
    await _deleteWhereIn(spaceIds, 'search_index', 'space_id', {searchIndex});
    await _deleteWhereIn(spaceIds, 'spaces', 'id', {spaces});
    final scopes = [...spaceIds, ...channels];
    await _deleteWhereIn(scopes, 'commands', 'scope_id', {commands});
    await _deleteWhereIn(scopes, 'notification_preferences', 'target_id', {
      notificationPreferences,
    });
    return paths;
  });

  /// Closes the database and deletes its file with the `-wal` and `-shm` files (R-SY-14, on
  /// `4012`); an in-memory database is only closed. The app deletes keys, tokens and the file
  /// cache bytes (M4-18).
  Future<void> wipe() async {
    await close();
    final path = file?.path;
    if (path == null) return;
    for (final name in [path, '$path-wal', '$path-shm', '$path-journal']) {
      final target = File(name);
      if (await target.exists()) await target.delete();
    }
  }

  Future<void> _deleteWhereIn(
    List<String> ids,
    String table,
    String column,
    Set<TableInfo<Table, Object?>> updates,
  ) async {
    if (ids.isEmpty) return;
    await customUpdate(
      'DELETE FROM $table WHERE $column IN ${placeholders(ids.length)}',
      variables: textVariables(ids),
      updates: updates,
      updateKind: UpdateKind.delete,
    );
  }
}
