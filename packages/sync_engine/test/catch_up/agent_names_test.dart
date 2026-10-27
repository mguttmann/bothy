// SPDX-License-Identifier: Apache-2.0

// The agents of a full snapshot are named on a new device (04 6.3 item 4; 15: the `principals`
// and `agents` tables give display names). The snapshot carries only their presence, and a
// device that signs in after an agent was created never receives its `agent.updated`, so the
// thread header and the running view showed the agent's ID (found on the desktop app against
// the demo server). After the commit the listener reads every agent it does not name yet
// (`getAgent`) and stores its card and principal row.

import 'package:domain/domain.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late World w;
  setUp(() => w = World());
  tearDown(() => w.dispose());

  final robin = PrincipalId.parse(robinId);
  final scout = id(0xa4);

  Map<String, Object?> agentJson(String agent, String handle, String name) => {
    'id': agent,
    'handle': handle,
    'display_name': name,
    'description': '',
    'billing_mode': 'organization',
    'tool_class': 'chat',
    'adapter_kind': 'demo',
    'status': 'active',
    'operator_config': <String, Object?>{},
    'presence': {'state': 'online'},
    'version': 1,
  };

  Map<String, Object?> run(String agent) => {
    'id': id(0x700),
    'thread_id': threadId(1),
    'channel_id': channelId(1),
    'agent_id': agent,
    'triggered_by': alexId,
    'state': 'running',
    'version': 1,
    'billing_mode_expected': 'organization',
    'cost': {'amount_micros': 0, 'currency': 'USD', 'is_estimate': true},
    'step_count': 0,
    'queued_at': at,
  };

  test('a full snapshot names its agents and the agents of its runs', () async {
    w.rest.routes['GET /agents/$robinId'] = (_) =>
        FakeReply(200, agentJson(robinId, 'robin', 'Robin'));
    w.rest.routes['GET /agents/$scout'] = (_) =>
        FakeReply(200, agentJson(scout, 'paper-scout', 'Paper Scout'));
    final snapshot = snapshotJson(
      agents: [
        {'agent_id': robinId, 'state': 'online'},
      ],
    )..['active_runs'] = [run(scout)];
    await w.connect(snapshot: snapshot);
    await w.push('catchup.complete', {'head_seq': 40});

    final named = await w.db.principalsDao.byId(robin);
    expect(named?.displayName, 'Robin');
    expect(named?.kind, PrincipalKind.agent);
    expect((await w.db.principalsDao.agentById(robin))?.handle, 'robin');
    expect(
      (await w.db.principalsDao.byId(PrincipalId.parse(scout)))?.displayName,
      'Paper Scout',
    );
  });

  test('an agent with presence and a run is read once', () async {
    w.rest.routes['GET /agents/$robinId'] = (_) =>
        FakeReply(200, agentJson(robinId, 'robin', 'Robin'));
    final snapshot = snapshotJson(
      agents: [
        {'agent_id': robinId, 'state': 'online'},
      ],
    )..['active_runs'] = [run(robinId)];
    await w.connect(snapshot: snapshot);
    await w.push('catchup.complete', {'head_seq': 40});

    expect(w.rest.count('/agents/$robinId'), 1);
    expect((await w.db.principalsDao.byId(robin))?.displayName, 'Robin');
  });

  test(
    'a refused read leaves the agent unnamed and the sync running',
    () async {
      await w.connect(
        snapshot: snapshotJson(
          agents: [
            {'agent_id': robinId, 'state': 'online'},
          ],
        ),
      );
      await w.push('catchup.complete', {'head_seq': 40});

      expect(w.rest.count('/agents/$robinId'), 1);
      expect(await w.db.principalsDao.byId(robin), isNull);
      expect(await w.cursor(), 40);
    },
  );

  test('a later agent.updated renames the agent by its version', () async {
    w.rest.routes['GET /agents/$robinId'] = (_) =>
        FakeReply(200, agentJson(robinId, 'robin', 'Robin'));
    await w.connect(
      snapshot: snapshotJson(
        agents: [
          {'agent_id': robinId, 'state': 'online'},
        ],
      ),
    );
    await w.push('catchup.complete', {'head_seq': 40});
    final renamed = agentJson(robinId, 'robin', 'Robin Bot')..['version'] = 2;
    await w.dispatch([
      envelope(
        'agent.updated',
        seq: 41,
        payload: {'agent': renamed},
        resourceKind: 'agent',
        resourceId: robinId,
        version: 2,
      ),
    ]);

    expect((await w.db.principalsDao.byId(robin))?.displayName, 'Robin Bot');
  });
}
