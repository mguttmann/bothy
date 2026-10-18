// SPDX-License-Identifier: Apache-2.0

// Step-up of the audit log (brief M4-29 Tests): reading, checking and exporting go through the
// step-up dialog because `docs/spec/actions.yaml` gives `audit.read` and `audit.verify`
// `step_up: true`; the fake asks exactly then, and this test holds the fake to the catalogue.
// The enumeration against the fake server's reading of the catalogue runs in apps/client
// (test/admin/audit_wire_test.dart).

import 'dart:io';

import 'package:admin_ui/admin_ui.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support.dart';

/// The `step_up` value of [action] in `docs/spec/actions.yaml` (read from the repository).
String catalogStepUp(String action) {
  var dir = Directory.current;
  while (!File('${dir.path}/docs/spec/actions.yaml').existsSync()) {
    dir = dir.parent;
  }
  final text = File('${dir.path}/docs/spec/actions.yaml').readAsStringSync();
  final entry = RegExp('- id: ${RegExp.escape(action)}\\n((?:  .*\\n)+)')
      .firstMatch(text)!;
  return RegExp(r'  step_up: (\S+)').firstMatch(entry[1]!)![1]!;
}

void main() {
  test('the fake asks for step-up exactly as actions.yaml says', () {
    expect(catalogStepUp('audit.read'), 'true');
    expect(catalogStepUp('audit.verify'), 'true');
    expect(stepUpOf['auditEvents'], catalogStepUp('audit.read') == 'true');
    expect(stepUpOf['exportAudit'], catalogStepUp('audit.read') == 'true');
    expect(
      stepUpOf['verifyAuditChain'],
      catalogStepUp('audit.verify') == 'true',
    );
  });

  late GlobalKey<NavigatorState> navigator;
  late FakeAdminRepository repository;
  late MemoryTarget target;

  setUp(() {
    navigator = GlobalKey<NavigatorState>();
    repository = FakeAdminRepository(
      prompt: StepUpDialogPrompt(() => navigator.currentContext),
    );
    target = MemoryTarget();
  });

  Future<void> confirm(WidgetTester tester, String purpose) async {
    await tester.pumpAndSettle();
    expect(find.byType(StepUpDialog), findsOneWidget);
    expect(find.text(purpose), findsOneWidget);
    await tester.tap(find.byKey(const ValueKey('stepUp.continue')));
    await tester.pumpAndSettle();
  }

  testWidgets('reading, checking and exporting each ask for step-up', (
    tester,
  ) async {
    await pumpAudit(
      tester,
      repository,
      saveTarget: target.call,
      navigatorKey: navigator,
    );
    await confirm(tester, 'Read the audit log');
    expect(find.byKey(const ValueKey('audit.entry.3')), findsOneWidget);
    expect(repository.stepUps, ['Read the audit log']);

    repository.stepUpFresh = false;
    await tester.tap(find.byKey(const ValueKey('audit.check')));
    await confirm(tester, 'Check the hash chain of the audit log');
    expect(find.byKey(const ValueKey('audit.chain.ok')), findsOneWidget);

    repository.stepUpFresh = false;
    await tester.enterText(
      find.byKey(const ValueKey('audit.filter.from')),
      '2026-10-16',
    );
    await tester.enterText(
      find.byKey(const ValueKey('audit.filter.to')),
      '2026-10-17',
    );
    await tester.testTextInput.receiveAction(TextInputAction.done);
    await confirm(tester, 'Read the audit log');
    repository.stepUpFresh = false;
    await tester.tap(find.byKey(const ValueKey('audit.export')));
    await confirm(tester, 'Export the audit log');
    await tester.runAsync(() => Future<void>.delayed(Duration.zero));
    await tester.pumpAndSettle();
    expect(target.bytes, repository.exportedBytes);

    expect(repository.stepUps, [
      'Read the audit log',
      'Check the hash chain of the audit log',
      'Read the audit log',
      'Export the audit log',
    ]);
  });

  testWidgets('a cancelled step-up reads nothing and offers to try again', (
    tester,
  ) async {
    await pumpAudit(tester, repository, navigatorKey: navigator);
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('stepUp.cancel')));
    await tester.pumpAndSettle();

    expect(find.byKey(const ValueKey('audit.stepUpNeeded')), findsOneWidget);
    expect(find.byKey(const ValueKey('audit.entry.3')), findsNothing);
    await tester.tap(find.text('Try again'));
    await confirm(tester, 'Read the audit log');
    expect(find.byKey(const ValueKey('audit.entry.3')), findsOneWidget);
  });

  testWidgets('a step-up cancelled on a new filter keeps that filter visible', (
    tester,
  ) async {
    await pumpAudit(tester, repository, navigatorKey: navigator);
    await confirm(tester, 'Read the audit log');
    repository.stepUpFresh = false;
    await tester.enterText(
      find.byKey(const ValueKey('audit.filter.kind')),
      'run',
    );
    await tester.testTextInput.receiveAction(TextInputAction.done);
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('stepUp.cancel')));
    await tester.pumpAndSettle();

    expect(find.byKey(const ValueKey('audit.stepUpNeeded')), findsOneWidget);
    expect(find.widgetWithText(TextField, 'run'), findsOneWidget);
    await tester.tap(find.text('Try again'));
    await confirm(tester, 'Read the audit log');
    expect(repository.auditRequests.last, contains('kind=run '));
    expect(find.byKey(const ValueKey('audit.entry.2')), findsOneWidget);
    expect(find.byKey(const ValueKey('audit.entry.3')), findsNothing);
  });
}
