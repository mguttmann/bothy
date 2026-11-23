// SPDX-License-Identifier: Apache-2.0

// The agent page after an outdated change and with a host that hands a new presence stream on
// every rebuild, and the switch of several agents while it runs: a `412 version_conflict`
// reads the agent again and says so, a new stream of the same agent is not listened to again,
// and neither a tap beside the dialog nor Escape closes it while the switch runs.

import 'dart:async';

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart' show ApiProblem;
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

import '../admin/support.dart';

/// A repository whose switch of several agents waits for [release].
class _HeldSwitch extends FakeAdminRepository {
  _HeldSwitch({required super.prompt});

  /// Completed by the test, in its own zone.
  Completer<void> release = Completer<void>();

  @override
  Future<SwitchAllResult> setAllModes(
    AgentBillingMode mode, {
    String? holderId,
    List<String>? agentIds,
    required String reason,
    required String purpose,
  }) async {
    await release.future;
    return super.setAllModes(
      mode,
      holderId: holderId,
      agentIds: agentIds,
      reason: reason,
      purpose: purpose,
    );
  }
}

void main() {
  late GlobalKey<NavigatorState> navigator;
  late _HeldSwitch repository;

  setUp(() {
    navigator = GlobalKey<NavigatorState>();
    repository = _HeldSwitch(
      prompt: StepUpDialogPrompt(() => navigator.currentContext),
    );
    repository.stepUpFresh = true;
  });

  Future<void> tap(WidgetTester tester, String key) async {
    await tester.ensureVisible(find.byKey(ValueKey(key)));
    await tester.tap(find.byKey(ValueKey(key)));
    await tester.pumpAndSettle();
  }

  String text(WidgetTester tester, String key) => [
    for (final t in tester.widgetList<Text>(
      find.descendant(
        of: find.byKey(ValueKey(key)),
        matching: find.byType(Text),
        matchRoot: true,
      ),
    ))
      t.data ?? t.textSpan?.toPlainText() ?? '',
  ].join(' | ');

  testWidgets('a version conflict reads the agent again and says so', (
    tester,
  ) async {
    await pumpInApp(
      tester,
      AgentPage(repository: repository, agentId: 'support', clock: testClock),
      navigatorKey: navigator,
    );
    expect(text(tester, 'agentPage.badge'), 'Personal');
    // Another owner switched the agent after the page loaded it.
    repository.agentRecordsById['support'] = fakeAgentRecord(
      'support',
      name: 'Support',
      mode: AgentBillingMode.organization,
    );
    repository.refuse['setMode'] = const ApiProblem(
      status: 412,
      code: 'version_conflict',
    );
    await tap(tester, 'agentPage.switch');
    await tester.enterText(
      find.byKey(const ValueKey('confirm.reason')),
      'Costs',
    );
    await tester.pump();
    await tap(tester, 'confirm.submit');

    expect(text(tester, 'agentPage.badge'), 'Organization');
    expect(
      text(tester, 'self.notice'),
      'Someone else changed this in the meantime. The page now shows the current state; '
      'make your change again.',
    );
  });

  testWidgets('a new presence stream of the same agent is not listened to '
      'again', (tester) async {
    var listens = 0;
    final source = StreamController<AgentLiveState>.broadcast(
      onListen: () => listens++,
    );
    addTearDown(source.close);
    late StateSetter rebuild;
    await pumpInApp(
      tester,
      StatefulBuilder(
        builder: (context, setState) {
          rebuild = setState;
          return AgentPage(
            repository: repository,
            agentId: 'support',
            // A new stream on every build, as a host that maps a store per call.
            presence: source.stream.where((_) => true),
            clock: testClock,
          );
        },
      ),
      navigatorKey: navigator,
    );
    source.add(const AgentLiveState(state: AgentLiveStateKind.degraded));
    await tester.pumpAndSettle();
    expect(text(tester, 'agentPage.state'), 'Degraded');

    rebuild(() {});
    await tester.pumpAndSettle();
    expect(listens, 1);
    expect(text(tester, 'agentPage.state'), 'Degraded');
  });

  testWidgets('the switch of several agents stays open while it runs', (
    tester,
  ) async {
    final results = <SwitchAllResult?>[];
    repository.release = Completer<void>();
    await pumpInApp(
      tester,
      Builder(
        builder: (context) => TextButton(
          key: const ValueKey('launch'),
          onPressed: () async => results.add(
            await SwitchAllDialog.show(
              context,
              repository: repository,
              agents: repository.agentRecordsById.values.toList(),
            ),
          ),
          child: const Text('launch'),
        ),
      ),
      navigatorKey: navigator,
    );
    await tap(tester, 'launch');
    await tap(tester, 'switchAll.mode.organization');
    await tester.enterText(
      find.byKey(const ValueKey('switchAll.reason')),
      'Costs',
    );
    await tester.pump();
    await tester.tap(find.byKey(const ValueKey('switchAll.submit')));
    await tester.pump();

    await tester.tapAt(const Offset(5, 5));
    await tester.pump();
    await tester.sendKeyEvent(LogicalKeyboardKey.escape);
    await tester.pump();
    expect(find.byType(SwitchAllDialog), findsOneWidget);

    repository.release.complete();
    await tester.pumpAndSettle();
    expect(find.byType(SwitchAllDialog), findsNothing);
    expect(results.single?.changedAgentIds, isNotEmpty);
  });
}
