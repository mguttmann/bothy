// SPDX-License-Identifier: Apache-2.0

// The live source of the console (brief M6-16 Design note 4; 25 5.1): getStatsLive at once and
// every viz.live_poll_interval, nothing while the tab is hidden and at once when it is shown,
// feature_disabled (and forbidden, not_found) ends polling with the code, other failures back
// off up to viz.stale_after while the view turns stale, and a frame brings the interval back.

import 'dart:async';
import 'dart:io';

import 'package:api_client/api_client.dart' as api;
import 'package:bothy_viz/bothy_viz.dart';
import 'package:console/platform/browser.dart';
import 'package:console/viz/polling_live_source.dart';
import 'package:domain/domain.dart';
import 'package:flutter_test/flutter_test.dart';

final Map<String, String> _files = {
  for (final f in Directory(
    '../../packages/bothy_viz/fixtures',
  ).listSync().whereType<File>())
    if (f.path.endsWith('.json')) f.uri.pathSegments.last: f.readAsStringSync(),
};

final class _Visibility implements PageVisibility {
  final StreamController<bool> _changes = StreamController<bool>.broadcast(
    sync: true,
  );

  @override
  bool hidden = false;

  void set(bool value) {
    hidden = value;
    _changes.add(value);
  }

  @override
  Stream<bool> get changes => _changes.stream;
}

void main() {
  final config = AppConfig.defaults();
  final poll = config.livePollInterval;
  final staleAfter = config.staleAfter;
  late FakeVizClock clock;
  late FixtureStatsQuerySource queries;
  late _Visibility visibility;
  late PollingLiveFrameSource source;
  late List<LiveFrameEvent> events;

  setUp(() {
    clock = FakeVizClock(DateTime.utc(2026, 10, 15, 10));
    queries = FixtureStatsQuerySource(
      VizFixtures((name) async => _files[name]!),
      available: _files.keys.toSet(),
    );
    visibility = _Visibility();
    source = PollingLiveFrameSource(
      queries: queries,
      clock: clock,
      pollInterval: poll,
      staleAfter: staleAfter,
      visibility: visibility,
    );
    events = [];
  });
  tearDown(() => source.stop());

  int lives() => queries.calls.where((c) => c == 'live').length;

  Future<void> advance(Duration by) async {
    clock.advance(by);
    await pumpEventQueue();
  }

  test('polls at once and every viz.live_poll_interval', () async {
    source.start().listen(events.add);
    await pumpEventQueue();
    expect(lives(), 1);
    expect(events.single, isA<LiveFrameFrame>());
    await advance(poll - const Duration(milliseconds: 1));
    expect(lives(), 1);
    await advance(const Duration(milliseconds: 1));
    expect(lives(), 2);
    expect(events.whereType<LiveFrameFrame>(), hasLength(2));
    expect(events.whereType<LiveFrameStale>(), isEmpty);
  });

  test('a hidden tab does not poll; shown again it polls at once', () async {
    source.start().listen(events.add);
    await pumpEventQueue();
    visibility.set(true);
    await advance(poll * 10);
    expect(lives(), 1);
    expect(events.last, isA<LiveFrameStale>(), reason: 'old data is stale');
    visibility.set(false);
    await pumpEventQueue();
    expect(lives(), 2);
    expect(events.last, isA<LiveFrameFrame>());
  });

  test('a tab hidden at the start polls only when shown', () async {
    visibility.hidden = true;
    source.start().listen(events.add);
    await advance(poll * 3);
    expect(lives(), 0);
    visibility.set(false);
    await pumpEventQueue();
    expect(lives(), 1);
  });

  for (final code in PollingLiveFrameSource.finalCodes) {
    test('$code ends polling and passes the code', () async {
      queries.failOn['live'] = api.ApiProblem(status: 403, code: code);
      source.start().listen(events.add);
      await pumpEventQueue();
      expect((events.single as LiveFrameError).code, code);
      await advance(poll * 10);
      expect(lives(), 1);
    });
  }

  test('other failures back off up to viz.stale_after; a frame brings the '
      'interval back', () async {
    queries.failOn['live'] = const api.ApiProblem(
      status: 429,
      code: 'rate_limited',
    );
    source.start().listen(events.add);
    await pumpEventQueue();
    expect(lives(), 1);
    expect(events, isEmpty, reason: 'no error event: the view keeps going');
    // The waits double from the interval and stop at viz.stale_after.
    var wait = poll;
    final waits = <Duration>[];
    for (var i = 0; i < 4; i++) {
      wait = wait * 2 > staleAfter ? staleAfter : wait * 2;
      waits.add(wait);
      final before = lives();
      await advance(wait - const Duration(milliseconds: 1));
      expect(lives(), before, reason: 'wait $i');
      await advance(const Duration(milliseconds: 1));
      expect(lives(), before + 1, reason: 'wait $i');
    }
    expect(waits.last, staleAfter);
    expect(events.whereType<LiveFrameStale>(), hasLength(1));
    queries.failOn.clear();
    await advance(staleAfter);
    expect(events.last, isA<LiveFrameFrame>());
    final before = lives();
    await advance(poll);
    expect(lives(), before + 1);
  });

  test('a new start for a space polls that space only', () async {
    source.start().listen(events.add);
    await pumpEventQueue();
    source.start(spaceId: '0192f4a0-7b1c-7000-8a00-000000000001');
    await pumpEventQueue();
    expect(lives(), 2);
    await advance(poll);
    expect(lives(), 3, reason: 'one timer, not two');
  });
}
