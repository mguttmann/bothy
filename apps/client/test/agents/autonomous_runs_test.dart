// SPDX-License-Identifier: Apache-2.0

// Autonomous runs in a thread (01 6.21, R-PR-37; brief AR-08) on the desktop apps (macOS,
// Windows) and the mobile apps (iOS, Android): the pending wake-ups above the composer with
// label, condition, earliest fire, deadline and Cancel for the persons of 07 2.14 item 5, live
// through the local table; the end state on the collapsed line of a finished run and when the
// thread continues; "Woke up: <label>" and "Continued after lost work" at wake and recovery
// runs; the notes of 09 5.8.4, 5.8.6, 18.19.10 and 18.19.11 as notes of the thread, translated,
// never as a person's message; and nothing of it for a thread whose agent never reported
// `wakes` (no wake-up exists), while the end state still shows.

import 'package:client/agents/providers.dart';
import 'package:client/agents/wakes/pending_wakes.dart';
import 'package:client/agents/wakes/wake_loader.dart';
import 'package:client/sync/wake_ops.dart';
import 'package:client/thread/history_column.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

import '../support/agent_fakes.dart';
import '../support/column_host.dart';
import '../support/structure.dart';
import '../support/thread_fixtures.dart';
import '../support/thread_host.dart';
import '../support/widgets.dart';

/// Records the threads whose wake-ups the view loads.
final class RecordingWakeLoader implements WakeLoader {
  final List<ThreadId> loaded = [];

  @override
  Future<void> load(ThreadId thread) async => loaded.add(thread);
}

/// The ID of wake-up [n].
WakeId wakeId(int n) => WakeId.parse(uuid(0x900 + n));

/// Wake-up [n] of thread 1 for [person] (Alex), from run [sourceRun].
RunWake wake(
  int n, {
  PrincipalId? person,
  RunWakeKind kind = RunWakeKind.prChecks,
  RunWakeReason reason = RunWakeReason.agent,
  RunWakeState state = RunWakeState.pending,
  String label = 'CI of #123',
  Map<String, Object?>? condition,
  int sourceRun = 1,
  RunId? firedRun,
  int version = 1,
  int notBeforeMinutes = 20,
}) => RunWake(
  id: wakeId(n),
  agentId: robin,
  threadId: threadId(1),
  sourceRunId: runId(sourceRun),
  triggeredBy: person ?? alex,
  kind: kind,
  reason: reason,
  label: label,
  condition:
      condition ??
      (kind == RunWakeKind.prChecks
          ? const {'repository': 'example/app', 'number': 123}
          : const <String, Object?>{}),
  notBefore: t0.add(Duration(minutes: notBeforeMinutes)),
  deadline: t0.add(const Duration(days: 7)),
  state: state,
  resultText: '',
  firedRunId: firedRun,
  createdAt: t0,
  version: ResourceVersion(version),
);

/// The anchor message of run [n] at position [seq].
Message anchor(int seq, int n) => msg(
  seq,
  author: robin,
  text: '',
  kind: MessageKind.run,
  origin: MessageOrigin.agent,
  minutes: seq,
).copyWith(runId: runId(n));

/// A note of the server (origin `system`, kind `text`, authored by the agent, D-AR-04-6).
Message note(int seq, String text) => msg(
  seq,
  author: robin,
  text: text,
  origin: MessageOrigin.system,
  minutes: seq,
);

/// The document of run [n] with [endState] and [wakePendingUntil].
Map<String, Object?> endedDoc(
  int n, {
  String state = 'completed',
  String? reason,
  String? endState,
  DateTime? wakePendingUntil,
  String triggerKind = 'message',
  WakeId? wake,
}) => {
  ...runDoc(n, state: state, reason: reason),
  'trigger_kind': triggerKind,
  'triggering_wake_id': ?wake?.value,
  'metrics': {
    'end_state': ?endState,
    'wake_pending_until': ?wakePendingUntil?.toIso8601String(),
  },
};

final L10nFormat format = L10nFormat(const Locale('en'));
String time(DateTime t) => format.time(t, timeZone: 'UTC');

