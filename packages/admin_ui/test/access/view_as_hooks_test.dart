// SPDX-License-Identifier: Apache-2.0

// The view-as hook of the console (brief M6-04 Design note 3): with the hook "Who has access?"
// and a person's detail hand the person to the host, which starts the stored view; without it
// the desktop keeps the dialog with the snapshot and the detail shows no button.

import 'package:admin_ui/admin_ui.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../admin/support.dart';

void main() {
  late GlobalKey<NavigatorState> navigator;
  late FakeAdminRepository repository;

  setUp(() {
    navigator = GlobalKey<NavigatorState>();
    repository = FakeAdminRepository(
      prompt: StepUpDialogPrompt(() => navigator.currentContext),
    )..extraAccess['campaigns'] = everySource();
  });

  Future<void> pumpAccess(WidgetTester tester, {AdminViewAs? onViewAs}) =>
      pumpInApp(
        tester,
        WhoHasAccess(
          repository: repository,
          target: campaigns,
          timeZone: 'Europe/Berlin',
          clock: testClock,
          onViewAs: onViewAs,
        ),
        navigatorKey: navigator,
      );

  Future<void> choose(WidgetTester tester, String id) async {
    await tester.tap(find.byKey(const ValueKey('access.viewAs')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(ValueKey('access.viewAs.$id')).last);
    await tester.pumpAndSettle();
  }

  testWidgets('with the hook the chosen person goes to the host', (
    tester,
  ) async {
    final viewed = <AdminPerson>[];
    await pumpAccess(tester, onViewAs: viewed.add);
    await tester.pumpAndSettle();

    await choose(tester, 'sam');

    expect(viewed.map((p) => p.displayName), ['Sam']);
    expect(find.byType(Dialog), findsNothing);
    expect(find.byKey(const ValueKey('stepUp.dialog')), findsNothing);
    expect(repository.calls, isNot(contains('viewAs sam')));
  });

  testWidgets('without the hook the dialog with the snapshot opens', (
    tester,
  ) async {
    await pumpAccess(tester);
    await tester.pumpAndSettle();

    await choose(tester, 'sam');

    expect(find.byType(ViewAsPerson), findsOneWidget);
  });

  Future<void> pumpPerson(WidgetTester tester, {AdminViewAs? onViewAs}) =>
      pumpInApp(
        tester,
        PersonDetail(
          repository: repository,
          userId: 'sam',
          timeZone: 'Europe/Berlin',
          clock: testClock,
          onViewAs: onViewAs,
        ),
        navigatorKey: navigator,
      );

  testWidgets('a person detail with the hook offers "View as this person"', (
    tester,
  ) async {
    final viewed = <AdminPerson>[];
    await pumpPerson(tester, onViewAs: viewed.add);
    await tester.pumpAndSettle();

    final button = find.byKey(const ValueKey('person.viewAs'));
    expect(
      find.descendant(of: button, matching: find.text('View as this person')),
      findsOneWidget,
    );
    await tester.tap(button);
    await tester.pumpAndSettle();

    expect(viewed.map((p) => p.id), ['sam']);
    // The host starts the view; the detail itself calls nothing.
    expect(repository.calls, isEmpty);
  });

  testWidgets('a person detail without the hook has no such button', (
    tester,
  ) async {
    await pumpPerson(tester);
    await tester.pumpAndSettle();

    expect(find.byKey(const ValueKey('person.viewAs')), findsNothing);
    expect(find.text('View as this person'), findsNothing);
  });
}
