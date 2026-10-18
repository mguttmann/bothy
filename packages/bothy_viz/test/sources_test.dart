// SPDX-License-Identifier: Apache-2.0

import 'dart:async';

import 'package:api_client/api_client.dart' as api;
import 'package:bothy_viz/bothy_viz.dart';
import 'package:domain/domain.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support/fixtures.dart';

void main() {
  final staleAfter = AppConfig.defaults().staleAfter;
  final start = DateTime.utc(2026, 10, 15, 10);

  Future<(FakeVizClock, FixtureLiveFrameSource)> source({
    bool loop = false,
  }) async {
    final clock = FakeVizClock(start);
    final frame = await fixtures.liveFrame('normal');
    final empty = await fixtures.liveFrame('empty');
    final step = Duration(milliseconds: frame.intervalMs);
    return (
      clock,
      FixtureLiveFrameSource(
        clock: clock,
        staleAfter: staleAfter,
        loop: loop,
        steps: [(after: step, frame: frame), (after: step, frame: empty)],
      ),
    );
  }

  test('the fixture live source emits frames on the fake clock', () async {
    final (clock, live) = await source();
    final events = <LiveFrameEvent>[];
    live.start().listen(events.add);
    expect(events, isEmpty);
    clock.advance(const Duration(seconds: 1));
    expect(events.single, isA<LiveFrameFrame>());
    expect((events.single as LiveFrameFrame).model.lanes, isNotEmpty);
    clock.advance(const Duration(seconds: 1));
    expect(events, hasLength(2));
    expect((events.last as LiveFrameFrame).model.lanes, isEmpty);
  });

  test('stale arrives once after viz.stale_after without a frame', () async {
    final (clock, live) = await source();
    final events = <LiveFrameEvent>[];
    live.start().listen(events.add);
    clock.advance(const Duration(seconds: 2));
    expect(events.whereType<LiveFrameStale>(), isEmpty);
    clock.advance(staleAfter - const Duration(milliseconds: 1));
    expect(events.whereType<LiveFrameStale>(), isEmpty);
    clock.advance(const Duration(milliseconds: 1));
    expect(events.last, isA<LiveFrameStale>());
    clock.advance(staleAfter * 3);
    expect(events.whereType<LiveFrameStale>(), hasLength(1));
  });

  test('a looping replay never goes stale', () async {
    final (clock, live) = await source(loop: true);
    final events = <LiveFrameEvent>[];
    live.start().listen(events.add);
    clock.advance(staleAfter * 4);
    expect(events.whereType<LiveFrameStale>(), isEmpty);
    expect(events.whereType<LiveFrameFrame>().length, greaterThan(4));
  });

  test('every error code becomes an error event', () async {
    final (_, live) = await source();
    final events = <LiveFrameEvent>[];
    live.start().listen(events.add);
    for (final code in [
      'forbidden',
      'not_found',
      'feature_disabled',
      'rate_limited',
      'teapot',
    ]) {
      live.simulateError(code);
    }
    expect(events.whereType<LiveFrameError>().map((e) => e.kind), [
      LiveFrameErrorCode.forbidden,
      LiveFrameErrorCode.notFound,
      LiveFrameErrorCode.featureDisabled,
      LiveFrameErrorCode.rateLimited,
      LiveFrameErrorCode.other,
    ]);
    expect(events.whereType<LiveFrameError>().last.code, 'teapot');
  });

  test('a new start(spaceId) replaces the old stream', () async {
    final (clock, live) = await source(loop: true);
    final first = <LiveFrameEvent>[];
    var firstDone = false;
    live.start().listen(first.add, onDone: () => firstDone = true);
    clock.advance(const Duration(seconds: 1));
    final second = <LiveFrameEvent>[];
    live
        .start(spaceId: '0192f4a0-7b1c-7000-8a00-000000000002')
        .listen(second.add);
    await Future<void>.delayed(Duration.zero);
    expect(firstDone, isTrue);
    expect(live.spaceId, '0192f4a0-7b1c-7000-8a00-000000000002');
    final before = first.length;
    clock.advance(const Duration(seconds: 3));
    expect(first, hasLength(before));
    expect(second.whereType<LiveFrameFrame>(), hasLength(3));
  });

  test('stop closes the stream and cancels every timer', () async {
    final (clock, live) = await source(loop: true);
    var done = false;
    live.start().listen((_) {}, onDone: () => done = true);
    await live.stop();
    await Future<void>.delayed(Duration.zero);
    expect(done, isTrue);
    expect(clock.pendingTimers, 0);
    expect(live.isRunning, isFalse);
  });

  test('the fake clock rejects a step back', () {
    expect(
      () => FakeVizClock(start).advance(const Duration(seconds: -1)),
      throwsArgumentError,
    );
  });

  test('the system clock fires its timers', () async {
    const clock = SystemVizClock();
    expect(clock.now().isAfter(start), isTrue);
    final fired = Completer<void>();
    clock.schedule(Duration.zero, fired.complete);
    await fired.future;
    clock.schedule(const Duration(days: 1), () => fail('cancelled')).cancel();
  });

  group('FixtureStatsQuerySource', () {
    final from = DateTime.utc(2026, 10, 9);
    final to = DateTime.utc(2026, 10, 16);

    test('answers every query operation with its fixture', () async {
      final source = FixtureStatsQuerySource(fixtures);
      expect((await source.live()).runs, isNotEmpty);
      expect((await source.timeline(from: from, to: to)).runs, isNotEmpty);
      expect((await source.slo()).items, hasLength(6));
      expect((await source.backends(from: from, to: to)).items, isNotEmpty);
      expect((await source.usage(from: from, to: to)).buckets, isNotEmpty);
      expect((await source.runs(from: from, to: to)).buckets, isNotEmpty);
      expect((await source.errors(from: from, to: to)).buckets, isNotEmpty);
      expect((await source.costs(from: from, to: to)).buckets, isNotEmpty);
    });

    test(
      'a state picks its files and falls back to normal without one',
      () async {
        final source = FixtureStatsQuerySource(
          fixtures,
          state: 'no_cost_permission',
          available: fixtureNames().toSet(),
        );
        expect(
          (await source.live()).runs.map((r) => r.costMicros),
          everyElement(isNull),
        );
        expect((await source.slo()).items, hasLength(6));
        source.state = 'empty';
        expect((await source.usage(from: from, to: to)).buckets, isEmpty);
      },
    );

    test('failWith throws the problem on every call', () async {
      const problem = api.ApiProblem(status: 403, code: 'forbidden');
      final source = FixtureStatsQuerySource(fixtures)..failWith = problem;
      await expectLater(source.slo(), throwsA(same(problem)));
      await expectLater(
        source.costs(from: from, to: to),
        throwsA(same(problem)),
      );
    });
  });
}