/// Opens thread 1 with [messages], [runs], [wakes] and [docs] on [platform]; Alex has
/// [capabilities] in the channel.
Future<(ColumnHost, RecordingWakeLoader)> openThread(
  WidgetTester tester, {
  List<Message> messages = const [],
  List<RunSummary> runs = const [],
  List<RunWake> wakes = const [],
  Map<RunId, Map<String, Object?>> docs = const {},
  List<Capability> capabilities = const [],
  String platform = 'macos',
}) async {
  final host = await ColumnHost.open(tester);
  final loader = RecordingWakeLoader();
  final reads = FakeAgentReads()..runs.addAll(docs);
  await host.run(tester, () async {
    await seedThread(
      host.db,
      messages: [
        msg(1, author: alex, text: '@robin watch the checks'),
        ...messages,
      ],
      capabilities: capabilities,
    );
    for (final r in runs) {
      await host.db.runsDao.upsertIfNewer(r);
    }
    for (final w in wakes) {
      await host.db.runsDao.upsertWakeIfNewer(w);
    }
  });
  await host.pump(
    tester,
    HistoryColumn(threadId: threadId(1)),
    location: '/t/${threadId(1).value}',
    platform: platform,
    overrides: [
      ...threadOverrides(FakeThreadReads(host.db)),
      runSourceProvider.overrideWithValue(StoreRunSource()),
      agentReadsProvider.overrideWithValue(reads),
      wakeLoaderProvider.overrideWithValue(loader),
    ],
  );
  return (host, loader);
}

Finder pendingRow(int n) =>
    find.byKey(ValueKey('wakes.pending.${wakeId(n).value}'));
Finder cancelButton(int n) =>
    find.byKey(ValueKey('wakes.cancel.${wakeId(n).value}'));

