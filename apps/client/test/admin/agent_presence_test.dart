// SPDX-License-Identifier: Apache-2.0

// The presence of the agent page of A5 in the desktop app (brief M4-28 Design note 1): the
// engine's EphemeralStore feeds the page; an agent the store does not know keeps the presence
// of getAgent, so the stream skips null.

import 'package:admin_ui/admin_ui.dart';
import 'package:auth/auth.dart' show SystemClock;
import 'package:client/admin/admin_route.dart';
import 'package:domain/domain.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/sync_engine.dart';

void main() {
  const agent = '0192f4a0-7b1c-7000-8000-000000000a01';

  test('the store feeds the page and skips an unknown presence', () async {
    final store = EphemeralStore(
      clock: const SystemClock(),
      typingTtl: const Duration(seconds: 5),
    );
    final seen = <AgentLiveState>[];
    final sub = agentPresenceOf(store, agent).listen(seen.add);
    await pumpEventQueue();
    expect(seen, isEmpty);

    store.presenceChanged(
      PrincipalId.parse(agent),
      const AgentPresence(
        state: AgentRuntimeState.degraded,
        reportedBillingMode: BillingMode.personal,
      ),
    );
    await pumpEventQueue();
    expect(seen.single.state, AgentLiveStateKind.degraded);
    expect(seen.single.reportedMode, AgentBillingMode.personal);

    store.presenceChanged(
      PrincipalId.parse(agent),
      const AgentPresence(state: AgentRuntimeState.offline),
    );
    await pumpEventQueue();
    expect(seen.last.state, AgentLiveStateKind.offline);
    expect(seen.last.reportedMode, isNull);
    await sub.cancel();
    await store.dispose();
  });

  test('an ID that is no UUID gives no presence', () async {
    final store = EphemeralStore(
      clock: const SystemClock(),
      typingTtl: const Duration(seconds: 5),
    );
    expect(await agentPresenceOf(store, 'support').toList(), isEmpty);
    await store.dispose();
  });
}
