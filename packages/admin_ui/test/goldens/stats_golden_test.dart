// SPDX-License-Identifier: Apache-2.0

// Goldens of A7 "Statistics" (brief M6-06 Tests): the owner with every section and the
// telemetry link, and a space manager whose "All spaces" and unmanaged space the server refused;
// light and dark, text scaling 1 and 2, German and English. One file per caller keeps each file
// below repo.max_file_kb.

import 'package:admin_ui/admin_ui.dart';
import 'package:alchemist/alchemist.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart' show WidgetTester;

import '../admin/support.dart';

Widget _frame(FakeAdminRepository r) => Material(
  type: MaterialType.transparency,
  child: SizedBox(
    width: 880,
    child: Padding(
      padding: const EdgeInsets.all(16),
      child: StatsScreen(
        source: r,
        timeZone: 'Europe/Berlin',
        clock: testClock,
        openExternal: (_) {},
      ),
    ),
  ),
);

final Map<String, Widget Function()> _callers = {
  'owner': () => _frame(_repository()..statsTelemetry = fakeTelemetryUrl),
  'space_manager': () => _frame(
    _repository()
      ..statsOwner = false
      ..statsManaged = {fakeSupport}
      ..statsTelemetry = null,
  ),
};

FakeAdminRepository _repository() =>
    FakeAdminRepository(prompt: StepUpDialogPrompt(() => null));

/// Lets the queries of the fake answer, then waits until every icon asset has loaded, bounded
/// only by the patience of the repository's test kits (see hints_golden_test.dart).
Future<void> _settle(WidgetTester tester) async {
  await tester.pumpAndSettle();
  await dsPumpIcons(tester, timeout: const Duration(minutes: 3));
}

void main() {
  for (final MapEntry(key: name, value: view) in _callers.entries) {
    goldenTest(
      'A7 for the $name in light and dark, text scale 1 and 2, de and en',
      fileName: 'stats_$name',
      pumpBeforeTest: _settle,
      builder: () => dsGoldenMatrix(
        name: name,
        builder: view,
        textScales: const [1.0, 2.0],
        locales: const [Locale('de'), Locale('en')],
      ),
    );
  }
}
