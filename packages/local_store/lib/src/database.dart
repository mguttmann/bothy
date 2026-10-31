// SPDX-License-Identifier: Apache-2.0

/// The drift database of the apps, schema version 12 (docs/spec/15-client-architecture.md 6, ADR
/// 0029): the mirror tables of 15 6.1 and the local-only tables, `host_key_pins` (M4-36) and
/// the sidebar view state `channel_view_states`, `shell_view_state` (M4-40, M4-42) and
/// `space_view_states` (FIX-device-mirroring-2) included; version 11 adds the code block wrap
/// choice of `shell_view_state` (M4-49), version 12 the personal channel and thread orders of
/// the sidebar (`user_space_states.channel_order`, `user_channel_states`, M4-51).
library;

import 'dart:io';

import 'package:drift/drift.dart';

import 'daos/commands_dao.dart';
import 'daos/file_cache_dao.dart';
import 'daos/forum_view_dao.dart';
import 'daos/host_key_pins_dao.dart';
import 'daos/instance_dao.dart';
import 'daos/messages_dao.dart';
import 'daos/outbox_dao.dart';
import 'daos/personal_dao.dart';
import 'daos/principals_dao.dart';
import 'daos/recent_targets_dao.dart';
import 'daos/runs_dao.dart';
import 'daos/search_dao.dart';
import 'daos/share_inbox_dao.dart';
import 'daos/structure_dao.dart';
import 'daos/sync_state_dao.dart';
import 'daos/threads_dao.dart';
import 'database.steps.dart';
import 'migration_rules.dart';
import 'tables/files.dart';
import 'tables/instance.dart';
import 'tables/local_only.dart';
import 'tables/personal.dart';
import 'tables/runs_agents.dart';
import 'tables/structure.dart';
import 'tables/threads_messages.dart';

part 'database.g.dart';

/// The local database; open it with `openLocalStore` or `openInMemoryForTests`.
@DriftDatabase(
  tables: [
    Spaces,
    UserSpaceStates,
    UserChannelStates,
    Channels,
    ChannelOperators,
    ForumTags,
    Threads,
    ThreadTags,
    Messages,
    ReactionsSummary,
    MyMentions,
    MessageComponents,
    Files,
    MessageAttachments,
    Runs,
    Agents,
    Principals,
    Commands,
    ReadStates,
    Drafts,
    UserSettings,
    NotificationPreferences,
    QuietHoursRules,
    InstanceTable,
    Outbox,
    SyncStateTable,
    LoadedRangesTable,
    FileCache,
    ShareInbox,
    ForumViewStates,
    RecentTargets,
    HostKeyPins,
    ChannelViewStates,
    ShellViewStates,
    SpaceViewStates,
  ],
  include: {'search.drift'},
)
class LocalDatabase extends _$LocalDatabase {
  /// Wraps [executor]; [file] is the database file (null in memory) and [clock] the time
  /// source of the local timestamps.
  LocalDatabase(super.executor, {this.file, DateTime Function()? clock})
    : clock = clock ?? DateTime.now;

  /// The database file, or null for an in-memory database.
  final File? file;

  /// Time source of `last_accessed_at`, `created_at` and `state_changed_at`.
  final DateTime Function() clock;

  @override
  int get schemaVersion => 12;

  /// Closes the database and releases its file, also while watched queries come and go.
  ///
  /// drift closes its stream store first: it awaits the close of each active stream while it
  /// iterates them, and a paused listener (riverpod pauses providers nobody watches) holds that
  /// close until it resumes. A stream that a cancelled listener removes meanwhile ends the loop
  /// with a [ConcurrentModificationError] before drift closes the executor, so the file stayed
  /// open and Windows refused to delete it after a 4012 or a logout (R-SY-14). The executor
  /// closes in that case anyway; the streams are dead with it.
  @override
  Future<void> close() async {
    try {
      await super.close();
    } on ConcurrentModificationError {
      await executor.close();
    }
  }

