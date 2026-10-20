// SPDX-License-Identifier: Apache-2.0

// The guided flow of 10 11 from the interface (brief M4-28 Tests, BR-07): an I1 conflict in
// the grant preview, or a `409 i1_violation` when adding a group member or an owner, offers
// "Switch <agent> to Organization"; taking it asks a reason and sends ONE request with
// `resolve_i1`; both changes appear, and the result says that the adapter still reports the
// old mode; a failing request changes nothing and shows the problem.

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart' show ApiProblem;
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../admin/support.dart';

void main() {
  late GlobalKey<NavigatorState> navigator;
  late FakeAdminRepository repository;
  late List<AdminGrant> created;

  /// Support runs on Alex's subscription; Sam would be a second principal (10 5.4).
  InvariantFinding support({bool ready = true}) => i1Finding(
    'support',
    holderId: 'alex',
    principalIds: const ['alex', 'sam'],
    organizationReady: ready,
  );

  setUp(() {
    navigator = GlobalKey<NavigatorState>();
    repository = FakeAdminRepository(
      prompt: StepUpDialogPrompt(() => navigator.currentContext),
    );
    repository.agentList.add(
      const AdminAgent(
        id: 'support',
        handle: 'support',
        displayName: 'Support',
      ),
    );
    created = [];
  });

  Future<void> tap(WidgetTester tester, String key) async {
    await tester.ensureVisible(find.byKey(ValueKey(key)));
    await tester.tap(find.byKey(ValueKey(key)));
    await tester.pumpAndSettle();
  }

  Future<void> type(WidgetTester tester, String key, String text) async {
    await tester.ensureVisible(find.byKey(ValueKey(key)));
    await tester.enterText(find.byKey(ValueKey(key)), text);
    await tester.pumpAndSettle();
  }

  /// Answers the step-up dialog; the confirmation then stays fresh, as on the server.
  Future<void> stepUp(WidgetTester tester) async {
    expect(find.byKey(const ValueKey('stepUp.dialog')), findsOneWidget);
    await tap(tester, 'stepUp.continue');
  }

  FilledButton submit(WidgetTester tester) =>
      tester.widget<FilledButton>(find.byKey(const ValueKey('confirm.submit')));

  /// Takes the guided option: the reason dialog, confirm disabled while empty, then [reason].
  Future<void> take(WidgetTester tester, String reason) async {
    await tap(tester, 'guided.offer');
    expect(find.text('Switch to Organization'), findsOneWidget);
    expect(submit(tester).onPressed, isNull, reason: 'the reason is required');
    await type(tester, 'confirm.reason', reason);
    expect(submit(tester).onPressed, isNotNull);
    await tap(tester, 'confirm.submit');
  }

  /// The result of step 5 of 10 11: the switch is saved and the adapter still reports
  /// personal (the server's warning `billing_mode_mismatch`), with how to switch it.
  Future<void> result(WidgetTester tester) async {
    expect(find.byKey(const ValueKey('guided.result')), findsOneWidget);
    expect(
      find.text('Support now uses Organization, and the change is saved.'),
      findsOneWidget,
    );
    expect(
      find.byKey(const ValueKey('guided.stillReports.support')),
      findsOneWidget,
    );
    expect(
      find.text('The adapter of Support still reports Personal.'),
      findsOneWidget,
    );
    expect(
      find.textContaining('changes the login of its runtime to Organization'),
      findsOneWidget,
    );
    await tap(tester, 'guided.close');
    expect(find.byKey(const ValueKey('guided.result')), findsNothing);
  }

  int count(String op) =>
      repository.calls.where((c) => c.startsWith('$op ')).length;

  group('grant', () {
    Future<void> wizard(WidgetTester tester, {bool ready = true}) async {
      repository.previewAnswer = (r) => GrantPreviewResult(
        allowed: false,
        effects: const [],
        violations: [support(ready: ready)],
        hints: const [],
        text: 'Rejected',
        textLocale: 'en',
      );
      repository.i1On['createGrant'] = [support(ready: ready)];
      await pumpInApp(
        tester,
        GrantWizard(
          repository: repository,
          timeZone: 'Europe/Berlin',
          clock: testClock,
          debounce: Duration.zero,
          onCreated: created.add,
        ),
        navigatorKey: navigator,
      );
      await tap(tester, 'subject.sam');
      await tap(tester, 'wizard.next');
      await tap(tester, 'target.space.marketing');
      await tap(tester, 'wizard.next');
      await tap(tester, 'level.trigger');
      await type(tester, 'wizard.reason', 'Campaign work');
    }

    testWidgets('the preview offers the guided option, one request switches '
        'and grants', (tester) async {
      await wizard(tester);

      // The existing violation stays, the explanation of 10 11 step 2 follows it.
      expect(
        find.byKey(const ValueKey('preview.violation.i1_violation')),
        findsOneWidget,
      );
      expect(find.byKey(const ValueKey('preview.i1Hint')), findsOneWidget);
      expect(find.byKey(const ValueKey('guided.explain')), findsOneWidget);
      expect(
        find.text(
          'This grant allows Sam to trigger the agent Support. Support uses the '
          'personal subscription of Alex. Support must first be switched to '
          'Organization; the organization key then bears the costs.',
        ),
        findsOneWidget,
      );
      expect(
        find.textContaining('does not report an organization key'),
        findsNothing,
      );
      expect(
        find.text('Switch Support to Organization and grant'),
        findsOneWidget,
      );
      expect(
        tester
            .widget<FilledButton>(find.byKey(const ValueKey('wizard.confirm')))
            .onPressed,
        isNull,
        reason:
            'the preview says allowed = false; the guided option is the way',
      );

      await take(tester, 'Sam joins Support');
      await stepUp(tester);
      await result(tester);

      expect(count('createGrant'), 1, reason: 'one request, never two');
      expect(repository.resolutions, ['createGrant support Sam joins Support']);
      expect(repository.grantList, hasLength(1));
      expect(repository.grantList.single.subjectId, 'sam');
      expect(
        repository.agentRecordsById['support']!.mode,
        AgentBillingMode.organization,
      );
      expect(created, hasLength(1));
    });

    testWidgets('a failing request changes nothing and shows the problem', (
      tester,
    ) async {
      repository.guidedFails['createGrant'] = const ApiProblem(
        status: 409,
        code: 'conflict',
      );
      await wizard(tester);
      await take(tester, 'Sam joins Support');
      await stepUp(tester);

      expect(find.byKey(const ValueKey('guided.result')), findsNothing);
      expect(
        find.text('This conflicts with the current state'),
        findsOneWidget,
      );
      expect(count('createGrant'), 1);
      expect(repository.grantList, isEmpty);
      expect(
        repository.agentRecordsById['support']!.mode,
        AgentBillingMode.personal,
      );
      expect(created, isEmpty);
    });

    testWidgets('a conflict that remains is explained with its principals', (
      tester,
    ) async {
      await wizard(tester);
      // The server finds a second agent the preview did not name.
      repository.i1On['createGrant'] = [
        support(),
        i1Finding('texter', holderId: 'alex', principalIds: const ['robin']),
      ];
      await take(tester, 'Sam joins Support');
      await stepUp(tester);

      expect(find.byKey(const ValueKey('guided.result')), findsNothing);
      expect(find.byKey(const ValueKey('i1.explanation')), findsOneWidget);
      expect(find.byKey(const ValueKey('i1.agent.texter')), findsOneWidget);
      expect(repository.grantList, isEmpty);
      expect(
        repository.agentRecordsById['support']!.mode,
        AgentBillingMode.personal,
      );
    });

    testWidgets('organization_ready false adds the not-ready sentence', (
      tester,
    ) async {
      await wizard(tester, ready: false);
      expect(
        find.text(
          'The adapter of Support does not report an organization key yet. '
          'Invocations fail until it is switched.',
        ),
        findsOneWidget,
      );
    });
  });

  group('group member', () {
    Future<void> addSam(WidgetTester tester) async {
      repository.i1On['addMember'] = [support()];
      await pumpInApp(
        tester,
        GroupDetail(
          repository: repository,
          groupId: 'support',
          timeZone: 'Europe/Berlin',
          clock: testClock,
        ),
        navigatorKey: navigator,
      );
      await tap(tester, 'group.addMember');
      await tap(tester, 'member.person');
      await tester.tap(find.text('Sam').last);
      await tester.pumpAndSettle();
      await tap(tester, 'confirm.submit');
      await stepUp(tester);
    }

    testWidgets('a 409 i1_violation offers the guided option, the second '
        'call carries resolve_i1', (tester) async {
      await addSam(tester);

      expect(find.byKey(const ValueKey('guided.explain')), findsOneWidget);
      expect(
        find.text(
          'This change allows Sam to trigger the agent Support. Support uses the '
          'personal subscription of Alex. Support must first be switched to '
          'Organization; the organization key then bears the costs.',
        ),
        findsOneWidget,
      );
      expect(
        find.text('Switch Support to Organization and continue'),
        findsOneWidget,
      );
      expect(find.byKey(const ValueKey('member.sam')), findsNothing);

      await take(tester, 'Sam joins Support');
      await result(tester);

      expect(count('addMember'), 2, reason: 'the 409 and the guided call');
      expect(repository.resolutions, ['addMember support Sam joins Support']);
      expect(find.byKey(const ValueKey('member.sam')), findsOneWidget);
      expect(find.byKey(const ValueKey('guided.explain')), findsNothing);
      expect(
        repository.agentRecordsById['support']!.mode,
        AgentBillingMode.organization,
      );
    });

    testWidgets('a failing guided call changes nothing', (tester) async {
      repository.guidedFails['addMember'] = const ApiProblem(
        status: 409,
        code: 'conflict',
      );
      await addSam(tester);
      await take(tester, 'Sam joins Support');

      expect(find.byKey(const ValueKey('guided.result')), findsNothing);
      expect(
        find.text('This conflicts with the current state'),
        findsOneWidget,
      );
      expect(find.byKey(const ValueKey('member.sam')), findsNothing);
      expect(
        repository.agentRecordsById['support']!.mode,
        AgentBillingMode.personal,
      );
    });
  });

  group('owner', () {
    Future<void> addSam(WidgetTester tester) async {
      repository.i1On['addOwner'] = [support()];
      await pumpInApp(
        tester,
        OwnersSection(
          repository: repository,
          timeZone: 'Europe/Berlin',
          clock: testClock,
        ),
        navigatorKey: navigator,
      );
      await tap(tester, 'owners.add');
      await tap(tester, 'owners.pick');
      await tester.tap(find.text('Sam').last);
      await tester.pumpAndSettle();
      await type(tester, 'confirm.reason', 'Deputy');
      await tap(tester, 'confirm.submit');
      await stepUp(tester);
    }

    testWidgets('a 409 i1_violation offers the guided option, the second '
        'call carries resolve_i1', (tester) async {
      await addSam(tester);

      expect(find.byKey(const ValueKey('guided.explain')), findsOneWidget);
      expect(
        find.text('Switch Support to Organization and continue'),
        findsOneWidget,
      );
      expect(repository.ownerIds, isNot(contains('sam')));

      await take(tester, 'Sam becomes deputy');
      await result(tester);

      expect(count('addOwner'), 2);
      expect(repository.calls.last, 'addOwner sam Deputy');
      expect(repository.resolutions, ['addOwner support Sam becomes deputy']);
      expect(repository.ownerIds, contains('sam'));
      expect(find.byKey(const ValueKey('owner.sam')), findsOneWidget);
      expect(
        repository.agentRecordsById['support']!.mode,
        AgentBillingMode.organization,
      );
    });

    testWidgets('a failing guided call changes nothing', (tester) async {
      repository.guidedFails['addOwner'] = const ApiProblem(
        status: 409,
        code: 'conflict',
      );
      await addSam(tester);
      await take(tester, 'Sam becomes deputy');

      expect(find.byKey(const ValueKey('guided.result')), findsNothing);
      expect(
        find.text('This conflicts with the current state'),
        findsOneWidget,
      );
      expect(repository.ownerIds, isNot(contains('sam')));
      expect(
        repository.agentRecordsById['support']!.mode,
        AgentBillingMode.personal,
      );
    });
  });
}
