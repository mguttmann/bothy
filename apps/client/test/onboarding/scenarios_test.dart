// SPDX-License-Identifier: Apache-2.0

// The five scenarios of 04 16 through the app's engine providers, headless (brief M4-18 Design
// note 9); integration_test/scenarios_test.dart runs the same drivers in the desktop app.

import 'dart:io';

import 'package:client/app/providers/ports.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/testing.dart';

import '../support/app_scenario_client.dart';
import '../support/fake_instance.dart';
import '../support/fakes.dart';

void main() {
  final scenarios = <String, Future<void> Function(FakeServer, WorldSeed, ScenarioClient)>{
    '16.1 send online': scenarioSendOnline,
    '16.2 send offline': scenarioSendOffline,
    '16.3 catch-up': scenarioCatchUp,
    '16.3 resync': scenarioResync,
    '16.4 access removal': scenarioAccessRemoval,
    '16.5 agent run': scenarioAgentRun,
  };
  for (final MapEntry(key: name, value: run) in scenarios.entries) {
    test(name, () async {
      final saved = HttpOverrides.current;
      HttpOverrides.global = null;
      final f = await FakeInstance.start();
      final network = f.ports.network as SwitchNetwork;
      final c = ProviderContainer(overrides: [appPortsProvider.overrideWithValue(f.ports)]);
      final client = AppScenarioClient(c, f.server, network);
      try {
        await run(f.server, f.seed, client);
      } finally {
        await client.stop();
        c.dispose();
        await f.stop();
        HttpOverrides.global = saved;
      }
    });
  }
}
