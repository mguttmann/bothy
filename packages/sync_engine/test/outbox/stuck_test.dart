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

  test('the limit comes from AppConfig', () async {
    final h = Harness.open(config: {'client.outbox_stuck_after': '30s'});
    addTearDown(h.close);
    await run(h, const Duration(seconds: 30));
  });
}
