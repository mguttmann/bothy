// SPDX-License-Identifier: Apache-2.0

// 04 16.5 against SyncEngine with its local database: an agent run with a follow-up and coalesced live box patches (brief M4-17, Tests).

import 'package:sync_engine/testing.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  test('16.5 agent run with follow-up and live box patches', () async {
    await withScenario(scenarioAgentRun);
  });
}
