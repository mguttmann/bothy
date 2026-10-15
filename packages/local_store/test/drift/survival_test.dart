// SPDX-License-Identifier: Apache-2.0

// The migration rule of 15 6.2 (Design note 4, docs/DECISIONS.md D-M4-12-6): outbox, drafts and
// share inbox survive every migration. Each pair of stored schema versions (`from < to`) is
// migrated with fixture rows in the preserved tables, which must be equal afterwards; with only
// version 1 the pair loop is empty, and the round trip of version 1 (create, fill, close,
// reopen with the key, compare) runs instead. A new schema version adds its fixture rows to
// `_fixtureRows`; the test fails when a stored version has none.

import 'dart:io';

import 'package:drift_dev/api/migrations_native.dart';
import 'package:local_store/local_store.dart';
import 'package:sqlite3/sqlite3.dart' show Database;
import 'package:test/test.dart';

import '../support/fixtures.dart';
import 'local_store/generated/schema.dart';

/// Fixture rows of the preserved tables per stored schema version, as column to value.
final Map<int, Map<String, List<Map<String, Object?>>>> _fixtureRows = {
  1: {
    'outbox': [
      {
        'id': uuid(1),
        'kind': 'postMessage',
        'thread_id': uuid(0x301),
        'depends_on': uuid(2),
        'payload': '{"content_md":"Hello from Alex"}',
        'expected_version': 3,
        'state': 'failed',
        'attempts': 2,
        'next_attempt_at': 1000,
        'last_error_code': 'conflict',
        'last_error_reason': 'version_mismatch',
        'conflict_current': '{"version":4}',
        'created_at': 10,
        'state_changed_at': 20,
      },
      {
        'id': uuid(2),
        'kind': 'createUpload',
        'thread_id': null,
        'depends_on': null,
        'payload': '{}',
        'expected_version': null,
        'state': 'pending',
        'attempts': 0,
        'next_attempt_at': null,
        'last_error_code': null,
        'last_error_reason': null,
        'conflict_current': null,
        'created_at': 5,
        'state_changed_at': 5,
      },
    ],
    'drafts': [
      {
        'id': uuid(0x401),
        'thread_id': uuid(0x301),
        'channel_id': uuid(0x201),
        'content_md': 'Draft by Sam',
        'reply_to_message_id': null,
        'file_ids': '[]',
        'version': 2,
        'updated_by_device_id': uuid(0x901),
        'updated_at': '2026-10-01T09:00:00.000Z',
      },
    ],
    'share_inbox': [
      {
        'id': uuid(3),
        'target_thread_id': null,
        'payload': '{"text":"From Robin"}',
        'file_paths': '["share/photo.png"]',
        'created_at': 30,
      },
    ],
  },
};

void _insert(Database raw, Map<String, List<Map<String, Object?>>> rows) {
  for (final MapEntry(key: table, value: list) in rows.entries) {
    for (final row in list) {
      final columns = row.keys.join(', ');
      final marks = List.filled(row.length, '?').join(', ');
      raw.execute(
        'INSERT INTO $table ($columns) VALUES ($marks)',
        row.values.toList(),
      );
    }
  }
}

Future<List<Map<String, Object?>>> _rows(
  LocalDatabase db,
  String table,
) async => [
  for (final row
      in await db.customSelect('SELECT * FROM $table ORDER BY id').get())
    row.data,
];

void main() {
  final verifier = SchemaVerifier(GeneratedHelper());

  test(
    'every stored schema version has fixture rows for every preserved table',
    () {
      for (final version in GeneratedHelper.versions) {
        expect(
          _fixtureRows[version]?.keys.toSet(),
          MigrationRules.preservedTables,
        );
      }
    },
  );

  test('the database equals the stored schema of its version', () async {
    final schema = await verifier.schemaAt(GeneratedHelper.versions.last);
    final db = LocalDatabase(schema.newConnection());
    await verifier.migrateAndValidate(db, GeneratedHelper.versions.last);
    await db.close();
  });

  for (final from in GeneratedHelper.versions) {
    for (final to in GeneratedHelper.versions.where((v) => v >= from)) {
      test(
        'outbox, drafts and share inbox survive the migration $from to $to',
        () async {
          final schema = await verifier.schemaAt(from);
          _insert(schema.rawDatabase, _fixtureRows[from]!);
          final db = LocalDatabase(schema.newConnection());
          await verifier.migrateAndValidate(db, to);
          for (final table in MigrationRules.preservedTables) {
            final after = await _rows(db, table);
            final expected = _fixtureRows[from]![table]!.toList()
              ..sort(
                (a, b) => (a['id']! as String).compareTo(b['id']! as String),
              );
            expect(after, hasLength(expected.length), reason: table);
            for (var i = 0; i < expected.length; i++) {
              // A later version may add columns; every column of the old row keeps its value.
              for (final MapEntry(:key, :value) in expected[i].entries) {
                expect(after[i][key], value, reason: '$table.$key');
              }
            }
          }
          await db.close();
        },
      );
    }
  }

  test('round trip of version 1: create, fill, close, reopen with the key, compare', () async {
    final dir = await tempDir();
    addTearDown(() => dir.delete(recursive: true));
    final file = File('${dir.path}/store.db');
    final keys = FixedKeySource(testKeyA);

    var db = await openLocalStore(file: file, keys: keys, inBackground: false);
    await db.outboxDao.enqueue(
      id: uuid(1),
      kind: 'postMessage',
      threadId: threadId(1),
      payload: '{"content_md":"Hello from Alex"}',
    );
    await db.outboxDao.recordFailure(
      uuid(1),
      code: 'conflict',
      reason: 'version_mismatch',
      conflictCurrent: '{"version":4}',
    );
    await db.personalDao.upsertDraftIfNewer(draft(1));
    await db.shareInboxDao.add(
      ShareInboxEntry(
        id: uuid(2),
        payload: '{"text":"From Robin"}',
        filePaths: const ['share/photo.png'],
        createdAt: at(3),
      ),
    );
    final outbox = await db.outboxDao.entriesFor(threadId(1));
    final drafts = await db.personalDao.allDrafts();
    final shares = await db.shareInboxDao.all();
    await db.close();

    db = await openLocalStore(file: file, keys: keys, inBackground: false);
    expect(await db.outboxDao.entriesFor(threadId(1)), outbox);
    expect(await db.personalDao.allDrafts(), drafts);
    expect(await db.shareInboxDao.all(), shares);
    expect(outbox, hasLength(1));
    expect(drafts, hasLength(1));
    expect(shares, hasLength(1));
    await db.close();
  });
}
