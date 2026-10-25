// SPDX-License-Identifier: Apache-2.0

// Accessibility of the mentions view (brief M4-32 Design note 1; 01 5.1 and 8): every entry is
// a labelled button that reads who mentioned, where and the text, unread entries say so in
// words.

import 'package:design_system/testing.dart';
import 'package:flutter_test/flutter_test.dart';

import '../goldens/search/cross_views_golden_test.dart' as golden;

void main() {
  dsAccessibilityTests(
    'the mentions view',
    build: golden.views['mentions_view']!,
  );
}
