// SPDX-License-Identifier: Apache-2.0

// Computer control on the host page of the console and the desktop app (brief CU-07
// requirements 14, 16 and 17; 27 4 and 11 items 2 and 3): the reported enablement, the
// operating system account and the permissions; the owner's live session on the host with
// Stop; "Switch enablement off" with its refusal and the note that switching on is only
// possible at the machine; no control switches it on; a host without the reported object (what
// a person who is no owner reads) shows no section; no image.

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart' show ApiProblem;
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support.dart';

const AdminHostComputer _enabled = AdminHostComputer(
  helper: 'connected',
  enabled: true,
  generation: 3,
  osAccount: 'alex',
  capture: 'granted',
  input: 'granted',
  locked: false,
);

void main() {
  late GlobalKey<NavigatorState> navigator;
  late FakeAdminRepository repository;

  setUp(() {
    navigator = GlobalKey<NavigatorState>();
    repository = hostRepository(navigator);
    repository.hostsById['desk'] = fakeHost(
      'desk',
      name: 'Desk',
      computer: _enabled,
    );
  });

  Future<void> page(WidgetTester tester) => pumpInApp(
    tester,
    SingleChildScrollView(
      child: HostDetail(
        repository: repository,
        hostId: 'desk',
        clock: testClock,
      ),
    ),
    navigatorKey: navigator,
  );

  testWidgets('the section shows the reported state and switches off', (
    tester,
  ) async {
    await page(tester);
    expect(find.text('Computer control'), findsWidgets);
    expect(find.text('Allowed on this desktop'), findsOneWidget);
    expect(find.text('Account: alex'), findsOneWidget);
    expect(
      find.text('Screen recording: granted, input: granted'),
      findsOneWidget,
    );
    expect(find.textContaining('only possible at the machine'), findsOneWidget);
    expect(find.byType(Switch), findsNothing);
    expect(find.byType(Image), findsNothing);
    await tapKey(tester, 'host.computer.disable');
    expect(repository.computerCalls, ['live desk', 'disable desk', 'live desk']);
    expect(find.text('Switched off from a client'), findsOneWidget);
    expect(find.byKey(const ValueKey('host.computer.disable')), findsNothing);
  });

  testWidgets('a refused switch-off shows the reason', (tester) async {
    repository.refuse['disableComputer'] = const ApiProblem(
      status: 403,
      code: 'forbidden',
      reason: 'owner_only',
    );
    await page(tester);
    await tapKey(tester, 'host.computer.disable');
    expect(
      find.textContaining('Could not switch the enablement off'),
      findsOneWidget,
    );
    expect(find.text('Allowed on this desktop'), findsOneWidget);
  });

  testWidgets('the live session stops from the host page', (tester) async {
    repository.liveComputer['desk'] = const AdminComputerSession(
      id: 'cs1',
      hostId: 'desk',
      desktop: 'Desk',
      state: 'active',
      actionCount: 4,
      frontApp: 'com.example.editor',
    );
    final handle = tester.ensureSemantics();
    await page(tester);
    expect(find.text('An agent controls this desktop'), findsOneWidget);
    expect(
      find.text('Controls Desk · com.example.editor · 4 actions'),
      findsOneWidget,
    );
    expect(
      find.bySemanticsLabel('Stop the computer control of Desk'),
      findsOneWidget,
    );
    await tapKey(tester, 'host.computer.stop');
    expect(repository.computerCalls, ['live desk', 'stop cs1', 'live desk']);
    expect(find.text('An agent controls this desktop'), findsNothing);
    handle.dispose();
  });

  testWidgets('a desktop that is not enabled offers no switch-off', (
    tester,
  ) async {
    repository.hostsById['desk'] = fakeHost(
      'desk',
      name: 'Desk',
      computer: const AdminHostComputer(
        helper: 'connected',
        enabled: false,
        generation: 3,
        osAccount: 'alex',
        capture: 'granted',
        input: 'denied',
        locked: true,
      ),
    );
    await page(tester);
    expect(find.text('Not allowed'), findsOneWidget);
    expect(
      find.text('Screen recording: granted, input: denied'),
      findsOneWidget,
    );
    expect(find.text('The screen is locked'), findsOneWidget);
    expect(find.byKey(const ValueKey('host.computer.disable')), findsNothing);
  });

  testWidgets('a host without the reported object has no section', (
    tester,
  ) async {
    repository.hostsById['desk'] = fakeHost('desk', name: 'Desk');
    await page(tester);
    expect(find.byKey(const ValueKey('host.computer')), findsNothing);
    expect(repository.computerCalls, isEmpty);
  });
}
