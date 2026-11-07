// SPDX-License-Identifier: Apache-2.0

// The shared flow of autonomous runs in the app against the fake server (brief AR-08 Tests;
// 01 6.21, 09 5.8), for the desktop test and the phone suite: a finished run of Robin for Alex
// with two pending wake-ups; the first shows above the composer with its condition; the second
// ends `expired` live and its note is a note of the thread, translated, without an avatar; a
// fired wake-up's run shows "Woke up: <label>" at its anchor; Cancel ends the first on the
// server and its row leaves the list.

import 'package:design_system/design_system.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';
import 'package:sync_engine/testing.dart';

import '../../test/support/until.dart' show testPatience;
import 'test_app.dart';

/// The objects the flow made in the world.
final class AutonomousWorld {
  AutonomousWorld._(
    this.thread,
    this.trigger,
    this.source,
    this.first,
    this.second,
  );

  /// The thread.
  final String thread;

  /// Alex's message that started the source run.
  final String trigger;

  /// The finished run of Robin.
  final JsonMap source;

  /// The pending wake-up `CI of #123` that the flow cancels.
  final JsonMap first;

  /// The pending wake-up `nightly` that the flow lets expire.
  final JsonMap second;
}

/// Adds the source run and the two wake-ups to [thread] before the sign-in.
AutonomousWorld seedAutonomous(FakeWorld world, WorldSeed seed, String thread) {
  final trigger = world.createMessage(seed.alex.id, thread, {
    'client_msg_id': world.nextId(),
    'content_md': 'watch the checks',
  }).message;
  final source = world.createRun(
    agentId: seed.robin.id,
    triggeringMessageId: trigger['id']! as String,
    triggeredBy: seed.alex.id,
  );
  world.setRunState(source['id']! as String, 'completed');
  final first = world.addWake(
    threadId: thread,
    agentId: seed.robin.id,
    sourceRunId: source['id']! as String,
    triggeredBy: seed.alex.id,
    kind: 'pr_checks',
    label: 'CI of #123',
    condition: {'repository': 'example/app', 'number': 123},
  );
  final second = world.addWake(
    threadId: thread,
    agentId: seed.robin.id,
    sourceRunId: source['id']! as String,
    triggeredBy: seed.alex.id,
    label: 'nightly',
  );
  return AutonomousWorld._(
    thread,
    trigger['id']! as String,
    source,
    first,
    second,
  );
}

/// Pumps until [condition] holds, failing after the patience of the tests.
Future<void> _until(
  WidgetTester tester,
  String what,
  bool Function() condition,
) async {
  final end = DateTime.now().add(testPatience);
  while (!condition()) {
    if (DateTime.now().isAfter(end)) fail('timed out: $what');
    await tester.pump(const Duration(milliseconds: 50));
  }
}

/// Runs the checks of the flow on the opened thread of [w].
Future<void> checkAutonomous(
  WidgetTester tester,
  TestApp app,
  AutonomousWorld w,
) async {
  final en = lookupAppLocalizations(const Locale('en'));
  final world = app.fake.server.world;
  final seed = app.fake.seed;
  final firstRow = find.byKey(ValueKey('wakes.pending.${w.first['id']}'));
  await app.waitFor(firstRow);
  await app.waitFor(
    find.descendant(
      of: firstRow,
      matching: find.textContaining(en.wakesConditionPrChecks('123')),
    ),
  );

  // An end comes live with its note, which is no message of a person.
  world.endWake(
    w.second['id']! as String,
    'expired',
    reason: 'deadline_passed',
  );
  final payload =
      world.log
              .lastWhere((e) => e.envelope['type'] == 'run.wake_changed')
              .envelope['payload']!
          as Map;
  final note = find.byKey(ValueKey('note.${payload['note_message_id']}'));
  await app.waitFor(note);
  expect(
    find.descendant(
      of: note,
      matching: find.text(
        en.noteWakeExpired(en.wakesNoteName(dsIsolate('nightly'))),
      ),
    ),
    findsOneWidget,
  );
  expect(
    find.descendant(of: note, matching: find.byType(DsAvatar)),
    findsNothing,
  );

  // A fired wake-up's run names it at its anchor.
  final third = world.addWake(
    threadId: w.thread,
    agentId: seed.robin.id,
    sourceRunId: w.source['id']! as String,
    triggeredBy: seed.alex.id,
    label: 'deploy',
  );
  final run = world.fireWake(third['id']! as String);
  FakeAgentInterface.of(world).postAnchor(seed.robin.id, run['id']! as String);
  await app.waitFor(find.text(en.runWokeUp(dsIsolate('deploy'))));

  // Cancel ends the first wake-up on the server; its row leaves the list.
  final cancel = find.byKey(ValueKey('wakes.cancel.${w.first['id']}'));
  await app.waitFor(cancel);
  await tester.ensureVisible(cancel);
  await tester.pump();
  await tester.tap(cancel);
  await _until(
    tester,
    'the cancel on the server',
    () => world.wakes[w.first['id']]!['state'] == 'cancelled',
  );
  await _until(tester, 'the row leaves', () => firstRow.evaluate().isEmpty);
}
