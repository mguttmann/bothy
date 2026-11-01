// SPDX-License-Identifier: Apache-2.0

part of 'world.dart';

/// One deletion of a space, channel or thread (01 6.18, D-DEL-1): when and by whom, whether the
/// deletion set the archive (D-DEL-2), and `trash_project_folder` of a channel.
final class FakeDeletion {
  /// Creates the record.
  FakeDeletion({
    required this.kind,
    required this.deletedAt,
    required this.deletedBy,
    required this.undoUntil,
    this.setArchive = false,
    this.trashProjectFolder = false,
  });

  /// `space`, `channel` or `thread` (API `DeletionInfo.target_kind`).
  final String kind;

  /// `deleted_at` as stored in the object.
  final String deletedAt;

  /// The principal who deleted it.
  final String deletedBy;

  /// The end of the undo window (`deleted_at` plus `chat.delete_undo_window`).
  final DateTime undoUntil;

  /// Whether the deletion set `archived_at` (an earlier archive stays on an undo).
  final bool setArchive;

  /// `ChannelDelete.trash_project_folder` as requested.
  final bool trashProjectFolder;
}

/// Deleting and restoring spaces, channels and threads and the deletion info of the dialog (01
/// 6.18; API `deleteSpace`, `restoreSpace`, `getSpaceDeletion` and the channel and thread
/// operations). The checks are the rights of D-DEL-4 as the fake knows them: an owner for a
/// space, `manage` for a channel, `write` and (creator or `manage`) for a thread, plus the
/// owner for `trash_project_folder`; the step-up is the handlers' (`requireCatalogStepUp`).
/// Each change appends the events of D-DEL-5: `space.archived` or `channel.archived` with
/// `deleted_at`, `thread.updated` with it, and on an undo `space.updated`, `channel.updated` or
/// `thread.updated` without it, always with a higher version. Simplifications (README): no
/// purge after the window, no `access.changed` and no run cancellation.
///
/// Example:
/// ```dart
/// final info = world.deleteThread(sam.id, threadId);
/// world.restoreThread(sam.id, threadId);
/// ```
extension DeletionWorld on FakeWorld {
  /// The space [spaceId] when [principalId] sees it; else `404 not_found`.
  JsonMap _deletableSpace(String principalId, String spaceId) {
    final space = spaces[spaceId];
    if (space == null || !seesSpace(principalId, spaceId)) {
      throw const FakeProblem(404, 'not_found');
    }
    return space;
  }

  /// The channel [channelId] when [principalId] sees it; else `404 not_found`.
  FakeChannel _deletableChannel(String principalId, String channelId) {
    final channel = channels[channelId];
    if (channel == null || !seesChannel(principalId, channelId)) {
      throw const FakeProblem(404, 'not_found');
    }
    return channel;
  }

  /// The thread [threadId], deleted or not, when [principalId] sees its channel; else
  /// `404 not_found`.
  JsonMap _deletableThread(String principalId, String threadId) {
    final thread = threads[threadId];
    if (thread == null ||
        !seesChannel(principalId, thread['channel_id']! as String)) {
      throw const FakeProblem(404, 'not_found');
    }
    return thread;
  }

  void _owner(String principalId) {
    if (people[principalId]?.isOwner != true) {
      throw const FakeProblem(403, 'forbidden');
    }
  }

  /// D-DEL-4 for a thread: `write` and (creator or `manage`), never in an archived or read only
  /// channel, never a main thread.
  void _mayDeleteThread(String principalId, JsonMap thread) {
    final channelId = thread['channel_id']! as String;
    final channel = channels[channelId]!;
    final level = levelOf(principalId, channelId);
    if (level != 'write' && level != 'trigger') {
      throw const FakeProblem(403, 'forbidden');
    }
    if (thread['created_by'] != principalId &&
        !manages(principalId, channelId)) {
      throw const FakeProblem(403, 'forbidden');
    }
    final spaceId = channel.spaceId;
    if (channel.json['archived_at'] != null ||
        (spaceId != null && spaces[spaceId]?['archived_at'] != null)) {
      throw const FakeProblem(
        403,
        'channel_read_only',
        reason: 'channel_archived',
      );
    }
    if (channel.json['read_only'] == true) {
      throw const FakeProblem(403, 'channel_read_only');
    }
    if (thread['kind'] == 'main') {
      throw const FakeProblem(409, 'conflict', reason: 'main_thread');
    }
  }

