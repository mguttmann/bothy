// SPDX-License-Identifier: Apache-2.0

// The own devices and sessions (brief M4-25 Tests): this device marked and no hardware label;
// revoke asks once and then for step-up; ending a session asks once and never for step-up;
// confirm with the right and with a wrong code; a pairing 404 and the step-up asked again;
// actions hidden where the server's answer does not allow them.

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support.dart';

void main() {
  late GlobalKey<NavigatorState> navigator;
  late FakeSelfRepository repository;
  late int thisDeviceRevoked;
  late int currentSessionEnded;

  setUp(() {
    navigator = GlobalKey<NavigatorState>();
    repository = FakeSelfRepository(
      prompt: StepUpDialogPrompt(() => navigator.currentContext),
    );
    thisDeviceRevoked = 0;
    currentSessionEnded = 0;
  });

  Future<void> devices(WidgetTester tester) => pumpInApp(
    tester,
    MyDevicesView(
      repository: repository,
      timeZone: 'Europe/Berlin',
      clock: testClock,
      onThisDeviceRevoked: () => thisDeviceRevoked++,
    ),
    navigatorKey: navigator,
  );

  Future<void> sessions(WidgetTester tester) => pumpInApp(
    tester,
    MySessionsView(
      repository: repository,
      timeZone: 'Europe/Berlin',
      clock: testClock,
      onCurrentSessionEnded: () => currentSessionEnded++,
    ),
    navigatorKey: navigator,
  );

  Finder inDevice(String id, Finder finder) =>
      find.descendant(of: find.byKey(ValueKey('device.$id')), matching: finder);

  Future<void> tap(WidgetTester tester, String key) async {
    await tester.tap(find.byKey(ValueKey(key)));
    await tester.pumpAndSettle();
  }

  testWidgets('the list marks this device and shows no hardware label', (
    tester,
  ) async {
    await devices(tester);

    expect(inDevice('laptop', find.text('This device')), findsOneWidget);
    expect(inDevice('phone', find.text('This device')), findsNothing);
    expect(inDevice('phone', find.text('iPhone or iPad')), findsOneWidget);
    expect(inDevice('phone', find.text('Version 1.4.0')), findsOneWidget);
    expect(
      inDevice('phone', find.textContaining('Last active Yesterday 8:30')),
      findsOneWidget,
    );
    expect(
      inDevice('tablet', find.text('Waiting for confirmation')),
      findsOneWidget,
    );
    expect(
      find.textContaining(RegExp('hardware', caseSensitive: false)),
      findsNothing,
    );
  });

  testWidgets('revoke asks once and then for step-up', (tester) async {
    await devices(tester);

    await tap(tester, 'device.phone.revoke');
    expect(find.byKey(const ValueKey('revoke.dialog')), findsOneWidget);
    expect(find.text('Revoke Alex phone?'), findsOneWidget);
    expect(repository.calls, isEmpty);
    await tester.enterText(
      find.byKey(const ValueKey('revoke.reason')),
      'Lost on the train',
    );
    await tap(tester, 'revoke.confirm');

    expect(find.byKey(const ValueKey('revoke.dialog')), findsNothing);
    expect(find.byKey(const ValueKey('stepUp.dialog')), findsOneWidget);
    expect(find.text('Revoke the device Alex phone'), findsOneWidget);
    await tap(tester, 'stepUp.continue');

    expect(repository.calls, ['revokeDevice phone Lost on the train']);
    expect(repository.stepUps, ['Revoke the device Alex phone']);
    expect(find.text('Alex phone was revoked.'), findsOneWidget);
    expect(find.byKey(const ValueKey('device.phone.revoke')), findsNothing);
    expect(thisDeviceRevoked, 0);
  });

  testWidgets('revoke without a typed reason sends the default reason', (
    tester,
  ) async {
    await devices(tester);
    await tap(tester, 'device.phone.revoke');
    await tap(tester, 'revoke.confirm');
    await tap(tester, 'stepUp.continue');

    expect(repository.calls, ['revokeDevice phone Revoked in the settings']);
  });

  testWidgets('a cancelled step-up changes nothing and says nothing', (
    tester,
  ) async {
    await devices(tester);
    await tap(tester, 'device.phone.revoke');
    await tap(tester, 'revoke.confirm');
    await tap(tester, 'stepUp.cancel');

    expect(
      repository.deviceList.firstWhere((d) => d.id == 'phone').state,
      SelfDeviceState.active,
    );
    expect(find.byKey(const ValueKey('self.notice')), findsNothing);
    expect(find.byKey(const ValueKey('device.phone.revoke')), findsOneWidget);
  });

  testWidgets('cancelling the confirmation sends nothing', (tester) async {
    await devices(tester);
    await tap(tester, 'device.phone.revoke');
    await tap(tester, 'revoke.cancel');

    expect(repository.calls, isEmpty);
    expect(repository.stepUps, isEmpty);
  });

  testWidgets('revoking this device hands over to the app', (tester) async {
    await devices(tester);
    await tap(tester, 'device.laptop.revoke');
    expect(
      find.text(
        'You are signed out on this device and its local data is '
        'deleted.',
      ),
      findsOneWidget,
    );
    await tap(tester, 'revoke.confirm');
    await tap(tester, 'stepUp.continue');

    expect(thisDeviceRevoked, 1);
  });

  testWidgets('confirm with the right code activates the device', (
    tester,
  ) async {
    await devices(tester);
    await tap(tester, 'device.tablet.confirm');
    await tester.enterText(
      find.byKey(const ValueKey('confirmDevice.code')),
      '48a1-207',
    );
    await tester.pump();
    expect(
      tester
          .widget<TextField>(find.byKey(const ValueKey('confirmDevice.code')))
          .controller!
          .text,
      '481207',
    );
    await tap(tester, 'confirmDevice.submit');
    expect(find.text('Confirm the waiting device Alex tablet'), findsOneWidget);
    await tap(tester, 'stepUp.continue');

    expect(repository.calls, ['approveDevice tablet 481207']);
    expect(find.text('Alex tablet is now active.'), findsOneWidget);
    expect(find.byKey(const ValueKey('device.tablet.confirm')), findsNothing);
  });

  testWidgets('confirm with a wrong code shows the rejection', (tester) async {
    await devices(tester);
    await tap(tester, 'device.tablet.confirm');
    await tester.enterText(
      find.byKey(const ValueKey('confirmDevice.code')),
      '000000',
    );
    await tester.pump();
    await tap(tester, 'confirmDevice.submit');
    await tap(tester, 'stepUp.continue');

    expect(
      find.text(
        'The waiting device was rejected because the code did not match.',
      ),
      findsOneWidget,
    );
    expect(inDevice('tablet', find.text('Revoked')), findsOneWidget);
  });

  testWidgets('a pairing 404 shows its text and asks for step-up again', (
    tester,
  ) async {
    await devices(tester);
    await tap(tester, 'devices.add');
    await tester.enterText(
      find.byKey(const ValueKey('addDevice.code')),
      'zzzz-zzzz',
    );
    await tester.pump();
    await tap(tester, 'addDevice.submit');
    expect(find.text('Add a new device to your account'), findsOneWidget);
    await tap(tester, 'stepUp.continue');
    expect(repository.calls, ['approvePairing ZZZZZZZZ']);
    expect(find.text('No open pairing with this code.'), findsOneWidget);

    await tap(tester, 'devices.add');
    await tester.enterText(
      find.byKey(const ValueKey('addDevice.code')),
      'hq7p-3mxr',
    );
    await tester.pump();
    await tap(tester, 'addDevice.submit');
    expect(find.byKey(const ValueKey('stepUp.dialog')), findsOneWidget);
    await tap(tester, 'stepUp.continue');

    expect(repository.stepUps, hasLength(2));
    expect(find.text('Alex desktop was added.'), findsOneWidget);
    expect(find.byKey(const ValueKey('device.new')), findsOneWidget);
  });

  testWidgets('a revoked device offers no action', (tester) async {
    await devices(tester);

    expect(inDevice('old', find.byType(OutlinedButton)), findsNothing);
    expect(inDevice('old', find.byType(FilledButton)), findsNothing);
    expect(inDevice('laptop', find.text('Enter code')), findsNothing);
  });

  testWidgets('a 403 hides the action and shows its reason', (tester) async {
    repository.refuse['revokeDevice'] = const ApiProblem(
      status: 403,
      code: 'forbidden',
      reason: 'owner_only',
    );
    await devices(tester);
    await tap(tester, 'device.phone.revoke');
    await tap(tester, 'revoke.confirm');

    expect(find.text('Only an owner can do this'), findsOneWidget);
    expect(find.byKey(const ValueKey('device.phone.revoke')), findsNothing);
    expect(find.byKey(const ValueKey('device.tablet.revoke')), findsOneWidget);
  });

  testWidgets('no answer shows the offline text', (tester) async {
    await devices(tester);
    final offline = _OfflineRepository(repository);
    await pumpInApp(
      tester,
      MyDevicesView(
        repository: offline,
        timeZone: 'Europe/Berlin',
        clock: testClock,
      ),
      navigatorKey: navigator,
    );
    await tap(tester, 'device.phone.revoke');
    await tap(tester, 'revoke.confirm');

    expect(
      find.text('No connection. Try again when you are online.'),
      findsOneWidget,
    );
  });

  testWidgets('ending a session asks once and never for step-up', (
    tester,
  ) async {
    await sessions(tester);
    expect(find.text('Password and one-time code'), findsOneWidget);
    expect(find.textContaining('Last used Yesterday 8:30'), findsOneWidget);
    expect(find.byKey(const ValueKey('session.current')), findsOneWidget);

    await tap(tester, 'session.s-phone.end');
    expect(find.byKey(const ValueKey('revoke.dialog')), findsOneWidget);
    await tap(tester, 'revoke.confirm');

    expect(find.byKey(const ValueKey('stepUp.dialog')), findsNothing);
    expect(repository.stepUps, isEmpty);
    expect(repository.calls, ['revokeSession s-phone']);
    expect(find.text('The session was ended.'), findsOneWidget);
    expect(find.byKey(const ValueKey('session.s-phone')), findsNothing);
    expect(currentSessionEnded, 0);
  });

  testWidgets('ending the current session hands over to the app', (
    tester,
  ) async {
    await sessions(tester);
    await tap(tester, 'session.s-laptop.end');
    await tap(tester, 'revoke.confirm');

    expect(currentSessionEnded, 1);
  });

  testWidgets('a session the server refuses to end loses its button', (
    tester,
  ) async {
    repository.refuse['revokeSession'] = const ApiProblem(
      status: 403,
      code: 'forbidden',
    );
    await sessions(tester);
    await tap(tester, 'session.s-phone.end');
    await tap(tester, 'revoke.confirm');

    expect(find.text('You are not allowed to do this'), findsOneWidget);
    expect(find.byKey(const ValueKey('session.s-phone.end')), findsNothing);
  });

  testWidgets('a list that cannot load offers to try again', (tester) async {
    final failing = _FailingRepository(repository);
    await pumpInApp(
      tester,
      MyDevicesView(repository: failing, timeZone: 'Europe/Berlin'),
    );
    expect(find.byKey(const ValueKey('devices.loadFailed')), findsOneWidget);
    failing.fail = false;
    await tester.tap(find.text('Try again'));
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('device.laptop')), findsOneWidget);
  });
}

class _OfflineRepository implements SelfRepository {
  _OfflineRepository(this._inner);

  final SelfRepository _inner;

  @override
  Future<List<SelfDevice>> devices() => _inner.devices();

  @override
  Future<List<SelfSession>> sessions() => _inner.sessions();

  @override
  Future<SelfDevice> approveDevice(
    String deviceId,
    String code, {
    required String purpose,
  }) => throw const SelfActionFailed(null);

  @override
  Future<SelfDevice> approvePairing(String code, {required String purpose}) =>
      throw const SelfActionFailed(null);

  @override
  Future<SelfDevice> revokeDevice(
    String deviceId, {
    required String reason,
    required String purpose,
  }) => throw const SelfActionFailed(null);

  @override
  Future<void> revokeSession(String sessionId) =>
      throw const SelfActionFailed(null);
}

class _FailingRepository extends _OfflineRepository {
  _FailingRepository(super.inner);

  bool fail = true;

  @override
  Future<List<SelfDevice>> devices() =>
      fail ? throw const SelfActionFailed(null) : super.devices();
}
