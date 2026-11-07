// SPDX-License-Identifier: Apache-2.0

// Accessibility of "New agent" (brief A1-08 Design note 6; 01 8): the ways in, the preview, the
// steps and the interview have labelled controls with their state in words, contrast and
// targets in light and dark, and keep every control at text scale 2.

import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';

import '../goldens/new_agent_golden_test.dart' as golden;

void main() {
  for (final name in const [
    'new_agent_start',
    'new_agent_preview',
    'new_agent_steps',
    'new_agent_interview',
  ]) {
    dsAccessibilityTests(
      name,
      build: golden.views[name]!,
      window: const Size(1280, 4000),
    );
  }
}
