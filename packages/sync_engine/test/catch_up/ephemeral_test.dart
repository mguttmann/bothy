// SPDX-License-Identifier: Apache-2.0

// Typing and presence in memory (04 12, brief M4-14 Tests): typing expires after the TTL and at
// the next message of the principal; presence comes from the snapshot and from events; run
// updates go raw to the run store hook.

import 'package:domain/domain.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late World w;
  setUp(() => w = World(overrides: {'sync.typing_ttl': '8s'}));
  tearDown(() => w.dispose());

  final t1 = ThreadId.parse(threadId(1));
  final sam = PrincipalId.parse(samId);
  final robin = PrincipalId.parse(robinId);

  Map<String, Object?> typing(DateTime expires) => envelope(
    'typing.started',
    payload: {
      'thread_id': threadId(1),
      'principal_id': samId,
      'expires_at': expires.toIso8601String(),
    },
    resourceKind: 'thread',
    resourceId: threadId(1),
  );

  Future<void> live() async {
    await w.connect(
      cursor: 30,
      snapshot: snapshotJson(
        agents: [
          {'agent_id': robinId, 'state': 'online'},
        ],
      ),
    );
    await w.push('catchup.complete', {'head_seq': 30});
  }

  test('typing expires after the TTL, even with a later expires_at', () async {
    await live();
    final now = w.h.clock.now();
    await w.dispatch([typing(now.add(const Duration(minutes: 5)))]);
    expect(w.catchUp.ephemeral.typingIn(t1), {sam});
    await w.h.clock.elapse(const Duration(seconds: 7));
    expect(w.catchUp.ephemeral.typingIn(t1), {sam});
    await w.h.clock.elapse(const Duration(seconds: 2));
    expect(w.catchUp.ephemeral.typingIn(t1), isEmpty);
  });

  test('typing ends at the next message of the principal', () async {
    await live();
    final now = w.h.clock.now();
    await w.dispatch([typing(now.add(const Duration(seconds: 6)))]);
    expect(w.catchUp.ephemeral.typingIn(t1), {sam});
    await w.dispatch([
      threadCreated(31, 1),
      messageCreated(32, 1, author: samId),
    ]);
    expect(w.catchUp.ephemeral.typingIn(t1), isEmpty);
    expect(
      await w.db.instanceDao.get(),
      isNull,
      reason: 'nothing ephemeral is stored',
    );
  });

  test('presence from the snapshot and from events', () async {
    await live();
    expect(
      w.catchUp.ephemeral.presenceOf(robin)!.state,
      AgentRuntimeState.fromCode('online'),
    );
    await w.dispatch([
      envelope(
        'agent.presence_changed',
        payload: {
          'agent_id': robinId,
          'state': 'degraded',
          'reported_billing_mode': 'personal',
        },
        resourceKind: 'agent',
        resourceId: robinId,
      ),
    ]);
    final p = w.catchUp.ephemeral.presenceOf(robin)!;
    expect(p.state, AgentRuntimeState.fromCode('degraded'));
    expect(p.reportedBillingMode, BillingMode.fromCode('personal'));
  });

  test(
    'run.updated goes raw to the run store hook, also when truncated',
    () async {
      await live();
      await w.dispatch([
        envelope(
          'run.updated',
          payload: {'run_id': id(0x401), 'version': 7},
          resourceKind: 'run',
          resourceId: id(0x401),
          version: 7,
          truncated: true,
        ),
      ]);
      expect(w.runUpdates.single, {'run_id': id(0x401), 'version': 7});
    },
  );
}
