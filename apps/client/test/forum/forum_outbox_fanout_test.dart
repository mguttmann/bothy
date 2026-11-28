// SPDX-License-Identifier: Apache-2.0

// Typing the first message of a new post (D-M4-20-2; AUD-PKG-Q03, #1252): every key press
// rewrites the draft entry, but the forum's waiting posts give no new list for it and the
// unsent draft text gives a value only when the text changes, so the sidebar does not rebuild
// for unchanged rows.

import 'package:client/app/providers/session.dart';
import 'package:client/forum/providers.dart';
import 'package:client/sync/thread_ops.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';

import '../support/structure.dart';

void main() {
  test('a draft write gives no new list of waiting posts', () async {
    final db = openInMemoryForTests();
    final container = ProviderContainer(
      overrides: [localDatabaseProvider.overrideWithValue(db)],
    );
    final forum = channelId(1).value;
    final posts = <List<PendingPost>>[];
    final drafts = <String?>[];
    container
      ..listen(pendingPostsProvider(forum), (_, next) {
        if (next.value case final value?) posts.add(value);
      }, fireImmediately: true)
      ..listen(unsentChannelDraftProvider(forum), (_, next) {
        if (next.hasValue) drafts.add(next.value);
      }, fireImmediately: true);
    await pumpEventQueue();
    await db.outboxDao.enqueue(
      id: 'p1',
      kind: ThreadOps.createThread,
      payload: '{"channel_id":"$forum","title":"Printer"}',
    );
    await pumpEventQueue();
    await db.outboxDao.enqueue(
      id: 'd1',
      kind: ThreadOps.putChannelDraft,
      payload: '{"channel_id":"$forum","content_md":"h"}',
    );
    await pumpEventQueue();
    for (final text in ['he', 'hel', 'hel']) {
      await db.outboxDao.updatePayload(
        'd1',
        '{"channel_id":"$forum","content_md":"$text"}',
      );
      await pumpEventQueue();
    }
    expect([for (final p in posts) p.length], [0, 1]);
    expect(drafts, [null, 'h', 'he', 'hel']);
    container.dispose();
    await db.close();
  });
}
