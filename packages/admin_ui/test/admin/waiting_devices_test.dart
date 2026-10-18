// SPDX-License-Identifier: Apache-2.0

// The queue of waiting devices (brief M4-26 Tests): assembled from the people, their pending
// devices and the session of each device (origin from auth_method); approve with the code
// and step-up; reject asks once and then for step-up (08 5.2).

import 'package:admin_ui/admin_ui.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support.dart';

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

  Future<void> queue(WidgetTester tester) => pumpInApp(
    tester,
    WaitingDevicesView(
      repository: repository,
      timeZone: 'Europe/Berlin',
      clock: testClock,
      onChanged: () => changed++,
    ),
    navigatorKey: navigator,
  );

  Future<void> tap(WidgetTester tester, String key) async {
    await tester.tap(find.byKey(ValueKey(key)));
    await tester.pumpAndSettle();
  }

  Finder inRow(Finder finder) => find.descendant(
    of: find.byKey(const ValueKey('waiting.sam-tablet')),
    matching: finder,
  );

  testWidgets('the queue shows person, device, platform, time and origin', (
    tester,
  ) async {
    await queue(tester);

    expect(inRow(find.text('Sam: Pixel')), findsOneWidget);
    expect(inRow(find.text('Android')), findsOneWidget);
    expect(inRow(find.textContaining('Logged in Today 10:12')), findsOneWidget);
    expect(inRow(find.text('via Login service')), findsOneWidget);
    // Device has no hardware or attestation field (brief requirement 15).
    expect(
      find.textContaining(RegExp('hardware|attested', caseSensitive: false)),
      findsNothing,
    );
    expect(find.byKey(const ValueKey('waiting.sam-laptop')), findsNothing);
  });

  testWidgets('approve with the code, then step-up', (tester) async {
    await queue(tester);

    await tap(tester, 'waiting.sam-tablet.approve');
    expect(find.text('Approve Pixel of Sam'), findsOneWidget);
    await tester.enterText(
      find.byKey(const ValueKey('approveDevice.code')),
      '48a12 07',
    );
    await tester.pump();
    await tap(tester, 'approveDevice.submit');
    expect(find.byKey(const ValueKey('stepUp.dialog')), findsOneWidget);
    expect(find.text('Approve the device Pixel of Sam'), findsOneWidget);
    await tap(tester, 'stepUp.continue');

    expect(repository.calls, ['approveDevice sam-tablet 481207']);
    expect(find.text('Pixel of Sam is now active.'), findsOneWidget);
    expect(find.byKey(const ValueKey('waiting.empty')), findsOneWidget);
    expect(changed, 1);
  });

  testWidgets('a wrong code rejects the device with a warning', (tester) async {
    await queue(tester);

    await tap(tester, 'waiting.sam-tablet.approve');
    await tester.enterText(
      find.byKey(const ValueKey('approveDevice.code')),
      '000000',
    );
    await tester.pump();
    await tap(tester, 'approveDevice.submit');
    await tap(tester, 'stepUp.continue');
    expect(
      find.text(
        'The waiting device was rejected because the code did not match.',
      ),
      findsOneWidget,
    );
  });

  testWidgets('reject asks once, then step-up', (tester) async {
    await queue(tester);

    await tap(tester, 'waiting.sam-tablet.reject');
    expect(
      find.text('The waiting device of Sam is revoked and cannot sign in.'),
      findsOneWidget,
    );
    expect(repository.calls, isEmpty);
    await tap(tester, 'confirm.submit');
    expect(find.text('Reject the device Pixel of Sam'), findsOneWidget);
    await tap(tester, 'stepUp.continue');

    expect(repository.calls.single, startsWith('revokeDevice sam-tablet'));
    expect(find.text('Pixel was rejected.'), findsOneWidget);
  });

  testWidgets('the shell counts the waiting devices and opens the queue', (
    tester,
  ) async {
    final places = <AdminLocation>[];
    tester.view
      ..physicalSize = const Size(1400, 1000)
      ..devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    await pumpInApp(
      tester,
      SizedBox(
        height: 900,
        child: AdminShell(
          repository: repository,
          location: AdminLocation.start,
          onNavigate: places.add,
          timeZone: 'Europe/Berlin',
          clock: testClock,
        ),
      ),
      navigatorKey: navigator,
    );

    expect(find.text('Devices (1 waiting)'), findsOneWidget);
    expect(find.byKey(const ValueKey('admin.nav.grants')), findsNothing);
    expect(find.byKey(const ValueKey('admin.nav.agents')), findsNothing);
    await tap(tester, 'admin.nav.devices');
    expect(places, [const AdminLocation(AdminSection.devices)]);
  });
}
