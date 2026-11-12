// SPDX-License-Identifier: Apache-2.0

// What the agent page reads and when (ruling AUD-AA1/3): the agent, its targets and the people
// are read side by side and once each; the names of the I1 explanation are read only when a
// `409 i1_violation` arrives, and then only the agents besides the people the page already
// has; and an answer for an agent the page no longer shows changes nothing.

import 'dart:async';

import 'package:admin_ui/admin_ui.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../admin/support.dart';

/// The fake repository that counts its directory reads and can hold `getAgent` back.
class _Counting extends FakeAdminRepository {
  _Counting({required super.prompt});

  /// How often each read was asked.
  final Map<String, int> reads = {};

  /// While set for an agent, its `getAgent` waits for the completer.
  final Map<String, Completer<void>> held = {};

  void _count(String read) => reads[read] = (reads[read] ?? 0) + 1;

  @override
  Future<AgentRecord> agentRecord(String agentId) async {
    _count('agentRecord');
    await held[agentId]?.future;
    return super.agentRecord(agentId);
  }

  @override
  Future<List<OperatedTarget>> operatedTargets(String agentId) {
    _count('operatedTargets');
    return super.operatedTargets(agentId);
  }

  @override
  Future<AdminPage<AdminPerson>> people({
    String? query,
    AdminPersonStatus? status,
    String? cursor,
  }) {
    _count('people');
    return super.people(query: query, status: status, cursor: cursor);
  }

  @override
  Future<AdminPage<AdminGroup>> groups({String? cursor}) {
    _count('groups');
    return super.groups(cursor: cursor);
  }

  @override
  Future<List<AdminAgent>> agents() {
    _count('agents');
    return super.agents();
  }

  @override
  Future<List<AdminTarget>> targets() {
    _count('targets');
    return super.targets();
  }

  @override
  Future<List<AdminHost>> hosts() {
    _count('hosts');
    return super.hosts();
  }
}

void main() {
  late GlobalKey<NavigatorState> navigator;
  late _Counting repository;

  setUp(() {
    navigator = GlobalKey<NavigatorState>();
    repository = _Counting(
      prompt: StepUpDialogPrompt(() => navigator.currentContext),
    );
  });

  Widget pageOf(String agentId) =>
      AgentPage(repository: repository, agentId: agentId, clock: testClock);

  Future<void> tap(WidgetTester tester, String key) async {
    await tester.ensureVisible(find.byKey(ValueKey(key)).last);
    await tester.tap(find.byKey(ValueKey(key)).last);
    await tester.pumpAndSettle();
  }

  testWidgets('the page reads each list once and no names it may never need', (
    tester,
  ) async {
    await pumpInApp(tester, pageOf('texter'), navigatorKey: navigator);

    expect(find.byKey(const ValueKey('agentPage.badge')), findsOneWidget);
    expect(repository.reads, {
      'agentRecord': 1,
      'operatedTargets': 1,
      'people': 1,
    });
  });

  testWidgets('the targets and the people are asked while the agent is still '
      'on its way', (tester) async {
    final agent = repository.held['texter'] = Completer<void>();
    await pumpInApp(
      tester,
      pageOf('texter'),
      navigatorKey: navigator,
      settle: false,
    );
    await tester.pump();

    expect(find.byType(CircularProgressIndicator), findsOneWidget);
    expect(repository.reads, {
      'agentRecord': 1,
      'operatedTargets': 1,
      'people': 1,
    });

    agent.complete();
    await tester.pumpAndSettle();
    expect(find.text('Channel Marketing/Kampagnen'), findsOneWidget);
  });

  testWidgets('a 409 i1_violation reads the agents, once, and names the '
      'principals', (tester) async {
    repository.i1On['setMode'] = [
      i1Finding('texter', holderId: 'sam', principalIds: ['sam', 'robin']),
    ];
    await pumpInApp(tester, pageOf('texter'), navigatorKey: navigator);
    expect(repository.reads['agents'], isNull);

    await tap(tester, 'agentPage.switch');
    await tap(tester, 'switch.holder');
    await tester.tap(find.text('Sam').last);
    await tester.pumpAndSettle();
    await tester.enterText(
      find.byKey(const ValueKey('confirm.reason')),
      'Sam pays',
    );
    await tester.pump();
    await tap(tester, 'confirm.submit');
    await tap(tester, 'stepUp.continue');

    expect(find.byKey(const ValueKey('i1.explanation')), findsOneWidget);
    expect(find.textContaining('Robin'), findsWidgets);
    expect(repository.reads['agents'], 1);
    expect(repository.reads['groups'], isNull);
    expect(repository.reads['targets'], isNull);
    expect(repository.reads['hosts'], isNull);
    expect(repository.reads['people'], 1);
  });

  testWidgets('an answer for an agent the page no longer shows changes '
      'nothing', (tester) async {
    final support = repository.held['support'] = Completer<void>();
    await pumpInApp(
      tester,
      pageOf('support'),
      navigatorKey: navigator,
      settle: false,
    );
    await tester.pump();

    // The host shows another agent before the first answer came.
    await pumpInApp(tester, pageOf('texter'), navigatorKey: navigator);
    expect(find.text('Texter'), findsWidgets);
    expect(find.text('Channel Marketing/Kampagnen'), findsOneWidget);

    support.complete();
    await tester.pumpAndSettle();
    expect(find.text('Texter'), findsWidgets);
    expect(find.text('Support'), findsNothing);
    expect(find.text('Channel Marketing/Kampagnen'), findsOneWidget);
  });
}
