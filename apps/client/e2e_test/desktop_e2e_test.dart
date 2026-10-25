// SPDX-License-Identifier: Apache-2.0

// The desktop E2E suite (brief M4-34; docs/spec/15-client-architecture.md 20 row "E2E on
// devices"): the app against the real server of the Compose stack of infra/compose/e2e.sh, one
// test per flow of the brief's product requirement 13, each from a clean data folder. `make
// e2e-desktop` runs it through tool/e2e_suite.sh; it is not part of integration_test/, so the
// platform jobs without a server never pick it up.

import 'package:integration_test/integration_test.dart';

import 'flows/agent_flows.dart';
import 'flows/channel_flows.dart';
import 'flows/file_flows.dart';
import 'flows/keyboard_flows.dart';
import 'flows/login_flows.dart';
import 'flows/messaging_flows.dart';
import 'flows/search_flows.dart';
import 'flows/session_flows.dart';
import 'support/e2e_context.dart';
import 'support/e2e_world.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();
  final c = E2eContext(E2eWorld.fromEnvironment())..register();
  loginFlows(c);
  messagingFlows(c);
  agentFlows(c);
  fileFlows(c);
  searchFlows(c);
  channelFlows(c);
  keyboardFlows(c);
  // Last: the revocation ends a device of Alex.
  sessionFlows(c);
}
