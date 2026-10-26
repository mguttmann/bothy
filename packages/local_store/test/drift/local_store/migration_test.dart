// SPDX-License-Identifier: Apache-2.0

// The migration steps of the local database (docs/DECISIONS.md D-M4-12-6): every stored schema
// version migrates to every later one and then equals the stored schema of that version;
// version 2 (M4-19) adds the local-only table `forum_view_states`, version 3 (M4-23) the
// local-only table `recent_targets`, version 4 (M4-36) the local-only table `host_key_pins` and
// version 5 (M5-08) the nullable column `instance.app_lock`; each keeps every row of the mirror. The preserved tables are checked by `survival_test.dart`.

import 'package:drift/drift.dart';
import 'package:drift_dev/api/migrations_native.dart';
import 'package:local_store/src/database.dart';
import 'package:test/test.dart';

import 'generated/schema.dart';
import 'generated/schema_v1.dart' as v1;
import 'generated/schema_v2.dart' as v2;
import 'generated/schema_v3.dart' as v3;
import 'generated/schema_v4.dart' as v4;
import 'generated/schema_v5.dart' as v5;

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
}
