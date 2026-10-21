// SPDX-License-Identifier: Apache-2.0

// The agent list of A5 (brief M4-28 Tests, fixed requirement 11): the badge "Personal" or
// "Organization" per row with the holder, the server's warning count, opening a row and "Switch
// all agents" opening the dialog of 10 9.1.

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../admin/support.dart';

void main() {
  late GlobalKey<NavigatorState> navigator;
  late FakeAdminRepository repository;
  late List<String> opened;
  late int changed;

  setUp(() {
    navigator = GlobalKey<NavigatorState>();
    repository = FakeAdminRepository(
      prompt: StepUpDialogPrompt(() => navigator.currentContext),
    );
    opened = [];
    changed = 0;
  });

  Future<void> tap(WidgetTester tester, String key) async {
    await tester.ensureVisible(find.byKey(ValueKey(key)));
    await tester.tap(find.byKey(ValueKey(key)));
    await tester.pumpAndSettle();
  }

  Future<void> list(WidgetTester tester) async {
    await pumpInApp(
      tester,
      AgentsList(
        repository: repository,
        onOpen: opened.add,
        onChanged: () => changed++,
      ),
      navigatorKey: navigator,
    );
    await tester.pumpAndSettle();
  }

  Finder inRow(String id, Finder matching) => find.descendant(
    of: find.byKey(ValueKey('agent.$id')),
    matching: matching,
  );

  testWidgets('every row shows its badge, the holder and the handle', (
    tester,
  ) async {
    await list(tester);

    expect(find.byKey(const ValueKey('agents')), findsOneWidget);
    expect(find.text('Agents'), findsOneWidget);
    expect(inRow('support', find.byType(AgentModeBadge)), findsOneWidget);
    expect(inRow('support', find.text('Personal')), findsOneWidget);
    expect(inRow('support', find.text('Holder: Alex')), findsOneWidget);
    expect(
      tester
          .widget<AgentModeBadge>(inRow('support', find.byType(AgentModeBadge)))
          .holderName,
      'Alex',
    );
    expect(inRow('texter', find.text('Organization')), findsOneWidget);
    expect(inRow('texter', find.textContaining('Holder')), findsNothing);
    expect(inRow('texter', find.textContaining('texter')), findsOneWidget);
    expect(inRow('support', find.text('Disabled')), findsNothing);
  });

  testWidgets('the warning count is the server\'s, a disabled agent says so', (
    tester,
  ) async {
    repository.agentRecordsById['texter'] = fakeAgentRecord(
      'texter',
      name: 'Texter',
      reported: AgentBillingMode.personal,
      keyType: 'unknown',
      active: false,
    );
    // The client never compares modes: a mismatch the server does not report has no warning.
    repository.agentRecordsById['support'] = fakeAgentRecord(
      'support',
      name: 'Support',
      mode: AgentBillingMode.personal,
      holderId: 'alex',
      reported: AgentBillingMode.organization,
      warnings: const [],
    );
    await list(tester);

    expect(
      tester
          .widget<Text>(find.byKey(const ValueKey('agent.texter.warnings')))
          .data,
      '2 warnings',
    );
    expect(inRow('texter', find.text('Disabled')), findsOneWidget);
    expect(find.byKey(const ValueKey('agent.support.warnings')), findsNothing);
  });

  testWidgets('a row opens its agent', (tester) async {
    await list(tester);
    await tap(tester, 'agent.texter');
    expect(opened, ['texter']);
  });

  testWidgets('an empty list says so', (tester) async {
    repository.agentRecordsById.clear();
    await list(tester);
    expect(find.text('No agents.'), findsOneWidget);
  });

  testWidgets('a refused list shows the server\'s reason and nothing else', (
    tester,
  ) async {
    repository.refuse['agentRecords'] = const ApiProblem(
      status: 403,
      code: 'forbidden',
    );
    await list(tester);
    expect(find.byKey(const ValueKey('admin.denied')), findsOneWidget);
    expect(find.byKey(const ValueKey('agents.switchAll')), findsNothing);
  });

  testWidgets('switch all opens the dialog; after a switch the list reloads', (
    tester,
  ) async {
    await list(tester);

    await tap(tester, 'agents.switchAll');
    expect(find.byType(SwitchAllDialog), findsOneWidget);
    expect(find.text('Switch several agents'), findsOneWidget);
    await tap(tester, 'switchAll.mode.organization');
    await tester.enterText(
      find.byKey(const ValueKey('switchAll.reason')),
      'Doubts',
    );
    await tester.pump();
    await tap(tester, 'switchAll.submit');
    await tap(tester, 'stepUp.continue');

    expect(find.byType(SwitchAllDialog), findsNothing);
    expect(find.text('1 agent changed'), findsOneWidget);
    expect(inRow('support', find.text('Organization')), findsOneWidget);
    expect(changed, 1);
  });

  testWidgets('a cancelled dialog changes nothing', (tester) async {
    await list(tester);
    await tap(tester, 'agents.switchAll');
    await tap(tester, 'switchAll.cancel');
    expect(find.byType(SwitchAllDialog), findsNothing);
    expect(repository.calls, isEmpty);
    expect(changed, 0);
  });
}
