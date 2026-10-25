// SPDX-License-Identifier: Apache-2.0

// The stats.live subscription of the gateway connection (04 6.2, 6.3, 12 item 4; 25 5.1; brief
// M6-16 Design note 1 and Tests).

import 'dart:convert';
import 'dart:io';

import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

import '../support/fakes.dart';

/// The fixture `StatsLiveFrame.<state>.json` of `bothy_viz` (25 7.2 item 3: the fake server
/// and the tests use the same fixtures).
Map<String, Object?> liveFixture(String state) => jsonDecode(
  File('../bothy_viz/fixtures/StatsLiveFrame.$state.json').readAsStringSync(),
) as Map<String, Object?>;

/// The fixture frame with the scope of [spaceId] (null: the instance).
Map<String, Object?> frameFor(String? spaceId, {String state = 'normal'}) => {
  ...liveFixture(state),
  'scope': spaceId == null
      ? {'kind': 'instance'}
      : {'kind': 'space', 'space_id': spaceId},
};

/// The `d` of a `dispatch` of `stats.live` with [frame].
Map<String, Object?> liveEnvelope(Map<String, Object?> frame) => {
  'id': '0192f4a0-7b1c-7000-8a00-00000000aa01',
  'type': 'stats.live',
  'schema_version': 1,
  'occurred_at': frame['at'],
  'scope': const <String, Object?>{},
  'resource': const {'kind': 'instance'},
  'payload': frame,
};

const String spaceA = '0192f4a0-7b1c-7000-8a00-000000000001';
const String spaceB = '0192f4a0-7b1c-7000-8a00-000000000002';

