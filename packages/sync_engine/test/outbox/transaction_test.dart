// SPDX-License-Identifier: Apache-2.0

// A change of the outbox over several rows is one database transaction: when a later step
// fails, the earlier ones roll back, so a failed entry and its replacement never stand side by
// side and a chain is never failed for an entry that stays (04 13.2 item 5, 04 13.3).

import 'package:local_store/local_store.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late Harness h;
  setUp(() => h = Harness.open());
  tearDown(() => h.close());

  /// Makes every delete of an outbox row fail, as a full disk or a closed database would.
  Future<void> refuseDeletes() => h.db.customStatement(
    'CREATE TEMP TRIGGER refuse_outbox_delete BEFORE DELETE ON outbox '
    "BEGIN SELECT RAISE(ABORT, 'refused'); END",
  );

  Future<OutboxEntry> failedEntry() async {
    h.fake('createMessage').then([problem(422, 'validation_failed')]);
    final e = await h.add('hello', thread: threadA);
    await h.outbox.start();
    await settle();
    expect((await h.entry(e.id))!.state, OutboxState.failed);
    return e;
  }

  test('a retry whose delete fails leaves only the failed entry', () async {
    final e = await failedEntry();
    await refuseDeletes();
    await expectLater(h.outbox.retry(e.id), throwsA(anything));
    final rows = await h.db.outboxDao.allEntries();
    expect([for (final r in rows) r.id], [e.id]);
    expect(rows.single.state, OutboxState.failed);
  });

  test(
    'an edit of a failed entry whose delete fails changes nothing',
    () async {
      final e = await failedEntry();
      await refuseDeletes();
      await expectLater(
        h.outbox.edit(e.id, {'content_md': 'edited'}),
        throwsA(anything),
      );
      final rows = await h.db.outboxDao.allEntries();
      expect([for (final r in rows) r.id], [e.id]);
    },
  );

  test(
    'a discard whose delete fails keeps its dependents as they were',
    () async {
      h.fake('createMessage');
      final parent = await h.add('parent', thread: threadA);
      final child = await h.add(
        'child',
        thread: threadB,
        dependsOn: [parent.id],
      );
      await refuseDeletes();
      await expectLater(h.outbox.discard(parent.id), throwsA(anything));
      expect((await h.entry(parent.id))!.state, OutboxState.pending);
      final kept = (await h.entry(child.id))!;
      expect(kept.state, OutboxState.pending);
      expect(kept.dependsOn, parent.id);
    },
  );
}
