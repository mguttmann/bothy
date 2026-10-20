// SPDX-License-Identifier: Apache-2.0

// The agent page of A5 (brief M4-28 Tests, fixed requirements 11 and 12; 01 5.2 A5; 10 8, 13):
// the badges Personal and Organization, the reported mode next to the configured one, every
// warning code of the server mapped to its text and no warning without a server code, no
// reported mode for an adapter that follows the configured mode per run, the operated targets
// and the presence stream of Design note 1.

import 'dart:async';

import 'package:admin_ui/admin_ui.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../admin/support.dart';

void main() {
  late GlobalKey<NavigatorState> navigator;
  late FakeAdminRepository repository;

  setUp(() {
    navigator = GlobalKey<NavigatorState>();
    repository = FakeAdminRepository(
      prompt: StepUpDialogPrompt(() => navigator.currentContext),
    );
  });

  Future<void> page(
    WidgetTester tester,
    String agentId, {
    Stream<AgentLiveState>? presence,
  }) => pumpInApp(
    tester,
    AgentPage(
      repository: repository,
      agentId: agentId,
      presence: presence,
      timeZone: 'Europe/Berlin',
      clock: testClock,
    ),
    navigatorKey: navigator,
  );

  String textOf(WidgetTester tester, String key) => [
    for (final t in tester.widgetList<Text>(
      find.descendant(
        of: find.byKey(ValueKey(key)),
        matching: find.byType(Text),
        matchRoot: true,
      ),
    ))
      t.data ?? t.textSpan?.toPlainText() ?? '',
  ].join(' | ');

  Iterable<String> warningKeys(WidgetTester tester) => [
    for (final w in tester.widgetList(
      find.byWidgetPredicate((w) {
        final key = w.key;
        return key is ValueKey<String> &&
            key.value.startsWith('agentPage.warning.');
      }),
    ))
      (w.key! as ValueKey<String>).value,
  ];

  testWidgets('a personal agent: badge, holder, adapter and runtime', (
    tester,
  ) async {
    await page(tester, 'support');

    expect(find.byKey(const ValueKey('agentPage')), findsOneWidget);
    expect(textOf(tester, 'agentPage.badge'), 'Personal');
    expect(find.text('Support'), findsWidgets);
    expect(find.text('Holder: Alex'), findsOneWidget);
    expect(find.text('Adapter hermes'), findsOneWidget);
    expect(find.text('Runtime version 2.3.1'), findsOneWidget);
    expect(find.textContaining('Last heartbeat'), findsOneWidget);
    expect(textOf(tester, 'agentPage.state'), 'Online');
    expect(textOf(tester, 'agentPage.configured'), 'Personal');
    expect(textOf(tester, 'agentPage.reported'), 'Personal');
    expect(find.text('Can prove: Organization, Personal'), findsOneWidget);
    expect(find.text('Evidence: resolved_credentials'), findsOneWidget);
    expect(warningKeys(tester), isEmpty);
    expect(find.byKey(const ValueKey('agentPage.switch')), findsOneWidget);
    expect(find.byKey(const ValueKey('agentPage.holder')), findsOneWidget);
    expect(find.byType(AgentTokens), findsOneWidget);
  });

  testWidgets('an organization agent: badge without holder or holder change', (
    tester,
  ) async {
    await page(tester, 'texter');

    expect(textOf(tester, 'agentPage.badge'), 'Organization');
    expect(textOf(tester, 'agentPage.configured'), 'Organization');
    expect(find.textContaining('Holder:'), findsNothing);
    expect(find.byKey(const ValueKey('agentPage.switch')), findsOneWidget);
    expect(find.byKey(const ValueKey('agentPage.holder')), findsNothing);
  });

  testWidgets('the reported mode stands next to the configured one, with the '
      "server's mismatch and how to switch the adapter", (tester) async {
    repository.agentRecordsById['texter'] = fakeAgentRecord(
      'texter',
      name: 'Texter',
      reported: AgentBillingMode.personal,
    );
    await page(tester, 'texter');

    expect(textOf(tester, 'agentPage.configured'), 'Organization');
    expect(textOf(tester, 'agentPage.reported'), 'Personal');
    expect(warningKeys(tester), ['agentPage.warning.billing_mode_mismatch']);
    expect(
      textOf(tester, 'agentPage.warning.billing_mode_mismatch'),
      'Adapter reports Personal, Organization is configured | '
      'Invocations fail until the adapter is switched.',
    );
    expect(
      textOf(tester, 'agentPage.switchHint'),
      contains('changes the login of its runtime to Organization'),
    );
  });

  testWidgets('every warning code of the server shows its text', (
    tester,
  ) async {
    repository.agentRecordsById['texter'] = fakeAgentRecord(
      'texter',
      name: 'Texter',
      reported: AgentBillingMode.personal,
      warnings: const [
        'billing_mode_mismatch',
        'billing_mode_unsupported',
        'billing_evidence_unknown',
        'tool_class_mismatch',
        'quota_exceeded_soon',
      ],
    );
    await page(tester, 'texter');

    expect(warningKeys(tester), [
      'agentPage.warning.billing_mode_mismatch',
      'agentPage.warning.billing_mode_unsupported',
      'agentPage.warning.billing_evidence_unknown',
      'agentPage.warning.tool_class_mismatch',
      'agentPage.warning.quota_exceeded_soon',
    ]);
    expect(
      textOf(tester, 'agentPage.warning.billing_mode_unsupported'),
      'Adapter cannot prove this mode | '
      'Invocations fail until the adapter is switched.',
    );
    expect(
      textOf(tester, 'agentPage.warning.billing_evidence_unknown'),
      'Login type of the runtime unknown | All invocations fail.',
    );
    expect(
      textOf(tester, 'agentPage.warning.tool_class_mismatch'),
      'Adapter reports a different tool class',
    );
    expect(
      textOf(tester, 'agentPage.warning.quota_exceeded_soon'),
      'Warning quota_exceeded_soon',
    );
  });

  testWidgets('no warning without a server code, even when the modes differ', (
    tester,
  ) async {
    repository.agentRecordsById['texter'] = fakeAgentRecord(
      'texter',
      name: 'Texter',
      reported: AgentBillingMode.personal,
      keyType: 'unknown',
      warnings: const [],
    );
    await page(tester, 'texter');

    expect(textOf(tester, 'agentPage.configured'), 'Organization');
    expect(textOf(tester, 'agentPage.reported'), 'Personal');
    expect(warningKeys(tester), isEmpty);
    expect(find.byKey(const ValueKey('agentPage.switchHint')), findsNothing);
    expect(find.textContaining('Adapter reports'), findsNothing);
  });

  testWidgets('an adapter that follows the configured mode per run reports '
      'no mode', (tester) async {
    repository.agentRecordsById['texter'] = fakeAgentRecord(
      'texter',
      name: 'Texter',
      supported: null,
    );
    await page(tester, 'texter');

    expect(
      textOf(tester, 'agentPage.reported'),
      'Follows the configured mode per run',
    );
    expect(find.text('Can prove: not reported'), findsOneWidget);
    expect(warningKeys(tester), isEmpty);
  });

  testWidgets('the operated targets, a default space marked', (tester) async {
    await page(tester, 'support');
    expect(
      textOf(tester, 'agentPage.target.marketing'),
      'Space Marketing, by default',
    );

    await page(tester, 'texter');
    expect(
      textOf(tester, 'agentPage.target.campaigns'),
      'Channel Marketing/Kampagnen',
    );

    // Another agent ID, so the page loads again.
    repository.operated['support'] = const [];
    await page(tester, 'support');
    expect(find.text('No space and no channel'), findsOneWidget);
  });

  testWidgets('the presence stream changes the shown state and reported mode; '
      "the warnings stay the server's", (tester) async {
    final presence = StreamController<AgentLiveState>();
    addTearDown(presence.close);
    await page(tester, 'support', presence: presence.stream);
    expect(textOf(tester, 'agentPage.state'), 'Online');
    expect(textOf(tester, 'agentPage.reported'), 'Personal');

    presence.add(
      const AgentLiveState(
        state: AgentLiveStateKind.degraded,
        reportedMode: AgentBillingMode.organization,
      ),
    );
    await tester.pumpAndSettle();
    expect(textOf(tester, 'agentPage.state'), 'Degraded');
    expect(textOf(tester, 'agentPage.reported'), 'Organization');
    expect(warningKeys(tester), isEmpty);

    presence.add(const AgentLiveState(state: AgentLiveStateKind.offline));
    await tester.pumpAndSettle();
    expect(textOf(tester, 'agentPage.state'), 'Offline');
    expect(textOf(tester, 'agentPage.reported'), 'Personal');
  });

  testWidgets('back returns to the list', (tester) async {
    var back = 0;
    await pumpInApp(
      tester,
      AgentPage(
        repository: repository,
        agentId: 'texter',
        clock: testClock,
        onBack: () => back++,
      ),
      navigatorKey: navigator,
    );
    await tester.tap(find.byKey(const ValueKey('agentPage.back')));
    await tester.pumpAndSettle();
    expect(back, 1);
  });
}
