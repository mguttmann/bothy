// SPDX-License-Identifier: Apache-2.0

// Goldens of the audit log (brief M4-29 Tests): the list with its filters, and the result of
// the chain check intact and broken; light and dark, two densities, text scaling 1 and 2,
// German and English.

import 'package:admin_ui/admin_ui.dart';
import 'package:alchemist/alchemist.dart';
import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';

import '../admin/support.dart';

Widget _frame(Widget child, {double? height}) => Material(
  type: MaterialType.transparency,
  child: SizedBox(
    width: 760,
    height: height,
    child: Padding(padding: const EdgeInsets.all(16), child: child),
  ),
);

const AuditChainCheck _ok = AuditChainCheck(
  ok: true,
  checkedFromSeq: 1,
  checkedToSeq: 18402,
  checkpointsCompared: 12,
  lastCheckpointSeq: 18390,
  warnings: [],
);

const AuditChainCheck _broken = AuditChainCheck(
  ok: false,
  checkedFromSeq: 18391,
  checkedToSeq: 18397,
  firstBrokenSeq: 18397,
  failureReason: 'hash_mismatch',
  checkpointsCompared: 1,
  lastCheckpointSeq: 18390,
  warnings: ['checkpoint_sink_not_configured'],
);

/// The scenes of the goldens, shared with the accessibility tests (test/a11y).
final Map<String, Widget Function()> views = {
  'list': () => _frame(
    AuditLogView(
      repository: FakeAdminRepository(prompt: StepUpDialogPrompt(() => null))
        ..stepUpFresh = true,
      timeZone: 'Europe/Berlin',
      clock: testClock,
      saveTarget: (_) async => null,
    ),
    height: 640,
  ),
  'chain_ok': () => _frame(const AuditChainPanel(result: _ok)),
  'chain_broken': () => _frame(const AuditChainPanel(result: _broken)),
};

void main() {
  for (final MapEntry(key: name, value: view) in views.entries) {
    goldenTest(
      '$name in light and dark, two densities, text scale 1 and 2, de and en',
      fileName: 'audit_$name',
      pumpBeforeTest: dsPumpIcons,
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
