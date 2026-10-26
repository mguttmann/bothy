// SPDX-License-Identifier: Apache-2.0

// Accessibility of M3 History with the compact live box on iOS and Android (brief M5-10 Design
// note 2; 01 5.3 M3, 5.4, 8): the live box summarizes agent and state with Stop always
// reachable, the approval buttons at full width, the collapsed line after the run, the input.
// The live box announces state changes at most once per `a11y.live_announce_min_interval`
// (packages/design_system/test/components/live_box_test.dart).

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../phone/support/phone_scenes.dart';
import 'support.dart';

void main() {
  for (final android in [false, true]) {
    phoneScreenTests(
      android
          ? 'M3 History while running on Android'
          : 'M3 History while running',
      build: () => runningScene(android: android),
      platform: android ? TargetPlatform.android : TargetPlatform.iOS,
      reachable: (l) => [
        find.byKey(const ValueKey('phone.back')),
        find.bySemanticsLabel(l.liveBoxStopRun('Robin')),
        find.byKey(const ValueKey('ds-component-once')),
        find.byKey(const ValueKey('ds-component-deny')),
        find.byType(TextField),
      ],
    );
  }
  phoneScreenTests(
    'M3 History waiting for an answer',
    build: waitingScene,
    reachable: (l) => [
      find.bySemanticsLabel(l.liveBoxStopRun('Robin')),
      find.byType(TextField),
    ],
  );
  phoneScreenTests(
    'M3 History after the run',
    build: collapsedScene,
    reachable: (l) => [
      find.byKey(const ValueKey('scene.livebox')),
      find.byType(TextField),
    ],
  );
}
