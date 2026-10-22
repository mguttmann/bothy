// SPDX-License-Identifier: Apache-2.0

// Red team round 3 (D-M4-35-17): a 401 step_up_required that answers an attach after the flow lost the
// tab (End, a closed tab) must not open a step-up prompt (D-M4-35-17).

import 'dart:async';

import 'package:flutter_test/flutter_test.dart';
import 'package:terminal_ui/terminal_ui.dart';

import 'support/fakes.dart';

void main() {
  test(
    'End during a pending attach answered by step_up_required runs no step-up',
    () async {
      final h = await Harness.create();
      await h.open();
      await h.connected();
      await h.controller.detach();
      await h.until((s) => s.phase == TerminalPhase.detached);
      h.api.attachGate = Completer<void>();
      h.api.attachErrors.add(stepUpRequired);
      final attaching = h.controller.attach();
      await h.settle();
      expect(h.log.last, 'attach');
      final before = h.stepUps.length;
      await h.controller.end();
      expect(h.controller.state.phase, TerminalPhase.ended);
      h.api.attachGate!.complete();
      await attaching;
      await h.settle();
      expect(h.stepUps.length, before);
      expect(h.log.where((e) => e == 'attach').length, 1);
      expect(h.controller.state.phase, TerminalPhase.ended);
    },
  );

  test('a closed tab during a pending attach answered by step_up_required runs no step-up', () async {
    final h = await Harness.create();
    await h.open();
    await h.connected();
    await h.controller.detach();
    await h.until((s) => s.phase == TerminalPhase.detached);
    h.api.attachGate = Completer<void>();
    h.api.attachErrors.add(stepUpRequired);
    final attaching = h.controller.attach();
    await h.settle();
    expect(h.log.last, 'attach');
    final before = h.stepUps.length;
    h.controller.dispose();
    h.api.attachGate!.complete();
    await attaching;
    await h.settle();
    expect(h.stepUps.length, before);
    expect(h.log.where((e) => e == 'attach').length, 1);
  });
}
