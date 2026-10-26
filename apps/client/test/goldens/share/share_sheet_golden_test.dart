// SPDX-License-Identifier: Apache-2.0

// The goldens of the Flutter target sheet of Android (brief M5-06 Tests; 01 5.3 M6, 5.4): the
// sheet with a chosen thread and a comment, and the sheet with a new post and a file over
// `file_max_upload_bytes` (the reason under the preview, Send disabled), on Android in the
// density `touch`, light and dark, text scaling 1 and 2, German and English. One file per
// scene and theme keeps each file below repo.max_file_kb.

import 'package:alchemist/alchemist.dart';
import 'package:client/app/form_factor.dart';
import 'package:client/share/share_sheet.dart';
import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';

import '../../phone/support/phone_app.dart' show phoneSize;
import '../../share/share_samples.dart';

const InstanceLimits _limits = InstanceLimits(
  messageMaxContentBytes: 65536,
  messageMaxAttachments: 10,
  fileMaxUploadBytes: 2000000,
  threadTitleMaxChars: 200,
  voiceMaxDurationMs: 600000,
);

/// [child] with the target platform Android, whatever the host of the test.
Widget _android(Widget child) => Builder(
  builder: (context) => Theme(
    data: Theme.of(context).copyWith(platform: TargetPlatform.android),
    child: child,
  ),
);

Widget _sheet({required bool refused}) {
  final controller = ShareSheetController(
    files: [
      if (refused)
        sharedFile('screen-recording.mov', 3000000)
      else
        sharedFile('screenshot.png', 1258291),
    ],
    text: refused ? '' : 'See error above',
    limits: _limits,
    recent: () async => sheetTargets,
    search: (_) async => const [],
  )..select(refused ? sheetTargets[1] : sheetTargets.first);
  return FormFactorScope(
    formFactor: FormFactor.phone,
    child: SizedBox.fromSize(
      size: phoneSize,
      child: ShareSheet(
        controller: controller,
        onCancel: () {},
        onSend: (_) async {},
      ),
    ),
  );
}

void main() {
  for (final refused in [false, true]) {
    final scene = refused ? 'refused' : 'chosen';
    for (final theme in DsThemeMode.values) {
      goldenTest(
        'the share sheet ($scene) on Android in ${theme.name}, text scale 1 and 2, '
        'de and en',
        fileName: 'share_sheet_${scene}_${theme.name}',
        pumpBeforeTest: dsPumpIcons,
        builder: () => dsGoldenMatrix(
          name: 'share sheet $scene',
          themes: [theme],
          densities: const [DsDensityMode.touch],
          textScales: const [1.0, 2.0],
          locales: const [Locale('de'), Locale('en')],
          builder: () => _android(_sheet(refused: refused)),
        ),
      );
    }
  }
}
