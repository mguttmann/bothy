// SPDX-License-Identifier: Apache-2.0

// No permission logic in the administration (brief M4-26 fixed requirement 9; 15 1 principle
// 1): a 403 of a list hides the section and shows the server's reason; a refused change hides
// its control; no source of the package reads grants.

import 'dart:io' as io;

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support.dart';

const ApiProblem _ownerOnly = ApiProblem(
  status: 403,
  code: 'forbidden',
  reason: 'owner_only',
);

void main() {
  late GlobalKey<NavigatorState> navigator;
  late FakeAdminRepository repository;

  setUp(() {
    navigator = GlobalKey<NavigatorState>();
    repository = FakeAdminRepository(
      prompt: StepUpDialogPrompt(() => navigator.currentContext),
    );
  });

  Future<void> pump(WidgetTester tester, Widget child) =>
      pumpInApp(tester, child, navigatorKey: navigator);

  const reason = 'Only an owner can do this';

  testWidgets('a 403 of the people list shows the reason and nothing else', (
    tester,
  ) async {
    repository.refuse['people'] = _ownerOnly;
    await pump(tester, PeopleList(repository: repository, onOpen: (_) {}));

    expect(find.byKey(const ValueKey('admin.denied')), findsOneWidget);
    expect(find.textContaining(reason), findsOneWidget);
    expect(find.byKey(const ValueKey('people.add')), findsNothing);
    expect(find.byKey(const ValueKey('people.search')), findsNothing);
    expect(find.byKey(const ValueKey('owners')), findsNothing);
  });

  testWidgets('a 403 of the groups and of the queue shows the reason', (
    tester,
  ) async {
    repository
      ..refuse['groups'] = _ownerOnly
      ..refuse['people'] = _ownerOnly;
    await pump(
      tester,
      Column(
        children: [
          GroupsList(repository: repository, onOpen: (_) {}),
          WaitingDevicesView(repository: repository),
        ],
      ),
    );

    expect(find.byKey(const ValueKey('admin.denied')), findsNWidgets(2));
    expect(find.byKey(const ValueKey('groups.create')), findsNothing);
  });

  testWidgets('a 403 of a part of a person hides only that part', (
    tester,
  ) async {
    repository.refuse['credentials'] = _ownerOnly;
    await pump(tester, PersonDetail(repository: repository, userId: 'sam'));

    expect(find.byKey(const ValueKey('admin.denied')), findsOneWidget);
    expect(find.byKey(const ValueKey('credentials')), findsNothing);
    expect(find.byKey(const ValueKey('device.sam-laptop')), findsOneWidget);
  });

  testWidgets('a refused change hides its control with the reason', (
    tester,
  ) async {
    repository.refuse['disablePerson'] = _ownerOnly;
    await pump(tester, PersonDetail(repository: repository, userId: 'sam'));

    await tester.tap(find.byKey(const ValueKey('person.disable')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('confirm.submit')));
    await tester.pumpAndSettle();

    expect(find.textContaining(reason), findsOneWidget);
    expect(find.byKey(const ValueKey('person.disable')), findsNothing);
    expect(find.byKey(const ValueKey('person.newCode')), findsOneWidget);
  });

  test('no source of admin_ui reads grants', () {
    final grants = RegExp(r'GrantsApi|listGrants|\bGrant\b|grant\.read');
    final offenders = [
      for (final f in io.Directory(
        'lib',
      ).listSync(recursive: true).whereType<io.File>())
        if (f.path.endsWith('.dart') && grants.hasMatch(f.readAsStringSync()))
          f.path,
    ];
    expect(offenders, isEmpty);
  });
}
