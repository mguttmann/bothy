// SPDX-License-Identifier: Apache-2.0

// Goldens of the permission screens (brief M4-27 Tests): each step of the grant wizard, the
// preview with hints (the example of 07 9.3, read at run time), "Who has access?" with every
// source kind and the frame of "View as person"; light and dark, two densities, text scaling
// 1 and 2, German and English. One file per view keeps each file below repo.max_file_kb.

import 'package:admin_ui/admin_ui.dart';
import 'package:alchemist/alchemist.dart';
import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';

import '../admin/support.dart';

Widget _frame(Widget child) => Material(
  type: MaterialType.transparency,
  child: SizedBox(
    width: 760,
    child: Padding(padding: const EdgeInsets.all(16), child: child),
  ),
);

FakeAdminRepository _repository() {
  final r = FakeAdminRepository(prompt: StepUpDialogPrompt(() => null))
    ..stepUpFresh = true
    ..extraAccess['campaigns'] = everySource()
    ..snapshots['sam'] = ViewAsResult(
      user: const AccessPrincipal(
        id: 'sam',
        kind: 'user',
        handle: 'sam',
        displayName: 'Sam',
      ),
      expiresAt: DateTime.utc(2026, 10, 17, 9, 15),
      spaces: const [
        ViewAsItem(
          id: 'marketing',
          name: 'Marketing',
          level: AccessLevel.read,
          capabilities: [],
        ),
      ],
      channels: const [
        ViewAsItem(
          id: 'campaigns',
          name: 'Kampagnen',
          level: AccessLevel.trigger,
          capabilities: [AccessCapability.upload],
          spaceId: 'marketing',
        ),
      ],
    );
  r.previewAnswer = (request) => GrantPreviewResult(
    allowed: true,
    effects: const [
      GrantEffect(
        principal: AccessPrincipal(
          id: 'sam',
          kind: 'user',
          handle: 'sam',
          displayName: 'Sam',
        ),
        before: AccessLevel.none,
        after: AccessLevel.trigger,
        capabilitiesAdded: [AccessCapability.upload],
      ),
    ],
    violations: const [],
    hints: const [
      GrantHint(
        code: 'read_horizon',
        agentId: 'texter',
        principalId: 'sam',
        channelId: 'internal',
      ),
      GrantHint(code: 'expiry_missing', principalId: 'sam'),
    ],
    text: previewExample(),
    textLocale: 'de',
  );
  return r;
}

GrantWizard _wizard(int step) => GrantWizard(
  repository: _repository(),
  subject: const GrantSubjectChoice(AdminSubjectKind.user, 'sam', 'Sam'),
  target: step > 0 ? campaigns : null,
  level: step > 1 ? AccessLevel.trigger : null,
  step: step,
  timeZone: 'Europe/Berlin',
  clock: testClock,
  debounce: Duration.zero,
  onCancel: () {},
);

final Map<String, Widget Function()> _views = {
  'wizard_who': () => _frame(_wizard(0)),
  'wizard_where': () => _frame(_wizard(1)),
  'wizard_preview': () => _frame(_wizard(2)),
  'who_has_access': () => _frame(
    WhoHasAccess(
      repository: _repository(),
      target: campaigns,
      timeZone: 'Europe/Berlin',
      clock: testClock,
    ),
  ),
  'view_as': () => _frame(
    ViewAsPerson(
      repository: _repository(),
      userId: 'sam',
      name: 'Sam',
      timeZone: 'Europe/Berlin',
      clock: testClock,
      onExit: () {},
    ),
  ),
};

void main() {
  for (final MapEntry(key: name, value: view) in _views.entries) {
    goldenTest(
      '$name in light and dark, two densities, text scale 1 and 2, de and en',
      fileName: 'grants_$name',
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