  @override
  MigrationStrategy get migration => MigrationStrategy(
    onCreate: (m) => m.createAll(),
    // `dart run drift_dev make-migrations` writes `database.steps.dart`; each step is built
    // from MigrationRules (docs/DECISIONS.md D-M4-12-6). A stored version above this one (a
    // database of a newer app) cannot be opened; the app wipes it and resyncs.
    onUpgrade: stepByStep(
      // Version 2 (M4-19): the local-only table `forum_view_states`; no row moves.
      from1To2: (m, schema) => m.createTable(schema.forumViewStates),
      // Version 3 (M4-23): the local-only table `recent_targets`; no row moves.
      from2To3: (m, schema) => m.createTable(schema.recentTargets),
      // Version 4 (M4-36): the local-only table `host_key_pins`; no row moves. From here on
      // it is a preserved table (MigrationRules), so later steps keep its rows.
      from3To4: (m, schema) => m.createTable(schema.hostKeyPins),
      // Version 5 (M5-08): the nullable column `instance.app_lock`; the row stays and the next
      // `getInstance` of the engine fills the policy.
      from4To5: (m, schema) =>
          m.addColumn(schema.instance, schema.instance.appLock),
      // Version 6 (M4-40): the local-only tables `channel_view_states` and `shell_view_state`
      // (01 6.8 item 7, D-SB-6) and the column `channels.project_bound` (R-DM-54), false in
      // every kept row; no row moves. The cursor goes, so the next connection loads a whole
      // snapshot that fills the flag of every channel (04 7.3).
      from5To6: (m, schema) async {
        await m.createTable(schema.channelViewStates);
        await m.createTable(schema.shellViewState);
        await m.addColumn(schema.channels, schema.channels.projectBound);
        await MigrationRules.resetCursor(m);
      },
      // Version 7 (M4-42): the column `shell_view_state.channel_column_hidden`, false in the
      // kept row (01 D1, D2: the channel column shows until the person hides it); no row moves.
      from6To7: (m, schema) => m.addColumn(
        schema.shellViewState,
        schema.shellViewState.channelColumnHidden,
      ),
      // Version 8 (M4-45): the columns `shell_view_state.session_column_hidden` and
      // `text_thread_session_shown`, false in the kept row (01 D1, D2: the session column shows
      // beside a forum post and is collapsed beside a thread of a text channel until the person
      // changes it); no row moves.
      from7To8: (m, schema) async {
        await m.addColumn(
          schema.shellViewState,
          schema.shellViewState.sessionColumnHidden,
        );
        await m.addColumn(
          schema.shellViewState,
          schema.shellViewState.textThreadSessionShown,
        );
      },
      // Version 9 (M4-46): the column `channel_view_states.posts_shown_in_full`, false in every
      // kept row (01 6.8 item 3: a forum group shows the limit until the person picks "Show
      // more"); no row moves.
      from8To9: (m, schema) => m.addColumn(
        schema.channelViewStates,
        schema.channelViewStates.postsShownInFull,
      ),
      // Version 10 (FIX-device-mirroring-2): the local-only table `space_view_states`, filled
      // from the stored `user_space_states.collapsed`, so every space keeps the state it showed
      // (01 6.7, R-PR-24: the collapse of a space stays on its device); no row moves.
      from9To10: (m, schema) async {
        await m.createTable(schema.spaceViewStates);
        await m.database.customStatement(
          'INSERT INTO space_view_states (space_id, collapsed) '
          'SELECT space_id, collapsed FROM user_space_states',
        );
      },
      // Version 11 (M4-49): the column `shell_view_state.code_blocks_unwrapped`, false in the
      // kept row (01 6.14 item 1: code blocks wrap until the person switches them); no row
      // moves.
      from10To11: (m, schema) async {
        await m.addColumn(
          schema.shellViewState,
          schema.shellViewState.codeBlocksUnwrapped,
        );
      },
      // Version 12 (M4-51): the column `user_space_states.channel_order`, null (the default
      // order) in every kept row, and the mirror table `user_channel_states`; no row moves. The
      // cursor goes, so the next connection loads a whole snapshot with the orders other
      // devices wrote before this upgrade (04 7.3, D-M4-51-2).
      from11To12: (m, schema) async {
        await m.addColumn(
          schema.userSpaceStates,
          schema.userSpaceStates.channelOrder,
        );
        await m.createTable(schema.userChannelStates);
        await MigrationRules.resetCursor(m);
      },
    ),
  );

  /// The mirror tables of 15 6.1 that a resync and a migration may discard: all of them except
  /// `drafts`, which hold unsent work (15 6.2, 04 7.3).
  List<TableInfo<Table, Object?>> get mirrorTables => [
    spaces,
    userSpaceStates,
    userChannelStates,
    channels,
    channelOperators,
    forumTags,
    threads,
    threadTags,
    messages,
    reactionsSummary,
    myMentions,
    messageComponents,
    files,
    messageAttachments,
    runs,
    agents,
    principals,
    commands,
    readStates,
    userSettings,
    notificationPreferences,
    quietHoursRules,
    instanceTable,
  ];

  /// The tables of `MigrationRules.preservedTables`.
  List<TableInfo<Table, Object?>> get preservedTables => [
    outbox,
    drafts,
    shareInbox,
    hostKeyPins,
  ];

  /// Spaces, sidebar states, channels, operators, forum tags and access views.
  late final StructureDao structureDao = StructureDao(this);

  /// Threads and their tags.
  late final ThreadsDao threadsDao = ThreadsDao(this);

  /// Messages, reactions, pins, mentions, components, files, attachments, loaded ranges.
  late final MessagesDao messagesDao = MessagesDao(this);

  /// Run summaries.
  late final RunsDao runsDao = RunsDao(this);

  /// Principals and agent cards.
  late final PrincipalsDao principalsDao = PrincipalsDao(this);

  /// Slash commands.
  late final CommandsDao commandsDao = CommandsDao(this);

  /// Read states, drafts, settings, notification preferences, quiet hours.
  late final PersonalDao personalDao = PersonalDao(this);

  /// Instance information.
  late final InstanceDao instanceDao = InstanceDao(this);

  /// Outbox entries.
  late final OutboxDao outboxDao = OutboxDao(this);

  /// Cursor, epoch and session fields.
  late final SyncStateDao syncStateDao = SyncStateDao(this);

  /// File cache rows.
  late final FileCacheDao fileCacheDao = FileCacheDao(this);

  /// Offline search.
  late final SearchDao searchDao = SearchDao(this);

  /// Share inbox.
  late final ShareInboxDao shareInboxDao = ShareInboxDao(this);

  /// How forums and channel groups look on this device: the Done section, the collapsed
  /// thread groups and the pinned forum overview.
  late final ForumViewDao forumViewDao = ForumViewDao(this);

  /// The recently visited targets of the quick switcher on this device.
  late final RecentTargetsDao recentTargetsDao = RecentTargetsDao(this);

  /// The pinned host keys of this device (15 23.3).
  late final HostKeyPinsDao hostKeyPinsDao = HostKeyPinsDao(this);
}
