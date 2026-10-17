// SPDX-License-Identifier: Apache-2.0

// The five scenarios of 04 16 (M4-17) through the engine providers of the running app
// (brief M4-18 Design note 9).

import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';
import 'package:sync_engine/testing.dart';

import '../test/support/app_scenario_client.dart';
import '../test/support/fakes.dart';
import 'support/test_app.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  final scenarios =
      <String, Future<void> Function(FakeServer, WorldSeed, ScenarioClient)>{
        '16.1 send online': scenarioSendOnline,
        '16.2 send offline': scenarioSendOffline,
        '16.3 catch-up': scenarioCatchUp,
        '16.3 resync': scenarioResync,
        '16.4 access removal': scenarioAccessRemoval,
        '16.5 agent run': scenarioAgentRun,
      };
  for (final MapEntry(key: name, value: run) in scenarios.entries) {
    testWidgets(name, (tester) async {
      final app = await TestApp.start(tester);
      final client = AppScenarioClient(
        app.container,
        app.fake.server,
        app.fake.ports.network as SwitchNetwork,
      );
      try {
        await run(app.fake.server, app.fake.seed, client);
      } finally {
        await client.stop();
        await app.stop();
      }
    });
  }
}
