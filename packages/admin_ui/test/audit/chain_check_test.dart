// SPDX-License-Identifier: Apache-2.0

// "Check chain" (brief M4-29 Tests, Design note 2): an intact chain shows "unchanged" with the
// checked number; a broken one shows the first broken entry, its reason and the warnings in the
// danger style. Without a filter the check has no range (the server starts at the last
// checkpoint); with a filter it checks the seq span of the listed entries.

import 'package:admin_ui/admin_ui.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support.dart';

void main() {
  late FakeAdminRepository repository;

  setUp(() {
    repository = FakeAdminRepository(prompt: RecordingPrompt())
      ..stepUpFresh = true;
  });

  Future<void> check(WidgetTester tester) async {
    await tester.tap(find.byKey(const ValueKey('audit.check')));
    await tester.pumpAndSettle();
  }

  testWidgets('an intact chain shows unchanged with the checked number', (
    tester,
  ) async {
    await pumpAudit(tester, repository);
    await check(tester);

    expect(repository.verifyRequests, ['']);
    final panel = find.byKey(const ValueKey('audit.chain.ok'));
    expect(panel, findsOneWidget);
    expect(find.text('Chain checked up to no. 3: unchanged.'), findsOneWidget);
    expect(
      find.text('Checked no. 1 to 3; checkpoints compared: 2.'),
      findsOneWidget,
    );
    expect(
      find.byKey(
        const ValueKey('audit.chain.warning.checkpoint_sink_not_configured'),
      ),
      findsOneWidget,
    );
    final banner = tester.widget<DsBanner>(
      find.descendant(of: panel, matching: find.byType(DsBanner)),
    );
    expect(banner.tone, DsTone.info);
  });

  testWidgets('a broken chain shows the first broken entry and the warnings', (
    tester,
  ) async {
    repository.brokenAt = 2;
    await pumpAudit(tester, repository);
    await check(tester);

    final panel = find.byKey(const ValueKey('audit.chain.broken'));
    expect(panel, findsOneWidget);
    expect(
      find.text('Chain broken at no. 2: the entry does not match its hash.'),
      findsOneWidget,
    );
    expect(
      find.textContaining('no checkpoint target is configured'),
      findsOneWidget,
    );
    final banner = tester.widget<DsBanner>(
      find.descendant(of: panel, matching: find.byType(DsBanner)),
    );
    expect(banner.tone, DsTone.danger);
  });

  testWidgets('with a filter the check covers the listed entries', (
    tester,
  ) async {
    await pumpAudit(tester, repository);
    await tester.enterText(
      find.byKey(const ValueKey('audit.filter.action')),
      'grant.create',
    );
    await tester.testTextInput.receiveAction(TextInputAction.done);
    await tester.pumpAndSettle();
    await check(tester);
    expect(repository.verifyRequests, ['1..1']);
  });

  testWidgets('unknown reasons and warnings show their code', (tester) async {
    await pumpInApp(
      tester,
      const AuditChainPanel(
        result: AuditChainCheck(
          ok: false,
          checkedFromSeq: 1,
          checkedToSeq: 9,
          firstBrokenSeq: 9,
          failureReason: 'new_reason',
          checkpointsCompared: 0,
          warnings: ['new_warning'],
        ),
      ),
    );
    expect(
      find.text('Chain broken at no. 9: reason new_reason.'),
      findsOneWidget,
    );
    expect(find.text('Warning: new_warning'), findsOneWidget);
  });
}
