// SPDX-License-Identifier: Apache-2.0

// Goldens of the emergency actions on the phone (brief M5-09 Tests): the people, one person
// with devices, grants and "Disable person", and the sheet of each action (revoke a device,
// revoke a grant, disable a person); light and dark in the `touch` density, text scaling 1 and
// 2, German and English, at the width of a phone.

import 'package:admin_ui/admin_ui.dart';
import 'package:alchemist/alchemist.dart';
import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../admin/support.dart';

Widget _phone(Widget child) => Material(
  type: MaterialType.transparency,
  child: SizedBox(width: 390, height: 640, child: child),
);

FakeAdminRepository _repository() =>
    FakeAdminRepository(prompt: StepUpDialogPrompt(() => null))
      ..stepUpFresh = true
      ..grantList.add(
        AdminGrant(
          id: 'g-sam',
          subjectKind: AdminSubjectKind.user,
          subjectId: 'sam',
          targetKind: AdminTargetKind.space,
          targetId: 'marketing',
          level: AccessLevel.write,
          capabilities: const [AccessCapability.upload],
          reason: 'Campaign work',
          grantedAt: DateTime.utc(2026, 10, 3),
          expiresAt: DateTime.utc(2027),
        ),
      );

/// The sheet of one action with its texts in the cell's language.
Widget _sheet(
  String Function(AppLocalizations l) title,
  String Function(AppLocalizations l) body, {
  bool reasonRequired = false,
}) => _phone(
  Builder(
    builder: (context) {
      final l = AppLocalizations.of(context);
      return Align(
        alignment: Alignment.bottomCenter,
        child: ColoredBox(
          color: context.dsColors.surfaceRaised,
          child: EmergencyConfirmSheet(
            title: title(l),
            body: body(l),
            stepUp: true,
            confirmLabel: l.adminRevoke,
            reasonRequired: reasonRequired,
          ),
        ),
      );
    },
  ),
);

/// The emergency screens by name, for the goldens and the phone accessibility tests (brief
/// M5-10).
final Map<String, Widget Function()> emergencyViews = {
  'home': () =>
      _phone(EmergencyHome(repository: _repository(), onOpen: (_) {})),
  'person': () => _phone(
    PersonEmergencyView(
      repository: _repository(),
      userId: 'sam',
      owner: true,
      timeZone: 'Europe/Berlin',
      clock: testClock,
    ),
  ),
  'sheet_device': () => _sheet(
    (l) => l.devicesRevokeTitle('Sam laptop'),
    (l) => l.emergencyRevokeDeviceBody('Sam laptop', 'Sam'),
  ),
  'sheet_grant': () => _sheet(
    (l) => l.adminRevokeGrantTitle('Sam'),
    (l) => l.emergencyRevokeGrantBody('Sam', l.adminTargetSpace('Marketing')),
    reasonRequired: true,
  ),
  'sheet_disable': () => _sheet(
    (l) => l.adminDisableTitle('Sam'),
    (l) => l.emergencyDisableBody('Sam'),
  ),
};

void main() {
  for (final MapEntry(key: name, value: view) in emergencyViews.entries) {
    goldenTest(
      '$name in light and dark, text scale 1 and 2, de and en',
      fileName: 'emergency_$name',
      pumpBeforeTest: dsPumpIcons,
      builder: () => dsGoldenMatrix(
        name: name,
        builder: view,
        densities: const [DsDensityMode.touch],
        textScales: const [1.0, 2.0],
        locales: const [Locale('de'), Locale('en')],
      ),
    );
  }
}
