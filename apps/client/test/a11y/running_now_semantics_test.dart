// SPDX-License-Identifier: Apache-2.0

// Accessibility of Running now (brief M4-32 Design note 1; 01 5.1 D10 and 8): every run reads
// its agent, place, state, duration, context and cost in words, the state never by color
// alone, and the filters are labelled buttons that say their choice.

import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../goldens/search/cross_views_golden_test.dart' as golden;

void main() {
  dsAccessibilityTests(
    'Running now',
    build: golden.views['running_now_d10']!,
    reachable: (l) => [
      find.byKey(const ValueKey('running.filter.space')),
      find.byKey(const ValueKey('running.filter.agent')),
      find.byKey(const ValueKey('running.filter.mine')),
    ],
  );
}
