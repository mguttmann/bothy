// SPDX-License-Identifier: Apache-2.0

// Accessibility of M7 Offline (brief M5-10 Design note 2; 01 5.3 M7, 8): the banner, the
// waiting message with "sending" and the failed one with its reason, Retry, Edit and Discard,
// all in words.

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../phone/support/phone_scenes.dart';
import 'support.dart';

void main() {
  phoneScreenTests(
    'M7 Offline',
    build: offlineScene,
    reachable: (l) => [
      find.byKey(const ValueKey('shell.offline')),
      find.text(l.messageEdit),
      find.text(l.messageDiscard),
      find.byKey(const ValueKey('phone.back')),
    ],
  );
}
