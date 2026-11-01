// SPDX-License-Identifier: Apache-2.0

// `ThreadsDao.advanceActivity` (brief FIX-autoarrange-delay): a new message moves the stored
// `last_activity_at` of its thread forward, never back, and leaves the version alone, so the
// next thread change of the server still applies by the version rule.

import 'package:local_store/local_store.dart';
import 'package:test/test.dart';

import 'support/fixtures.dart';

void main() {
  late LocalDatabase db;
  setUp(() => db = openInMemoryForTests());
  tearDown(() => db.close());

  test(
    'moves forward only, keeps the version, ignores unknown threads',
    () async {
      final dao = db.threadsDao;
      await dao.upsertIfNewer(thread(1));
      expect(await dao.advanceActivity(threadId(1), at(5)), isTrue);
      expect((await dao.byId(threadId(1)))!.lastActivityAt, at(5));
      expect((await dao.byId(threadId(1)))!.version.value, 1);

      expect(await dao.advanceActivity(threadId(1), at(3)), isFalse);
      expect(await dao.advanceActivity(threadId(1), at(5)), isFalse);
      expect((await dao.byId(threadId(1)))!.lastActivityAt, at(5));

      expect(await dao.advanceActivity(threadId(2), at(9)), isFalse);
      expect(await dao.byId(threadId(2)), isNull);

      // The next version of the server replaces the row as before.
      expect(
        await dao.upsertIfNewer(thread(1, version: 2, title: 'New')),
        isTrue,
      );
      expect((await dao.byId(threadId(1)))!.title, 'New');
    },
  );
}
