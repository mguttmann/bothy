// SPDX-License-Identifier: Apache-2.0

// FIFO per thread, threads in parallel (04 13.2 item 2, 15 5).

import 'dart:async';

import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late Harness h;
  setUp(() => h = Harness.open());
  tearDown(() => h.close());

  test('a thread sends strictly in order, other threads in parallel', () async {
    final exec = h.fake('createMessage');
    final answers = List.generate(5, (_) => Completer<OutboxResponse>());
    exec.then(answers);
    final a1 = await h.add('a1', thread: threadA);
    final b1 = await h.add('b1', thread: threadB);
    final a2 = await h.add('a2', thread: threadA);
    final b2 = await h.add('b2', thread: threadB);
    final a3 = await h.add('a3', thread: threadA);

    await h.outbox.start();
    await settle();
    // One request per thread in flight: a1 and b1 run in parallel.
    expect(exec.ids, [a1.id, b1.id]);
    expect(exec.inFlight, 2);

    answers[0].complete(const OutboxResponse());
    await settle();
    expect(exec.ids, [
      a1.id,
      b1.id,
      a2.id,
    ], reason: 'a2 only after the answer to a1');

    answers[1].complete(const OutboxResponse());
    await settle();
    expect(exec.ids, [a1.id, b1.id, a2.id, b2.id]);

    answers[2].complete(const OutboxResponse());
    answers[3].complete(const OutboxResponse());
    await settle();
    expect(exec.ids, [a1.id, b1.id, a2.id, b2.id, a3.id]);
    answers[4].complete(const OutboxResponse());
    await settle();
    expect(await h.db.outboxDao.entriesFor(threadA), isEmpty);
    expect(await h.db.outboxDao.entriesFor(threadB), isEmpty);
    expect(h.confirmed, [a1.id, b1.id, a2.id, b2.id, a3.id]);
  });

  test(
    'a write outside a thread runs in the lane its executor names',
    () async {
      final settings = h.fake(
        'putMySetting',
        lane: (e) => 'setting:${e.payload}',
      );
      final first = Completer<OutboxResponse>();
      settings.then([first]);
      final s1 = await h.outbox.enqueue(
        const OutboxRequest(kind: 'putMySetting', payload: {'key': 'ui.theme'}),
      );
      final s2 = await h.outbox.enqueue(
        const OutboxRequest(
          kind: 'putMySetting',
          payload: {'key': 'ui.density'},
        ),
      );
      final s3 = await h.outbox.enqueue(
        const OutboxRequest(kind: 'putMySetting', payload: {'key': 'ui.theme'}),
      );
      await h.outbox.start();
      await settle();
      expect(settings.ids, [
        s1.id,
        s2.id,
      ], reason: 'other keys run in parallel');
      first.complete(const OutboxResponse());
      await settle();
      expect(settings.ids, [s1.id, s2.id, s3.id]);
    },
  );

  test('the depth counts the entries that are not final', () async {
    h.fake('createMessage').then([problem(400, 'validation_failed')]);
    final depths = <int>[];
    final sub = h.outbox.depth.listen(depths.add);
    await settle();
    await h.add('a1', thread: threadA);
    await h.add('a2', thread: threadA);
    await settle();
    expect(depths.last, 2);
    await h.outbox.start();
    await settle();
    expect(depths.last, 0, reason: 'a1 failed, a2 confirmed');
    expect(depths.first, 0);
    await sub.cancel();
  });
}
