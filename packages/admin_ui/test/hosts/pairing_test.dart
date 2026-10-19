// SPDX-License-Identifier: Apache-2.0

// The pairing wizard of A8 (brief M4-37 Tests, Design note 1; 24 3.2, TM-T19): the code with
// the command for the host and its remaining time; a wrong fingerprint shows
// `fingerprint_mismatch` with the attempts the server names; the last attempt returns the host
// to `pairing` and asks for a new code; the right prefix, typed with separators and in lower
// case, confirms the host with the mandatory answers and the session users.

import 'package:admin_ui/admin_ui.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support.dart';

void main() {
  late GlobalKey<NavigatorState> navigator;
  late FakeAdminRepository repository;
  late List<AdminHost> done;

  setUp(() {
    navigator = GlobalKey<NavigatorState>();
    repository = hostRepository(navigator);
    done = [];
  });

  Future<void> wizard(WidgetTester tester, {AdminHost? host}) => pumpInApp(
    tester,
    PairingWizard(
      repository: repository,
      host: host,
      clock: testClock,
      onDone: done.add,
    ),
    navigatorKey: navigator,
  );

  /// A host `build` in `confirming` that reported `term-alex` and `term-sam`.
  AdminHost confirming() {
    repository.hostsById['build'] = fakeHost(
      'build',
      name: 'Build host',
      state: HostLifecycle.pairing,
      keyFingerprint: null,
      keyGeneration: 0,
    );
    repository.simulatePair('build', ['term-alex', 'term-sam']);
    return repository.hostsById['build']!;
  }

  /// Answers the mandatory questions of step 3 and types [prefix].
  Future<void> answer(WidgetTester tester, String prefix) async {
    await typeKey(tester, 'fingerprint.prefix', prefix);
    await tapKey(tester, 'llmChoice.organization');
    await tapKey(tester, 'serverHost.no');
    await typeKey(tester, 'pairing.reason', 'Build machine');
  }

  testWidgets(
    'step 1 creates the host; step 2 shows the command and the time',
    (tester) async {
      await wizard(tester);
      expect(textOf(tester, 'pairing.step'), 'Step 1 of 3');
      await typeKey(tester, 'createHost.name', 'Build host');
      await pick(tester, 'createHost.assignee', 'Robin');
      await typeKey(tester, 'createHost.reason', 'Build machine');
      await tapKey(tester, 'pairing.create');
      await confirmStepUp(tester);

      expect(
        repository.calls.last,
        'createHost Build host robin Build machine',
      );
      expect(textOf(tester, 'pairing.step'), 'Step 2 of 3');
      final code = repository.pairingCodes.single;
      expect(
        textOf(tester, 'pairing.command'),
        'bothy-host pair --server https://chat.example.com --code $code',
      );
      expect(textOf(tester, 'pairing.expires'), 'Expires in 14:12');
      expect(find.byKey(const ValueKey('pairing.copy')), findsOneWidget);

      // "New code" issues another one and replaces the shown code.
      await tapKey(tester, 'pairing.newCode');
      expect(repository.pairingCodes, hasLength(2));
      expect(
        textOf(tester, 'pairing.command'),
        endsWith(repository.pairingCodes.last),
      );

      // Before the host reported, "Next" says so and stays on step 2.
      await tapKey(tester, 'pairing.next');
      expect(find.byKey(const ValueKey('pairing.notYet')), findsOneWidget);
      expect(textOf(tester, 'pairing.step'), 'Step 2 of 3');

      repository.simulatePair('build-host', ['term-alex', 'term-sam']);
      await tapKey(tester, 'pairing.next');
      expect(textOf(tester, 'pairing.step'), 'Step 3 of 3');
    },
  );

  testWidgets(
    'a wrong fingerprint shows fingerprint_mismatch with the attempts left',
    (tester) async {
      await wizard(tester, host: confirming());
      expect(textOf(tester, 'pairing.step'), 'Step 3 of 3');
      expect(find.text('Confirm with passkey / device'), findsOneWidget);
      await answer(tester, 'ZZZZ-ZZZZ');
      await tapKey(tester, 'pairing.confirm');
      await confirmStepUp(tester);

      final banner = textOf(tester, 'fingerprint.mismatch');
      expect(banner, contains('The fingerprint does not match the host'));
      expect(banner, contains('2 attempts left'));
      expect(textOf(tester, 'pairing.step'), 'Step 3 of 3');
      expect(repository.hostsById['build']!.state, HostLifecycle.confirming);
      expect(done, isEmpty);
    },
  );

  testWidgets(
    'the last attempt returns the host to pairing and asks for a new code',
    (tester) async {
      await wizard(tester, host: confirming());
      await answer(tester, 'ZZZZ-ZZZZ');
      for (var i = 0; i < 3; i++) {
        await tapKey(tester, 'pairing.confirm');
        if (i == 0) await confirmStepUp(tester);
      }

      expect(repository.hostsById['build']!.state, HostLifecycle.pairing);
      expect(repository.hostsById['build']!.keyFingerprint, isNull);
      expect(
        find.byKey(const ValueKey('fingerprint.fallback')),
        findsOneWidget,
      );
      expect(textOf(tester, 'pairing.step'), 'Step 2 of 3');
      expect(find.byKey(const ValueKey('pairing.command')), findsNothing);
      expect(find.byKey(const ValueKey('pairing.newCode')), findsOneWidget);
    },
  );

  testWidgets(
    'the right prefix confirms with the answers and the session users',
    (tester) async {
      await wizard(tester, host: confirming());
      // The mandatory questions have no default: "Confirm" waits for them.
      await typeKey(tester, 'fingerprint.prefix', '7kq2 9xta');
      await typeKey(tester, 'pairing.reason', 'Build machine');
      final confirm = find.byKey(const ValueKey('pairing.confirm'));
      expect(tester.widget<FilledButton>(confirm).onPressed, isNull);
      await tapKey(tester, 'llmChoice.organization');
      expect(tester.widget<FilledButton>(confirm).onPressed, isNull);
      await tapKey(tester, 'serverHost.no');
      expect(tester.widget<FilledButton>(confirm).onPressed, isNotNull);

      await tapKey(tester, 'sessionRows.add');
      await pick(tester, 'sessionRow.0.person', 'Sam');
      await pick(tester, 'sessionRow.0.name', 'term-sam');
      await tapKey(tester, 'pairing.confirm');
      await confirmStepUp(tester);

      expect(
        repository.calls.last,
        'confirmHost build 7kq2 9xta organization false sam=term-sam '
        'Build machine',
      );
      final host = repository.hostsById['build']!;
      expect(host.state, HostLifecycle.active);
      expect(host.sessionUsers.single.osUser, 'term-sam');
      expect(done.single.state, HostLifecycle.active);
    },
  );

  testWidgets('only the rows shown on step 3 are sent, also after Back', (
    tester,
  ) async {
    final host = confirming();
    await pumpInApp(
      tester,
      PairingWizard(
        repository: repository,
        host: host,
        code: HostPairingCodeInfo(
          hostId: 'build',
          code: 'bothy_hpc_EXAMPLE000000000000000000000000000000000000_EXAMPL',
          expiresAt: DateTime.utc(2026, 10, 17, 9, 14, 12),
        ),
        clock: testClock,
        onDone: done.add,
      ),
      navigatorKey: navigator,
    );
    await tapKey(tester, 'pairing.next');
    expect(textOf(tester, 'pairing.step'), 'Step 3 of 3');
    await tapKey(tester, 'sessionRows.add');
    await pick(tester, 'sessionRow.0.person', 'Sam');
    await pick(tester, 'sessionRow.0.name', 'term-sam');
    // Back to the code and forward again: the row is still shown and still sent.
    await tapKey(tester, 'pairing.back');
    expect(textOf(tester, 'pairing.step'), 'Step 2 of 3');
    await tapKey(tester, 'pairing.next');
    expect(textOf(tester, 'pairing.step'), 'Step 3 of 3');
    expect(find.byKey(const ValueKey('sessionRow.0')), findsOneWidget);
    await tapKey(tester, 'sessionRow.0.remove');
    expect(find.byKey(const ValueKey('sessionRow.0')), findsNothing);
    await answer(tester, '7KQ2-9XTA');
    await tapKey(tester, 'pairing.confirm');
    await confirmStepUp(tester);
    // No row on the screen: no session user is sent.
    expect(
      repository.calls.last,
      endsWith('organization false  Build machine'),
    );
    expect(repository.hostsById['build']!.sessionUsers, isEmpty);
  });

  testWidgets('after the fallback to pairing no earlier row is sent', (
    tester,
  ) async {
    await wizard(tester, host: confirming());
    await tapKey(tester, 'sessionRows.add');
    await pick(tester, 'sessionRow.0.person', 'Sam');
    await pick(tester, 'sessionRow.0.name', 'term-sam');
    await answer(tester, 'ZZZZ-ZZZZ');
    for (var i = 0; i < 3; i++) {
      await tapKey(tester, 'pairing.confirm');
      if (i == 0) await confirmStepUp(tester);
    }
    expect(textOf(tester, 'pairing.step'), 'Step 2 of 3');
    await tapKey(tester, 'pairing.newCode');
    repository.simulatePair('build', ['term-robin']);
    await tapKey(tester, 'pairing.next');
    expect(find.byKey(const ValueKey('sessionRow.0')), findsNothing);
    await answer(tester, '7KQ2-9XTA');
    await tapKey(tester, 'pairing.confirm');
    expect(
      repository.calls.last,
      endsWith('organization false  Build machine'),
    );
    expect(repository.hostsById['build']!.state, HostLifecycle.active);
  });
}
