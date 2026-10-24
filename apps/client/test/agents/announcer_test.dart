// SPDX-License-Identifier: Apache-2.0

// Live announcements of the live box (brief M4-21 Tests; 01 8; components.md 2.4): a simulated
// patch stream through the mapping of the box announces only state changes and questions, at
// most once per `a11y.live_announce_min_interval`; patches to steps and numbers never.

import 'package:client/agents/run_mapping.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/sync_engine.dart';

import '../support/agent_fakes.dart';
import '../support/structure.dart';
import '../support/widgets.dart';

void main() {
  test('only state changes and questions are announced, merged within the '
      'interval', () {
    const interval = Duration(seconds: 10);
    var now = t0;
    final said = <String>[];
    final announcer = LiveAnnouncer(
      minInterval: interval,
      clock: () => now,
      sink: said.add,
    );
    var version = 0;
    void observe(
      String state, {
      String? statusLine,
      Map<String, Object?> steps = const {},
      double? context,
    }) {
      final run = runFromDocument(
        RunDocument.fromJson(
          runDoc(
            1,
            version: ++version,
            state: state,
            statusLine: statusLine,
            steps: steps,
            contextPct: context,
          ),
        ),
        agentName: 'Robin',
        now: now,
      );
      // As DsLiveBox observes its run (components.md 2.4).
      announcer.observe(
        state: en.liveBoxAnnounceState(run.agentName, dsRunStateWord(en, run)),
        questionId: run.questionId,
        question: en.liveBoxAnnounceQuestion(run.agentName),
      );
    }

    observe('running');
    // Twenty patches of steps, numbers and the status line: nothing to announce.
    for (var i = 0; i < 20; i++) {
      now = now.add(const Duration(seconds: 1));
      observe(
        'running',
        statusLine: 'step $i',
        context: i.toDouble(),
        steps: {'s$i': step('s$i', i)},
      );
    }
    expect(said, isEmpty);

    // A question: announced at once.
    now = now.add(const Duration(seconds: 1));
    observe('waiting_input');
    expect(said, [en.liveBoxAnnounceQuestion('Robin')]);
    // Patches while waiting keep the question; nothing new.
    observe('waiting_input', statusLine: 'still waiting');
    expect(said, hasLength(1));

    // Two state changes within the interval merge into one, the later one.
    now = now.add(const Duration(seconds: 2));
    observe('running');
    now = now.add(const Duration(seconds: 2));
    observe('completed');
    expect(said, hasLength(1));
    now = now.add(interval);
    announcer.flush();
    expect(said, [
      en.liveBoxAnnounceQuestion('Robin'),
      en.liveBoxAnnounceState('Robin', en.liveBoxStateCompleted),
    ]);
  });
}
