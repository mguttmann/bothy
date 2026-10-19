// SPDX-License-Identifier: Apache-2.0

// Accessibility of screen A8 and the pairing wizard (01 8; docs/quality/accessibility-checklist.md
// section "Screen A8: hosts"): every control has a label, text meets the contrast guideline,
// the page title is a heading, and the answer of a wrong fingerprint is announced.

import 'package:admin_ui/admin_ui.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support.dart';

void main() {
  late GlobalKey<NavigatorState> navigator;
  late FakeAdminRepository repository;

  setUp(() {
    navigator = GlobalKey<NavigatorState>();
    repository = hostRepository(navigator)..stepUpFresh = true;
    repository.hostsById['build'] = fakeHost(
      'build',
      name: 'Build host',
      assignedUserId: 'robin',
      reported: const [HostReportedName('term-sam')],
      doctor: const [HostDoctorEntry(check: 'hd1_user', ok: true)],
    );
  });

  testWidgets('the host page: labelled controls, contrast, a heading', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    await pumpInApp(
      tester,
      HostDetail(repository: repository, hostId: 'build', clock: testClock),
      navigatorKey: navigator,
    );
    await expectLater(tester, meetsGuideline(labeledTapTargetGuideline));
    await expectLater(tester, meetsGuideline(textContrastGuideline));
    expect(
      tester.getSemantics(find.text('Build host').first),
      matchesSemantics(label: 'Build host', isHeader: true),
    );
    semantics.dispose();
  });

  testWidgets('the pairing wizard: labelled fields, an announced mismatch', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    repository.hostsById['build'] = changedHost(
      repository.hostsById['build']!,
      state: HostLifecycle.confirming,
    );
    await pumpInApp(
      tester,
      PairingWizard(
        repository: repository,
        host: repository.hostsById['build'],
        clock: testClock,
      ),
      navigatorKey: navigator,
    );
    await expectLater(tester, meetsGuideline(labeledTapTargetGuideline));
    await expectLater(tester, meetsGuideline(textContrastGuideline));
    await typeKey(tester, 'fingerprint.prefix', 'ZZZZZZZZ');
    await tapKey(tester, 'llmChoice.organization');
    await tapKey(tester, 'serverHost.no');
    await typeKey(tester, 'pairing.reason', 'Builds');
    await tapKey(tester, 'pairing.confirm');
    final banner = tester.getSemantics(
      find.byKey(const ValueKey('fingerprint.mismatch')),
    );
    expect(banner, containsSemantics(isLiveRegion: true));
    semantics.dispose();
  });
}
