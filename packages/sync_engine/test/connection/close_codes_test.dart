// SPDX-License-Identifier: Apache-2.0

// One test per close code of 04 6.4: a live connection is closed by the server with the code,
// and the test checks the next phase and reason, the waiting time before the next attempt, the
// next frame after `hello` and the side effects (brief M4-13, Tests).

import 'package:auth/auth.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

import '../support/fakes.dart';

/// What happened after the close.
class Observed {
  Observed(this.state, this.wait, this.nextOp);

  /// The state right after the close was handled.
  final ConnectionState state;

  /// Time from the close to the next connect, or null without one.
  final Duration? wait;

  /// The first op the client sent on the next connection after `hello`.
  final String? nextOp;
}

/// Brings [h] live, closes with [code] and [reason], lets 10 s pass (every wait of the table
/// is shorter; the fake clock opens the next connection exactly at the end of its wait) and
/// answers the `hello` of the next connection; without one, 2 more minutes must pass without
/// a connection.
Future<Observed> closeLive(Harness h, int? code, {String? reason}) async {
  final ch = await h.goLive();
  expect(h.phase, ConnectionPhase.live);
  final closedAt = h.clock.now();
  ch.serverClose(code, reason);
  await settle();
  final state = h.connection.state;
  await h.clock.elapse(const Duration(seconds: 10));
  if (h.transport.channels.length == 1) {
    await h.clock.elapse(const Duration(minutes: 2));
    if (h.transport.channels.length == 1) return Observed(state, null, null);
  }
  final next = h.transport.last;
  final wait = next.openedAt.difference(closedAt);
  final sent = await h.answerHello();
  return Observed(state, wait, sent['op']! as String);
}

Matcher within(Duration max) => predicate<Duration?>(
  (d) => d != null && d >= Duration.zero && d <= max,
  'a wait within [0, $max]',
);

