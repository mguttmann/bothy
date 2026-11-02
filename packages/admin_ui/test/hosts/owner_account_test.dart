// SPDX-License-Identifier: Apache-2.0

// The owner account of a host (ADR 0073, D-OA-11): the host settings show it with a warning, an
// assignment of it needs the explicit opt-in, and a refusal for anyone but the owner himself is
// explained.

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
    repository.hostsById['home'] = fakeHost(
      'home',
      name: 'Home server',
      reported: const [HostReportedName('owner', ownerAccount: true)],
    );
  });

  Future<void> page(WidgetTester tester) => pumpInApp(
    tester,
    HostDetail(repository: repository, hostId: 'home', clock: testClock),
    navigatorKey: navigator,
  );

  testWidgets('the unassigned owner account carries the warning', (
    tester,
  ) async {
    await page(tester);
    expect(
      textOf(tester, 'reported.owner'),
      contains('Owner account with administrator rights'),
    );
  });

  testWidgets('assigning the owner account needs the explicit opt-in', (
    tester,
  ) async {
    await page(tester);
    await tapKey(tester, 'reported.owner.assign');
    expect(
      textOf(tester, 'assign.ownerAccountWarning'),
      contains('administrator rights'),
    );
    await pick(tester, 'assign.person', 'Alex');
    await typeKey(tester, 'confirm.reason', 'Work on the server');
    final submit = find.byKey(const ValueKey('confirm.submit'));
    expect(tester.widget<FilledButton>(submit).onPressed, isNull);
    await tapKey(tester, 'assign.ownerAccount');
    expect(tester.widget<FilledButton>(submit).onPressed, isNotNull);
    await tapKey(tester, 'confirm.submit');
    await confirmStepUp(tester);

    expect(
      repository.calls.last,
      'updateHost home - false alex=owner!owner_account  Work on the server',
    );
    final row = textOf(tester, 'sessionUser.alex');
    expect(row, contains('owner'));
    expect(row, contains('administrator rights'));
  });

  testWidgets('the owner account for another person is refused and explained', (
    tester,
  ) async {
    await page(tester);
    await tapKey(tester, 'reported.owner.assign');
    await pick(tester, 'assign.person', 'Sam');
    await typeKey(tester, 'confirm.reason', 'Sam works here');
    await tapKey(tester, 'assign.ownerAccount');
    await tapKey(tester, 'confirm.submit');
    await confirmStepUp(tester);

    expect(
      textOf(tester, 'sessionUsers.problem'),
      contains('Only an owner can bind the owner account'),
    );
    expect(repository.hostsById['home']!.sessionUsers, isEmpty);
  });

  testWidgets('a separate session user has no opt-in', (tester) async {
    repository.hostsById['home'] = fakeHost(
      'home',
      name: 'Home server',
      reported: const [HostReportedName('term-sam')],
    );
    await page(tester);
    await tapKey(tester, 'reported.term-sam.assign');
    expect(find.byKey(const ValueKey('assign.ownerAccount')), findsNothing);
    expect(
      find.byKey(const ValueKey('assign.ownerAccountWarning')),
      findsNothing,
    );
  });

  test('a confirmation row with the owner account needs the opt-in', () {
    final row = SessionUserRow()
      ..userId = 'alex'
      ..osUser = 'owner'
      ..ownerAccountName = true;
    expect(row.assignment, isNull);
    row.ownerAccount = true;
    expect(row.assignment?.ownerAccount, isTrue);
    final plain = SessionUserRow()
      ..userId = 'sam'
      ..osUser = 'term-sam';
    expect(plain.assignment?.ownerAccount, isFalse);
  });
}
