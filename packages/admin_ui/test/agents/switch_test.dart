// SPDX-License-Identifier: Apache-2.0

// Switching the backend of one agent and changing its holder (brief M4-28 Tests, fixed
// requirements 13 and 14; 10 5.4, 9.1): a reason is required, the change goes through the
// step-up, to organization clears the holder, to personal needs a holder, a `409
// i1_violation` is explained with the conflicts without names the caller may not read and
// changes nothing, and a refused switch hides its control (15 1 principle 1).

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart' show ApiProblem;
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../admin/support.dart';

void main() {
  late GlobalKey<NavigatorState> navigator;
  late FakeAdminRepository repository;
  late int changed;

  setUp(() {
    navigator = GlobalKey<NavigatorState>();
    repository = FakeAdminRepository(
      prompt: StepUpDialogPrompt(() => navigator.currentContext),
    );
    changed = 0;
  });

  Future<void> page(WidgetTester tester, String agentId) => pumpInApp(
    tester,
    AgentPage(
      repository: repository,
      agentId: agentId,
      clock: testClock,
      onChanged: () => changed++,
    ),
    navigatorKey: navigator,
  );

  Future<void> tap(WidgetTester tester, String key) async {
    await tester.ensureVisible(find.byKey(ValueKey(key)).last);
    await tester.tap(find.byKey(ValueKey(key)).last);
    await tester.pumpAndSettle();
  }

  Future<void> stepUp(WidgetTester tester, String purpose) async {
    expect(find.byKey(const ValueKey('stepUp.dialog')), findsOneWidget);
    expect(find.text(purpose), findsOneWidget);
    await tap(tester, 'stepUp.continue');
    repository.stepUpFresh = false;
  }

  // The menu shows the items without their keys (those stay on the field), so pick by name.
  Future<void> pick(WidgetTester tester, String name) async {
    await tester.tap(find.text(name).last);
    await tester.pumpAndSettle();
  }

  Future<void> reason(WidgetTester tester, String text) async {
    await tester.enterText(find.byKey(const ValueKey('confirm.reason')), text);
    await tester.pump();
  }

  bool canConfirm(WidgetTester tester) =>
      tester
          .widget<FilledButton>(find.byKey(const ValueKey('confirm.submit')))
          .onPressed !=
      null;

  String textOf(WidgetTester tester, String key) => [
    for (final t in tester.widgetList<Text>(
      find.descendant(
        of: find.byKey(ValueKey(key)),
        matching: find.byType(Text),
        matchRoot: true,
      ),
    ))
      t.data ?? '',
  ].join(' | ');

  testWidgets('a reason is required; to organization clears the holder after '
      'the step-up', (tester) async {
    await page(tester, 'support');

    await tap(tester, 'agentPage.switch');
    expect(find.text('Switch Support'), findsOneWidget);
    // The other mode than the current is preselected; organization needs no holder.
    expect(
      tester
          .widget<ChoiceChip>(
            find.byKey(const ValueKey('switch.mode.organization')),
          )
          .selected,
      isTrue,
    );
    expect(find.textContaining('The holder is cleared.'), findsOneWidget);
    expect(find.byKey(const ValueKey('switch.holder')), findsNothing);
    expect(canConfirm(tester), isFalse);
    await reason(tester, '   ');
    expect(canConfirm(tester), isFalse);
    await reason(tester, 'Costs to the organization');
    expect(canConfirm(tester), isTrue);

    await tap(tester, 'confirm.submit');
    await stepUp(tester, 'Switch the backend of Support');

    expect(repository.calls, [
      'setMode support organization - Costs to the organization',
    ]);
    expect(repository.agentRecordsById['support']!.holderId, isNull);
    expect(textOf(tester, 'agentPage.badge'), 'Organization');
    expect(textOf(tester, 'agentPage.configured'), 'Organization');
    expect(find.textContaining('Holder:'), findsNothing);
    expect(find.text('Support now uses Organization'), findsOneWidget);
    expect(changed, 1);
  });

  testWidgets('to personal needs a holder', (tester) async {
    await page(tester, 'texter');

    await tap(tester, 'agentPage.switch');
    expect(
      tester
          .widget<ChoiceChip>(
            find.byKey(const ValueKey('switch.mode.personal')),
          )
          .selected,
      isTrue,
    );
    expect(find.textContaining('only the holder may trigger'), findsOneWidget);
    await reason(tester, 'Sam pays');
    expect(canConfirm(tester), isFalse);

    await tap(tester, 'switch.holder');
    // Only active people are offered.
    expect(find.byKey(const ValueKey('switch.holder.alex')), findsWidgets);
    await pick(tester, 'Sam');
    expect(canConfirm(tester), isTrue);

    await tap(tester, 'confirm.submit');
    await stepUp(tester, 'Switch the backend of Texter');

    expect(repository.calls, ['setMode texter personal sam Sam pays']);
    expect(textOf(tester, 'agentPage.badge'), 'Personal');
    expect(find.text('Holder: Sam'), findsOneWidget);
    expect(find.text('Texter now uses Personal'), findsOneWidget);
  });

  testWidgets('switching back to organization in the dialog drops the holder', (
    tester,
  ) async {
    await page(tester, 'texter');

    await tap(tester, 'agentPage.switch');
    await tap(tester, 'switch.holder');
    await pick(tester, 'Sam');
    await tap(tester, 'switch.mode.organization');
    expect(find.byKey(const ValueKey('switch.holder')), findsNothing);
    await reason(tester, 'Stay');
    await tap(tester, 'confirm.submit');
    await stepUp(tester, 'Switch the backend of Texter');

    expect(repository.calls, ['setMode texter organization - Stay']);
  });

  testWidgets('a 409 i1_violation is explained with the conflicts and changes '
      'nothing', (tester) async {
    repository.i1On['setMode'] = [
      i1Finding(
        'texter',
        holderId: 'sam',
        principalIds: ['sam', 'robin', 'hidden-id'],
      ),
    ];
    await page(tester, 'texter');

    await tap(tester, 'agentPage.switch');
    await tap(tester, 'switch.holder');
    await pick(tester, 'Sam');
    await reason(tester, 'Sam pays');
    await tap(tester, 'confirm.submit');
    await stepUp(tester, 'Switch the backend of Texter');

    expect(find.byKey(const ValueKey('i1.explanation')), findsOneWidget);
    final line = textOf(tester, 'i1.agent.texter');
    expect(line, contains('Texter'));
    expect(line, contains('Sam'));
    expect(line, contains('Robin'));
    expect(line, contains('1 more you cannot see'));
    expect(line, isNot(contains('hidden-id')));
    expect(find.byKey(const ValueKey('self.notice')), findsOneWidget);

    expect(
      repository.agentRecordsById['texter']!.mode,
      AgentBillingMode.organization,
    );
    expect(textOf(tester, 'agentPage.badge'), 'Organization');
    expect(changed, 0);
  });

  testWidgets('the holder change asks a reason and the step-up', (
    tester,
  ) async {
    await page(tester, 'support');

    await tap(tester, 'agentPage.holder');
    expect(find.text('Change the holder of Support'), findsWidgets);
    await tap(tester, 'holder.person');
    // The current holder is not offered.
    expect(find.byKey(const ValueKey('holder.person.alex')), findsNothing);
    await pick(tester, 'Robin');
    expect(canConfirm(tester), isFalse);
    await reason(tester, 'Robin takes over');
    expect(canConfirm(tester), isTrue);

    await tap(tester, 'confirm.submit');
    await stepUp(tester, 'Change the holder of Support');

    expect(repository.calls, ['setHolder support robin Robin takes over']);
    expect(find.text('Holder: Robin'), findsOneWidget);
    expect(find.text('Robin is now the holder of Support'), findsOneWidget);
    expect(changed, 1);
  });

  testWidgets('a holder change that breaks I1 is explained', (tester) async {
    repository.i1On['setHolder'] = [
      i1Finding('support', holderId: 'sam', principalIds: ['sam', 'robin']),
    ];
    await page(tester, 'support');

    await tap(tester, 'agentPage.holder');
    await tap(tester, 'holder.person');
    await pick(tester, 'Sam');
    await reason(tester, 'Sam takes over');
    await tap(tester, 'confirm.submit');
    await stepUp(tester, 'Change the holder of Support');

    expect(find.byKey(const ValueKey('i1.explanation')), findsOneWidget);
    expect(textOf(tester, 'i1.agent.support'), contains('Robin'));
    expect(find.text('Holder: Alex'), findsOneWidget);
  });

  testWidgets('a refused switch hides the controls', (tester) async {
    repository.refuse['setMode'] = const ApiProblem(
      status: 403,
      code: 'forbidden',
    );
    await page(tester, 'support');

    await tap(tester, 'agentPage.switch');
    await reason(tester, 'Try');
    await tap(tester, 'confirm.submit');

    expect(find.byKey(const ValueKey('agentPage.switch')), findsNothing);
    expect(find.byKey(const ValueKey('self.notice')), findsOneWidget);
  });

  testWidgets('cancel changes nothing', (tester) async {
    await page(tester, 'support');
    await tap(tester, 'agentPage.switch');
    await tap(tester, 'confirm.cancel');
    expect(repository.calls, isEmpty);
    expect(find.byKey(const ValueKey('self.notice')), findsNothing);
  });
}
