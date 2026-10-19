// SPDX-License-Identifier: Apache-2.0

// The migration steps of the local database (docs/DECISIONS.md D-M4-12-6): every stored schema
// version migrates to every later one and then equals the stored schema of that version;
// version 2 (M4-19) adds the local-only table `forum_view_states` and keeps every row of the
// mirror. The preserved tables are checked by `survival_test.dart`.

import 'package:drift/drift.dart';
import 'package:drift_dev/api/migrations_native.dart';
import 'package:local_store/src/database.dart';
import 'package:test/test.dart';

import 'generated/schema.dart';
import 'generated/schema_v1.dart' as v1;
import 'generated/schema_v2.dart' as v2;

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
}
