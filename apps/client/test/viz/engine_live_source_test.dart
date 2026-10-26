// SPDX-License-Identifier: Apache-2.0

// The live source of the apps (brief M6-16 Design notes 2 and 3): start subscribes the gateway
// connection to the scope, each stats.live of it becomes a frame, the end of the subscription the
// error event with its code, no frame for viz.stale_after "stale", and stop ends the
// subscription.

import 'dart:convert';
import 'dart:io';

import 'package:bothy_viz/bothy_viz.dart';
import 'package:client/viz/engine_live_source.dart';
import 'package:domain/domain.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/sync_engine.dart';

const String _space = '0192f4a0-7b1c-7000-8a00-000000000001';

Map<String, Object?> _envelope(String? spaceId) {
  final frame = jsonDecode(
    File('../../packages/bothy_viz/fixtures/StatsLiveFrame.normal.json')
        .readAsStringSync(),
  ) as Map<String, Object?>;
  frame['scope'] = spaceId == null
      ? {'kind': 'instance'}
      : {'kind': 'space', 'space_id': spaceId};
  return {
    'id': '0192f4a0-7b1c-7000-8a00-00000000aa01',
    'type': 'stats.live',
    'schema_version': 1,
    'occurred_at': frame['at'],
    'scope': const <String, Object?>{},
    'resource': const {'kind': 'instance'},
    'payload': frame,
  };
}

void main() {
  late List<GatewayFrame> sent;
  late StatsSubscription stats;
  late FakeVizClock clock;
  late EngineLiveFrameSource source;
  late List<LiveFrameEvent> events;
  final staleAfter = AppConfig.defaults().staleAfter;

  setUp(() {
    sent = [];
    stats = StatsSubscription(
      trySend: (f) {
        sent.add(f);
        return true;
      },
    );
    clock = FakeVizClock(DateTime.utc(2026, 10, 15, 10));
    source = EngineLiveFrameSource(stats, clock: clock, staleAfter: staleAfter);
    events = [];
  });
  tearDown(() => stats.dispose());

  test('start subscribes the scope and passes its frames on', () {
    source.start(spaceId: _space).listen(events.add);
    expect(sent.single, isA<StatsSubscribeFrame>());
    expect(sent.single.toData(), {'space_id': _space});
    stats.onLive(_envelope(_space));
    expect(events.single, isA<LiveFrameFrame>());
    expect((events.single as LiveFrameFrame).frame.scope.spaceId, _space);
  });

  test('no frame for viz.stale_after marks the stream stale; a frame ends '
      'it', () {
    source.start().listen(events.add);
    stats.onLive(_envelope(null));
    clock.advance(staleAfter - const Duration(milliseconds: 1));
    expect(events.whereType<LiveFrameStale>(), isEmpty);
    clock.advance(const Duration(milliseconds: 1));
    expect(events.last, isA<LiveFrameStale>());
    stats.onLive(_envelope(null));
    expect(events.last, isA<LiveFrameFrame>());
  });

  test('the end of the subscription becomes the error with its code', () {
    source.start().listen(events.add);
    stats.onError('stats.subscribe', 'feature_disabled');
    final error = events.single as LiveFrameError;
    expect(error.kind, LiveFrameErrorCode.featureDisabled);
  });

  test('stop ends the subscription and no frame arrives after it', () async {
    source.start().listen(events.add);
    await source.stop();
    expect(sent.last, isA<StatsUnsubscribeFrame>());
    stats.onLive(_envelope(null));
    expect(events, isEmpty);
  });

  test('a new start for another space switches the subscription', () {
    source.start().listen(events.add);
    final second = <LiveFrameEvent>[];
    source.start(spaceId: _space).listen(second.add);
    expect(sent.map((f) => f.toData()), [
      <String, Object?>{},
      {'space_id': _space},
    ]);
    stats.onLive(_envelope(null));
    expect(second, isEmpty, reason: 'a frame of the old scope');
    stats.onLive(_envelope(_space));
    expect(second.single, isA<LiveFrameFrame>());
  });
}
