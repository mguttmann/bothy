// SPDX-License-Identifier: Apache-2.0

// The goldens of E1 to E7 in the phone form (brief M5-02 Tests; 01 5.6: on the phone the screens
// fill the screen): every screen state of the desktop goldens and E2 without the passkey button
// (a build without an associated domain), in the density `touch`, light and dark, text scaling 1
// and 2, German and English. One file per screen state keeps each file below
// repo.max_file_kb.

import 'package:alchemist/alchemist.dart';
import 'package:client/app/form_factor.dart';
import 'package:client/onboarding/e2_login.dart';
import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';

import '../../phone/support/phone_app.dart' show phoneSize;
import '../../support/instances.dart';
import '../onboarding/onboarding_golden_test.dart' as desktop;

Map<String, Widget Function()> screens() => {
  ...desktop.screens(),
  'e2_local_without_passkey': () => LoginScreen(
    instance: localWithPassword,
    passkeysAvailable: false,
    onProvider: (_) {},
    onPasskey: (_) {},
    onPassword: (_, _, _) {},
    onEnroll: () {},
    onPair: () {},
    onChangeServer: () {},
  ),
};

void main() {
  for (final MapEntry(key: name, value: screen) in screens().entries) {
    goldenTest(
      '$name on the phone in light and dark, text scale 1 and 2, de and en',
      fileName: 'onboarding_phone_$name',
      builder: () => dsGoldenMatrix(
        name: name,
        densities: const [DsDensityMode.touch],
        textScales: const [1.0, 2.0],
        locales: const [Locale('de'), Locale('en')],
        builder: () => FormFactorScope(
          formFactor: FormFactor.phone,
          child: SizedBox.fromSize(size: phoneSize, child: screen()),
        ),
      ),
    );
  }
}
