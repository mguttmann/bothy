// SPDX-License-Identifier: Apache-2.0

// Accessibility of M5 Record voice message (brief M5-10 Design note 2; 01 5.3 M5, 8): every
// state of the recording on the phone. The button mode of the gesture, the live region of the
// remaining time and the notice before the limit are tested in
// test/phone/compose/voice_button_a11y_test.dart.

import 'package:flutter/material.dart';

import '../../goldens/phone/voice_golden_test.dart' as voice;
import 'support.dart';

void main() {
  for (final MapEntry(key: name, value: scene) in voice.voiceScenes.entries) {
    phoneScreenTests('M5 Recording $name', build: () => Center(child: scene()));
  }
}
