// SPDX-License-Identifier: Apache-2.0

// Reconnecting of 04 6.5: full jitter within [0, min(cap, base * 2^n)], reset after ready, and
// an immediate attempt on a network change or the foreground that does not raise n (brief
// M4-13, Tests).

import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

import '../support/fakes.dart';

void main() {
  group('ReconnectBackoff', () {
    const base = Duration(seconds: 1);
    const cap = Duration(seconds: 60);

    test('waits stay within [0, min(cap, base * 2^n)]', () {
      for (final seed in [1, 2, 3]) {
        final b = ReconnectBackoff(
          base: base,
          cap: cap,
          random: SeededRandom(seed),
        );
        for (var n = 0; n < 12; n++) {
          expect(b.attempt, n);
          final bound = n >= 6 ? cap : base * (1 << n);
          expect(b.bound(n), bound);
          final w = b.next();
          expect(w, greaterThanOrEqualTo(Duration.zero));
          expect(w, lessThanOrEqualTo(bound));
        }
      }
    });

    test('full jitter uses the random value over the whole range', () {
      final b = ReconnectBackoff(base: base, cap: cap, random: _Fixed(0.5));
      b.next();
      b.next();
      expect(b.next(), const Duration(seconds: 2)); // 0.5 * 4 s
    });

    test('reset starts at 0 again', () {
      final b = ReconnectBackoff(base: base, cap: cap, random: SeededRandom(1));
      b.next();
      b.next();
      b.reset();
      expect(b.attempt, 0);
    });

    test('a huge attempt number stays at the cap', () {
      final b = ReconnectBackoff(base: base, cap: cap, random: _Fixed(1));
      for (var i = 0; i < 80; i++) {
        b.next();
      }
      expect(b.next(), cap);
    });
  });

  group('connection', () {
    late Harness h;
    setUp(() => h = Harness());
    tearDown(() => h.dispose());

    test('failed attempts raise n, ready resets it', () async {
      await h.signIn();
      h.transport.failNext = 3;
      h.connection.start();
      await settle();
      expect(h.phase, ConnectionPhase.backoff);
      expect(h.connection.backoffAttempt, 1);
      await h.clock.elapse(const Duration(minutes: 2));
      expect(h.transport.attempts, 4);
      expect(h.connection.backoffAttempt, 3);
      h.transport.last.push('hello', helloData());
      await settle();
      h.transport.last.push('ready', readyData());
      await settle();
      expect(h.connection.backoffAttempt, 0);
    });

    test('the configured base and cap bound the waits', () async {
      await h.dispose();
      h = Harness(
        overrides: {
          'sync.reconnect_backoff_base': '3s',
          'sync.reconnect_backoff_cap': '5s',
        },
      );
      await h.signIn();
      h.transport.failNext = 4;
      h.connection.start();
      await settle();
      final waits = <Duration>[];
      var last = h.clock.now();
      for (var i = 0; i < 4; i++) {
        final before = h.transport.attempts;
        while (h.transport.attempts == before) {
          await h.clock.elapse(const Duration(milliseconds: 100));
        }
        waits.add(h.clock.now().difference(last));
        last = h.clock.now();
      }
      expect(waits[0], lessThanOrEqualTo(const Duration(milliseconds: 3100)));
      for (final w in waits.skip(1)) {
        expect(w, lessThanOrEqualTo(const Duration(milliseconds: 5100)));
      }
    });

    test('a network change connects at once without raising n', () async {
      await h.signIn();
      h.transport.failNext = 2;
      h.connection.start();
      await settle();
      expect(h.connection.backoffAttempt, 1);
      h.network.set(true);
      await settle();
      expect(h.transport.attempts, 2);
      expect(
        h.connection.backoffAttempt,
        2,
        reason: 'the failed attempt counts',
      );
      h.network.set(true);
      await settle();
      expect(h.transport.attempts, 3);
      expect(h.connection.backoffAttempt, 2, reason: 'a success adds nothing');
    });

    test('the foreground connects at once without raising n', () async {
      await h.signIn();
      h.transport.failNext = 1;
      h.connection.start();
      await settle();
      expect(h.phase, ConnectionPhase.backoff);
      h.lifecycle.set(AppActivity.foreground);
      await settle();
      expect(h.transport.attempts, 2);
      expect(h.connection.backoffAttempt, 1);
    });

    test('without network the connection waits offline', () async {
      await h.signIn();
      h.network.online = false;
      h.connection.start();
      await settle();
      expect(h.phase, ConnectionPhase.offline);
      expect(h.connection.state.reason, ConnectionReason.networkUnavailable);
      expect(h.transport.attempts, 0);
      h.network.set(true);
      await settle();
      expect(h.transport.attempts, 1);
    });
  });
}

class _Fixed implements RandomSource {
  _Fixed(this.value);

  final double value;

  @override
  double nextDouble() => value;
}
