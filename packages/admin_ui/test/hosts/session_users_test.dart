// SPDX-License-Identifier: Apache-2.0

// Session users of a host (brief M4-37 Tests, Design note 3; 24 2.2, T12, TM-T28): only names
// the host reported and that are not given to someone are offered, never as free text; a
// taken name shows `session_user_taken` with an explanation, a name the host no longer reports
// shows `session_user_not_on_host`; ending an assignment says that the name stays taken, and it
// does.

import 'package:admin_ui/admin_ui.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support.dart';

void main() {
  late GlobalKey<NavigatorState> navigator;
  late FakeAdminRepository repository;

  setUp(() {
    navigator = GlobalKey<NavigatorState>();
    repository = hostRepository(navigator);
    repository.hostsById['build'] = fakeHost(
      'build',
      name: 'Build host',
      reported: const [
        HostReportedName('term-alex', workdirs: ['server', 'apps']),
        HostReportedName('term-sam'),
        HostReportedName('term-robin'),
      ],
      sessionUsers: [
        HostSessionUserInfo(
          userId: 'alex',
          osUser: 'term-alex',
          createdAt: DateTime.utc(2026, 10, 2),
        ),
      ],
    );
    repository.taken['build'] = {'term-alex': 'alex', 'term-robin': 'robin'};
  });

  Future<void> page(WidgetTester tester) => pumpInApp(
    tester,
    HostDetail(repository: repository, hostId: 'build', clock: testClock),
    navigatorKey: navigator,
  );

  testWidgets('the rows show assignments, work folders and unassigned names', (
    tester,
  ) async {
    await page(tester);
    final alex = textOf(tester, 'sessionUser.alex');
    expect(alex, contains('Alex'));
    expect(alex, contains('term-alex'));
    expect(alex, contains('server, apps'));
    expect(find.byKey(const ValueKey('reported.term-sam')), findsOneWidget);
    expect(find.byKey(const ValueKey('reported.term-alex')), findsNothing);
    expect(
      find.textContaining('Without a session user no terminal (SR12)'),
      findsOneWidget,
    );
  });

  testWidgets('only reported names that are not given to someone are offered', (
    tester,
  ) async {
    await page(tester);
    await tapKey(tester, 'sessionUsers.assign');
    await tapKey(tester, 'assign.name');
    expect(find.text('term-sam').hitTestable(), findsWidgets);
    expect(find.text('term-robin').hitTestable(), findsWidgets);
    expect(find.text('term-alex').hitTestable(), findsNothing);
    await tester.tap(find.text('term-sam').last);
    await tester.pumpAndSettle();
    // The name is a choice, never a free text: the only text field is the reason.
    expect(
      find.descendant(
        of: find.byKey(const ValueKey('confirm.dialog')),
        matching: find.byType(TextField),
      ),
      findsOneWidget,
    );
    await pick(tester, 'assign.person', 'Sam');
    await typeKey(tester, 'confirm.reason', 'Sam builds here');
    await tapKey(tester, 'confirm.submit');
    await confirmStepUp(tester);

    expect(
      repository.calls.last,
      'updateHost build - false sam=term-sam  Sam builds here',
    );
    expect(textOf(tester, 'sessionUser.sam'), contains('term-sam'));
  });

  testWidgets('a taken name shows session_user_taken with an explanation', (
    tester,
  ) async {
    await page(tester);
    await tapKey(tester, 'reported.term-robin.assign');
    await pick(tester, 'assign.person', 'Sam');
    await typeKey(tester, 'confirm.reason', 'Sam builds here');
    await tapKey(tester, 'confirm.submit');
    await confirmStepUp(tester);

    final text = textOf(tester, 'sessionUsers.problem');
    expect(text, contains('stays bound to that person on this host'));
    expect(repository.hostsById['build']!.sessionUsers, hasLength(1));
  });

  testWidgets('a name the host no longer reports is explained', (tester) async {
    await page(tester);
    // The host's next report drops term-sam while the page still shows it.
    repository.hostsById['build'] = changedHost(
      repository.hostsById['build']!,
      reported: const [HostReportedName('term-alex')],
    );
    await tapKey(tester, 'reported.term-sam.assign');
    await pick(tester, 'assign.person', 'Sam');
    await typeKey(tester, 'confirm.reason', 'Sam builds here');
    await tapKey(tester, 'confirm.submit');
    await confirmStepUp(tester);

    expect(
      textOf(tester, 'sessionUsers.problem'),
      contains('The host has not reported this name'),
    );
  });

  testWidgets('ending an assignment says the name stays taken, and it does', (
    tester,
  ) async {
    await page(tester);
    await tapKey(tester, 'sessionUser.alex.end');
    expect(textOf(tester, 'confirm.body'), contains('stays taken'));
    await typeKey(tester, 'confirm.reason', 'Alex moved');
    await tapKey(tester, 'confirm.submit');
    await confirmStepUp(tester);

    expect(repository.calls.last, 'updateHost build - false  alex Alex moved');
    expect(find.byKey(const ValueKey('sessionUser.alex')), findsNothing);

    // The name is reported again but stays bound to Alex: giving it to Sam is refused.
    await tapKey(tester, 'reported.term-alex.assign');
    await pick(tester, 'assign.person', 'Sam');
    await typeKey(tester, 'confirm.reason', 'Reuse');
    await tapKey(tester, 'confirm.submit');
    expect(
      textOf(tester, 'sessionUsers.problem'),
      contains('stays bound to that person on this host'),
    );
  });
}
