// SPDX-License-Identifier: Apache-2.0

// The agent card of A5 with a built-in agent (brief A1-09 Scope: the agent page actions; 01
// 6.20 item 6): the card of an agent of kind claude_agent shows the built-in section with its
// actions when the host offers them, another agent never does, and "Switch billing" after
// hosted_holder_mismatch opens the billing switch of the card.

import 'package:admin_ui/admin_ui.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../admin/support.dart';
import '../new_agent/fake_new_agent_port.dart';

AgentRecord _builtin() => const AgentRecord(
  id: 'a',
  handle: 'notes',
  displayName: 'Notes',
  active: true,
  adapterKind: builtinAdapterKind,
  description: '',
  mode: AgentBillingMode.personal,
  holderId: 'alex',
  toolClass: 'chat',
  runtime: AgentRuntimeInfo(state: AgentLiveStateKind.online),
  version: 1,
);

void main() {
  late GlobalKey<NavigatorState> navigator;
  late FakeAdminRepository repository;
  late FakeNewAgentPort port;

  setUp(() {
    navigator = GlobalKey<NavigatorState>();
    repository = FakeAdminRepository(
      prompt: StepUpDialogPrompt(() => navigator.currentContext),
    );
    repository.agentRecordsById['a'] = _builtin();
    port = FakeNewAgentPort();
  });

  Future<void> page(
    WidgetTester tester,
    String id, {
    bool withHost = true,
  }) async {
    await pumpInApp(
      tester,
      AgentPage(
        repository: repository,
        agentId: id,
        builtin: withHost
            ? NewAgentHost(
                port: port,
                newKey: () => 'key',
                announceMinInterval: const Duration(seconds: 10),
              )
            : null,
      ),
      navigatorKey: navigator,
    );
    await tester.pumpAndSettle();
  }

  testWidgets('a built-in agent shows its section with the actions', (
    tester,
  ) async {
    port.addAgent(
      'a',
      definitions: [
        {
          'instructions': 'Keep the notes.',
          'skills': <Object?>[],
          'mcp_servers': <Object?>[],
          'settings': {'tool_class_wish': 'chat'},
          'origin': {'kind': 'manual'},
        },
      ],
    );
    await page(tester, 'a');
    expect(find.byKey(const ValueKey('builtinAgent')), findsOneWidget);
    expect(find.byKey(const ValueKey('builtinAgent.edit')), findsOneWidget);
    expect(find.text('Built-in agent'), findsOneWidget);
  });

  testWidgets('another agent has no such section', (tester) async {
    await page(tester, 'texter');
    expect(find.byKey(const ValueKey('agentPage')), findsOneWidget);
    expect(find.byKey(const ValueKey('builtinAgent')), findsNothing);
  });

  testWidgets('without a host the card shows no section', (tester) async {
    port.addAgent('a');
    await page(tester, 'a', withHost: false);
    expect(find.byKey(const ValueKey('builtinAgent')), findsNothing);
  });

  testWidgets(
    'Switch billing after hosted_holder_mismatch opens the billing switch',
    (tester) async {
      port
        ..offerValue = const NewAgentOffer(
          templates: [],
          runners: [alexRunner, devRunner],
        )
        ..addAgent(
          'a',
          hostedBy: 'runner-2',
          billing: 'personal',
          holder: 'alex',
          definitions: [
            {
              'instructions': 'Keep the notes.',
              'skills': <Object?>[],
              'mcp_servers': <Object?>[],
              'settings': {'tool_class_wish': 'chat'},
              'origin': {'kind': 'manual'},
            },
          ],
        );
      await page(tester, 'a');
      final move = find.byKey(const ValueKey('builtinAgent.changeRunner'));
      await tester.ensureVisible(move);
      await tester.tap(move);
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey('agentHost.runner.runner-1')));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey('agentHost.confirm')));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey('agentHost.switchBilling')));
      await tester.pumpAndSettle();
      expect(find.byType(SwitchModeDialog), findsOneWidget);
    },
  );
}
