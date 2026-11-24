// SPDX-License-Identifier: Apache-2.0

// The ways out of a failed or outdated step on the host page and in the pairing wizard: a
// `412 version_conflict` reads the host again and says so, a failed read after a new code
// keeps the code on the screen, the wizard reads the host by the code's ID when the read after
// "Create" failed, and a tap beside the wizard does not drop the one-time code.

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support.dart';

/// A repository whose next [failHostReads] reads of a host fail without an answer.
class _FlakyHosts extends FakeAdminRepository {
  _FlakyHosts({required super.prompt});

  int failHostReads = 0;

  @override
  Future<AdminHost> host(String hostId) async {
    if (failHostReads > 0) {
      failHostReads--;
      throw const SelfActionFailed(null);
    }
    return super.host(hostId);
  }
}

void main() {
  late GlobalKey<NavigatorState> navigator;
  late _FlakyHosts repository;

  setUp(() {
    navigator = GlobalKey<NavigatorState>();
    repository = _FlakyHosts(
      prompt: StepUpDialogPrompt(() => navigator.currentContext),
    );
    repository.hostsById['build'] = fakeHost(
      'build',
      name: 'Build host',
      assignedUserId: 'robin',
      keyGeneration: 2,
    );
  });

  Future<void> page(WidgetTester tester) => pumpInApp(
    tester,
    HostDetail(repository: repository, hostId: 'build', clock: testClock),
    navigatorKey: navigator,
  );

  Future<void> act(WidgetTester tester, String key, String reason) async {
    await tapKey(tester, key);
    await typeKey(tester, 'confirm.reason', reason);
    await tapKey(tester, 'confirm.submit');
  }

  testWidgets('a version conflict reads the host again and says so', (
    tester,
  ) async {
    await page(tester);
    // Another owner disabled the host after the page loaded it.
    repository.hostsById['build'] = changedHost(
      repository.hostsById['build']!,
      state: HostLifecycle.disabled,
      ownerDisabled: true,
    );
    repository.refuse['setHostState'] = const ApiProblem(
      status: 412,
      code: 'version_conflict',
    );
    await act(tester, 'host.disable', 'Maintenance');

    expect(textOf(tester, 'host.state'), 'disabled');
    expect(
      textOf(tester, 'self.notice'),
      'Someone else changed this in the meantime. The page now shows the current state; '
      'make your change again.',
    );
    // The next attempt starts from the current host.
    repository.refuse.remove('setHostState');
    await act(tester, 'host.enable', 'Back');
    await confirmStepUp(tester);
    expect(repository.calls.last, 'setHostState build active Back');
    expect(textOf(tester, 'host.state'), 'active');
  });

  testWidgets('the assignee dialog keeps an assignee the people list lacks', (
    tester,
  ) async {
    repository.peopleById.remove('robin');
    await page(tester);
    await tapKey(tester, 'host.assignee.change');
    expect(tester.takeException(), isNull);
    expect(find.byKey(const ValueKey('assignee.person')), findsOneWidget);
    expect(
      find.descendant(
        of: find.byKey(const ValueKey('assignee.person')),
        matching: find.text('robin'),
      ),
      findsOneWidget,
    );
  });

  testWidgets('re-pairing shows the code although the host read failed', (
    tester,
  ) async {
    await page(tester);
    repository.failHostReads = 1;
    await act(tester, 'host.repair', 'New disk');
    await confirmStepUp(tester);
    expect(repository.calls.last, 'newPairingCode build New disk');
    expect(
      textOf(tester, 'pairing.command'),
      endsWith(repository.pairingCodes.last),
    );
    expect(textOf(tester, 'pairing.step'), 'Step 2 of 3');
  });

  testWidgets('after a failed read the wizard reads the host by its code', (
    tester,
  ) async {
    repository.hostsById.clear();
    await pumpInApp(
      tester,
      PairingWizard(repository: repository, clock: testClock),
      navigatorKey: navigator,
    );
    await typeKey(tester, 'createHost.name', 'Lab host');
    await typeKey(tester, 'createHost.reason', 'Lab');
    repository.failHostReads = 1;
    await tapKey(tester, 'pairing.create');
    await confirmStepUp(tester);
    expect(textOf(tester, 'pairing.step'), 'Step 2 of 3');

    // "Next" reads the host by the ID of the code: it has not reported its keys yet.
    await tapKey(tester, 'pairing.next');
    expect(find.byKey(const ValueKey('pairing.notYet')), findsOneWidget);

    repository.simulatePair('lab-host', ['term-alex']);
    await tapKey(tester, 'pairing.next');
    expect(textOf(tester, 'pairing.step'), 'Step 3 of 3');
  });

  testWidgets('a tap beside the wizard keeps the code', (tester) async {
    await pumpInApp(
      tester,
      Builder(
        builder: (context) => TextButton(
          key: const ValueKey('launch'),
          onPressed: () => PairingWizard.show(
            context,
            repository: repository,
            clock: testClock,
          ),
          child: const Text('launch'),
        ),
      ),
      navigatorKey: navigator,
    );
    await tapKey(tester, 'launch');
    await tester.tapAt(const Offset(5, 5));
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('pairing.dialog')), findsOneWidget);
    await tapKey(tester, 'pairing.close');
    expect(find.byKey(const ValueKey('pairing.dialog')), findsNothing);
  });
}
