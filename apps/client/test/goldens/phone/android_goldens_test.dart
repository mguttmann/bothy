// SPDX-License-Identifier: Apache-2.0

// The goldens of the Android variant (brief M5-03 Tests; 01 5.4): M1 Spaces, M2 Post list and M3
// History in the Android frame (the top app bar with the up arrow instead of the iOS back
// button), and E2 with and without the passkey button (a build with and without an App Links
// host), with the target platform Android, in the density `touch`, light and dark, text scaling
// 1 and 2, German and English. One file per screen and theme keeps each file below
// repo.max_file_kb.

import 'package:alchemist/alchemist.dart';
import 'package:client/app/form_factor.dart';
import 'package:client/onboarding/e2_login.dart';
import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';

import '../../phone/support/phone_app.dart' show phoneSize;
import '../../phone/support/phone_scenes.dart';
import '../../support/instances.dart';

/// [child] with the target platform Android, whatever the host of the test.
Widget android(Widget child) => Builder(
  builder: (context) => Theme(
    data: Theme.of(context).copyWith(platform: TargetPlatform.android),
    child: child,
  ),
);

Widget _login({required bool passkeys}) => FormFactorScope(
  formFactor: FormFactor.phone,
  child: SizedBox.fromSize(
    size: phoneSize,
    child: LoginScreen(
      instance: localWithPassword,
      passkeysAvailable: passkeys,
      onProvider: (_) {},
      onPasskey: (_) {},
      onPassword: (_, _, _) {},
      onEnroll: () {},
      onPair: () {},
      onChangeServer: () {},
    ),
  ),
);

/// The screens of the Android goldens by name.
Map<String, Widget Function()> androidScreens() => {
  ...androidPhoneScenes,
  'e2_app_links': () => _login(passkeys: true),
  'e2_without_app_links': () => _login(passkeys: false),
};

void main() {
  for (final MapEntry(key: name, value: screen) in androidScreens().entries) {
    for (final theme in DsThemeMode.values) {
      goldenTest(
        '$name on Android in ${theme.name}, text scale 1 and 2, de and en',
        fileName: 'android_${name}_${theme.name}',
        pumpBeforeTest: dsPumpIcons,
        builder: () => dsGoldenMatrix(
          name: name,
          themes: [theme],
          densities: const [DsDensityMode.touch],
          textScales: const [1.0, 2.0],
          locales: const [Locale('de'), Locale('en')],
          builder: () => android(screen()),
        ),
      );
    }
  }
}
