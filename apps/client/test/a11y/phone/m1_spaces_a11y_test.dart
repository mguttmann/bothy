// SPDX-License-Identifier: Apache-2.0

// Accessibility of M1 Spaces on iOS and Android (brief M5-10 Design note 2; 01 5.3 M1, 5.4,
// 8): the tree of spaces and channels with their state in words, Jump to, Running now and the
// tab bar.

import 'package:client/phone/phone_navigation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../phone/support/phone_scenes.dart';
import 'support.dart';

void main() {
  for (final android in [false, true]) {
    phoneScreenTests(
      android ? 'M1 Spaces on Android' : 'M1 Spaces',
      build: android ? androidPhoneScenes['m1_spaces']! : spacesScene,
      platform: android ? TargetPlatform.android : TargetPlatform.iOS,
      // The badge of the Running tab says what it counts, never a bare number.
      semantics: (tester, l) {
        final tab = tester.getSemantics(
          find.byKey(const ValueKey('phone.tab.running')),
        );
        expect(tab.label, startsWith(l.phoneTabRunningCount(2)));
      },
      reachable: (l) => [
        find.byKey(const ValueKey('sidebar.jumpTo')),
        find.byKey(const ValueKey('sidebar.runningNow')),
        for (final tab in PhoneTab.values)
          find.byKey(ValueKey('phone.tab.${tab.name}')),
      ],
    );
  }
}
