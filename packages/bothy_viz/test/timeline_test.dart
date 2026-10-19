// SPDX-License-Identifier: Apache-2.0

import 'package:bothy_viz/bothy_viz.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support/views.dart';

void main() {
  final from = DateTime.utc(2026, 10, 15, 10);
  final to = DateTime.utc(2026, 10, 15, 11);
  final speeds = TimelinePlayback.parseSpeeds(
    AppConfig.defaults().timelineSpeeds,
  );

  TimelinePlayback playback(FakeVizClock clock) =>
      TimelinePlayback(from: from, to: to, clock: clock, speeds: speeds);

  test('the start is the minute of from, the origin of the jump slider', () {
    final clock = FakeVizClock(to);
    final p = TimelinePlayback(
      from: from.add(const Duration(seconds: 30, milliseconds: 5)),
      to: to,
      clock: clock,
      speeds: speeds,
    );
    expect(p.start, from);
    expect(p.position.difference(p.start).inMinutes, p.minutes);
  });

  test('the speeds come from viz.timeline_speeds', () {
    expect(speeds, isNotEmpty);
    expect(speeds, everyElement(greaterThanOrEqualTo(1)));
    expect(TimelinePlayback.parseSpeeds(['60', ' 300', 'x', '0']), [60, 300]);
    expect(
      () => TimelinePlayback(
        from: from,
        to: to,
        clock: FakeVizClock(from),
        speeds: const [],
      ),
      throwsArgumentError,
    );
  });

  test(
    'starts paused at the end; play at the end starts again at the start',
    () {
      final clock = FakeVizClock(from);
      final p = playback(clock);
      expect(p.playing, isFalse);
      expect(p.position, to);
      p.play();
      expect(p.playing, isTrue);
      expect(p.position, from);
    },
  );

  for (final speed in speeds) {
    test('speed $speed moves one minute every 60/$speed seconds', () {
      final clock = FakeVizClock(from);
      final p = playback(clock)
        ..setSpeed(speed)
        ..play();
      final tick = Duration(
        microseconds: Duration.microsecondsPerMinute ~/ speed,
      );
      clock.advance(tick * 3);
      expect(p.position, from.add(const Duration(minutes: 3)));
      expect(p.speed, speed);
    });
  }

  test('pause stops, jump moves to a minute, the end stops the playback', () {
    final clock = FakeVizClock(from);
    final p = playback(clock)..play();
    final tick = Duration(
      microseconds: Duration.microsecondsPerMinute ~/ p.speed,
    );
    clock.advance(tick * 2);
    p.pause();
    expect(p.playing, isFalse);
    clock.advance(tick * 10);
    expect(p.position, from.add(const Duration(minutes: 2)));
    p.jumpTo(from.add(const Duration(minutes: 30, seconds: 40)));
    expect(p.position, from.add(const Duration(minutes: 30)));
    p.jumpTo(to.add(const Duration(hours: 1)));
    expect(p.position, to);
    p.jumpTo(from.subtract(const Duration(hours: 1)));
    expect(p.position, from);
    p.play();
    clock.advance(tick * 100);
    expect(p.position, to);
    expect(p.playing, isFalse);
    expect(() => p.setSpeed(7), throwsArgumentError);
    p.dispose();
  });

  test('a speed change while playing keeps playing at the new speed', () {
    final clock = FakeVizClock(from);
    final p = playback(clock)..play();
    p.setSpeed(speeds.last);
    clock.advance(
      Duration(microseconds: Duration.microsecondsPerMinute ~/ speeds.last),
    );
    expect(p.position, from.add(const Duration(minutes: 1)));
    expect(p.playing, isTrue);
  });

  testWidgets('the view plays, pauses, jumps and switches speed', (
    tester,
  ) async {
    final world = ViewWorld();
    await pumpWorld(tester, world, world.view(VizViewKind.timeline));
    expect(find.textContaining('Playhead'), findsOneWidget);
    await tester.tap(find.byKey(const ValueKey('viz-timeline-play')));
    await tester.pump();
    expect(find.text('Pause'), findsOneWidget);
    final speed = speeds.last;
    await tester.tap(find.byKey(ValueKey('viz-timeline-speed-$speed')));
    await tester.pump();
    final before = tester
        .widget<Slider>(find.byKey(const ValueKey('viz-timeline-jump')))
        .value;
    world.clock.advance(
      Duration(microseconds: Duration.microsecondsPerMinute ~/ speed) * 5,
    );
    await tester.pump();
    final after = tester
        .widget<Slider>(find.byKey(const ValueKey('viz-timeline-jump')))
        .value;
    expect(after - before, 5);
    await tester.tap(find.byKey(const ValueKey('viz-timeline-play')));
    await tester.pump();
    expect(find.text('Play'), findsOneWidget);
    tester
        .widget<Slider>(find.byKey(const ValueKey('viz-timeline-jump')))
        .onChanged!(20);
    await tester.pump();
    expect(
      tester
          .widget<Slider>(find.byKey(const ValueKey('viz-timeline-jump')))
          .value,
      20,
    );
  });

  testWidgets('runs_truncated shows the marker', (tester) async {
    final world = ViewWorld(state: 'truncated');
    await pumpWorld(tester, world, world.view(VizViewKind.timeline));
    expect(find.text('More runs than shown'), findsOneWidget);
  });

  testWidgets('a period over midnight names the day of the playhead', (
    tester,
  ) async {
    final world = ViewWorld(state: 'day_boundary');
    await pumpWorld(tester, world, world.view(VizViewKind.timeline));
    final slider = find.byKey(const ValueKey('viz-timeline-jump'));
    String playhead() =>
        tester.widget<Text>(find.textContaining('Playhead')).data!;
    final end = playhead();
    tester.widget<Slider>(slider).onChanged!(0);
    await tester.pump();
    final start = playhead();
    // The days differ: the start is on the 15th, the end on the 16th.
    expect(start.split(',').first, isNot(end.split(',').first));
  });
}
