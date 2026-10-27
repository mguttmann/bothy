// SPDX-License-Identifier: Apache-2.0

// Accessibility of M2 Post list on iOS and Android (brief M5-10 Design note 2; 01 5.3 M2, 5.4,
// 8): back, New post, the filters with their state and every post with its state in words.

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../phone/support/phone_scenes.dart';
import 'support.dart';

void main() {
  for (final android in [false, true]) {
    phoneScreenTests(
      android ? 'M2 Post list on Android' : 'M2 Post list',
      build: () => postListScene(android: android),
      platform: android ? TargetPlatform.android : TargetPlatform.iOS,
      reachable: (l) => [
        find.byKey(const ValueKey('phone.back')),
        find.byKey(const ValueKey('postList.newPost')),
        find.byKey(const ValueKey('postList.filter.waiting')),
      ],
    );
  }
}
