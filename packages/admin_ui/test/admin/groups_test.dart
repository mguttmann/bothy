// SPDX-License-Identifier: Apache-2.0

// Groups (brief M4-26 Tests): create, rename, delete, members with expiry; every change asks
// once and then for the step-up (05: group.create, group.update, group.delete,
// group.member.add, group.member.remove).

import 'package:admin_ui/admin_ui.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support.dart';

void main() {
  late GlobalKey<NavigatorState> navigator;
  late FakeAdminRepository repository;
  late List<String> opened;
  late int back;

  setUp(() {
    navigator = GlobalKey<NavigatorState>();
    repository = FakeAdminRepository(
      prompt: StepUpDialogPrompt(() => navigator.currentContext),
    );
    opened = [];
    back = 0;
  });

  Future<void> tap(WidgetTester tester, String key) async {
    await tester.ensureVisible(find.byKey(ValueKey(key)));
    await tester.tap(find.byKey(ValueKey(key)));
    await tester.pumpAndSettle();
  }

  Future<void> stepUp(WidgetTester tester, String purpose) async {
    expect(find.byKey(const ValueKey('stepUp.dialog')), findsOneWidget);
    expect(find.text(purpose), findsOneWidget);
    await tap(tester, 'stepUp.continue');
    repository.stepUpFresh = false;
  }

  Future<void> detail(WidgetTester tester) => pumpInApp(
    tester,
    GroupDetail(
      repository: repository,
      groupId: 'support',
      timeZone: 'Europe/Berlin',
      clock: testClock,
      onBack: () => back++,
    ),
    navigatorKey: navigator,
  );

  testWidgets('create a group asks once, then step-up, and opens it', (
    tester,
  ) async {
    await pumpInApp(
      tester,
      GroupsList(repository: repository, onOpen: opened.add),
      navigatorKey: navigator,
    );
    expect(find.text('1 member'), findsOneWidget);

    await tap(tester, 'groups.create');
    expect(
      find.text('A new group without members is created.'),
      findsOneWidget,
    );
    await tester.enterText(
      find.byKey(const ValueKey('groupName.name')),
      'Research',
    );
    await tester.pump();
    await tap(tester, 'confirm.submit');
    await stepUp(tester, 'Create the group Research');

    expect(repository.calls, ['createGroup Research']);
    expect(find.byKey(const ValueKey('group.research')), findsOneWidget);
    expect(opened, ['research']);
  });

  testWidgets('rename and delete a group', (tester) async {
    await detail(tester);

    await tap(tester, 'group.rename');
    await tester.enterText(
      find.byKey(const ValueKey('groupName.name')),
      'Help desk',
    );
    await tester.pump();
    await tap(tester, 'confirm.submit');
    await stepUp(tester, 'Rename the group Support');
    expect(find.text('Help desk'), findsWidgets);

    await tap(tester, 'group.delete');
    expect(
      find.textContaining('The members keep their accounts'),
      findsOneWidget,
    );
    await tap(tester, 'confirm.submit');
    await stepUp(tester, 'Delete the group Help desk');
    expect(repository.groupsById, isEmpty);
    expect(back, 1);
  });

  testWidgets('members with an expiry: add and remove', (tester) async {
    await detail(tester);

    expect(find.byKey(const ValueKey('member.robin.expires')), findsOneWidget);

    await tap(tester, 'group.addMember');
    await tap(tester, 'member.person');
    await tester.tap(find.text('Sam').last);
    await tester.pumpAndSettle();
    await tester.enterText(
      find.byKey(const ValueKey('member.until')),
      '2026-02-30',
    );
    await tester.pump();
    expect(find.text('Enter a date as YYYY-MM-DD.'), findsOneWidget);
    await tester.enterText(
      find.byKey(const ValueKey('member.until')),
      '2026-12-31',
    );
    await tester.pump();
    await tap(tester, 'confirm.submit');
    await stepUp(tester, 'Add Sam to Support');
    expect(
      repository.calls.last,
      'addMember support sam 2026-12-31T00:00:00.000Z',
    );
    expect(find.byKey(const ValueKey('member.sam.expires')), findsOneWidget);
    expect(find.text('Sam was added to Support.'), findsOneWidget);

    await tap(tester, 'member.robin.remove');
    expect(find.text('Remove Robin from Support?'), findsOneWidget);
    await tap(tester, 'confirm.submit');
    await stepUp(tester, 'Remove Robin from Support');
    expect(find.byKey(const ValueKey('member.robin')), findsNothing);
  });

  test('parseDay reads YYYY-MM-DD only', () {
    expect(parseDay('2026-12-31'), DateTime.utc(2026, 12, 31));
    expect(parseDay('2026-02-30'), isNull);
    expect(parseDay('31.12.2026'), isNull);
  });
}
