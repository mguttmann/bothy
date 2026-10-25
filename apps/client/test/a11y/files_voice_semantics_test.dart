// SPDX-License-Identifier: Apache-2.0

// Accessibility of files and voice messages (brief M4-32 Design note 1; 01 8; components.md
// 11): every attachment reads its name, size and state (checking, blocked, failed) in words,
// a preview image has a label, and a voice message offers play and its transcript as text.

import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../goldens/files/files_golden_test.dart' as golden;

void main() {
  dsAccessibilityTests(
    'the file tiles',
    build: golden.tiles,
    window: const Size(1280, 1400),
    semantics: (tester, l) {
      expect(find.bySemanticsLabel(RegExp('^report.pdf, ')), findsWidgets);
    },
  );
  dsAccessibilityTests(
    'the image previews',
    build: golden.images,
    window: const Size(1280, 1400),
  );
  dsAccessibilityTests(
    'the voice messages',
    build: golden.voice,
    window: const Size(1280, 1400),
    semantics: (tester, l) {
      expect(find.bySemanticsLabel(RegExp('Transcript')), findsWidgets);
    },
  );
}
