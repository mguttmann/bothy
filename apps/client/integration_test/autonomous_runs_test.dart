// SPDX-License-Identifier: Apache-2.0

// Autonomous runs in the desktop app against the fake server (brief AR-08 Tests; 01 6.21, 09
// 5.8): the flow of support/autonomous_flow.dart on the opened forum post (pending wake-ups,
// the live note of an end, a wake run, Cancel). The phone suite runs the same flow through
// integration_test/phone/autonomous_runs_phone_test.dart.

import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';

import 'support/agent_flow.dart';
import 'support/autonomous_flow.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('pending wake-ups, the note of an end, a wake run, cancel', (
    tester,
  ) async {
    late AutonomousWorld w;
    final opened = await openAgentThread(
      tester,
      setup: (world, seed, forum, thread) =>
          w = seedAutonomous(world, seed, thread),
    );
    try {
      await checkAutonomous(tester, opened.app, w);
    } finally {
      await opened.app.stop();
    }
  });
}
