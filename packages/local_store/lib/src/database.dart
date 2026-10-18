// SPDX-License-Identifier: Apache-2.0

/// The drift database of the apps, schema version 1 (docs/spec/15-client-architecture.md 6, ADR
/// 0029): the mirror tables of 15 6.1 except `host_key_pins` (M4-36) and the local-only tables.
library;

import 'dart:io';

import 'package:drift/drift.dart';

import 'daos/commands_dao.dart';
import 'daos/file_cache_dao.dart';
import 'daos/instance_dao.dart';
import 'daos/messages_dao.dart';
import 'daos/outbox_dao.dart';
import 'daos/personal_dao.dart';
import 'daos/principals_dao.dart';
import 'daos/runs_dao.dart';
import 'daos/search_dao.dart';
import 'daos/share_inbox_dao.dart';
import 'daos/structure_dao.dart';
import 'daos/sync_state_dao.dart';
import 'daos/threads_dao.dart';
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
  int get schemaVersion => 1;

  @override
  MigrationStrategy get migration => MigrationStrategy(
    onCreate: (m) => m.createAll(),
    // Version 1 has no step yet. From version 2 on, `dart run drift_dev make-migrations`
    // writes `database.steps.dart` and this becomes `stepByStep(...)` with the steps built
    // from MigrationRules (docs/DECISIONS.md D-M4-12-6). Until then any other stored version
    // (a database of a newer app) cannot be opened; the app wipes it and resyncs.
    onUpgrade: (m, from, to) async => throw UnsupportedError(
      'no migration of the local database from version $from to $to',
    ),
  );

  /// The mirror tables of 15 6.1 that a resync and a migration may discard: all of them except
  /// `drafts`, which hold unsent work (15 6.2, 04 7.3).
  List<TableInfo<Table, Object?>> get mirrorTables => [
    spaces,
    userSpaceStates,
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
}
