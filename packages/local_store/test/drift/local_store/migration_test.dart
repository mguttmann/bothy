// SPDX-License-Identifier: Apache-2.0

// The migration steps of the local database (docs/DECISIONS.md D-M4-12-6): every stored schema
// version migrates to every later one and then equals the stored schema of that version;
// version 2 (M4-19) adds the local-only table `forum_view_states`, version 3 (M4-23) the
// local-only table `recent_targets`, version 4 (M4-36) the local-only table `host_key_pins`,
// version 5 (M5-08) the nullable column `instance.app_lock` and version 6 (M4-40) the local-only
// tables `channel_view_states` and `shell_view_state` with the column `channels.project_bound`
// and a cleared cursor, version 7 (M4-42) the column `shell_view_state.channel_column_hidden`,
// version 8 (M4-45) the columns `shell_view_state.session_column_hidden` and
// `text_thread_session_shown`, version 9 (M4-46) the column
// `channel_view_states.posts_shown_in_full` and version 10 (FIX-device-mirroring-2) the
// local-only table `space_view_states`, filled from `user_space_states.collapsed`, and version
// 11 (M4-49) the column `shell_view_state.code_blocks_unwrapped`, version 12 (M4-51) the
// column `user_space_states.channel_order` and the mirror table `user_channel_states` with a
// cleared cursor, version 13 (M4-52) the local-only table `thread_seen_states`; each keeps every row of the mirror. The preserved
// tables are checked by
// `survival_test.dart`.

import 'package:drift/drift.dart';
import 'package:drift_dev/api/migrations_native.dart';
import 'package:local_store/src/database.dart';
import 'package:test/test.dart';

import 'generated/schema.dart';
import 'generated/schema_v1.dart' as v1;
import 'generated/schema_v10.dart' as v10;
import 'generated/schema_v11.dart' as v11;
import 'generated/schema_v12.dart' as v12;
import 'generated/schema_v13.dart' as v13;
import 'generated/schema_v2.dart' as v2;
import 'generated/schema_v3.dart' as v3;
import 'generated/schema_v4.dart' as v4;
import 'generated/schema_v5.dart' as v5;
import 'generated/schema_v6.dart' as v6;
import 'generated/schema_v7.dart' as v7;
import 'generated/schema_v8.dart' as v8;
import 'generated/schema_v9.dart' as v9;

