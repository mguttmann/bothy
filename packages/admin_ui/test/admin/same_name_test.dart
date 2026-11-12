// SPDX-License-Identifier: Apache-2.0

// Two people with the same display name (ruling AUD-AM1/4): only the handle is unique (03
// `users`), so every choice between people shows the handle next to the name, the grant wizard
// finds a person by the handle too, and the step-up of what follows from the choice names both:
// "Add owner", "Add member", the subject of a grant, "View as" and the person filter of the
// audit log.

import 'package:admin_ui/admin_ui.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../audit/support.dart' show pumpAudit;
import 'support.dart';

void main() {
  late GlobalKey<NavigatorState> navigator;
  late FakeAdminRepository repository;

  setUp(() {
    navigator = GlobalKey<NavigatorState>();
    repository =
        FakeAdminRepository(
            prompt: StepUpDialogPrompt(() => navigator.currentContext),
          )
          ..extraAccess['campaigns'] = everySource()
          // A second Sam: the same display name, another handle.
          ..peopleById['sam2'] = AdminPerson(
            id: 'sam2',
            handle: 'sam.k',
            displayName: 'Sam',
            status: AdminPersonStatus.active,
            isOwner: false,
            createdAt: DateTime.utc(2026, 10, 2),
          );
  });

  Future<void> tap(WidgetTester tester, String key) async {
    await tester.ensureVisible(find.byKey(ValueKey(key)).last);
    await tester.tap(find.byKey(ValueKey(key)).last);
    await tester.pumpAndSettle();
  }

  Future<void> stepUp(WidgetTester tester, String purpose) async {
    expect(find.byKey(const ValueKey('stepUp.dialog')), findsOneWidget);
    expect(find.text(purpose), findsOneWidget);
    await tap(tester, 'stepUp.continue');
  }

  /// The handle [handle] inside the entry with [key] of an open choice.
  Finder handleIn(String key, String handle) => find.descendant(
    of: find.byKey(ValueKey(key)),
    matching: find.text('@$handle'),
  );

  /// Both Sams of the open choice whose entries have the key prefix [prefix] show their handle.
  void expectBothHandles(String prefix) {
    expect(handleIn('$prefix.sam', 'sam'), findsWidgets);
    expect(handleIn('$prefix.sam2', 'sam.k'), findsWidgets);
    expect(handleIn('$prefix.sam2', 'sam'), findsNothing);
  }

  testWidgets('the grant wizard shows and searches the handle and names it in '
      'the step-up', (tester) async {
    await pumpInApp(
      tester,
      GrantWizard(
        repository: repository,
        timeZone: 'Europe/Berlin',
        clock: testClock,
        debounce: Duration.zero,
      ),
      navigatorKey: navigator,
    );
    expectBothHandles('subject');

    await tester.enterText(
      find.byKey(const ValueKey('wizard.search')),
      'SAM.K',
    );
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('subject.sam2')), findsOneWidget);
    expect(find.byKey(const ValueKey('subject.sam')), findsNothing);
    // The handle as people write it finds the person too.
    await tester.enterText(
      find.byKey(const ValueKey('wizard.search')),
      '@sam.k',
    );
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('subject.sam2')), findsOneWidget);

    await tap(tester, 'subject.sam2');
    await tap(tester, 'wizard.next');
    await tap(tester, 'target.channel.campaigns');
    await tap(tester, 'wizard.next');
    await tap(tester, 'level.read');
    await tester.enterText(
      find.byKey(const ValueKey('wizard.reason')),
      'Campaign work',
    );
    await tester.pumpAndSettle();
    await tap(tester, 'wizard.confirm');
    await stepUp(
      tester,
      'Create the access grant for Sam (@sam.k) on Channel Marketing/Kampagnen',
    );
    expect(repository.calls.last, startsWith('createGrant sam2 '));
  });

  testWidgets('"Add owner" shows the handle and names it in the step-up', (
    tester,
  ) async {
    await pumpInApp(
      tester,
      PeopleList(
        repository: repository,
        timeZone: 'Europe/Berlin',
        onOpen: (_) {},
      ),
      navigatorKey: navigator,
    );
    await tap(tester, 'owners.add');
    await tap(tester, 'owners.pick');
    expectBothHandles('owners.pick');

    await tap(tester, 'owners.pick.sam2');
    await tap(tester, 'confirm.submit');
    await stepUp(tester, 'Make Sam (@sam.k) an owner');
    expect(repository.calls.last, startsWith('addOwner sam2'));
  });

  testWidgets('"Add member" shows the handle and names it in the step-up', (
    tester,
  ) async {
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
    expectBothHandles('member.person');

    await tap(tester, 'member.person.sam2');
    await tap(tester, 'confirm.submit');
    await stepUp(tester, 'Add Sam (@sam.k) to Support');
    expect(repository.calls.last, startsWith('addMember support sam2'));
  });

  testWidgets('"View as" shows the handle and names it in the step-up', (
    tester,
  ) async {
    await pumpInApp(
      tester,
      WhoHasAccess(
        repository: repository,
        target: campaigns,
        timeZone: 'Europe/Berlin',
        clock: testClock,
      ),
      navigatorKey: navigator,
    );
    await tap(tester, 'access.viewAs');
    expectBothHandles('access.viewAs');

    await tap(tester, 'access.viewAs.sam2');
    await stepUp(tester, 'View as Sam (@sam.k)');
  });

  testWidgets('the person filter of the audit log shows the handle', (
    tester,
  ) async {
    repository.stepUpFresh = true;
    await pumpAudit(tester, repository);
    await tap(tester, 'audit.filter.actor');
    expectBothHandles('audit.filter.actor');

    await tap(tester, 'audit.filter.actor.sam2');
    expect(repository.auditRequests.last, contains('actor=sam2 '));
  });
}
