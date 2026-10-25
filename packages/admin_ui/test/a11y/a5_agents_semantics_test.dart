// SPDX-License-Identifier: Apache-2.0

// Accessibility of A5 Agents (brief M4-32 Design note 1; 01 5.6 and 8): the agent list, an agent page, the backend switch, the switch of all agents and the guided flow say mode, presence and costs in words, with contrast and targets in light and dark and every control at text scale 2.

import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';

import '../goldens/agents_golden_test.dart' as golden;

void main() {
  for (final name in const [
    'list',
    'page',
    'switch',
    'switch_all',
    'guided_preview',
    'i1',
    'guided_result',
  ]) {
    dsAccessibilityTests(
      'A5 $name',
      build: golden.views[name]!,
      window: const Size(1280, 1400),
    );
  }
}
