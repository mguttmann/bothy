// SPDX-License-Identifier: Apache-2.0

// The hosts list, the host page and its state actions (brief M4-37 Scope, fixed requirements
// 11, 12, 17; 01 5.2 A8; 24 3): the shell lists "Hosts" for owners only; the page shows the
// fields of A8; disable, enable, re-pair and retire ask for a reason and go through step-up;
// re-pairing shows a new code; a refused list shows the server's reason.

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart';
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
      assignedUserId: 'robin',
      keyGeneration: 2,
    );
    repository.sessionCounts['build'] = 2;
  });

  Future<void> shell(WidgetTester tester, AdminLocation at) => pumpInApp(
    tester,
    SizedBox(
      height: 900,
      child: AdminShell(
        key: ValueKey('shell.${repository.callerOwner}'),
        repository: repository,
        location: at,
        clock: testClock,
        onNavigate: (_) {},
      ),
    ),
    navigatorKey: navigator,
  );

  testWidgets('the shell lists Hosts for an owner only', (tester) async {
    await shell(tester, AdminLocation.start);
    expect(find.byKey(const ValueKey('admin.nav.hosts')), findsOneWidget);

    repository.callerOwner = false;
    await shell(tester, AdminLocation.start);
    expect(find.byKey(const ValueKey('admin.nav.hosts')), findsNothing);
  });

  testWidgets('the list shows the hosts and opens one', (tester) async {
    final opened = <String>[];
    await pumpInApp(
      tester,
      HostsList(repository: repository, onOpen: opened.add, clock: testClock),
      navigatorKey: navigator,
    );
    final row = textOf(tester, 'hostRow.build');
    expect(row, contains('Build host'));
    expect(row, contains('active'));
    expect(row, contains('Robin'));
    await tapKey(tester, 'hostRow.build');
    expect(opened, ['build']);
    expect(find.byKey(const ValueKey('hosts.add')), findsOneWidget);
  });

  testWidgets('a refused list shows the reason and nothing else', (
    tester,
  ) async {
    repository.refuse['hosts'] = const ApiProblem(
      status: 403,
      code: 'forbidden',
      reason: 'owner_only',
    );
    await pumpInApp(
      tester,
      HostsList(repository: repository, onOpen: (_) {}, clock: testClock),
      navigatorKey: navigator,
    );
    expect(find.byKey(const ValueKey('admin.denied')), findsOneWidget);
    expect(find.byKey(const ValueKey('hosts.add')), findsNothing);
  });

  testWidgets('the page shows the fields of A8', (tester) async {
    await pumpInApp(
      tester,
      HostDetail(repository: repository, hostId: 'build', clock: testClock),
      navigatorKey: navigator,
    );
    expect(textOf(tester, 'host.state'), 'active');
    expect(textOf(tester, 'host.assignee'), contains('Robin'));
    expect(textOf(tester, 'host.platform'), contains('1.2.0'));
    expect(textOf(tester, 'host.connection'), contains('connected'));
    expect(textOf(tester, 'host.fingerprint'), contains('7KQ2 9XTA M4PB 1C8D'));
    expect(textOf(tester, 'host.fingerprint'), contains('Key generation 2'));
    expect(textOf(tester, 'serverHost.declared'), contains('No'));
    expect(textOf(tester, 'host.profiles'), contains('Shell, Claude Code'));
    expect(textOf(tester, 'host.sessions'), contains('2'));
  });

  testWidgets('disable, enable, re-pair and retire ask a reason and step-up', (
    tester,
  ) async {
    await pumpInApp(
      tester,
      HostDetail(repository: repository, hostId: 'build', clock: testClock),
      navigatorKey: navigator,
    );
    Future<void> act(String key, String reason) async {
      await tapKey(tester, key);
      final submit = find.byKey(const ValueKey('confirm.submit'));
      expect(tester.widget<FilledButton>(submit).onPressed, isNull);
      await typeKey(tester, 'confirm.reason', reason);
      await tapKey(tester, 'confirm.submit');
    }

    await act('host.disable', 'Maintenance');
    await confirmStepUp(tester);
    expect(repository.calls.last, 'setHostState build disabled Maintenance');
    expect(textOf(tester, 'host.state'), 'disabled');

    await act('host.enable', 'Back');
    expect(repository.calls.last, 'setHostState build active Back');
    expect(textOf(tester, 'host.state'), 'active');

    await act('host.repair', 'New disk');
    expect(repository.calls.last, 'newPairingCode build New disk');
    expect(
      textOf(tester, 'pairing.command'),
      endsWith(repository.pairingCodes.last),
    );
    await tapKey(tester, 'pairing.close');
    expect(textOf(tester, 'host.state'), 'Pairing');
    // Pairing continues from the page; retiring is for an active or disabled host (24 3.1).
    expect(find.byKey(const ValueKey('host.pair')), findsOneWidget);
    expect(find.byKey(const ValueKey('host.retire')), findsNothing);
  });

  testWidgets('retire ends the actions of the host', (tester) async {
    await pumpInApp(
      tester,
      HostDetail(repository: repository, hostId: 'build', clock: testClock),
      navigatorKey: navigator,
    );
    await tapKey(tester, 'host.retire');
    expect(textOf(tester, 'confirm.body'), contains('for good'));
    await typeKey(tester, 'confirm.reason', 'Sold');
    await tapKey(tester, 'confirm.submit');
    await confirmStepUp(tester);
    expect(repository.calls.last, 'setHostState build retired Sold');
    expect(textOf(tester, 'host.state'), 'retired');
    for (final key in ['host.retire', 'host.disable', 'host.repair']) {
      expect(find.byKey(ValueKey(key)), findsNothing, reason: key);
    }
  });
}
