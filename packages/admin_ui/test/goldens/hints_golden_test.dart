// SPDX-License-Identifier: Apache-2.0

// Goldens of "Permission hints" and "Expiring soon" (brief M6-07 Tests): every kind of hint and
// of expiring access with its icon, kind text, sentence and actions; light and dark, two
// densities, text scaling 1 and 2, German and English. One file per view keeps each file below
// repo.max_file_kb.

import 'package:admin_ui/admin_ui.dart';
import 'package:alchemist/alchemist.dart';
import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart' show WidgetTester;

import '../admin/support.dart';

Widget _frame(Widget child) => Material(
  type: MaterialType.transparency,
  child: SizedBox(
    width: 760,
    child: Padding(padding: const EdgeInsets.all(16), child: child),
  ),
);

FakeAdminRepository _repository() =>
    FakeAdminRepository(prompt: StepUpDialogPrompt(() => null))..seedHints();

final Map<String, Widget Function()> _views = {
  'permission_hints': () => _frame(
    PermissionHintsView(
      repository: _repository(),
      onNavigate: (_) {},
      timeZone: 'Europe/Berlin',
      clock: testClock,
    ),
  ),
  'expiring': () => _frame(
    ExpiringAccessView(
      repository: _repository(),
      onNavigate: (_) {},
      timeZone: 'Europe/Berlin',
      clock: testClock,
    ),
  ),
};

/// Waits until every icon asset has loaded, bounded only by the patience of the repository's
/// test kits (`testkit.Patience` of the Go services, 3 minutes): the helper's default of
/// 10 seconds of wall-clock time ran out once on a loaded machine while the assets were
/// still loading, and the test framework's own timeout stays the outer bound.
Future<void> _pumpIcons(WidgetTester tester) =>
    dsPumpIcons(tester, timeout: const Duration(minutes: 3));

void main() {
  for (final MapEntry(key: name, value: view) in _views.entries) {
    goldenTest(
      '$name in light and dark, two densities, text scale 1 and 2, de and en',
      fileName: 'hints_$name',
      pumpBeforeTest: _pumpIcons,
      builder: () => dsGoldenMatrix(
        name: name,
        builder: view,
        densities: const [DsDensityMode.comfortable, DsDensityMode.compact],
        textScales: const [1.0, 2.0],
        locales: const [Locale('de'), Locale('en')],
      ),
    );
  }
}