void main() {
  driftRuntimeOptions.dontWarnAboutMultipleDatabases = true;
  late SchemaVerifier verifier;

  setUpAll(() => verifier = SchemaVerifier(GeneratedHelper()));

  const versions = GeneratedHelper.versions;
  for (final (i, from) in versions.indexed) {
    for (final to in versions.skip(i + 1)) {
      test(
        'the schema after the migration $from to $to is the stored one',
        () async {
          final schema = await verifier.schemaAt(from);
          final db = LocalDatabase(schema.newConnection());
          await verifier.migrateAndValidate(db, to);
          await db.close();
        },
      );
    }
  }

  test('the migration 1 to 2 keeps the sidebar states', () async {
    final row = v1.UserSpaceStatesData(
      spaceId: '0192f4a0-7b1c-7000-8000-000000000101',
      collapsed: 1,
      sortKey: 'a0V',
      version: 3,
    );
    await verifier.testWithDataIntegrity(
      oldVersion: 1,
      newVersion: 2,
      createOld: v1.DatabaseAtV1.new,
      createNew: v2.DatabaseAtV2.new,
      openTestedDatabase: LocalDatabase.new,
      createItems: (batch, oldDb) =>
          batch.insertAll(oldDb.userSpaceStates, [row]),
      validateItems: (newDb) async {
        expect(await newDb.select(newDb.userSpaceStates).get(), [
          v2.UserSpaceStatesData(
            spaceId: row.spaceId,
            collapsed: 1,
            sortKey: 'a0V',
            version: 3,
          ),
        ]);
        expect(await newDb.select(newDb.forumViewStates).get(), isEmpty);
      },
    );
  });

  test('the migration 2 to 3 keeps the forum view states and adds no recent target', () async {
    final row = v2.ForumViewStatesData(
      channelId: '0192f4a0-7b1c-7000-8000-000000000201',
      doneCollapsed: 0,
    );
    await verifier.testWithDataIntegrity(
      oldVersion: 2,
      newVersion: 3,
      createOld: v2.DatabaseAtV2.new,
      createNew: v3.DatabaseAtV3.new,
      openTestedDatabase: LocalDatabase.new,
      createItems: (batch, oldDb) =>
          batch.insertAll(oldDb.forumViewStates, [row]),
      validateItems: (newDb) async {
        expect(await newDb.select(newDb.forumViewStates).get(), [
          v3.ForumViewStatesData(channelId: row.channelId, doneCollapsed: 0),
        ]);
        expect(await newDb.select(newDb.recentTargets).get(), isEmpty);
      },
    );
  });

  test(
    'the migration 3 to 4 keeps the recent targets and adds no pin',
    () async {
      final row = v3.RecentTargetsData(
        kind: 'channel',
        targetId: '0192f4a0-7b1c-7000-8000-000000000201',
        visitedAt: 7,
      );
      await verifier.testWithDataIntegrity(
        oldVersion: 3,
        newVersion: 4,
        createOld: v3.DatabaseAtV3.new,
        createNew: v4.DatabaseAtV4.new,
        openTestedDatabase: LocalDatabase.new,
        createItems: (batch, oldDb) =>
            batch.insertAll(oldDb.recentTargets, [row]),
        validateItems: (newDb) async {
          expect(await newDb.select(newDb.recentTargets).get(), [
            v4.RecentTargetsData(
              kind: row.kind,
              targetId: row.targetId,
              visitedAt: row.visitedAt,
            ),
          ]);
          expect(await newDb.select(newDb.hostKeyPins).get(), isEmpty);
        },
      );
    },
  );

  test(
    'the migration 4 to 5 keeps the instance row without a policy',
    () async {
      final row = v4.InstanceData(
        instanceId: '0192f4a0-7b1c-7000-8000-000000000301',
        productName: 'Example Chat',
        brandingVersion: 2,
        localeDefault: 'en',
        specVersion: '1.0.0',
        features: '{}',
        limits: '{}',
      );
      await verifier.testWithDataIntegrity(
        oldVersion: 4,
        newVersion: 5,
        createOld: v4.DatabaseAtV4.new,
        createNew: v5.DatabaseAtV5.new,
        openTestedDatabase: LocalDatabase.new,
        createItems: (batch, oldDb) => batch.insertAll(oldDb.instance, [row]),
        validateItems: (newDb) async {
          expect(await newDb.select(newDb.instance).get(), [
            v5.InstanceData(
              instanceId: row.instanceId,
              productName: 'Example Chat',
              brandingVersion: 2,
              localeDefault: 'en',
              specVersion: '1.0.0',
              features: '{}',
              limits: '{}',
            ),
          ]);
        },
      );
    },
  );

  test('the migration 5 to 6 keeps the channels unbound, adds the view state '
      'tables and clears cursor and epoch', () async {
    final channel = v5.ChannelsData(
      id: '0192f4a0-7b1c-7000-8000-000000000201',
      spaceId: '0192f4a0-7b1c-7000-8000-000000000101',
      kind: 'forum',
      name: 'dev',
      topic: '',
      sortKey: 'a0',
      readOnly: 0,
      dmParticipantIds: '[]',
      createdAt: '2026-10-01T09:00:00.000Z',
      version: 4,
      settingsVersion: 2,
      level: 'write',
      capabilities: '[]',
    );
    final state = v5.SyncStateData(
      id: 1,
      cursor: 812,
      syncEpoch: 'epoch-3',
      headSeq: 812,
      permissionVersion: 7,
      lastSnapshotAt: 40,
    );
    await verifier.testWithDataIntegrity(
      oldVersion: 5,
      newVersion: 6,
      createOld: v5.DatabaseAtV5.new,
      createNew: v6.DatabaseAtV6.new,
      openTestedDatabase: LocalDatabase.new,
      createItems: (batch, oldDb) {
        batch
          ..insertAll(oldDb.channels, [channel])
          ..insertAll(oldDb.syncState, [state]);
      },
      validateItems: (newDb) async {
        expect(await newDb.select(newDb.channels).get(), [
          v6.ChannelsData(
            id: channel.id,
            spaceId: channel.spaceId,
            kind: 'forum',
            name: 'dev',
            topic: '',
            sortKey: 'a0',
            readOnly: 0,
            dmParticipantIds: '[]',
            createdAt: '2026-10-01T09:00:00.000Z',
            version: 4,
            settingsVersion: 2,
            projectBound: 0,
            level: 'write',
            capabilities: '[]',
          ),
        ]);
        // The next connection loads a whole snapshot that fills project_bound (04 7.3).
        expect(await newDb.select(newDb.syncState).get(), [
          const v6.SyncStateData(
            id: 1,
            headSeq: 812,
            permissionVersion: 7,
            lastSnapshotAt: 40,
          ),
        ]);
        expect(await newDb.select(newDb.channelViewStates).get(), isEmpty);
        expect(await newDb.select(newDb.shellViewState).get(), isEmpty);
      },
    );
  });

  test('the migration 6 to 7 keeps the pinned overview and shows the channel '
      'column', () async {
    const row = v6.ShellViewStateData(id: 1, overviewPinned: 1);
    await verifier.testWithDataIntegrity(
      oldVersion: 6,
      newVersion: 7,
      createOld: v6.DatabaseAtV6.new,
      createNew: v7.DatabaseAtV7.new,
      openTestedDatabase: LocalDatabase.new,
      createItems: (batch, oldDb) =>
          batch.insertAll(oldDb.shellViewState, [row]),
      validateItems: (newDb) async {
        expect(await newDb.select(newDb.shellViewState).get(), [
          const v7.ShellViewStateData(
            id: 1,
            overviewPinned: 1,
            channelColumnHidden: 0,
          ),
        ]);
      },
    );
  });

  test('the migration 7 to 8 keeps the pin and the hidden channel column and '
      'keeps the default session column of both layouts', () async {
    const row = v7.ShellViewStateData(
      id: 1,
      overviewPinned: 1,
      channelColumnHidden: 1,
    );
    await verifier.testWithDataIntegrity(
      oldVersion: 7,
      newVersion: 8,
      createOld: v7.DatabaseAtV7.new,
      createNew: v8.DatabaseAtV8.new,
      openTestedDatabase: LocalDatabase.new,
      createItems: (batch, oldDb) =>
          batch.insertAll(oldDb.shellViewState, [row]),
      validateItems: (newDb) async {
        expect(await newDb.select(newDb.shellViewState).get(), [
          const v8.ShellViewStateData(
            id: 1,
            overviewPinned: 1,
            channelColumnHidden: 1,
            sessionColumnHidden: 0,
            textThreadSessionShown: 0,
          ),
        ]);
      },
    );
  });

  test('the migration 8 to 9 keeps the collapsed groups and shows every forum '
      'group with the limit', () async {
    const rows = [
      v8.ChannelViewStatesData(channelId: 'a', groupCollapsed: 1),
      v8.ChannelViewStatesData(channelId: 'b', groupCollapsed: 0),
    ];
    await verifier.testWithDataIntegrity(
      oldVersion: 8,
      newVersion: 9,
      createOld: v8.DatabaseAtV8.new,
      createNew: v9.DatabaseAtV9.new,
      openTestedDatabase: LocalDatabase.new,
      createItems: (batch, oldDb) =>
          batch.insertAll(oldDb.channelViewStates, rows),
      validateItems: (newDb) async {
        expect(await newDb.select(newDb.channelViewStates).get(), [
          const v9.ChannelViewStatesData(
            channelId: 'a',
            groupCollapsed: 1,
            postsShownInFull: 0,
          ),
          const v9.ChannelViewStatesData(
            channelId: 'b',
            groupCollapsed: 0,
            postsShownInFull: 0,
          ),
        ]);
      },
    );
  });

  test('the migration 9 to 10 keeps the collapse state of every space on this '
      'device', () async {
    const rows = [
      v9.UserSpaceStatesData(spaceId: 'a', collapsed: 1, version: 3),
      v9.UserSpaceStatesData(
        spaceId: 'b',
        collapsed: 0,
        sortKey: 'a0V',
        version: 1,
      ),
    ];
    await verifier.testWithDataIntegrity(
      oldVersion: 9,
      newVersion: 10,
      createOld: v9.DatabaseAtV9.new,
      createNew: v10.DatabaseAtV10.new,
      openTestedDatabase: LocalDatabase.new,
      createItems: (batch, oldDb) =>
          batch.insertAll(oldDb.userSpaceStates, rows),
      validateItems: (newDb) async {
        expect(await newDb.select(newDb.userSpaceStates).get(), hasLength(2));
        expect(await newDb.select(newDb.spaceViewStates).get(), [
          const v10.SpaceViewStatesData(spaceId: 'a', collapsed: 1),
          const v10.SpaceViewStatesData(spaceId: 'b', collapsed: 0),
        ]);
      },
    );
  });

  test('the migration 10 to 11 keeps the view state of the shell and lets '
      'code blocks wrap', () async {
    const row = v10.ShellViewStateData(
      id: 1,
      overviewPinned: 1,
      channelColumnHidden: 1,
      sessionColumnHidden: 1,
      textThreadSessionShown: 1,
    );
    await verifier.testWithDataIntegrity(
      oldVersion: 10,
      newVersion: 11,
      createOld: v10.DatabaseAtV10.new,
      createNew: v11.DatabaseAtV11.new,
      openTestedDatabase: LocalDatabase.new,
      createItems: (batch, oldDb) =>
          batch.insertAll(oldDb.shellViewState, [row]),
      validateItems: (newDb) async {
        expect(await newDb.select(newDb.shellViewState).get(), [
          const v11.ShellViewStateData(
            id: 1,
            overviewPinned: 1,
            channelColumnHidden: 1,
            sessionColumnHidden: 1,
            textThreadSessionShown: 1,
            codeBlocksUnwrapped: 0,
          ),
        ]);
      },
    );
  });

  test(
    'the migration 11 to 12 keeps the sidebar states with the default '
    'channel order, adds no thread order and clears cursor and epoch',
    () async {
      const rows = [
        v11.UserSpaceStatesData(spaceId: 'a', collapsed: 1, version: 3),
        v11.UserSpaceStatesData(
          spaceId: 'b',
          collapsed: 0,
          sortKey: 'a0V',
          version: 1,
          updatedByDeviceId: 'd',
        ),
      ];
      const state = v11.SyncStateData(
        id: 1,
        cursor: 812,
        syncEpoch: 'epoch-3',
        headSeq: 812,
        permissionVersion: 7,
        lastSnapshotAt: 40,
      );
      await verifier.testWithDataIntegrity(
        oldVersion: 11,
        newVersion: 12,
        createOld: v11.DatabaseAtV11.new,
        createNew: v12.DatabaseAtV12.new,
        openTestedDatabase: LocalDatabase.new,
        createItems: (batch, oldDb) {
          batch
            ..insertAll(oldDb.userSpaceStates, rows)
            ..insertAll(oldDb.syncState, [state]);
        },
        validateItems: (newDb) async {
          expect(await newDb.select(newDb.userSpaceStates).get(), [
            const v12.UserSpaceStatesData(
              spaceId: 'a',
              collapsed: 1,
              version: 3,
            ),
            const v12.UserSpaceStatesData(
              spaceId: 'b',
              collapsed: 0,
              sortKey: 'a0V',
              version: 1,
              updatedByDeviceId: 'd',
            ),
          ]);
          expect(await newDb.select(newDb.userChannelStates).get(), isEmpty);
          // The next connection loads a whole snapshot with the orders of other devices (04 7.3).
          expect(await newDb.select(newDb.syncState).get(), [
            const v12.SyncStateData(
              id: 1,
              headSeq: 812,
              permissionVersion: 7,
              lastSnapshotAt: 40,
            ),
          ]);
        },
      );
    },
  );

  test(
    'the migration 12 to 13 adds an empty seen table and keeps the cursor',
    () async {
      const state = v12.SyncStateData(
        id: 1,
        cursor: 812,
        syncEpoch: 'epoch-3',
        headSeq: 812,
        permissionVersion: 7,
        lastSnapshotAt: 40,
      );
      await verifier.testWithDataIntegrity(
        oldVersion: 12,
        newVersion: 13,
        createOld: v12.DatabaseAtV12.new,
        createNew: v13.DatabaseAtV13.new,
        openTestedDatabase: LocalDatabase.new,
        createItems: (batch, oldDb) =>
            batch.insertAll(oldDb.syncState, [state]),
        validateItems: (newDb) async {
          expect(await newDb.select(newDb.threadSeenStates).get(), isEmpty);
          // Local-only view state: the mirror and the cursor stay (no snapshot needed).
          expect(await newDb.select(newDb.syncState).get(), [
            const v13.SyncStateData(
              id: 1,
              cursor: 812,
              syncEpoch: 'epoch-3',
              headSeq: 812,
              permissionVersion: 7,
              lastSnapshotAt: 40,
            ),
          ]);
        },
      );
    },
  );
}