void main() {
  for (final platform in ['macos', 'windows', 'ios', 'android']) {
    group(platform, () {
      testWidgets('a pending wake-up shows above the composer with label, '
          'condition, earliest fire, deadline and Cancel', (tester) async {
        final (host, loader) = await openThread(
          tester,
          wakes: [wake(1)],
          platform: platform,
        );
        // The view loads the thread's wake-ups when it opens (09 5.8.2).
        expect(loader.loaded, contains(threadId(1)));
        expect(pendingRow(1), findsOneWidget);
        final row = find.descendant(
          of: pendingRow(1),
          matching: find.byType(Text),
        );
        final texts = [
          for (final t in tester.widgetList<Text>(row)) t.data ?? '',
        ];
        expect(texts.join(' '), contains('CI of #123'));
        expect(texts.join(' '), contains(en.wakesConditionPrChecks('123')));
        expect(
          texts.join(' '),
          contains(en.wakesEarliest(time(t0.add(const Duration(minutes: 20))))),
        );
        expect(
          texts.join(' '),
          contains(
            en.wakesDeadline(
              wakeTime(
                format,
                t0.add(const Duration(days: 7)),
                now: t0,
                timeZone: 'UTC',
              ),
            ),
          ),
        );
        // The pending list sits above the composer.
        final list = tester.getRect(
          find.byKey(const ValueKey('wakes.pending')),
        );
        final composer = tester.getRect(find.byKey(const ValueKey('composer')));
        expect(list.bottom, lessThanOrEqualTo(composer.top + 1));
        expect(cancelButton(1), findsOneWidget);
        await host.dispose(tester);
      });

      testWidgets('Cancel puts one cancelRunWake into the outbox', (
        tester,
      ) async {
        final (host, _) = await openThread(
          tester,
          wakes: [wake(1)],
          platform: platform,
        );
        await tester.tap(cancelButton(1));
        await tester.tap(cancelButton(1), warnIfMissed: false);
        await host.settle(tester);
        final all = await host.run(tester, host.db.outboxDao.allEntries);
        final sent = [
          for (final e in all)
            if (e.kind == WakeOps.cancelRunWake) e,
        ];
        expect(sent, hasLength(1));
        expect(sent.single.payload, contains(wakeId(1).value));
        await host.dispose(tester);
      });

      testWidgets('the end state of a finished run shows on its line; a '
          'pending wake-up says when the thread continues', (tester) async {
        final (host, _) = await openThread(
          tester,
          messages: [anchor(2, 1)],
          runs: [run(1, RunState.completed)],
          wakes: [wake(1)],
          docs: {
            runId(1): endedDoc(
              1,
              endState: 'waiting',
              wakePendingUntil: t0.add(const Duration(minutes: 20)),
            ),
          },
          platform: platform,
        );
        final line = find.byKey(ValueKey('run.end.${runId(1).value}'));
        expect(
          find.descendant(
            of: line,
            matching: find.text(en.runEndStateLabel(en.runEndStateWaiting)),
          ),
          findsOneWidget,
        );
        expect(
          find.descendant(
            of: line,
            matching: find.text(
              en.runContinuesAt(
                format.relativeDay(
                  t0.add(const Duration(minutes: 20)),
                  now: t0,
                  timeZone: 'UTC',
                ),
                time(t0.add(const Duration(minutes: 20))),
              ),
            ),
          ),
          findsOneWidget,
        );
        expect(find.text(en.noteEndedWaiting), findsNothing);
        await host.dispose(tester);
      });

      testWidgets('a wake run shows Woke up with its label, a recovery run '
          'Continued after lost work', (tester) async {
        final (host, _) = await openThread(
          tester,
          messages: [anchor(2, 1), anchor(3, 2), anchor(4, 3)],
          runs: [
            run(1, RunState.completed),
            run(2, RunState.completed),
            run(3, RunState.completed),
          ],
          wakes: [
            wake(1, state: RunWakeState.fired, firedRun: runId(2)),
            wake(
              2,
              kind: RunWakeKind.at,
              reason: RunWakeReason.recovery,
              label: '',
              state: RunWakeState.fired,
              sourceRun: 2,
              firedRun: runId(3),
            ),
          ],
          platform: platform,
        );
        expect(
          find.text(en.runWokeUp(dsIsolate('CI of #123'))),
          findsOneWidget,
        );
        expect(find.text(en.runContinuedAfterLostWork), findsOneWidget);
        // The run of a message gets neither.
        expect(
          find.byKey(ValueKey('run.trigger.${runId(1).value}')),
          findsNothing,
        );
        await host.dispose(tester);
      });

      testWidgets('the notes of wake-up ends and caps are translated notes of '
          'the thread, never a message of a person', (tester) async {
        final (host, _) = await openThread(
          tester,
          messages: [
            note(
              2,
              "Wake-up 'CI of #123' expired: its deadline passed before it could fire",
            ),
            note(3, "Wake-up 'CI of #123' cancelled (cancelled_by_person)"),
            note(
              4,
              "Wake-up 'nightly' refused (autonomous_chain_max): a reply of the person continues the work",
            ),
            note(
              5,
              'Autonomous work stopped (autonomous_idle_max): a reply continues it',
            ),
            note(
              6,
              "Wake-up 'nightly' refused: agents may not run on alone in this channel",
            ),
            note(
              7,
              "Wake-up 'tests' failed: its check could not run (wake_check_failed)",
            ),
          ],
          platform: platform,
        );
        final name = en.wakesNoteName(dsIsolate('CI of #123'));
        expect(find.text(en.noteWakeExpired(name)), findsOneWidget);
        expect(find.text(en.noteWakeCancelled(name)), findsOneWidget);
        expect(find.text(en.noteChainCap), findsOneWidget);
        expect(find.text(en.noteIdleCap), findsOneWidget);
        expect(
          find.text(
            en.noteWakeRefusedNotAllowed(
              en.wakesNoteName(dsIsolate('nightly')),
            ),
          ),
          findsOneWidget,
        );
        expect(
          find.text(en.noteWakeFailed(en.wakesNoteName(dsIsolate('tests')))),
          findsOneWidget,
        );
        // Notes are no messages: no author line, no avatar of Robin.
        for (final seq in [2, 3, 4, 5, 6, 7]) {
          final row = find.byKey(ValueKey('note.${messageId(seq).value}'));
          expect(row, findsOneWidget);
          expect(
            find.descendant(of: row, matching: find.byType(DsAvatar)),
            findsNothing,
          );
          expect(
            find.descendant(of: row, matching: find.text('Robin')),
            findsNothing,
          );
        }
        expect(find.byType(DsMessage), findsOneWidget); // Only Alex's message.
        await host.dispose(tester);
      });

      testWidgets('a waiting end without a pending wake-up gets the note, '
          'the usage limit its notice', (tester) async {
        final (host, _) = await openThread(
          tester,
          messages: [anchor(2, 1), anchor(3, 2)],
          runs: [
            run(1, RunState.completed),
            run(2, RunState.failed, reason: 'usage_limited'),
          ],
          wakes: [
            wake(
              1,
              kind: RunWakeKind.at,
              reason: RunWakeReason.usageLimit,
              label: '',
              sourceRun: 2,
              notBeforeMinutes: 90,
            ),
          ],
          docs: {
            runId(1): endedDoc(1, endState: 'waiting'),
            runId(2): endedDoc(2, state: 'failed', reason: 'usage_limited'),
          },
          platform: platform,
        );
        expect(find.text(en.noteEndedWaiting), findsOneWidget);
        final at = time(t0.add(const Duration(minutes: 90)));
        // On the run's line and as the pending wake-up above the composer.
        expect(find.text(en.wakesUsageLimitContinues(at)), findsNWidgets(2));
        await host.dispose(tester);
      });

      testWidgets('without permission the usage limit says to reply after '
          'the reset', (tester) async {
        final (host, _) = await openThread(
          tester,
          messages: [anchor(2, 1), anchor(3, 2)],
          runs: [
            run(1, RunState.failed, reason: 'usage_limited'),
            run(2, RunState.failed, reason: 'usage_limited'),
          ],
          wakes: [
            wake(
              1,
              kind: RunWakeKind.at,
              reason: RunWakeReason.usageLimit,
              label: '',
              state: RunWakeState.refused,
              sourceRun: 1,
              notBeforeMinutes: 90,
            ),
          ],
          docs: {
            runId(1): endedDoc(1, state: 'failed', reason: 'usage_limited'),
            runId(2): endedDoc(2, state: 'failed', reason: 'usage_limited'),
          },
          platform: platform,
        );
        expect(
          find.text(
            en.wakesUsageLimitReplyAfter(
              time(t0.add(const Duration(minutes: 90))),
            ),
          ),
          findsOneWidget,
        );
        expect(find.text(en.wakesUsageLimitReplyLater), findsOneWidget);
        await host.dispose(tester);
      });
    });
  }

  testWidgets('Cancel is only for the wake-up\'s person and manage', (
    tester,
  ) async {
    final (host, _) = await openThread(tester, wakes: [wake(1, person: sam)]);
    expect(pendingRow(1), findsOneWidget);
    expect(cancelButton(1), findsNothing);
    await host.dispose(tester);

    final (managed, _) = await openThread(
      tester,
      wakes: [wake(1, person: sam)],
      capabilities: const [Capability.manage],
    );
    expect(cancelButton(1), findsOneWidget);
    await managed.dispose(tester);
  });

  testWidgets('a change of the local table shows at once (run.wake_changed)', (
    tester,
  ) async {
    final (host, _) = await openThread(tester, wakes: [wake(1)]);
    expect(pendingRow(1), findsOneWidget);
    await host.run(
      tester,
      () => host.db.runsDao.upsertWakeIfNewer(
        wake(1, state: RunWakeState.cancelled, version: 2),
      ),
    );
    await host.settle(tester);
    expect(pendingRow(1), findsNothing);
    expect(find.byKey(const ValueKey('wakes.pending')), findsNothing);
    await host.run(tester, () => host.db.runsDao.upsertWakeIfNewer(wake(2)));
    await host.settle(tester);
    expect(pendingRow(2), findsOneWidget);
    await host.dispose(tester);
  });

  testWidgets('an agent without wakes: no wake-up view, the end state still '
      'shows', (tester) async {
    final (host, _) = await openThread(
      tester,
      messages: [anchor(2, 1)],
      runs: [run(1, RunState.completed)],
      docs: {runId(1): endedDoc(1, endState: 'done')},
    );
    expect(find.byKey(const ValueKey('wakes.pending')), findsNothing);
    expect(find.textContaining('Woke up'), findsNothing);
    expect(find.text(en.runEndStateLabel(en.runEndStateDone)), findsOneWidget);
    await host.dispose(tester);
  });

  testWidgets('a run without an end state (an older runner) shows none', (
    tester,
  ) async {
    final (host, _) = await openThread(
      tester,
      messages: [anchor(2, 1)],
      runs: [run(1, RunState.completed)],
      docs: {runId(1): endedDoc(1)},
    );
    expect(find.byKey(ValueKey('run.end.${runId(1).value}')), findsNothing);
    expect(find.text(en.noteEndedWaiting), findsNothing);
    await host.dispose(tester);
  });

  testWidgets('a note of the thread reads in German', (tester) async {
    final host = await ColumnHost.open(tester);
    await host.run(
      tester,
      () => seedThread(
        host.db,
        messages: [
          note(
            1,
            "Wake-up 'CI of #123' expired: its deadline passed before it could fire",
          ),
        ],
      ),
    );
    await host.pump(
      tester,
      HistoryColumn(threadId: threadId(1)),
      location: '/t/${threadId(1).value}',
      locale: const Locale('de'),
      overrides: [
        ...threadOverrides(FakeThreadReads(host.db)),
        runSourceProvider.overrideWithValue(StoreRunSource()),
        agentReadsProvider.overrideWithValue(FakeAgentReads()),
        wakeLoaderProvider.overrideWithValue(RecordingWakeLoader()),
      ],
    );
    final de = lookupAppLocalizations(const Locale('de'));
    expect(
      find.text(de.noteWakeExpired(de.wakesNoteName(dsIsolate('CI of #123')))),
      findsOneWidget,
    );
    await host.dispose(tester);
  });
}
