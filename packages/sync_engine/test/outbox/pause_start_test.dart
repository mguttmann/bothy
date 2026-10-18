// SPDX-License-Identifier: Apache-2.0

// The outbox sends only while started, after `catchup.complete` (04 13.4).

import 'dart:async';

import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late Harness h;
  setUp(() => h = Harness.open());
  tearDown(() => h.close());

  test('nothing is sent before start; start sends in FIFO order', () async {
    final exec = h.fake('createMessage');
    final a1 = await h.add('a1', thread: threadA);
    final a2 = await h.add('a2', thread: threadA);
    await settle();
    expect(exec.calls, isEmpty);
    await h.outbox.start();
    await settle();
    expect(exec.ids, [a1.id, a2.id]);
  });

  test(
    'pause lets a running request finish and starts none; start resumes',
    () async {
      final exec = h.fake('createMessage');
      final held = Completer<OutboxResponse>();
      exec.then([held]);
      final a1 = await h.add('a1', thread: threadA);
      final a2 = await h.add('a2', thread: threadA);
      await h.outbox.start();
      await settle();
      await h.outbox.pause();
      held.complete(const OutboxResponse());
      await settle();
      expect(
        await h.entry(a1.id),
        isNull,
        reason: 'the running request finished',
      );
      final a3 = await h.add('a3', thread: threadA);
      await h.time.advance(const Duration(minutes: 5));
      expect(exec.ids, [a1.id]);
      expect((await h.entry(a2.id))!.state, OutboxState.pending);

      await h.outbox.start();
      await settle();
      expect(exec.ids, [a1.id, a2.id, a3.id]);
    },
  );

  test('a retry that falls due while paused waits for start', () async {
    final exec = h.fake('createMessage')..then([problem(500, 'internal')]);
    final e = await h.add('a1', thread: threadA);
    await h.outbox.start();
    await settle();
    await h.outbox.pause();
    await h.time.advance(const Duration(minutes: 10));
    expect(exec.ids, [e.id]);
    await h.outbox.start();
    await settle();
    expect(exec.ids, [e.id, e.id]);
  });
}
