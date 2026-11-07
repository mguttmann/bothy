// SPDX-License-Identifier: Apache-2.0

// Autonomous runs on the phone against the fake server (brief AR-08 Tests; 01 6.21, 09 5.8):
// the flow of support/autonomous_flow.dart on a forum post opened from the spaces page
// (pending wake-ups above the composer, the live note of an end, a wake run, Cancel).

import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';

import '../support/autonomous_flow.dart';
import 'support/phone_flow.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('pending wake-ups, the note of an end, a wake run, cancel', (
    tester,
  ) async {
    await asPhone(() async {
      final app = await startPhone(tester);
      final world = app.fake.server.world;
      final seed = app.fake.seed;
      final forum = world.addForum(seed.space, 'deploys', {
        seed.alex.id: 'trigger',
        seed.robin.id: 'write',
      });
      final thread = world.addThread(
        forum,
        'Release checklist',
        createdBy: seed.alex.id,
      );
      final w = seedAutonomous(world, seed, thread);
      try {
        await signIn(app);
        await openThread(app, forum, thread, w.trigger);
        await checkAutonomous(tester, app, w);
      } finally {
        await app.stop();
      }
    });
  });
}