void main() {
  late Harness h;
  setUp(() => h = Harness());
  tearDown(() => h.dispose());

  const base = Duration(seconds: 1); // sync.reconnect_backoff_base (default)

  group('backoff and resume', () {
    for (final code in [1001, 4000, 4001, 4002, 4003, 4005, 4009, 4014]) {
      test('$code backs off within [0, base] and resumes', () async {
        final o = await closeLive(h, code);
        expect(o.state.phase, ConnectionPhase.backoff);
        expect(o.state.reason, ConnectionReason.closedByServer(code));
        expect(o.wait, within(base));
        expect(o.nextOp, 'resume');
        expect(h.refreshes, 0);
      });
    }

    test('an unknown code of 4000 to 4099 backs off', () async {
      final o = await closeLive(h, 4099);
      expect(o.state.phase, ConnectionPhase.backoff);
      expect(o.wait, within(base));
      expect(o.nextOp, 'resume');
    });

    test('a drop without a close code backs off and resumes', () async {
      final o = await closeLive(h, null);
      expect(o.state.phase, ConnectionPhase.backoff);
      expect(o.state.reason, ConnectionReason.none);
      expect(o.wait, within(base));
      expect(o.nextOp, 'resume');
    });
  });

  test('1000 in the foreground reconnects with backoff', () async {
    final o = await closeLive(h, 1000);
    expect(o.state.phase, ConnectionPhase.backoff);
    expect(o.state.reason, ConnectionReason.closedByServer(1000));
    expect(o.wait, within(base));
    expect(o.nextOp, 'resume');
  });

  test(
    '1000 in the background does not reconnect until the foreground',
    () async {
      await h.dispose();
      h = Harness(keepConnectedInBackground: true);
      final ch = await h.goLive();
      h.lifecycle.set(AppActivity.background);
      await settle();
      expect(h.phase, ConnectionPhase.live, reason: 'desktop stays connected');
      ch.serverClose(1000);
      await settle();
      expect(h.phase, ConnectionPhase.offline);
      expect(h.connection.state.reason, ConnectionReason.closedByServer(1000));
      await h.clock.elapse(const Duration(seconds: 30));
      expect(h.transport.channels, hasLength(1));
      h.lifecycle.set(AppActivity.foreground);
      await settle();
      expect(h.transport.channels, hasLength(2));
      expect((await h.answerHello())['op'], 'resume');
    },
  );

  test('4004 renews the token once and reconnects at once', () async {
    final o = await closeLive(h, 4004);
    expect(h.refreshes, 1);
    expect(o.wait, Duration.zero);
    expect(o.nextOp, 'resume');
    final d = h.transport.last.lastData('resume')!;
    expect(tokenTag(d['access_token']! as String), 'A2');
  });

  test('4004 with a failed refresh signs out', () async {
    h.api.routes['POST /auth/token/refresh'] = (_) {
      h.refreshes++;
      return FakeReply.problem(401, 'unauthenticated');
    };
    final o = await closeLive(h, 4004);
    expect(h.refreshes, 1);
    expect(o.state.phase, ConnectionPhase.signedOut);
    expect(o.wait, isNull);
  });

  test('4006 connects with identify', () async {
    final o = await closeLive(h, 4006);
    expect(o.state.phase, ConnectionPhase.backoff);
    expect(o.wait, within(base));
    expect(o.nextOp, 'identify');
  });

  test('4007 shows the update and never reconnects', () async {
    final o = await closeLive(h, 4007);
    expect(o.state.phase, ConnectionPhase.updateRequired);
    expect(o.state.reason, ConnectionReason.protocolUnsupported);
    expect(o.wait, isNull);
  });

  test('4008 waits for retry_after of the close reason', () async {
    final o = await closeLive(h, 4008, reason: 'retry_after=7');
    expect(o.state.phase, ConnectionPhase.backoff);
    expect(o.state.reason, ConnectionReason.rateLimited);
    expect(o.wait, const Duration(seconds: 7));
    expect(o.nextOp, 'resume');
  });

  test('4008 without a readable wait uses the normal backoff', () async {
    final o = await closeLive(h, 4008, reason: 'slow down');
    expect(o.state.reason, ConnectionReason.rateLimited);
    expect(o.wait, within(base));
  });

  test('4011 renews the token, then resumes', () async {
    final o = await closeLive(h, 4011);
    expect(h.refreshes, 1);
    expect(o.wait, Duration.zero);
    expect(o.nextOp, 'resume');
  });

  test('a second 4011 before resumed waits the backoff', () async {
    final ch = await h.goLive();
    ch.serverClose(4011);
    await settle();
    expect(h.transport.channels, hasLength(2));
    await h.answerHello();
    final closedAt = h.clock.now();
    h.transport.last.serverClose(4011);
    await settle();
    expect(h.refreshes, 2);
    expect(h.phase, ConnectionPhase.backoff);
    while (h.transport.channels.length == 2) {
      await h.clock.elapse(const Duration(milliseconds: 50));
    }
    expect(h.transport.last.openedAt.difference(closedAt), within(base * 2));
  });

  test('4012 signs out with deletion of local data and stays down', () async {
    final o = await closeLive(h, 4012);
    expect(o.state.phase, ConnectionPhase.signedOut);
    expect(o.state.reason, ConnectionReason.revoked);
    expect(o.wait, isNull);
    final out = h.authEvents.whereType<SignedOut>().single;
    expect(out.reason, SignOutReason.sessionRevoked);
    expect(out.deleteLocalData, isTrue);
  });

  test('4013 reconnects within [0, base] and resumes', () async {
    final o = await closeLive(h, 4013);
    expect(o.state.phase, ConnectionPhase.backoff);
    expect(o.state.reason, ConnectionReason.closedByServer(4013));
    expect(o.wait, within(base));
    expect(o.nextOp, 'resume');
  });

  test('4015 is unexpected and does not reconnect', () async {
    final o = await closeLive(h, 4015);
    expect(o.state.phase, ConnectionPhase.offline);
    expect(o.state.reason, ConnectionReason.closedByServer(4015));
    expect(o.wait, isNull);
    expect(h.log.any((l) => l.contains('unexpected close code 4015')), isTrue);
  });

  test('4016 waits for the pending device poll', () async {
    var polls = 0;
    h.api.routes['GET /devices/$deviceId'] = (_) {
      polls++;
      return FakeReply(200, deviceBody(polls < 2 ? 'pending' : 'active'));
    };
    final ch = await h.goLive();
    ch.serverClose(4016);
    await settle();
    expect(h.phase, ConnectionPhase.offline);
    expect(h.connection.state.reason, ConnectionReason.devicePending);
    expect(polls, 1);
    expect(h.transport.channels, hasLength(1));
    h.network.set(true);
    await settle();
    expect(h.transport.channels, hasLength(1), reason: 'network does not wake');
    await h.clock.elapse(const Duration(seconds: 10));
    expect(polls, 2);
    expect(h.transport.channels, hasLength(2));
    expect((await h.answerHello())['op'], 'resume');
  });

  test('4017 waits for the foreground', () async {
    final o0 = await h.goLive();
    o0.serverClose(4017);
    await settle();
    expect(h.phase, ConnectionPhase.offline);
    expect(h.connection.state.reason, ConnectionReason.deviceReplaced);
    await h.clock.elapse(const Duration(seconds: 30));
    h.network.set(true);
    await settle();
    expect(h.transport.channels, hasLength(1));
    h.lifecycle.set(AppActivity.background);
    h.lifecycle.set(AppActivity.foreground);
    await settle();
    expect(h.transport.channels, hasLength(2));
    expect((await h.answerHello())['op'], 'resume');
  });

  test('4017 also reconnects on user action', () async {
    final ch = await h.goLive();
    ch.serverClose(4017);
    await settle();
    await h.connection.connectNow();
    await settle();
    expect(h.transport.channels, hasLength(2));
  });

  test('4018 waits for user action and renews no token', () async {
    final ch = await h.goLive();
    ch.serverClose(4018);
    await settle();
    expect(h.phase, ConnectionPhase.offline);
    expect(h.connection.state.reason, ConnectionReason.forbidden);
    await h.clock.elapse(const Duration(minutes: 5));
    h.network.set(true);
    h.lifecycle.set(AppActivity.background);
    h.lifecycle.set(AppActivity.foreground);
    await settle();
    expect(h.transport.channels, hasLength(1));
    expect(h.refreshes, 0);
    await h.connection.connectNow();
    await settle();
    expect(h.transport.channels, hasLength(2));
  });

  test('the close table covers every code of 04 6.4', () {
    expect(closeCodeTable.keys.toSet(), {
      1000,
      1001,
      for (var c = 4000; c <= 4009; c++) c,
      for (var c = 4011; c <= 4018; c++) c,
    });
    for (final entry in closeCodeTable.entries) {
      final r = reactTo(entry.key, null, foreground: true);
      expect(r.code, entry.key);
    }
    expect(reactTo(4012, null, foreground: true).signOut, isTrue);
    expect(reactTo(4004, null, foreground: true).renewToken, isTrue);
    expect(reactTo(4011, null, foreground: true).renewToken, isTrue);
    expect(reactTo(4006, null, foreground: true).identify, isTrue);
    expect(
      reactTo(4008, 'retry_after=12', foreground: true).retryAfter,
      const Duration(seconds: 12),
    );
    expect(
      reactTo(1000, null, foreground: false).reconnect,
      Reconnect.onForeground,
    );
    expect(reactTo(4050, null, foreground: true).reconnect, Reconnect.backoff);
  });
}