void main() {
  group('StatsSubscription', () {
    late List<GatewayFrame> sent;
    late bool identified;
    late StatsSubscription stats;
    late List<StatsLiveUpdate> updates;

    setUp(() {
      sent = [];
      identified = true;
      updates = [];
      stats = StatsSubscription(
        trySend: (f) {
          if (!identified) return false;
          sent.add(f);
          return true;
        },
      );
      stats.updates.listen(updates.add);
    });
    tearDown(() => stats.dispose());

    test('subscribe sends stats.subscribe once while live', () {
      stats.subscribe(spaceId: spaceA);
      stats.subscribe(spaceId: spaceA);
      expect(sent, hasLength(1));
      final frame = sent.single as StatsSubscribeFrame;
      expect(frame.op, 'stats.subscribe');
      expect(frame.toData(), {'space_id': spaceA});
      stats.subscribe();
      expect(sent, hasLength(2), reason: 'another scope replaces it');
      expect(sent.last.toData(), isEmpty, reason: 'the instance has no member');
    });

    test(
      'while not identified nothing is sent; ready sends the subscription',
      () {
        identified = false;
        stats.subscribe(spaceId: spaceA);
        expect(sent, isEmpty);
        identified = true;
        stats.onReady();
        expect(sent.single.toData(), {'space_id': spaceA});
      },
    );

    test('no resend after resumed: the server restores it (04 6.3)', () {
      stats.subscribe(spaceId: spaceA);
      stats.onResumed();
      expect(sent, hasLength(1));
    });

    test('a change made while disconnected is sent after resumed', () {
      stats.subscribe(spaceId: spaceA);
      identified = false;
      stats.subscribe(spaceId: spaceB);
      identified = true;
      stats.onResumed();
      expect(sent.map((f) => f.toData()), [
        {'space_id': spaceA},
        {'space_id': spaceB},
      ]);
      identified = false;
      stats.unsubscribe();
      identified = true;
      stats.onResumed();
      expect(sent.last, isA<StatsUnsubscribeFrame>());
    });

    test('resend after identify (a new gateway session)', () {
      stats.subscribe(spaceId: spaceA);
      stats.onReady();
      expect(sent.map((f) => f.toData()), [
        {'space_id': spaceA},
        {'space_id': spaceA},
      ]);
    });

    test('nothing to send after ready without a subscription', () {
      stats.onReady();
      stats.onResumed();
      expect(sent, isEmpty);
    });

    for (final code in const [
      'forbidden',
      'not_found',
      'feature_disabled',
      'rate_limited',
    ]) {
      test('error $code of stats.subscribe clears the subscription and passes '
          'the code', () {
        stats.subscribe(spaceId: spaceA);
        stats.onError('stats.subscribe', code);
        expect(stats.current, isNull);
        expect(updates.single, isA<StatsLiveEnded>());
        expect((updates.single as StatsLiveEnded).code, code);
        expect(sent.last, isA<StatsUnsubscribeFrame>());
        stats.onReady();
        expect(sent, hasLength(2), reason: 'not sent again after identify');
      });
    }

    test('a refused replacement ends the earlier subscription the server keeps '
        '(the server keeps it on a denial)', () {
      stats.subscribe(spaceId: spaceA);
      stats.subscribe(spaceId: spaceB);
      stats.onError('stats.subscribe', 'forbidden');
      expect(sent.map((f) => f.op), [
        'stats.subscribe',
        'stats.subscribe',
        'stats.unsubscribe',
      ]);
      expect(stats.current, isNull);
      stats.unsubscribe();
      expect(sent, hasLength(3), reason: 'nothing held any more');
    });

    test('a refusal while not identified ends the subscription on the next '
        'resume', () {
      stats.subscribe(spaceId: spaceA);
      identified = false;
      stats.onError('stats.subscribe', 'rate_limited');
      expect(sent, hasLength(1));
      identified = true;
      stats.onResumed();
      expect(sent.last, isA<StatsUnsubscribeFrame>());
      stats.onResumed();
      expect(sent, hasLength(2));
    });

    test('errors of other ops do not touch the subscription', () {
      stats.subscribe(spaceId: spaceA);
      stats.onError('focus', 'validation_failed');
      expect(stats.current?.spaceId, spaceA);
      expect(updates, isEmpty);
    });

    test('an error without a subscription is not passed on', () {
      stats.onError('stats.subscribe', 'forbidden');
      expect(updates, isEmpty);
    });

    test('unsubscribe sends stats.unsubscribe once; without a subscription '
        'nothing', () {
      stats.unsubscribe();
      expect(sent, isEmpty);
      stats.subscribe();
      stats.unsubscribe();
      stats.unsubscribe();
      expect(sent.map((f) => f.op), ['stats.subscribe', 'stats.unsubscribe']);
      expect(sent.last.toData(), isEmpty);
    });

    test(
      'frames of the subscribed scope arrive decoded, others are dropped',
      () {
        stats.onLive(liveEnvelope(frameFor(spaceA)));
        expect(updates, isEmpty, reason: 'no subscription');
        stats.subscribe(spaceId: spaceA);
        stats.onLive(liveEnvelope(frameFor(spaceB)));
        stats.onLive(liveEnvelope(frameFor(null)));
        expect(updates, isEmpty, reason: 'frames of an old scope');
        stats.onLive(liveEnvelope(frameFor(spaceA)));
        final frame = (updates.single as StatsLiveReceived).frame;
        expect(frame.scope.spaceId, spaceA);
        expect(frame.runs, isNotEmpty);
      },
    );

    test('a malformed frame is skipped and logged without content', () {
      final log = <String>[];
      final s = StatsSubscription(trySend: (_) => true, log: log.add);
      addTearDown(s.dispose);
      final got = <StatsLiveUpdate>[];
      s.updates.listen(got.add);
      s.subscribe();
      s.onLive(liveEnvelope({'at': 'not a time'}));
      expect(got, isEmpty);
      expect(log.single, 'stats.live skipped: malformed frame');
    });
  });

  group('GatewayConnection', () {
    test('stats.live goes to the subscription, never to the listener, so '
        'nothing reaches the local database', () async {
      final listener = RecordingListener();
      final h = Harness(listener: listener);
      addTearDown(h.dispose);
      final got = <StatsLiveUpdate>[];
      h.connection.stats.updates.listen(got.add);
      h.connection.stats.subscribe();
      final ch = await h.goLive();
      expect(ch.sentOps, ['identify', 'stats.subscribe']);
      final before = (await h.db.syncStateDao.read()).cursor;
      ch.push('dispatch', liveEnvelope(frameFor(null)));
      await settle();
      expect(got.single, isA<StatsLiveReceived>());
      expect(listener.frames, isEmpty);
      expect((await h.db.syncStateDao.read()).cursor, before);
    });

    test('resumed keeps the subscription, ready after the window sends it '
        'again, error ends it', () async {
      final h = Harness(listener: RecordingListener());
      addTearDown(h.dispose);
      final got = <StatsLiveUpdate>[];
      h.connection.stats.updates.listen(got.add);
      var ch = await h.goLive();
      h.connection.stats.subscribe(spaceId: spaceA);
      expect(ch.sentOps.last, 'stats.subscribe');
      ch.serverClose(1001);
      await settle();
      await h.clock.elapse(const Duration(seconds: 5));
      await h.answerHello();
      ch = h.transport.last;
      ch.push('resumed', resumedData());
      await settle();
      expect(ch.sentOps, ['resume'], reason: 'restored by the server');
      // Offline beyond resume_window_ms: the next connection identifies.
      h.network.set(false);
      ch.serverClose(null);
      await settle();
      await h.clock.elapse(resumeWindow + const Duration(seconds: 1));
      h.network.set(true);
      await settle();
      await h.answerHello();
      ch = h.transport.last;
      ch.push('ready', readyData(gatewaySessionId: 'gs-2'));
      await settle();
      expect(ch.sentOps, ['identify', 'stats.subscribe']);
      expect(ch.lastData('stats.subscribe'), {'space_id': spaceA});
      ch.push('error', {'op': 'stats.subscribe', 'code': 'forbidden'});
      await settle();
      expect((got.single as StatsLiveEnded).code, 'forbidden');
      expect(h.connection.stats.current, isNull);
    });
  });
}
