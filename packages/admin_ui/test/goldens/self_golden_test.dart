// SPDX-License-Identifier: Apache-2.0

// Goldens of the device and session views and their dialogs (brief M4-25 Tests): light and
// dark, two densities, text scaling 1 and 2, German and English. One file per view keeps each
// file below repo.max_file_kb.

import 'package:admin_ui/admin_ui.dart';
import 'package:alchemist/alchemist.dart';
import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';

import '../self/support.dart';

Widget _frame(Widget child, {double width = 720}) => Material(
  type: MaterialType.transparency,
  child: SizedBox(
    width: width,
    child: Padding(padding: const EdgeInsets.all(16), child: child),
  ),
);

FakeSelfRepository _repository() =>
    FakeSelfRepository(prompt: StepUpDialogPrompt(() => null));

final Map<String, Widget Function()> _views = {
  'devices': () => _frame(
    MyDevicesView(
      repository: _repository(),
      timeZone: 'Europe/Berlin',
      clock: testClock,
    ),
  ),
  'sessions': () => _frame(
    MySessionsView(
      repository: _repository(),
      timeZone: 'Europe/Berlin',
      clock: testClock,
    ),
  ),
  'step_up_dialog': () => _frame(
    const StepUpDialog(purposeText: 'Revoke the device Alex phone'),
    width: 560,
  ),
  'confirm_device_dialog': () =>
      _frame(const ConfirmDeviceDialog(deviceName: 'Alex tablet'), width: 560),
  'add_device_dialog': () => _frame(const AddDeviceDialog(), width: 560),
  'revoke_confirm': () => _frame(
    const RevokeConfirmDialog(
      title: 'Revoke Alex phone?',
      body:
          'The device is signed out and deletes its local data at its next '
          'contact.',
      confirmLabel: 'Revoke',
      withReason: true,
    ),
    width: 560,
  ),
};

void main() {
  for (final MapEntry(key: name, value: view) in _views.entries) {
    goldenTest(
      '$name in light and dark, two densities, text scale 1 and 2, de and en',
      fileName: 'self_$name',
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
