// SPDX-License-Identifier: Apache-2.0

// The live stream of the fake (brief M6-16 Boundaries: the fake server serves stats.subscribe
// and getStatsLive from the fixtures of bothy_viz): the subscription per gateway session, the
// denials of 25 5.1, resume and identify, the frames a test sends, and getStatsLive.

import 'dart:convert';
import 'dart:io';

import 'package:dio/dio.dart';
import 'package:sync_engine/testing.dart';
import 'package:test/test.dart';

import 'support.dart';

Map<String, Object?> fixture(String name) =>
    jsonDecode(File('../bothy_viz/fixtures/$name').readAsStringSync())
        as Map<String, Object?>;

void main() {
  late FakeWorldHarness h;

  setUp(() async => h = await FakeWorldHarness.start());
  tearDown(() => h.stop());

  Future<(RawGateway, RawClient)> live(TestDevice device) async {
    final s = await signedIn(h, device);
    final raw = RawClient(h.server.baseUri, device.key);
    final g = await RawGateway.connect(h.server.gatewayUri, raw);
    await g.identify(s.token, s.deviceId);
    await g.waitFor('ready');
    return (g, raw);
  }

  test('a subscription receives the frames a test sends, with its scope; '
      'unsubscribe ends it', () async {
    final (g, raw) = await live(h.device());
    addTearDown(raw.close);
    final frame = fixture('StatsLiveFrame.normal.json');
    expect(h.server.gateway.sendStatsLive(frame), 0);
    g.send('stats.subscribe', {'space_id': h.seed.space});
    g.send('heartbeat', {'last_seq': 0});
    await g.waitFor('heartbeat_ack');
    final conn = h.server.gateway.connections.single;
    expect(conn.statsSubscription?.spaceId, h.seed.space);
    expect(h.server.gateway.sendStatsLive(frame), 1);
    await g.waitFor('dispatch');
    final d = g.data('dispatch').single;
    expect(d['type'], 'stats.live');
    expect(d.containsKey('seq'), isFalse, reason: 'ephemeral');
    final payload = d['payload']! as Map<String, Object?>;
    expect(payload['scope'], {'kind': 'space', 'space_id': h.seed.space});
    expect(payload.containsKey('sli'), isFalse);
    g.send('stats.unsubscribe', {});
    g.send('heartbeat', {'last_seq': 0});
    await g.waitFor('heartbeat_ack', count: 2);
    expect(conn.statsSubscription, isNull);
    expect(h.server.gateway.sendStatsLive(frame), 0);
    expect(h.server.violations, isEmpty);
  });

  test('denials answer error and keep the connection (25 5.1)', () async {
    final (g, raw) = await live(h.device(login: WorldSeed.samLogin));
    addTearDown(raw.close);
    g.send('stats.subscribe', {});
    await g.waitFor('error');
    g.send('stats.subscribe', {'space_id': h.seed.space});
    await g.waitFor('error', count: 2);
    h.server.world.statsLiveEnabled = false;
    g.send('stats.subscribe', {'space_id': h.seed.space});
    await g.waitFor('error', count: 3);
    expect(g.data('error'), [
      {'op': 'stats.subscribe', 'code': 'forbidden'},
      {'op': 'stats.subscribe', 'code': 'forbidden'},
      {'op': 'stats.subscribe', 'code': 'feature_disabled'},
    ]);
    expect(g.done, isFalse);
    expect(h.server.gateway.connections.single.statsSubscription, isNull);
  });

  test('the loss of stats.read ends the subscription with one error', () async {
    final alex = h.device();
    final (g, raw) = await live(alex);
    addTearDown(raw.close);
    g.send('stats.subscribe', {});
    g.send('heartbeat', {'last_seq': 0});
    await g.waitFor('heartbeat_ack');
    h.server.gateway.endStatsSubscriptions(h.seed.alex.id);
    await g.waitFor('error');
    expect(g.data('error').single, {
      'op': 'stats.subscribe',
      'code': 'forbidden',
    });
    expect(h.server.gateway.connections.single.statsSubscription, isNull);
  });

  test('getStatsLive answers the stored frame with the scope of the request, '
      'or feature_disabled', () async {
    final alex = h.device();
    await signedIn(h, alex);
    final stats = alex.api.getStatsApi();
    h.server.world.stats['getStatsLive'] = fixture(
      'StatsLiveFrame.normal.json',
    );
    final instance = (await stats.getStatsLive()).data!;
    expect(instance.scope.kind.name, 'instance');
    expect(instance.runs, isNotEmpty);
    final space = (await stats.getStatsLive(spaceId: h.seed.space)).data!;
    expect(space.scope.spaceId, h.seed.space);
    expect(space.sli, isNull);
    h.server.world.statsLiveEnabled = false;
    await expectLater(
      stats.getStatsLive(),
      throwsA(
        isA<DioException>().having(
          (e) => e.response?.statusCode,
          'status',
          403,
        ),
      ),
    );
    expect(h.server.violations, isEmpty);
  });

  test('getStatsTimeline checks the range as the server does: full minutes, '
      'at most stats.timeline_max_range', () async {
    final alex = h.device();
    await signedIn(h, alex);
    final stats = alex.api.getStatsApi();
    Future<int?> status(DateTime from, DateTime to) async {
      try {
        await stats.getStatsTimeline(from: from, to: to);
        return 200;
      } on DioException catch (e) {
        return e.response?.statusCode;
      }
    }

    final to = DateTime.utc(2026, 10, 15, 10, 30);
    expect(await status(to.subtract(const Duration(hours: 24)), to), 200);
    expect(
      await status(
        to.subtract(const Duration(hours: 24)),
        to.add(const Duration(seconds: 5)),
      ),
      400,
      reason: 'not a full minute',
    );
    expect(
      await status(to.subtract(const Duration(hours: 24, minutes: 1)), to),
      400,
      reason: 'longer than stats.timeline_max_range',
    );
  });
}
