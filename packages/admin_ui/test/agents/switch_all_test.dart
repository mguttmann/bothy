// SPDX-License-Identifier: Apache-2.0

// Switching several agents (brief M4-28 Tests and Design note 3; 10 9.1, BR-08): "All to
// Organization" needs only a reason; "All to Personal" needs a holder and an explicit list,
// proposed from the last global switch in the audit (step-up of audit.read) and editable; one
// violating agent changes nothing and the dialog shows the server's conflicts.

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../admin/support.dart';

const _purpose = 'Switch the backend of several agents';

void main() {
  late GlobalKey<NavigatorState> navigator;
  late FakeAdminRepository repository;
  late List<SwitchAllResult?> results;

  setUp(() {
    navigator = GlobalKey<NavigatorState>();
    repository = FakeAdminRepository(
      prompt: StepUpDialogPrompt(() => navigator.currentContext),
    );
    results = [];
    // The state after an earlier "all to organization": Support ran on Alex's subscription.
    repository.agentRecordsById['support'] = fakeAgentRecord(
      'support',
      name: 'Support',
      reported: AgentBillingMode.personal,
    );
  });

  Future<void> tap(WidgetTester tester, String key) async {
    await tester.ensureVisible(find.byKey(ValueKey(key)));
    await tester.tap(find.byKey(ValueKey(key)));
    await tester.pumpAndSettle();
  }

  Future<void> stepUp(WidgetTester tester) async {
    expect(find.byKey(const ValueKey('stepUp.dialog')), findsOneWidget);
    expect(find.text(_purpose), findsOneWidget);
    await tap(tester, 'stepUp.continue');
    repository.stepUpFresh = false;
  }

  Future<void> open(WidgetTester tester) async {
    await pumpInApp(
      tester,
      Builder(
        builder: (context) => TextButton(
          key: const ValueKey('launch'),
          onPressed: () async => results.add(
            await SwitchAllDialog.show(
              context,
              repository: repository,
              agents: repository.agentRecordsById.values.toList(),
            ),
          ),
          child: const Text('launch'),
        ),
      ),
      navigatorKey: navigator,
    );
    await tap(tester, 'launch');
    expect(find.text('Switch several agents'), findsOneWidget);
  }

  bool checked(WidgetTester tester, String id) =>
      tester
          .widget<CheckboxListTile>(find.byKey(ValueKey('switchAll.agent.$id')))
          .value ??
      false;

  Future<void> reason(WidgetTester tester, String text) async {
    await tester.enterText(
      find.byKey(const ValueKey('switchAll.reason')),
      text,
    );
    await tester.pump();
  }

  bool canSubmit(WidgetTester tester) =>
      tester
          .widget<FilledButton>(find.byKey(const ValueKey('switchAll.submit')))
          .onPressed !=
      null;

  testWidgets('all to organization needs only a reason', (tester) async {
    repository.agentRecordsById['support'] = fakeAgentRecord(
      'support',
      name: 'Support',
      mode: AgentBillingMode.personal,
      holderId: 'alex',
      reported: AgentBillingMode.personal,
    );
    await open(tester);

    await tap(tester, 'switchAll.mode.organization');
    expect(
      find.textContaining('Every agent then runs on the organization key'),
      findsOneWidget,
    );
    expect(find.byKey(const ValueKey('switchAll.holder')), findsNothing);
    expect(find.byKey(const ValueKey('switchAll.agent.support')), findsNothing);
    expect(canSubmit(tester), isFalse);
    expect(repository.stepUps, isEmpty);

    await reason(tester, 'Doubts about personal use');
    expect(canSubmit(tester), isTrue);
    await tap(tester, 'switchAll.submit');
    await stepUp(tester);

    // Without agent IDs: the server takes every personal agent (10 9.1).
    expect(repository.calls, [
      'setAllModes organization - - Doubts about personal use',
    ]);
    expect(repository.stepUps, [_purpose]);
    expect(find.byType(SwitchAllDialog), findsNothing);
    expect(results.single?.changedAgentIds, ['support']);
    expect(
      repository.agentRecordsById['support']!.mode,
      AgentBillingMode.organization,
    );
  });

  testWidgets('all to personal proposes the agents of the last global switch', (
    tester,
  ) async {
    repository.proposal = const SwitchAllProposal([
      'support',
    ], holderId: 'alex');
    await open(tester);

    await tap(tester, 'switchAll.mode.personal');
    // The proposal reads the audit, which has step-up (05 audit.read).
    await stepUp(tester);
    expect(
      find.textContaining('The checked agents then run on the personal'),
      findsOneWidget,
    );
    expect(
      find.text(
        'Proposed: the agents the last switch of several agents to '
        'Organization changed',
      ),
      findsOneWidget,
    );
    expect(checked(tester, 'support'), isTrue);
    expect(checked(tester, 'texter'), isFalse);
    expect(
      find.descendant(
        of: find.byKey(const ValueKey('switchAll.holder')),
        matching: find.text('Alex'),
      ),
      findsOneWidget,
    );

    // The list is editable.
    await tap(tester, 'switchAll.agent.texter');
    await tap(tester, 'switchAll.agent.support');
    expect(checked(tester, 'texter'), isTrue);
    expect(checked(tester, 'support'), isFalse);
    await tap(tester, 'switchAll.agent.support');
    await tap(tester, 'switchAll.agent.texter');

    await reason(tester, 'Back to personal');
    await tap(tester, 'switchAll.submit');
    await stepUp(tester);

    expect(repository.calls, [
      'setAllModes personal alex support Back to personal',
    ]);
    expect(repository.stepUps, [_purpose, _purpose]);
    expect(results.single?.changedAgentIds, ['support']);
    expect(repository.agentRecordsById['support']!.holderId, 'alex');
  });

  testWidgets('without a proposal the person picks holder and agents', (
    tester,
  ) async {
    await open(tester);

    await tap(tester, 'switchAll.mode.personal');
    await stepUp(tester);
    expect(
      find.text(
        'No earlier switch to propose from. Check the agents yourself.',
      ),
      findsOneWidget,
    );
    expect(checked(tester, 'support'), isFalse);
    expect(checked(tester, 'texter'), isFalse);

    await reason(tester, 'Sam pays');
    expect(canSubmit(tester), isFalse);
    await tap(tester, 'switchAll.holder');
    await tester.tap(find.text('Sam').last);
    await tester.pumpAndSettle();
    expect(canSubmit(tester), isFalse);
    await tap(tester, 'switchAll.agent.texter');
    expect(canSubmit(tester), isTrue);
    await tap(tester, 'switchAll.submit');
    await stepUp(tester);

    expect(repository.calls, ['setAllModes personal sam texter Sam pays']);
  });

  testWidgets('BR-08: one violating agent changes nothing and is explained', (
    tester,
  ) async {
    repository.proposal = const SwitchAllProposal([
      'support',
    ], holderId: 'alex');
    repository.i1On['setAllModes'] = [
      i1Finding('support', holderId: 'alex', principalIds: ['alex', 'sam']),
    ];
    final before = Map.of(repository.agentRecordsById);
    await open(tester);

    await tap(tester, 'switchAll.mode.personal');
    await stepUp(tester);
    await tap(tester, 'switchAll.agent.texter');
    await reason(tester, 'Back to personal');
    await tap(tester, 'switchAll.submit');
    await stepUp(tester);

    expect(repository.calls, [
      'setAllModes personal alex support,texter Back to personal',
    ]);
    // The dialog stays open with the server's conflicts.
    expect(find.byType(SwitchAllDialog), findsOneWidget);
    expect(find.byKey(const ValueKey('i1.explanation')), findsOneWidget);
    expect(find.byKey(const ValueKey('i1.agent.support')), findsOneWidget);
    expect(
      find.text(
        'Support would use the subscription of Alex, but these may trigger '
        'it too: Sam',
      ),
      findsOneWidget,
    );
    expect(find.byKey(const ValueKey('switchAll.failure')), findsOneWidget);
    expect(results, isEmpty);
    for (final MapEntry(:key, :value) in before.entries) {
      expect(identical(repository.agentRecordsById[key], value), isTrue);
    }

    await tap(tester, 'switchAll.cancel');
    expect(results, [null]);
  });

  testWidgets('another failure shows its text; the dialog stays', (
    tester,
  ) async {
    repository.refuse['setAllModes'] = const ApiProblem(
      status: 409,
      code: 'version_conflict',
    );
    await open(tester);
    await tap(tester, 'switchAll.mode.organization');
    await reason(tester, 'Doubts');
    await tap(tester, 'switchAll.submit');

    expect(find.byKey(const ValueKey('switchAll.failure')), findsOneWidget);
    expect(find.byKey(const ValueKey('i1.explanation')), findsNothing);
    expect(find.byType(SwitchAllDialog), findsOneWidget);
  });
}
