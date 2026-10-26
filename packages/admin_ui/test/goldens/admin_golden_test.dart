// SPDX-License-Identifier: Apache-2.0

// Goldens of the administration (brief M4-26 Tests): the shell, the people list, a person's
// detail, a group and the queue of waiting devices; light and dark, two densities, text
// scaling 1 and 2, German and English. One file per view keeps each file below
// repo.max_file_kb.

import 'package:admin_ui/admin_ui.dart';
import 'package:alchemist/alchemist.dart';
import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';

import '../admin/support.dart';

Widget _frame(Widget child, {double width = 760}) => Material(
  type: MaterialType.transparency,
  child: SizedBox(
    width: width,
    child: Padding(padding: const EdgeInsets.all(16), child: child),
  ),
);

FakeAdminRepository _repository() =>
    FakeAdminRepository(prompt: StepUpDialogPrompt(() => null));

/// The scenes of the goldens, shared with the accessibility tests (test/a11y).
final Map<String, Widget Function()> views = {
  'shell': () => SizedBox(
    width: 1100,
    height: 640,
    child: AdminShell(
      repository: _repository(),
      location: const AdminLocation(AdminSection.groups),
      onNavigate: (_) {},
      onExit: () {},
      timeZone: 'Europe/Berlin',
      clock: testClock,
    ),
  ),
  'people': () => _frame(
    PeopleList(
      repository: _repository(),
      timeZone: 'Europe/Berlin',
      onOpen: (_) {},
    ),
  ),
  'person': () => _frame(
    PersonDetail(
      repository: _repository(),
      userId: 'sam',
      timeZone: 'Europe/Berlin',
      clock: testClock,
      onBack: () {},
    ),
  ),
  'group': () => _frame(
    GroupDetail(
      repository: _repository(),
      groupId: 'support',
      timeZone: 'Europe/Berlin',
      clock: testClock,
      onBack: () {},
    ),
  ),
  'waiting_devices': () => _frame(
    WaitingDevicesView(
      repository: _repository(),
      timeZone: 'Europe/Berlin',
      clock: testClock,
    ),
  ),
};

void main() {
  for (final MapEntry(key: name, value: view) in views.entries) {
    goldenTest(
      '$name in light and dark, two densities, text scale 1 and 2, de and en',
      fileName: 'admin_$name',
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
