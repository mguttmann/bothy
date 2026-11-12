// SPDX-License-Identifier: Apache-2.0

// Desktop integration tests in one app build (docs/decisions.d/AUD-client-misc-01.md, ruling
// AUD-CM01/2): `make apps-platform` builds the app once for every `flutter test` call, so this
// entry runs the files below in one build on macOS and on Windows, as
// `phone_suite_test.dart` does for the phone tests. Each file also runs alone. A new desktop
// integration test gets its group here or a line of its own in `platform-tests.json`;
// `tools/ci/tests/test_integration_tests_run.py` fails for a file that no command reaches.

import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';

import 'agent_typing_test.dart' as agent_typing;
import 'card_test.dart' as card;
import 'command_test.dart' as command;
import 'edit_agent_test.dart' as edit_agent;
import 'guided_flow_test.dart' as guided_flow;
import 'host_admin_test.dart' as host_admin;
import 'interaction_rules_test.dart' as interaction_rules;
import 'new_agent_test.dart' as new_agent;
import 'removal_test.dart' as removal;
import 'run_followup_test.dart' as run_followup;
import 'upload_outbox_test.dart' as upload_outbox;
import 'voice_test.dart' as voice;

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();
  group('interaction rules', interaction_rules.main);
  group('removal', removal.main);
  group('uploads in the outbox', upload_outbox.main);
  group('voice message', voice.main);
  group('card', card.main);
  group('command', command.main);
  group('run with a follow-up', run_followup.main);
  group('agent typing', agent_typing.main);
  group('guided flow', guided_flow.main);
  group('host administration', host_admin.main);
  group('new agent', new_agent.main);
  group('edit agent', edit_agent.main);
}