  /// Marks [object] deleted by [principalId] (`deleted_at`, and `archived_at` for a space or
  /// channel that is not archived yet), raises its version and records the deletion.
  FakeDeletion _markDeleted(
    String id,
    String kind,
    JsonMap object,
    String principalId, {
    bool trashProjectFolder = false,
  }) {
    final at = now();
    final stamped = stamp(at);
    final setArchive = kind != 'thread' && object['archived_at'] == null;
    object['deleted_at'] = stamped;
    if (setArchive) object['archived_at'] = stamped;
    object['version'] = (object['version']! as int) + 1;
    return deletions[id] = FakeDeletion(
      kind: kind,
      deletedAt: stamped,
      deletedBy: principalId,
      undoUntil: at.add(deleteUndoWindow),
      setArchive: setArchive,
      trashProjectFolder: trashProjectFolder,
    );
  }

  /// The pending deletion of [id] for an undo: null when [id] is not deleted (the restore has
  /// no effect), `409 conflict` with `undo_window_passed` after the window.
  FakeDeletion? _undoable(String id) {
    final d = deletions[id];
    if (d == null) return null;
    if (!now().isBefore(d.undoUntil)) {
      throw const FakeProblem(409, 'conflict', reason: 'undo_window_passed');
    }
    return d;
  }

  /// Clears the deletion of [id] from [object] (and the archive it set) and raises the version.
  void _clearDeleted(String id, JsonMap object, FakeDeletion d) {
    object.remove('deleted_at');
    if (d.setArchive) object.remove('archived_at');
    object['version'] = (object['version']! as int) + 1;
    deletions.remove(id);
  }

  /// `deleteSpace`: an owner deletes [spaceId] (D-DEL-2, D-DEL-4) and `space.archived` with
  /// `deleted_at` goes out; returns the `DeletionInfo`. Deleting again answers the deletion.
  JsonMap deleteSpace(String principalId, String spaceId) {
    final space = _deletableSpace(principalId, spaceId);
    _owner(principalId);
    if (!deletions.containsKey(spaceId)) {
      _markDeleted(spaceId, 'space', space, principalId);
      append(
        'space.archived',
        resource: {'kind': 'space', 'id': spaceId, 'version': space['version']},
        scope: {'space_id': spaceId},
        payload: {'space': space},
        actor: principalId,
      );
    }
    return deletionInfo(principalId, 'space', spaceId);
  }

  /// `restoreSpace`: undoes the deletion of [spaceId] before `undo_until` with `space.updated`
  /// without `deleted_at`; returns the `Space`.
  JsonMap restoreSpace(String principalId, String spaceId) {
    final space = _deletableSpace(principalId, spaceId);
    _owner(principalId);
    final d = _undoable(spaceId);
    if (d == null) return space;
    _clearDeleted(spaceId, space, d);
    append(
      'space.updated',
      resource: {'kind': 'space', 'id': spaceId, 'version': space['version']},
      scope: {'space_id': spaceId},
      payload: {'space': space},
      actor: principalId,
    );
    return space;
  }

  /// `deleteChannel`: whoever has `manage` deletes [channelId] (D-DEL-2, D-DEL-4) and
  /// `channel.archived` with `deleted_at` goes out; returns the `DeletionInfo`.
  /// [trashProjectFolder] needs a forum with a project binding (`400 validation_failed`
  /// otherwise), an owner (`403`) and an ok folder check (`409 conflict`,
  /// `project_folder_refused`); it sets `trash_state` `requested` (D-DEL-10, D-DEL-11). A dm
  /// channel is never deleted (`409 conflict`, `dm_channel`).
  JsonMap deleteChannel(
    String principalId,
    String channelId, {
    bool trashProjectFolder = false,
  }) {
    final channel = _deletableChannel(principalId, channelId);
    if (!manages(principalId, channelId)) {
      throw const FakeProblem(403, 'forbidden');
    }
    if (channel.json['kind'] == 'dm') {
      throw const FakeProblem(409, 'conflict', reason: 'dm_channel');
    }
    if (!deletions.containsKey(channelId)) {
      if (trashProjectFolder) {
        if (channel.json['project_bound'] != true) {
          throw const FakeProblem(
            400,
            'validation_failed',
            errors: [
              {'pointer': '/trash_project_folder', 'code': 'not_project_bound'},
            ],
          );
        }
        _owner(principalId);
        final folder = projectFolders[channelId];
        final check = folder?['trash_check'] as JsonMap?;
        if (folder == null || check?['ok'] != true) {
          throw const FakeProblem(
            409,
            'conflict',
            reason: 'project_folder_refused',
          );
        }
        folder['trash_state'] = 'requested';
      }
      _markDeleted(
        channelId,
        'channel',
        channel.json,
        principalId,
        trashProjectFolder: trashProjectFolder,
      );
      append(
        'channel.archived',
        resource: {
          'kind': 'channel',
          'id': channelId,
          'version': channel.json['version'],
        },
        scope: {'space_id': ?channel.spaceId, 'channel_id': channelId},
        payload: {'channel': channel.json},
        actor: principalId,
      );
    }
    return deletionInfo(principalId, 'channel', channelId);
  }

