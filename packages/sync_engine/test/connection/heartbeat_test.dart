// SPDX-License-Identifier: Apache-2.0

// Heartbeat of 04 6.3 item 5: a random first delay, then one heartbeat per interval with
// `last_seq`; a missing acknowledgement closes and reconnects (brief M4-13, Tests).

import 'package:domain/domain.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

import '../support/fakes.dart';

/// Time from `ready` to the first heartbeat with random [seed].
Future<Duration> firstDelay(int seed) async {
  final h = Harness(seed: seed);
  try {
    final ch = await h.goLive();
    final readyAt = h.clock.now();
    for (var ms = 0; ms <= heartbeatInterval.inMilliseconds; ms += 100) {
      if (ch.sentOps.contains('heartbeat')) break;
      await h.clock.elapse(const Duration(milliseconds: 100));
    }
    expect(ch.sentOps, contains('heartbeat'));
    return h.clock.now().difference(readyAt);
  } finally {
    await h.dispose();
  }
}

void main() {
  test('the first heartbeat waits a random fraction of the interval', () async {
    final a = await firstDelay(1);
    final b = await firstDelay(2);
    for (final d in [a, b]) {
      expect(d, greaterThanOrEqualTo(Duration.zero));
      expect(d, lessThanOrEqualTo(heartbeatInterval));
    }
    expect(a, isNot(b), reason: 'two seeds give two delays');
  });

  test('later heartbeats come every interval with last_seq', () async {
    final listener = RecordingListener();
    final h = Harness(listener: listener);
    addTearDown(h.dispose);
    final ch = await h.goLive(cursor: 30);
    await h.clock.elapse(heartbeatInterval);
    expect(ch.sentOps.where((o) => o == 'heartbeat'), hasLength(1));
    expect(ch.lastData('heartbeat'), {'last_seq': 30});
    ch.push('heartbeat_ack', {'server_time': '2026-10-15T10:00:10.000Z'});
    listener.control!.lastProcessedSeq = const GlobalSeq(44);
    await h.clock.elapse(heartbeatInterval);
    expect(ch.sentOps.where((o) => o == 'heartbeat'), hasLength(2));
    expect(ch.lastData('heartbeat'), {'last_seq': 44});
    expect(ch.isClosed, isFalse);
  });

  test(
    'a missing heartbeat_ack closes with 1000 and reconnects with resume',
    () async {
      final h = Harness();
      addTearDown(h.dispose);
      final ch = await h.goLive();
      await h.clock.elapse(heartbeatInterval); // first heartbeat sent, no ack
      expect(ch.sentOps, contains('heartbeat'));
      expect(ch.isClosed, isFalse);
      await h.clock.elapse(heartbeatInterval);
      expect(ch.clientCloseCode, 1000);
      expect(h.states.map((s) => s.phase), contains(ConnectionPhase.backoff));
      await h.clock.elapse(const Duration(seconds: 2));
      expect(h.transport.channels, hasLength(2));
      expect((await h.answerHello())['op'], 'resume');
    },
  );

  test('no heartbeat before ready', () async {
    final h = Harness();
    addTearDown(h.dispose);
    final ch = await h.open();
    await h.answerHello();
    await h.clock.elapse(heartbeatInterval * 3);
    expect(ch.sentOps, ['identify']);
  });
}
