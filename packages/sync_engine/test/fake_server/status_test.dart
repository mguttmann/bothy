// SPDX-License-Identifier: Apache-2.0

// The status page of the fake (brief M7-21 Tests): the component of an agent only for a person
// with `agent.read`, never on the public page and never in the overall state; the public page
// only with `status.public_enabled`; the owner operations refused for anyone else; each change
// appends one `status.updated`.

import 'package:api_client/api_client.dart';
import 'package:dio/dio.dart';
import 'package:sync_engine/testing.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late FakeWorldHarness h;
  late String lotse;

  setUp(() async {
    h = await FakeWorldHarness.start();
    final world = h.server.world;
    world
      ..addStatusComponent('api', healthSource: 'api')
      ..addStatusComponent('push', healthSource: 'push', state: 'degraded');
    final agent = world.addPerson(
      handle: 'lotse',
      displayName: 'Lotse',
      kind: 'agent',
    );
    lotse = world.addStatusComponent(
      'Lotse',
      agentId: agent.id,
      state: 'major_outage',
    );
    world.agentReaders[agent.id] = {h.seed.alex.id};
  });
  tearDown(() => h.stop());

  Future<StatusPage> pageOf(String login, int key) async {
    final d = h.device(login: login, keySeed: key);
    await signedIn(h, d);
    return (await d.api.getStatusApi().getStatus()).data!;
  }

  test('the component of an agent only with agent.read', () async {
    final alex = await pageOf(WorldSeed.alexLogin, 1);
    expect(alex.components.map((c) => c.id), contains(lotse));
    // The agent's outage never counts for the overall state.
    expect(alex.overallState, StatusComponentState.degraded);
    final sam = await pageOf(WorldSeed.samLogin, 2);
    expect(sam.components.map((c) => c.name), ['api', 'push']);
  });

  test('the public page only with status.public_enabled', () async {
    final d = h.device();
    final api = d.api.getStatusApi();
    final refused = await api.getPublicStatus().then<Object?>(
      (_) => null,
      onError: (Object e) => e,
    );
    expect((refused! as DioException).response?.statusCode, 404);
    h.server.world.statusPublicEnabled = true;
    final page = (await api.getPublicStatus()).data!;
    expect(page.components.map((c) => c.name), ['api', 'push']);
  });

  test('owner operations: 403 for Sam, one event per change', () async {
    final sam = h.device(login: WorldSeed.samLogin, keySeed: 2);
    await signedIn(h, sam);
    final refused = await sam.api
        .getStatusApi()
        .deleteStatusComponent(componentId: lotse)
        .then<Object?>((_) => null, onError: (Object e) => e);
    expect((refused! as DioException).response?.statusCode, 403);

    final alex = h.device();
    await signedIn(h, alex);
    final before = h.server.world.log.length;
    await alex.api.getStatusApi().deleteStatusComponent(componentId: lotse);
    final events = h.server.world.log.skip(before).toList();
    expect(events.map((e) => e.type), ['status.updated']);
  });
}
