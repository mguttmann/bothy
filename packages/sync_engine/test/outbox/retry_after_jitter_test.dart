// SPDX-License-Identifier: Apache-2.0

// FIX-upload-retry-storm: a `Retry-After` is a floor, not a meeting time. The outbox waits the
// server's seconds plus the full-jitter backoff of the attempt, so lanes refused together do
// not come back together (04 13.2 item 4, addendum of 2026-10-30), and an executor can keep
// what an attempt achieved in its own entry (`ApiCallContext.checkpoint`), so the next attempt
// resumes there instead of starting over.

import 'dart:math';

import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late Harness h;
  setUp(() => h = Harness.open());
  tearDown(() => h.close());

  const base = Duration(seconds: 2); // client.outbox_retry_base default

  test('429 waits at least Retry-After and at most Retry-After plus the '
      'backoff bound', () async {
    final exec = h.fake('createMessage')
      ..then([
        problem(429, 'rate_limited', retryAfter: const Duration(seconds: 12)),
      ]);
    final e = await h.add('hello', thread: threadA);
    await h.outbox.start();
    await settle();
    final waiting = (await h.entry(e.id))!;
    expect(waiting.state, OutboxState.waiting);
    final delay = waiting.nextAttemptAt!.difference(h.time.now);
    expect(delay >= const Duration(seconds: 12), isTrue, reason: '$delay');
    expect(delay <= const Duration(seconds: 12) + base, isTrue);
    await h.time.advance(delay);
    expect(exec.calls, hasLength(2));
  });

  test(
    'lanes refused with the same Retry-After wake at different times',
    () async {
      final lanes = h.fake('upload', lane: (e) => 'upload:${e.id}');
      lanes.then(
        List.filled(
          6,
          problem(429, 'rate_limited', retryAfter: const Duration(seconds: 1)),
        ),
      );
      final ids = <String>[];
      for (var i = 0; i < 6; i++) {
        final e = await h.outbox.enqueue(
          OutboxRequest(kind: 'upload', payload: {'n': i}),
        );
        ids.add(e.id);
      }
      await h.outbox.start();
      await settle();
      final due = <DateTime>{
        for (final id in ids) (await h.entry(id))!.nextAttemptAt!,
      };
      expect(due, hasLength(6), reason: 'every lane draws its own jitter');
      for (final d in due) {
        expect(d.difference(h.time.now) >= const Duration(seconds: 1), isTrue);
      }
    },
  );

  test('the wait grows with the attempts on a repeated Retry-After', () async {
    final exec = h.fake('createMessage');
    exec.then(
      List.filled(
        8,
        problem(429, 'rate_limited', retryAfter: const Duration(seconds: 1)),
      ),
    );
    final e = await h.add('hello', thread: threadA);
    await h.outbox.start();
    await settle();
    var longest = Duration.zero;
    for (var retry = 0; retry < 8; retry++) {
      final delay = (await h.entry(e.id))!.nextAttemptAt!
          .difference(h.time.now);
      final bound = base * (1 << retry);
      expect(delay >= const Duration(seconds: 1), isTrue);
      expect(delay <= const Duration(seconds: 1) + bound, isTrue);
      if (delay > longest) longest = delay;
      await h.time.advance(delay);
    }
    expect(
      longest > const Duration(seconds: 5),
      isTrue,
      reason: 'a lane that keeps hitting the limit backs off further',
    );
  });

  test('retryWait adds the full-jitter backoff to Retry-After', () {
    final wait = retryWait(
      retryAfter: const Duration(seconds: 3),
      base: base,
      cap: const Duration(minutes: 5),
      retry: 0,
      random: _Fixed(0.5),
    );
    expect(wait, const Duration(seconds: 4));
    expect(
      retryWait(
        base: base,
        cap: const Duration(minutes: 5),
        retry: 0,
        random: _Fixed(0.5),
      ),
      const Duration(seconds: 1),
      reason: 'without Retry-After the plain backoff',
    );
  });

  group('checkpoint', () {
    test('the next attempt reads what the last one kept', () async {
      final seen = <Map<String, Object?>>[];
      h.executors.register('upload', (e, ctx) async {
        seen.add(decodePayload(e.payload));
        if (seen.length == 1) {
          await ctx.checkpoint({'file_id': 'f1'});
          throw problem(503, 'unavailable');
        }
        return const OutboxResponse();
      }, lane: (e) => 'upload:${e.id}');
      final e = await h.outbox.enqueue(
        const OutboxRequest(kind: 'upload', payload: {'name': 'a.png'}),
      );
      await h.outbox.start();
      await settle();
      final waiting = (await h.entry(e.id))!;
      expect(decodePayload(waiting.payload), {
        'name': 'a.png',
        'file_id': 'f1',
      });
      await h.time.advance(waiting.nextAttemptAt!.difference(h.time.now));
      expect(seen, hasLength(2));
      expect(seen[1]['file_id'], 'f1');
      expect(await h.entry(e.id), isNull);
    });

    test('a checkpoint of a discarded entry changes nothing', () async {
      late Future<void> Function() keep;
      final held = h.fake('upload', lane: (e) => 'upload:${e.id}');
      h.executors.register('upload', (e, ctx) async {
        keep = () => ctx.checkpoint({'file_id': 'f1'});
        return held.call(e, ctx);
      }, lane: (e) => 'upload:${e.id}');
      held.then([problem(503, 'unavailable')]);
      final e = await h.outbox.enqueue(
        const OutboxRequest(kind: 'upload', payload: {'name': 'a.png'}),
      );
      await h.outbox.start();
      await settle();
      await h.outbox.discard(e.id);
      await keep();
      expect(await h.entry(e.id), isNull);
    });
  });
}

final class _Fixed implements Random {
  _Fixed(this.value);
  final double value;

  @override
  double nextDouble() => value;

  @override
  bool nextBool() => false;

  @override
  int nextInt(int max) => 0;
}
