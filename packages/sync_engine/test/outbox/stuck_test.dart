// SPDX-License-Identifier: Apache-2.0

// "Sending delayed" after `client.outbox_stuck_after` in the state waiting (15 7.5).

import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  Future<void> run(Harness h, Duration stuckAfter) async {
    h.fake('createMessage').then([
      problem(503, 'unavailable', retryAfter: const Duration(hours: 1)),
    ]);
    final e = await h.add('hello', thread: threadA);
    final labels = <OutboxStateLabel>[];
    final sub = h.outbox
        .watchThread(threadA)
        .listen((v) => labels.add(v.single.stateLabel));
    await h.outbox.start();
    await settle();
    expect(labels.last, OutboxStateLabel.waiting);
    await h.time.advance(stuckAfter - const Duration(seconds: 1));
    expect(labels.last, OutboxStateLabel.waiting);
    await h.time.advance(const Duration(seconds: 1));
    expect(
      labels.last,
      OutboxStateLabel.delayed,
      reason: 'no database change needed',
    );
    final view = (await h.outbox.watchThread(threadA).first).single;
    expect(view.id, e.id);
    expect(view.stateLabel, OutboxStateLabel.delayed);
    await sub.cancel();
  }

  test('an entry waiting longer than the default reports delayed', () async {
    final h = Harness.open();
    addTearDown(h.close);
    await run(h, const Duration(minutes: 2));
  });

  test('a view cancelled while it is computed arms no delay timer', () async {
    // The view reads the database before it arms the timer of "delayed"; a cancel in that
    // gap must not leave a timer behind (a widget test fails on a pending timer).
    final h = Harness.open();
    addTearDown(h.close);
    h.fake('createMessage').then([
      problem(503, 'unavailable', retryAfter: const Duration(hours: 1)),
    ]);
    final first = await h.add('hello', thread: threadA);
    // The view of an entry with a dependency reads that dependency first.
    await h.add('again', thread: threadA, dependsOn: [first.id]);
    await h.outbox.start();
    await settle();
    final idle = h.time.activeTimers;
    for (var turns = 0; turns < 400; turns++) {
      final sub = h.outbox.watchThread(threadA).listen((_) {});
      // Microtask by microtask, so one of the cancels falls between the read and the timer.
      for (var i = 0; i < turns; i++) {
        await Future<void>.value();
      }
      await sub.cancel();
      await settle();
      expect(h.time.activeTimers, idle, reason: 'cancel after $turns turns');
    }
  });

  test('the limit comes from AppConfig', () async {
    final h = Harness.open(config: {'client.outbox_stuck_after': '30s'});
    addTearDown(h.close);
    await run(h, const Duration(seconds: 30));
  });
}
