// SPDX-License-Identifier: Apache-2.0

// Several writes put into the outbox together (FEAT-NEW-CHAT-ATTACH, ruling FNCA/2): the
// uploads of a new post and the post that attaches them are stored in one transaction, so a
// crash between them leaves either all of them or none; an entry of the batch may wait for an
// earlier one of the same batch, and the batch is sent like single entries.

import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late Harness h;
  late ScriptedExecutor uploads;
  late ScriptedExecutor posts;
  setUp(() {
    h = Harness.open();
    uploads = h.fake('upload', lane: (_) => 'upload:forum');
    posts = h.fake('createThread', lane: (_) => 'forum:forum');
  });
  tearDown(() => h.close());

  const u1 = '0192f4a0-7b1c-7000-8a00-0000000000c1';
  const u2 = '0192f4a0-7b1c-7000-8a00-0000000000c2';
  const post = '0192f4a0-7b1c-7000-8a00-0000000000d1';

  List<OutboxRequest> batch() => [
    const OutboxRequest(kind: 'upload', id: u1, payload: {'name': 'a.png'}),
    const OutboxRequest(kind: 'upload', id: u2, payload: {'name': 'b.png'}),
    const OutboxRequest(
      kind: 'createThread',
      id: post,
      payload: {'content_md': ''},
      dependsOn: [u1, u2],
    ),
  ];

  test('stores every entry of the batch and the post waits for both '
      'uploads', () async {
    final stored = await h.outbox.enqueueAll(batch());
    expect([for (final e in stored) e.id], [u1, u2, post]);
    expect(stored.last.dependsOn, '$u1,$u2');
    expect(stored.every((e) => e.state == OutboxState.pending), isTrue);
    await h.outbox.start();
    await settle();
    expect(uploads.ids, [u1, u2], reason: 'one upload lane, in order');
    expect(posts.ids, [post], reason: 'after both uploads');
  });

  test('stores nothing when one entry of the batch cannot be stored', () async {
    // The ID of the post exists already: the insert of the third entry fails.
    await h.outbox.enqueue(
      const OutboxRequest(kind: 'createThread', id: post, payload: {}),
    );
    await expectLater(h.outbox.enqueueAll(batch()), throwsA(anything));
    expect(await h.entry(u1), isNull);
    expect(await h.entry(u2), isNull);
    expect((await h.db.outboxDao.allEntries()).single.id, post);
  });

  test('an entry that names an unknown entry fails visibly, as with '
      'enqueue', () async {
    final stored = await h.outbox.enqueueAll([
      const OutboxRequest(
        kind: 'createThread',
        id: post,
        payload: {},
        dependsOn: ['0192f4a0-7b1c-7000-8a00-0000000000ff'],
      ),
    ]);
    expect(stored.single.state, OutboxState.failed);
    expect(stored.single.lastErrorCode, 'dependency_discarded');
  });
}
