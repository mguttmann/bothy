// SPDX-License-Identifier: Apache-2.0

// Accessibility of the live box (brief M4-32 Design note 1; 01 6.2 and 8; components.md 8):
// the box is a container whose label summarizes agent, state, time and context, Stop names the
// agent, and the collapsed line and the interrupted line keep their state in words. The rule
// "only state changes and questions are announced, at most once per interval, patches never"
// is tested with a simulated patch stream in
// packages/design_system/test/components/live_box_test.dart.

import 'package:design_system/testing.dart';
import 'package:flutter_test/flutter_test.dart';

import '../goldens/agents/agents_golden_test.dart' as golden;

void main() {
  dsAccessibilityTests(
    'the live box',
    build: golden.liveBoxes,
    semantics: (tester, l) {
      // Three boxes and the collapsed line of the finished run.
      final boxes = find.bySemanticsLabel(RegExp('^Run of Robin: '));
      expect(boxes, findsNWidgets(4));
      // A container: the summary opens its label, Stop is a node of its own inside it.
      final running = tester.getSemantics(
        find.bySemanticsLabel(RegExp('^Run of Robin: Running, for ')),
      );
      expect(running.hasChildren, isTrue);
      expect(running.isMergedIntoParent, isFalse);
      expect(
        find.bySemanticsLabel(RegExp('^Run of Robin: Waiting')),
        findsOneWidget,
      );
      expect(
        tester.getSemantics(
          find.bySemanticsLabel(l.liveBoxStopRun('Robin')).first,
        ),
        isSemantics(isButton: true, hasTapAction: true),
      );
    },
  );
}
