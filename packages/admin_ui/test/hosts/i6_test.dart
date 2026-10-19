// SPDX-License-Identifier: Apache-2.0

// I6 on screen A8 (brief M4-37 Tests, Design note 4; 24 7.3): the choice "Personal" says
// before any answer that only the holder may open a terminal, not even other owners; a
// `409 i6_violation` of the confirmation or of a change of the declaration is explained with
// the server's hint, not just rejected.

import 'package:admin_ui/admin_ui.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support.dart';

const String _explained =
    'Personal: only this person may open a terminal here, not even other owners.';

void main() {
  late GlobalKey<NavigatorState> navigator;
  late FakeAdminRepository repository;

  setUp(() {
    navigator = GlobalKey<NavigatorState>();
    repository = hostRepository(navigator);
    // Robin holds a terminal grant on the host, so a personal login of Sam violates I6.
    repository.grantList.add(
      AdminGrant(
        id: 'g-robin',
        subjectKind: AdminSubjectKind.user,
        subjectId: 'robin',
        targetKind: AdminTargetKind.host,
        targetId: 'build',
        capabilities: const [AccessCapability.terminal],
        reason: 'Builds',
        grantedAt: DateTime.utc(2026, 10, 10),
      ),
    );
  });

  testWidgets('the confirmation explains an I6 violation with the hint', (
    tester,
  ) async {
    repository.hostsById['build'] = fakeHost(
      'build',
      name: 'Build host',
      state: HostLifecycle.pairing,
      keyFingerprint: null,
    );
    repository.simulatePair('build', ['term-sam']);
    await pumpInApp(
      tester,
      PairingWizard(
        repository: repository,
        host: repository.hostsById['build'],
        clock: testClock,
        onDone: (_) {},
      ),
      navigatorKey: navigator,
    );
    await typeKey(tester, 'fingerprint.prefix', '7KQ29XTA');
    await tapKey(tester, 'llmChoice.personal');
    expect(textOf(tester, 'llm.i6Explained'), _explained);
    await pick(tester, 'llmChoice.holder', 'Sam');
    await tapKey(tester, 'serverHost.no');
    await typeKey(tester, 'pairing.reason', 'Personal host of Sam');
    await tapKey(tester, 'pairing.confirm');
    await confirmStepUp(tester);

    final text = textOf(tester, 'i6.violation');
    expect(text, contains('Only the holder of the personal LLM login'));
    expect(text, contains('not even other owners'));
    expect(
      text,
      contains('Revoke the other grants or declare organization first.'),
    );
    expect(repository.hostsById['build']!.state, HostLifecycle.confirming);
  });

  testWidgets('changing the declaration to personal explains an I6 violation', (
    tester,
  ) async {
    repository.hostsById['build'] = fakeHost('build', name: 'Build host');
    await pumpInApp(
      tester,
      HostDetail(repository: repository, hostId: 'build', clock: testClock),
      navigatorKey: navigator,
    );
    expect(textOf(tester, 'llm.current'), contains('Organization'));
    await tapKey(tester, 'llm.change');
    await tapKey(tester, 'llmChoice.personal');
    expect(textOf(tester, 'llm.i6Explained'), _explained);
    await pick(tester, 'llmChoice.holder', 'Sam');
    await typeKey(tester, 'llm.reason', 'Sam uses a subscription');
    await tapKey(tester, 'confirm.submit');
    await confirmStepUp(tester);

    final text = textOf(tester, 'i6.violation');
    expect(text, contains('not even other owners'));
    expect(
      text,
      contains('Revoke the other grants or declare organization first.'),
    );
    expect(repository.hostsById['build']!.llmLogin, HostLlmLogin.organization);
  });
}
