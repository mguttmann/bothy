// SPDX-License-Identifier: Apache-2.0

// Goldens of the status page (brief M7-21 Tests): the owner's page with every component state,
// the component of an agent, an open incident, a maintenance window and the history, and the
// page while the server is unreachable; light and dark, text scale 1 and 2.

import 'package:admin_ui/admin_ui.dart';
import 'package:alchemist/alchemist.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart' show WidgetTester;

import '../admin/support.dart';

Widget _frame({required bool unreachable}) {
  final r = FakeAdminRepository(prompt: StepUpDialogPrompt(() => null));
  return Material(
    type: MaterialType.transparency,
    child: SizedBox(
      width: 720,
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: StatusScreen(
          source: LiveStatusSource(
            loader: r.status,
            changes: const Stream.empty(),
            unreachable: unreachable
                ? Stream.value(true)
                : const Stream.empty(),
          ),
          manage: r,
          timeZone: 'Europe/Berlin',
          clock: testClock,
        ),
      ),
    ),
  );
}

/// Lets the reads of the fake answer, then waits until every icon asset has loaded, bounded
/// only by the patience of the repository's test kits (see hints_golden_test.dart).
Future<void> _settle(WidgetTester tester) async {
  await tester.pumpAndSettle();
  await dsPumpIcons(tester, timeout: const Duration(minutes: 3));
}

void main() {
  for (final unreachable in [false, true]) {
    final name = unreachable ? 'unreachable' : 'owner';
    goldenTest(
      'the status page ($name) in light and dark, text scale 1 and 2',
      fileName: 'status_$name',
      pumpBeforeTest: _settle,
      builder: () => dsGoldenMatrix(
        name: name,
        builder: () => _frame(unreachable: unreachable),
        textScales: const [1.0, 2.0],
      ),
    );
  }
}