  /// `restoreChannel`: undoes the deletion of [channelId] before `undo_until`, drops a folder
  /// request and sends `channel.updated` without `deleted_at`; returns the `Channel`. In a
  /// deleted space `409 conflict` with `space_deleted`.
  JsonMap restoreChannel(String principalId, String channelId) {
    final channel = _deletableChannel(principalId, channelId);
    if (!manages(principalId, channelId)) {
      throw const FakeProblem(403, 'forbidden');
    }
    final spaceId = channel.spaceId;
    if (spaceId != null && deletions.containsKey(spaceId)) {
      throw const FakeProblem(409, 'conflict', reason: 'space_deleted');
    }
    final d = _undoable(channelId);
    if (d == null) return channel.json;
    _clearDeleted(channelId, channel.json, d);
    projectFolders[channelId]?.remove('trash_state');
    append(
      'channel.updated',
      resource: {
        'kind': 'channel',
        'id': channelId,
        'version': channel.json['version'],
      },
      scope: {'space_id': ?spaceId, 'channel_id': channelId},
      payload: {'channel': channel.json},
      actor: principalId,
    );
    return channel.json;
  }

  /// `deleteThread`: the creator or whoever has `manage` deletes [threadId] (D-DEL-3, D-DEL-4)
  /// and `thread.updated` with `deleted_at` goes out; returns the `DeletionInfo`.
  JsonMap deleteThread(String principalId, String threadId) {
    final thread = _deletableThread(principalId, threadId);
    if (!deletions.containsKey(threadId)) {
      _mayDeleteThread(principalId, thread);
      _markDeleted(threadId, 'thread', thread, principalId);
      append(
        'thread.updated',
        resource: {
          'kind': 'thread',
          'id': threadId,
          'version': thread['version'],
        },
        scope: _scopeOf(threadId),
        payload: {'thread': thread},
        actor: principalId,
      );
    }
    return deletionInfo(principalId, 'thread', threadId);
  }

  /// `restoreThread`: undoes the deletion of [threadId] before `undo_until` with
  /// `thread.updated` without `deleted_at`; returns the `Thread`.
  JsonMap restoreThread(String principalId, String threadId) {
    final thread = _deletableThread(principalId, threadId);
    _mayDeleteThread(principalId, thread);
    final d = _undoable(threadId);
    if (d == null) return thread;
    _clearDeleted(threadId, thread, d);
    append(
      'thread.updated',
      resource: {
        'kind': 'thread',
        'id': threadId,
        'version': thread['version'],
      },
      scope: _scopeOf(threadId),
      payload: {'thread': thread},
      actor: principalId,
    );
    return thread;
  }

  /// The API `DeletionInfo` of the [kind] target [id] for [principalId] (D-DEL-6): the counts of
  /// what the deletion removes, the state (`live`, `pending` until `undo_until`, then `final`),
  /// and for an owner and a forum with a project binding the project folder.
  JsonMap deletionInfo(String principalId, String kind, String id) {
    final channelIds = switch (kind) {
      'space' => [
        for (final c in channels.values)
          if (c.spaceId == id && c.json['kind'] != 'dm') c.id,
      ],
      'channel' => [id],
      _ => <String>[],
    };
    final threadIds = kind == 'thread'
        ? [id]
        : [
            for (final t in threads.values)
              if (channelIds.contains(t['channel_id'])) t['id']! as String,
          ];
    final live = [
      for (final t in threadIds)
        for (final m in messagesOf(t))
          if (m['deleted_at'] == null) m,
    ];
    final d = deletions[id];
    final folder =
        kind == 'channel' &&
            channels[id]?.json['project_bound'] == true &&
            people[principalId]?.isOwner == true
        ? projectFolders[id]
        : null;
    return {
      'target_kind': kind,
      'target_id': id,
      'state': d == null
          ? 'live'
          : now().isBefore(d.undoUntil)
          ? 'pending'
          : 'final',
      if (d != null) ...{
        'deleted_at': d.deletedAt,
        'undo_until': stamp(d.undoUntil),
      },
      'contents': {
        'channels': kind == 'thread' ? 0 : channelIds.length,
        'threads': threadIds.where((t) => threads[t]!['kind'] != 'main').length,
        'messages': live.length,
        'files': live.fold<int>(
          0,
          (n, m) => n + (m['attachments']! as List<Object?>).length,
        ),
        'sessions': {
          for (final r in runs.values)
            if (threadIds.contains(r['thread_id'])) r['thread_id'],
        }.length,
      },
      'project_folder': ?folder,
    };
  }
}
