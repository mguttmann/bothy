// SPDX-License-Identifier: Apache-2.0

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

import 'support.dart';

const _interval = Duration(seconds: 10);

List<DsStepData> _steps(int n) => [
  for (var i = 0; i < n; i++)
    DsStepData(
      id: 's$i',
      kind: DsStepKind.toolCall,
      name: 'tool$i',
      status: DsStepStatus.completed,
      duration: const Duration(seconds: 2),
    ),
];

DsRunData _run(
  DsRunState state, {
  String? reason,
  List<DsStepData> steps = const [],
  Duration? stalledFor,
  String? waitingFor,
  DsCost? cost,
}) => DsRunData(
  id: 'r1',
  agentName: 'Robin',
  state: state,
  stateReason: reason,
  cancelledByName: 'Sam',
  waitingForName: waitingFor,
  elapsed: const Duration(minutes: 4, seconds: 12),
  contextPercent: 62,
  tokens: 48100,
  cost: cost,
  billingMode: DsBillingMode.organization,
  statusLine: 'Runs `make check`',
  lastTextExcerpt: 'The suite is green.',
  steps: steps,
  toolCount: 3,
  subagentCount: 2,
  stalledFor: stalledFor,
);

void main() {
  testWidgets('each state has its spine color, icon and word', (tester) async {
    final l = await en();
    final colors = DsColors.light;
    final accent = DsAccent.of(dsDefaultAccent, Brightness.light);
    final table = {
      DsRunState.queued: (colors.runQueued, DsIcons.runQueued, l.liveBoxStateQueued),
      DsRunState.running: (accent.runRunning, DsIcons.runRunning, l.liveBoxStateRunning),
      DsRunState.waitingInput: (
        colors.runWaiting,
        DsIcons.runWaiting,
        l.liveBoxStateWaitingForYou,
      ),
      DsRunState.completed: (
        colors.runCompleted,
        DsIcons.runCompleted,
        l.liveBoxStateCompleted,
      ),
      DsRunState.failed: (colors.runFailed, DsIcons.runFailed, l.liveBoxStateFailed),
      DsRunState.cancelled: (
        colors.runCancelled,
        DsIcons.runCancelled,
        l.liveBoxStateCancelled,
      ),
    };
    for (final MapEntry(key: state, value: (color, glyph, word)) in table.entries) {
      await pumpDs(tester, DsLiveBox(_run(state), visibleSteps: 6, announceMinInterval: _interval));
      final box = tester.state<DsLiveBoxState>(find.byType(DsLiveBox));
      expect(box.spineColor, color, reason: state.name);
      expect(
        tester.widgetList<DsIcon>(find.byType(DsIcon)).first.glyph,
        glyph,
        reason: state.name,
      );
      expect(find.text(word), findsOneWidget, reason: state.name);
    }
  });

  testWidgets('waiting for another person names them', (tester) async {
    final l = await en();
    await pumpDs(
      tester,
      DsLiveBox(_run(DsRunState.waitingInput, waitingFor: 'Alex'), visibleSteps: 6, announceMinInterval: _interval),
    );
    expect(find.text(l.liveBoxStateWaitingFor('Alex')), findsOneWidget);
  });

  testWidgets('reasons are translated, an unknown one shows its code', (
    tester,
  ) async {
    final l = await en();
    await pumpDs(
      tester,
      DsLiveBox(_run(DsRunState.failed, reason: 'billing_mode_mismatch'), visibleSteps: 6, announceMinInterval: _interval),
    );
    expect(find.text(l.liveBoxReasonBillingModeMismatch), findsOneWidget);
    expect(find.text('billing_mode_mismatch'), findsNothing);

    await pumpDs(
      tester,
      DsLiveBox(_run(DsRunState.cancelled, reason: 'user_cancelled'), visibleSteps: 6, announceMinInterval: _interval),
    );
    expect(find.text(l.liveBoxReasonUserCancelled('Sam')), findsOneWidget);

    await pumpDs(
      tester,
      DsLiveBox(_run(DsRunState.failed, reason: 'future_reason'), visibleSteps: 6, announceMinInterval: _interval),
    );
    expect(find.text(l.liveBoxStateFailed), findsOneWidget);
    final code = tester.widget<Text>(find.text('future_reason'));
    expect(code.style?.fontFamily, DsType.codeSm.fontFamily);
    expect(code.style?.fontSize, DsType.codeSm.fontSize);
  });

  testWidgets('at most visibleSteps chips plus the earlier steps link', (
    tester,
  ) async {
    final l = await en();
    await pumpDs(
      tester,
      DsLiveBox(_run(DsRunState.running, steps: _steps(9)), visibleSteps: 6, announceMinInterval: _interval),
    );
    expect(find.byType(DsToolChip), findsNWidgets(6));
    expect(find.text(l.liveBoxEarlierSteps(3)), findsOneWidget);
    expect(find.text('tool8'), findsOneWidget);
    expect(find.text('tool0'), findsNothing);
    await tester.tap(find.text(l.liveBoxEarlierSteps(3)));
    await tester.pump();
    expect(find.byType(DsToolChip), findsNWidgets(9));
  });

  testWidgets('numbers are in type.code-sm; header, status line, last text, stop', (
    tester,
  ) async {
    final l = await en();
    var stops = 0;
    await pumpDs(
      tester,
      DsLiveBox(
        _run(DsRunState.running),
        visibleSteps: 6,
        announceMinInterval: _interval,
        onStop: () => stops++,
      ),
    );
    final elapsed = tester.widget<Text>(find.text('4:12'));
    expect(elapsed.style?.fontFamily, DsType.codeSm.fontFamily);
    expect(find.text(l.liveBoxTokens('48,100')), findsOneWidget);
    expect(find.text(l.liveBoxContext('62%')), findsOneWidget);
    expect(find.byType(DsStatusLine), findsOneWidget);
    expect(find.text('The suite is green.'), findsOneWidget);
    expect(find.byType(DsBillingBadge), findsOneWidget);
    expect(find.text(l.liveBoxFollowupHint), findsOneWidget);
    await tester.tap(find.bySemanticsLabel(l.liveBoxStopRun('Robin')));
    expect(stops, 1);
    // The container label leads; the plain texts inside follow it.
    expect(
      find.bySemanticsLabel(
        RegExp(
          '^${RegExp.escape(l.liveBoxSemanticsLabel('Robin', l.liveBoxStateRunning, '4:12', 62))}',
        ),
      ),
      findsOneWidget,
    );
  });

  testWidgets('a waiting run shows the question instead of the follow-up note', (
    tester,
  ) async {
    final l = await en();
    await pumpDs(
      tester,
      DsLiveBox(
        _run(DsRunState.waitingInput),
        visibleSteps: 6,
        announceMinInterval: _interval,
        question: const Text('question'),
      ),
    );
    expect(find.text('question'), findsOneWidget);
    expect(find.text(l.liveBoxFollowupHint), findsNothing);
  });

  testWidgets('stall banner', (tester) async {
    final l = await en();
    await pumpDs(
      tester,
      DsLiveBox(
        _run(DsRunState.running, stalledFor: const Duration(minutes: 4, seconds: 30)),
        visibleSteps: 6,
        announceMinInterval: _interval,
      ),
    );
    expect(find.byType(DsBanner), findsOneWidget);
    expect(find.text(l.liveBoxStall(4)), findsOneWidget);
  });

  testWidgets('collapsed line with Estimate', (tester) async {
    final l = await en();
    var expands = 0;
    await pumpDs(
      tester,
      DsLiveBox.collapsed(
        _run(
          DsRunState.completed,
          cost: const DsCost(
            DsMoney(1.2, 'USD'),
            mode: DsBillingMode.organization,
            isEstimate: true,
          ),
        ).copyWithElapsed(const Duration(minutes: 28)),
        onExpand: () => expands++,
      ),
    );
    final text = allText(tester);
    expect(
      text,
      contains(l.liveBoxRan(const L10nFormat(Locale('en')).durationShort(const Duration(minutes: 28)))),
    );
    expect(text, contains(l.liveBoxTools(3)));
    expect(text, contains(l.liveBoxSubagents(2)));
    expect(text, contains(l.costEstimate));
    await tester.tap(find.byType(DsLiveBox));
    expect(expands, 1);
  });

  testWidgets('mobile line: state, time, status line, stop with target size', (
    tester,
  ) async {
    final l = await en();
    var opens = 0;
    var stops = 0;
    await pumpDs(
      tester,
      DsLiveBox.mobile(
        _run(DsRunState.running),
        onOpen: () => opens++,
        onStop: () => stops++,
      ),
      density: DsDensityMode.touch,
    );
    expect(find.text(l.liveBoxStateRunning), findsOneWidget);
    expect(find.text('4:12'), findsOneWidget);
    expect(find.byType(DsStatusLine), findsOneWidget);
    final stop = find.bySemanticsLabel(l.liveBoxStopRun('Robin'));
    expect(tester.getSize(stop).height, greaterThanOrEqualTo(DsDensity.touch.targetMin));
    await tester.tap(stop);
    expect(stops, 1);
    await tester.tap(find.text(l.liveBoxStateRunning));
    expect(opens, 1);
  });

  testWidgets('tool chip: status suffixes, label and detail disclosure', (
    tester,
  ) async {
    final l = await en();
    final twelve = const L10nFormat(Locale('en')).durationShort(const Duration(seconds: 12));
    final cases = {
      DsStepStatus.pending: l.toolChipWaiting,
      DsStepStatus.running: l.toolChipRunning(twelve),
      DsStepStatus.completed: twelve,
      DsStepStatus.failed: l.toolChipFailed,
      DsStepStatus.cancelled: l.toolChipCancelled,
    };
    for (final MapEntry(key: status, value: suffix) in cases.entries) {
      await pumpDs(
        tester,
        DsToolChip(
          DsStepData(
            id: 's',
            kind: DsStepKind.toolCall,
            name: 'grep',
            title: 'Search the tree',
            status: status,
            duration: const Duration(seconds: 12),
            detail: 'stdout: 3 matches',
          ),
        ),
      );
      expect(find.text(suffix), findsOneWidget, reason: status.name);
    }
    expect(
      find.bySemanticsLabel(
        RegExp('^${RegExp.escape(l.toolChipLabel(l.toolChipKindToolCall, 'grep', ''))}'),
      ),
      findsOneWidget,
    );
    expect(find.text('stdout: 3 matches'), findsNothing);
    await tester.tap(find.byType(DsToolChip));
    await tester.pump();
    expect(find.text('stdout: 3 matches'), findsOneWidget);
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
  });

  group('LiveAnnouncer', () {
    late DateTime now;
    late List<String> sent;
    late LiveAnnouncer announcer;

    setUp(() {
      now = DateTime.utc(2026, 10, 15, 12);
      sent = [];
      announcer = LiveAnnouncer(
        minInterval: const Duration(seconds: 10),
        clock: () => now,
        sink: sent.add,
      );
    });

    test('the first observation announces nothing', () {
      announcer.observe(state: 'Running', questionId: null);
      expect(sent, isEmpty);
    });

    test('a state change announces once', () {
      announcer.observe(state: 'Running', questionId: null);
      now = now.add(const Duration(seconds: 30));
      announcer.observe(state: 'Done', questionId: null);
      expect(sent, ['Done']);
    });

    test('a patch with the same state announces nothing', () {
      announcer.observe(state: 'Running', questionId: null);
      for (var i = 0; i < 20; i++) {
        now = now.add(const Duration(seconds: 1));
        announcer.observe(state: 'Running', questionId: null);
      }
      announcer.flush();
      expect(sent, isEmpty);
    });

    test('a new question announces', () {
      announcer.observe(state: 'Waiting', questionId: null, question: 'Q');
      now = now.add(const Duration(seconds: 30));
      announcer.observe(state: 'Waiting', questionId: 'q1', question: 'Question from Robin');
      expect(sent, ['Question from Robin']);
    });

    test('two changes within the interval give one announcement, the later', () {
      announcer.observe(state: 'Queued', questionId: null);
      now = now.add(const Duration(seconds: 30));
      announcer.observe(state: 'Running', questionId: null);
      expect(sent, ['Running']);
      now = now.add(const Duration(seconds: 2));
      announcer.observe(state: 'Waiting', questionId: null);
      now = now.add(const Duration(seconds: 2));
      announcer.observe(state: 'Done', questionId: null);
      expect(sent, ['Running']);
      expect(announcer.pendingDue, DateTime.utc(2026, 10, 15, 12, 0, 40));
      now = now.add(const Duration(seconds: 6));
      announcer.flush();
      expect(sent, ['Running', 'Done']);
      announcer.flush();
      expect(sent, ['Running', 'Done']);
    });

    test('a change back to the announced state cancels the pending one', () {
      announcer.observe(state: 'Running', questionId: null);
      now = now.add(const Duration(seconds: 30));
      announcer.observe(state: 'Waiting', questionId: null);
      now = now.add(const Duration(seconds: 1));
      announcer.observe(state: 'Done', questionId: null);
      now = now.add(const Duration(seconds: 1));
      announcer.observe(state: 'Waiting', questionId: null);
      now = now.add(const Duration(seconds: 10));
      announcer.flush();
      expect(sent, ['Waiting']);
    });
  });

  testWidgets('the live box announces a state change, never a patch', (
    tester,
  ) async {
    final l = await en();
    final announced = <String>[];
    tester.binding.defaultBinaryMessenger.setMockDecodedMessageHandler<Object?>(
      SystemChannels.accessibility,
      (message) async {
        final map = message! as Map<Object?, Object?>;
        if (map['type'] == 'announce') {
          announced.add((map['data']! as Map<Object?, Object?>)['message']! as String);
        }
        return null;
      },
    );
    Future<void> show(DsRunData run) => pumpDs(
      tester,
      DsLiveBox(run, visibleSteps: 6, announceMinInterval: _interval),
    );
    await show(_run(DsRunState.running, steps: _steps(1)));
    await show(_run(DsRunState.running, steps: _steps(2)));
    await tester.pump(const Duration(seconds: 20));
    expect(announced, isEmpty);
    await show(_run(DsRunState.completed, steps: _steps(2)));
    await tester.pump();
    expect(announced, [l.liveBoxAnnounceState('Robin', l.liveBoxStateCompleted)]);
  });
}

extension on DsRunData {
  DsRunData copyWithElapsed(Duration elapsed) => DsRunData(
    id: id,
    agentName: agentName,
    state: state,
    elapsed: elapsed,
    cost: cost,
    toolCount: toolCount,
    subagentCount: subagentCount,
  );
}
