// SPDX-License-Identifier: Apache-2.0

// The goldens of the update notices (brief M7-13 Report item 4; 15 20 row "Golden"): "Update
// required" with the update path and "Update recommended" with its close button, and E1 "App
// too old" with the update action, in light and dark, text scaling 1 and 2, German and English.

import 'package:alchemist/alchemist.dart';
import 'package:client/app/update/update_notice.dart';
import 'package:client/onboarding/e1_server_address.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';

void noop() {}

/// The width of a phone, the narrowest place the banners show.
const double phoneWidth = 390;

Map<String, (Size, Widget Function())> scenes() => {
  'update_required': (
    const Size(phoneWidth, 440),
    () => const UpdateRequiredBannerView(onUpdate: noop),
  ),
  'update_recommended': (
    const Size(phoneWidth, 440),
    () => const RecommendedUpdateBannerView(
      version: '1.5.0',
      onUpdate: noop,
      onDismiss: noop,
    ),
  ),
  'e1_app_too_old_update': (
    const Size(640, 900),
    () => ServerAddressScreen(
      address: 'https://chat.example.com',
      status: ServerStatus.appTooOld,
      minVersion: '2.0.0',
      onConnect: (_) {},
      onUpdate: noop,
    ),
  ),
};

void main() {
  for (final MapEntry(key: name, value: (size, scene)) in scenes().entries) {
    goldenTest(
      '$name in light and dark, text scale 1 and 2, de and en',
      fileName: name,
      builder: () => dsGoldenMatrix(
        name: name,
        builder: () => SizedBox.fromSize(
          size: size,
          child: Material(
            child: Align(alignment: Alignment.topCenter, child: scene()),
          ),
        ),
        textScales: const [1.0, 2.0],
        locales: const [Locale('de'), Locale('en')],
      ),
    );
  }
}
