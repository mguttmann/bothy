// SPDX-License-Identifier: Apache-2.0

// The member bar of a team channel (design cross-project-handoff.md MVP items 2 and 6): every
// member with its state, the pause line, "Stop all" and the entry to the dialog, with
// accessible labels.

import 'package:client/agents/providers.dart' show threadRunsProvider;
import 'package:client/team/team_header.dart';
import 'package:client/team/team_model.dart';
import 'package:client/team/team_port.dart';
import 'package:design_system/design_system.dart' show DsButton;
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/structure.dart';
import '../support/team_fakes.dart';
import '../support/widgets.dart';

final TeamView twoMembers = team(
  members: [
    member(1, 'coding'),
    member(2, 'qa', agent: nova, homeN: 3, threadN: 3),
  ],
);

Future<List<String>> pumpView(
  WidgetTester tester,
  TeamView t,
  Map<String, TeamMemberState> states,
) async {
  desktopWindow(tester);
  final calls = <String>[];
  await tester.pumpWidget(
    hostScreen(
      Scaffold(
        body: TeamHeaderView(
          team: t,
          states: states,
          onStopAll: () => calls.add('stop'),
          onManage: () => calls.add('manage'),
        ),
      ),
    ),
  );
  return calls;
}

bool canPress(WidgetTester tester, String key) =>
    tester.widget<DsButton>(find.byKey(ValueKey(key))).onPressed != null;

void main() {
  testWidgets('every member with its state, named for screen readers', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    await pumpView(tester, twoMembers, {
      member(1, 'coding').id: TeamMemberState.running,
      member(2, 'qa').id: TeamMemberState.idle,
    });
    expect(
      find.bySemanticsLabel(
        en.teamMemberSemantics('coding', en.teamStateRunning),
      ),
      findsOneWidget,
    );
    expect(
      find.bySemanticsLabel(en.teamMemberSemantics('qa', en.teamStateIdle)),
      findsOneWidget,
    );
    expect(find.bySemanticsLabel(en.teamHeader), findsOneWidget);
    expect(find.byKey(const ValueKey('team.header.pause')), findsNothing);
    semantics.dispose();
  });

  testWidgets('stop all only while a member works; manage opens the dialog', (
    tester,
  ) async {
    var calls = await pumpView(tester, twoMembers, {
      member(1, 'coding').id: TeamMemberState.idle,
      member(2, 'qa').id: TeamMemberState.idle,
    });
    expect(canPress(tester, 'team.header.stopAll'), isFalse);
    await tester.tap(find.byKey(const ValueKey('team.header.manage')));
    expect(calls, ['manage']);

    calls = await pumpView(tester, twoMembers, {
      member(1, 'coding').id: TeamMemberState.running,
    });
    await tester.tap(find.byKey(const ValueKey('team.header.stopAll')));
    expect(calls, ['stop']);
  });

  testWidgets('a paused channel says why and how to go on', (tester) async {
    for (final (pause, text) in [
      (TeamPause.turnBudget, en.teamPausedTurnBudget),
      (TeamPause.pingpong, en.teamPausedPingpong),
      (TeamPause.stopped, en.teamPausedStopped),
      (TeamPause.unknown, en.teamPausedOther),
    ]) {
      await pumpView(
        tester,
        team(members: [member(1, 'coding')], pause: pause),
        {member(1, 'coding').id: TeamMemberState.paused},
      );
      expect(find.text(text), findsOneWidget);
    }
  });

  testWidgets('the bar reads the state from the runs of each work thread', (
    tester,
  ) async {
    desktopWindow(tester);
    final running = RunSummary(
      id: RunId.parse(uuid(0x701)),
      threadId: threadId(3),
      channelId: channelId(3),
      agentId: nova,
      triggeredBy: alex,
      state: RunState.running,
      version: const ResourceVersion(1),
      billingModeExpected: BillingMode.organization,
      cost: const Money(amountMicros: 0, currency: 'USD', isEstimate: true),
      stepCount: 0,
      queuedAt: t0,
    );
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          threadRunsProvider(threadId(2).value)
              .overrideWith((ref) => Stream.value(const [])),
          threadRunsProvider(threadId(3).value)
              .overrideWith((ref) => Stream.value([running])),
        ],
        child: hostScreen(
          Scaffold(
            body: TeamHeaderBar(channel: channel(1), team: twoMembers),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(
      find.text(en.teamMemberSemantics('coding', en.teamStateIdle)),
      findsOneWidget,
    );
    expect(
      find.text(en.teamMemberSemantics('qa', en.teamStateRunning)),
      findsOneWidget,
    );
    expect(canPress(tester, 'team.header.stopAll'), isTrue);
  });
}
