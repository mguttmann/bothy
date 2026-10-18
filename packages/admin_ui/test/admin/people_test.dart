// SPDX-License-Identifier: Apache-2.0

// People of A1 (brief M4-26 Tests): list, search and filters; create with the one-time code;
// disable with the sessions sentence; enable with i1_violation explained; credentials and
// devices of a person; owners with the last-owner rejection. Every change asks once, then for
// the step-up through the dialog (01 5.2, 05 2.4).

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support.dart';

void main() {
  late GlobalKey<NavigatorState> navigator;
  late FakeAdminRepository repository;
  late List<String> opened;

  setUp(() {
    navigator = GlobalKey<NavigatorState>();
    repository = FakeAdminRepository(
      prompt: StepUpDialogPrompt(() => navigator.currentContext),
    );
    opened = [];
  });

  Future<void> list(WidgetTester tester) => pumpInApp(
    tester,
    PeopleList(
      repository: repository,
      timeZone: 'Europe/Berlin',
      onOpen: opened.add,
    ),
    navigatorKey: navigator,
  );

  Future<void> detail(WidgetTester tester, String id) => pumpInApp(
    tester,
    PersonDetail(
      repository: repository,
      userId: id,
      timeZone: 'Europe/Berlin',
      clock: testClock,
    ),
    navigatorKey: navigator,
  );

  Future<void> tap(WidgetTester tester, String key) async {
    await tester.ensureVisible(find.byKey(ValueKey(key)));
    await tester.tap(find.byKey(ValueKey(key)));
    await tester.pumpAndSettle();
  }

  Future<void> stepUp(WidgetTester tester, String purpose) async {
    expect(find.byKey(const ValueKey('stepUp.dialog')), findsOneWidget);
    expect(find.text(purpose), findsOneWidget);
    await tap(tester, 'stepUp.continue');
  }

  testWidgets('the list shows people, owners and opens a person', (
    tester,
  ) async {
    await list(tester);

    expect(find.byKey(const ValueKey('person.alex')), findsOneWidget);
    expect(find.byKey(const ValueKey('person.sam')), findsOneWidget);
    expect(
      find.descendant(
        of: find.byKey(const ValueKey('person.alex')),
        matching: find.text('Owner'),
      ),
      findsOneWidget,
    );
    expect(find.byKey(const ValueKey('owner.alex')), findsOneWidget);
    await tap(tester, 'person.sam');
    expect(opened, ['sam']);
  });

  testWidgets('search and state filter are request parameters', (tester) async {
    await list(tester);

    await tester.enterText(find.byKey(const ValueKey('people.search')), 'sa');
    await tester.testTextInput.receiveAction(TextInputAction.done);
    await tester.pumpAndSettle();
    expect(repository.listRequests.last, 'sa|');
    expect(find.byKey(const ValueKey('person.alex')), findsNothing);
    expect(find.byKey(const ValueKey('person.sam')), findsOneWidget);

    await tap(tester, 'people.status');
    await tester.tap(find.text('Disabled').last);
    await tester.pumpAndSettle();
    expect(repository.listRequests.last, 'sa|disabled');
    expect(find.text('No person matches.'), findsOneWidget);
  });

  testWidgets('the group filter filters the loaded page with a note', (
    tester,
  ) async {
    await list(tester);

    await tap(tester, 'people.group');
    await tester.tap(find.text('Support').last);
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('people.groupNote')), findsOneWidget);
    expect(find.byKey(const ValueKey('person.robin')), findsOneWidget);
    expect(find.byKey(const ValueKey('person.sam')), findsNothing);
  });

  testWidgets('create asks once, then step-up, then shows the code once', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    await list(tester);

    await tap(tester, 'people.add');
    expect(find.byKey(const ValueKey('confirm.dialog')), findsOneWidget);
    expect(
      find.textContaining('the enrollment code is shown once'),
      findsOneWidget,
    );
    await tester.enterText(
      find.byKey(const ValueKey('createPerson.name')),
      'Robin K',
    );
    await tester.enterText(
      find.byKey(const ValueKey('createPerson.handle')),
      'Robin K',
    );
    await tester.pump();
    expect(
      find.text(
        'Use lower case letters, digits, dots, hyphens and underscores.',
      ),
      findsOneWidget,
    );
    await tester.enterText(
      find.byKey(const ValueKey('createPerson.handle')),
      'robin.k',
    );
    await tester.pump();
    await tap(tester, 'confirm.submit');
    await stepUp(tester, 'Create the person Robin K');

    expect(repository.calls, ['createPerson robin.k']);
    expect(find.byKey(const ValueKey('code.dialog')), findsOneWidget);
    expect(find.text('en_4KX9  2MQA  0000'), findsOneWidget);
    expect(
      find.bySemanticsLabel(RegExp('e, n, _, 4, K, X, 9')),
      findsOneWidget,
    );
    final copied = <String>[];
    tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
      SystemChannels.platform,
      (call) async {
        if (call.method == 'Clipboard.setData') {
          copied.add((call.arguments as Map)['text'] as String);
        }
        return null;
      },
    );
    addTearDown(
      () => tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
        SystemChannels.platform,
        null,
      ),
    );
    await tap(tester, 'code.copy');
    expect(copied, ['en_4KX92MQA0000']);
    expect(find.text('Copied'), findsOneWidget);
    await tap(tester, 'code.close');
    expect(find.byKey(const ValueKey('code.dialog')), findsNothing);
    // Closing discards the code: nothing on the screen shows it again.
    expect(find.textContaining('2MQA'), findsNothing);
    expect(find.byKey(const ValueKey('person.robin.k')), findsOneWidget);
    semantics.dispose();
  });

  testWidgets('a cancelled step-up changes nothing', (tester) async {
    await list(tester);

    await tap(tester, 'people.add');
    await tester.enterText(
      find.byKey(const ValueKey('createPerson.name')),
      'Robin K',
    );
    await tester.enterText(
      find.byKey(const ValueKey('createPerson.handle')),
      'robink',
    );
    await tester.pump();
    await tap(tester, 'confirm.submit');
    await tap(tester, 'stepUp.cancel');
    expect(repository.peopleById.containsKey('robink'), isFalse);
    expect(find.byKey(const ValueKey('code.dialog')), findsNothing);
  });

  testWidgets('a repeated code request shows the one-time-secret text', (
    tester,
  ) async {
    repository.refuse['newEnrollmentCode'] = const ApiProblem(
      status: 409,
      code: 'conflict',
      reason: 'one_time_secret',
    );
    await detail(tester, 'sam');

    await tap(tester, 'person.newCode');
    expect(find.textContaining('Every older code of Sam'), findsOneWidget);
    await tap(tester, 'confirm.submit');
    expect(
      find.text(
        'The code was shown once and cannot be shown again. Issue a new code.',
      ),
      findsOneWidget,
    );
    expect(find.byKey(const ValueKey('code.dialog')), findsNothing);
  });

  testWidgets('disable says that every session ends, then asks step-up', (
    tester,
  ) async {
    await detail(tester, 'sam');

    await tap(tester, 'person.disable');
    expect(
      find.text(
        'Sam is disabled. All sessions of Sam end now and all devices are '
        'revoked.',
      ),
      findsOneWidget,
    );
    expect(repository.calls, isEmpty);
    await tester.enterText(
      find.byKey(const ValueKey('confirm.reason')),
      'Left the team',
    );
    await tap(tester, 'confirm.submit');
    await stepUp(tester, 'Disable Sam');

    expect(repository.calls, ['disablePerson sam Left the team']);
    expect(find.text('Sam is disabled. All sessions ended.'), findsOneWidget);
    expect(find.byKey(const ValueKey('person.enable')), findsOneWidget);
    expect(find.byKey(const ValueKey('session.s-laptop')), findsNothing);
  });

  testWidgets('enable explains i1_violation', (tester) async {
    repository.peopleById['sam'] = AdminPerson(
      id: 'sam',
      handle: 'sam',
      displayName: 'Sam',
      status: AdminPersonStatus.disabled,
      isOwner: false,
      createdAt: DateTime.utc(2026, 10),
    );
    repository.i1.add('sam');
    await detail(tester, 'sam');

    await tap(tester, 'person.enable');
    await tap(tester, 'confirm.submit');
    await stepUp(tester, 'Enable Sam');

    expect(
      find.textContaining('This change would let others use a personal agent'),
      findsOneWidget,
    );
    expect(
      find.textContaining('Change the mode of the affected agent'),
      findsOneWidget,
    );
  });

  testWidgets('credentials, devices and sessions of a person', (tester) async {
    await detail(tester, 'sam');

    expect(find.text('Local password set'), findsOneWidget);
    expect(find.text('One-time code app set up'), findsOneWidget);
    expect(find.byKey(const ValueKey('identity.id-sam')), findsOneWidget);
    expect(find.byKey(const ValueKey('passkey.pk-sam')), findsOneWidget);
    expect(find.byKey(const ValueKey('device.sam-laptop')), findsOneWidget);
    expect(find.byKey(const ValueKey('device.sam-tablet')), findsOneWidget);
    expect(find.byKey(const ValueKey('session.s-tablet')), findsOneWidget);

    await tap(tester, 'passkey.pk-sam.revoke');
    expect(find.text('Revoke the passkey Sam laptop?'), findsOneWidget);
    await tap(tester, 'confirm.submit');
    await stepUp(tester, 'Revoke a passkey of Sam');
    expect(find.byKey(const ValueKey('passkey.pk-sam')), findsNothing);

    // The confirmation is still fresh: unlinking asks once and no step-up again.
    await tap(tester, 'identity.id-sam.unlink');
    await tap(tester, 'confirm.submit');
    expect(find.byKey(const ValueKey('stepUp.dialog')), findsNothing);
    expect(find.byKey(const ValueKey('identity.id-sam')), findsNothing);

    // Ending a session asks once and never for a step-up (05: session.revoke).
    repository.stepUpFresh = false;
    await tap(tester, 'session.s-laptop.end');
    await tap(tester, 'confirm.submit');
    expect(find.byKey(const ValueKey('stepUp.dialog')), findsNothing);
    expect(repository.calls.last, 'endSession s-laptop');
    expect(find.byKey(const ValueKey('session.s-laptop')), findsNothing);

    await tap(tester, 'device.sam-laptop.revoke');
    await tap(tester, 'confirm.submit');
    await stepUp(tester, 'Revoke the device Sam laptop');
    expect(repository.calls.last, startsWith('revokeDevice sam-laptop'));
  });

  testWidgets('owners: add one, and the last owner cannot be revoked', (
    tester,
  ) async {
    await list(tester);

    await tap(tester, 'owner.alex.revoke');
    expect(find.text('Revoke the owner Alex?'), findsOneWidget);
    await tap(tester, 'confirm.submit');
    await stepUp(tester, 'Revoke the owner Alex');
    expect(find.text('At least one active owner must remain'), findsOneWidget);
    expect(repository.ownerIds, ['alex']);

    await tap(tester, 'owners.add');
    await tap(tester, 'owners.pick');
    await tester.tap(find.text('Sam').last);
    await tester.pumpAndSettle();
    await tap(tester, 'confirm.submit');
    expect(repository.calls.last, startsWith('addOwner sam'));
    expect(find.text('Sam is now an owner.'), findsOneWidget);
    expect(find.byKey(const ValueKey('owner.sam')), findsOneWidget);
  });

  testWidgets('a person that does not exist shows the reason', (tester) async {
    await detail(tester, 'nobody');

    expect(find.byKey(const ValueKey('admin.denied')), findsOneWidget);
    expect(find.byKey(const ValueKey('person.disable')), findsNothing);
  });
}
