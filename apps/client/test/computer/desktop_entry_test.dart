// SPDX-License-Identifier: Apache-2.0

// The desktop entry of a host on the phone (brief CU-07 requirements 14, 16 and 17; 27 4 and
// 11 item 3): the reported enablement, the operating system account and the permissions, the
// live session with Stop, "Switch enablement off" with its refusal and the note that switching
// on is only possible at the machine; no control switches the enablement on; a host without
// the reported object (any person who is no owner) shows nothing.

import 'dart:async';

import 'package:api_client/api_client.dart'
    show HostComputer, HostComputerPermissionsCaptureEnum;
import 'package:client/computer/computer_status.dart';
import 'package:client/computer/desktop_entry.dart';
import 'package:client/computer/providers.dart';
import 'package:client/notifications_settings/providers.dart'
    show isOwnerProvider;
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';
import 'package:sync_engine/sync_engine.dart' show HostEvent;

import 'support.dart';

ComputerDesktopView view({HostComputer? computer, int? disabledGeneration}) =>
    ComputerDesktopView.of(
      hostId: deskHost,
      name: 'Desk',
      computer: computer ?? hostComputer(),
      disabledGeneration: disabledGeneration,
    )!;

void main() {
  late FakeComputerApi api;
  late StreamController<HostEvent> events;
  setUp(() {
    api = FakeComputerApi();
    events = StreamController<HostEvent>.broadcast(sync: true);
  });
  tearDown(() => events.close());

  Future<int> pump(
    WidgetTester tester,
    ComputerDesktopView desktop, {
    Locale locale = const Locale('en'),
  }) async {
    var changed = 0;
    final c = ProviderContainer(
      overrides: [
        computerPortsProvider.overrideWithValue(
          ComputerPorts(api: api, events: events.stream),
        ),
        isOwnerProvider.overrideWith((ref) async => true),
      ],
    );
    addTearDown(c.dispose);
    const ds = DsContext(
      theme: DsThemeMode.light,
      density: DsDensityMode.touch,
      motion: DsMotionMode.reduced,
      accent: dsDefaultAccent,
    );
    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: c,
        child: MaterialApp(
          theme: buildDsTheme(ds),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          locale: locale,
          builder: (context, child) =>
              DsThemeScope(dsContext: ds, child: child!),
          home: Scaffold(
            body: SingleChildScrollView(
              child: ComputerDesktopEntry(
                desktop: desktop,
                onChanged: () => changed++,
              ),
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    return changed;
  }

  test('a host without the reported object is no desktop entry', () {
    expect(
      ComputerDesktopView.of(
        hostId: deskHost,
        name: 'Desk',
        computer: null,
        disabledGeneration: null,
      ),
      isNull,
    );
  });

  testWidgets('an enabled desktop shows its state and switches off', (
    tester,
  ) async {
    await pump(tester, view());
    expect(find.text('Computer control'), findsOneWidget);
    expect(find.text('Allowed on this desktop'), findsOneWidget);
    expect(find.text('Account: alex'), findsOneWidget);
    expect(
      find.text('Screen recording: granted, input: granted'),
      findsOneWidget,
    );
    expect(find.textContaining('only possible at the machine'), findsOneWidget);
    // No switch turns the enablement on (27 4 item 1).
    expect(find.byType(Switch), findsNothing);
    expect(find.byType(Checkbox), findsNothing);
    expect(find.byType(Image), findsNothing);
    await tester.tap(find.text('Switch enablement off'));
    await tester.pumpAndSettle();
    expect(api.calls, ['list', 'disable $deskHost']);
    expect(
      find.text('Computer control of Desk is switched off.'),
      findsOneWidget,
    );
  });

  testWidgets('a refused switch-off is shown', (tester) async {
    api.failure = problem(403, 'forbidden', reason: 'owner_only');
    await pump(tester, view());
    await tester.tap(find.text('Switch enablement off'));
    await tester.pumpAndSettle();
    expect(
      find.textContaining('Could not switch the enablement off'),
      findsOneWidget,
    );
  });

  testWidgets('a desktop without enablement offers no switch-off', (
    tester,
  ) async {
    await pump(tester, view(computer: hostComputer(enabled: false)));
    expect(find.text('Not allowed'), findsOneWidget);
    expect(find.text('Switch enablement off'), findsNothing);
    expect(find.textContaining('only possible at the machine'), findsOneWidget);
  });

  testWidgets('switched off from a client stays off until the machine', (
    tester,
  ) async {
    await pump(tester, view(disabledGeneration: 3));
    expect(find.text('Switched off from a client'), findsOneWidget);
    expect(find.text('Switch enablement off'), findsNothing);
  });

  testWidgets('helper and permission states', (tester) async {
    await pump(tester, view(computer: hostComputer(helper: 'absent')));
    expect(find.text('No computer helper connected'), findsOneWidget);
    await pump(
      tester,
      view(
        computer: hostComputer(
          locked: true,
          capture: HostComputerPermissionsCaptureEnum.denied,
        ),
      ),
    );
    expect(
      find.text('Screen recording: denied, input: granted'),
      findsOneWidget,
    );
    expect(find.text('The screen is locked'), findsOneWidget);
  });

  testWidgets('the live session on the desktop stops from here', (
    tester,
  ) async {
    api.sessions = [computerSession(1)];
    final handle = tester.ensureSemantics();
    await pump(tester, view());
    expect(find.text('An agent controls this desktop'), findsOneWidget);
    expect(
      find.bySemanticsLabel('Stop the computer control of Desk'),
      findsOneWidget,
    );
    expect(
      find.bySemanticsLabel('Switch the computer control of Desk off'),
      findsOneWidget,
    );
    await tester.tap(find.text('Stop'));
    await tester.pumpAndSettle();
    expect(api.calls, ['list', 'stop ${computerSessionId(1)}']);
    expect(find.text('An agent controls this desktop'), findsNothing);
    handle.dispose();
  });

  testWidgets('German texts', (tester) async {
    await pump(tester, view(), locale: const Locale('de'));
    expect(find.text('Computer-Steuerung'), findsOneWidget);
    expect(find.text('Erlaubnis ausschalten'), findsOneWidget);
    expect(find.textContaining('Computer-Steuerung erlauben'), findsOneWidget);
  });

  test('the status of a stopped session is closed', () {
    expect(
      ComputerStatus.fromSession(computerSession(1)).phase,
      ComputerPhase.active,
    );
  });
}
