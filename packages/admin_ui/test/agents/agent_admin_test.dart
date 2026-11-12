// SPDX-License-Identifier: Apache-2.0

// The administration of an adapter agent on A5 (rulings AUD-AA1/5 to AUD-AA1/8; 11 13 step 1;
// 09 2.1, 2.2; 07 2.11): "Add adapter agent" asks a name, a handle and one of the three
// adapter kinds, goes through the step-up and opens the new agent; the agent page shows the
// tool class and the features of an adapter agent and changes them with a reason or a summary,
// disables with the effects said before and enables again, where `409 i1_violation` is
// explained; a `403` hides every owner control; a built-in agent keeps its own section and
// gains only "Enable".

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart' show ApiProblem;
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../admin/support.dart';
import 'new_agent/fake_new_agent_port.dart';

void main() {
  late GlobalKey<NavigatorState> navigator;
  late FakeAdminRepository repository;
  late List<String> opened;
  late int changed;

  setUp(() {
    navigator = GlobalKey<NavigatorState>();
    repository = FakeAdminRepository(
      prompt: StepUpDialogPrompt(() => navigator.currentContext),
    );
    opened = [];
    changed = 0;
  });

  Future<void> list(WidgetTester tester) => pumpInApp(
    tester,
    AgentsList(
      repository: repository,
      onOpen: opened.add,
      onChanged: () => changed++,
    ),
    navigatorKey: navigator,
  );

  Future<void> page(
    WidgetTester tester,
    String agentId, {
    NewAgentHost? builtin,
  }) => pumpInApp(
    tester,
    AgentPage(
      repository: repository,
      agentId: agentId,
      clock: testClock,
      builtin: builtin,
      onChanged: () => changed++,
    ),
    navigatorKey: navigator,
  );

  Future<void> tap(WidgetTester tester, String key) async {
    await tester.ensureVisible(find.byKey(ValueKey(key)).last);
    await tester.tap(find.byKey(ValueKey(key)).last);
    await tester.pumpAndSettle();
  }

  Future<void> type(WidgetTester tester, String key, String text) async {
    await tester.enterText(find.byKey(ValueKey(key)), text);
    await tester.pumpAndSettle();
  }

  Future<void> pick(WidgetTester tester, String name) async {
    await tester.tap(find.text(name).last);
    await tester.pumpAndSettle();
  }

  Future<void> stepUp(WidgetTester tester, String purpose) async {
    expect(find.byKey(const ValueKey('stepUp.dialog')), findsOneWidget);
    expect(find.text(purpose), findsOneWidget);
    await tap(tester, 'stepUp.continue');
    repository.stepUpFresh = false;
  }

  bool canConfirm(WidgetTester tester) =>
      tester
          .widget<FilledButton>(find.byKey(const ValueKey('confirm.submit')))
          .onPressed !=
      null;

  String textOf(WidgetTester tester, String key) =>
      tester.widget<Text>(find.byKey(ValueKey(key))).data ?? '';

  const forbidden = ApiProblem(status: 403, code: 'forbidden');

  group('"Add adapter agent"', () {
    testWidgets('asks a name, a handle of the allowed form and an adapter', (
      tester,
    ) async {
      await list(tester);
      await tap(tester, 'agents.add');

      expect(find.text('Add adapter agent'), findsWidgets);
      expect(canConfirm(tester), isFalse);
      await type(tester, 'addAgent.name', 'Dev runner');
      await type(tester, 'addAgent.handle', 'Dev Runner');
      expect(
        find.text(
          'Use lower-case letters, digits, dot, underscore and hyphen, '
          'starting with a letter or digit.',
        ),
        findsOneWidget,
      );
      await type(tester, 'addAgent.handle', 'dev-runner');
      // Without an adapter kind nothing is created.
      expect(canConfirm(tester), isFalse);

      await tap(tester, 'addAgent.kind');
      // A built-in agent is created by "New agent", never here.
      expect(
        [
          for (final kind in adapterKinds)
            find.byKey(ValueKey('addAgent.kind.$kind')).evaluate().isNotEmpty,
        ],
        [true, true, true],
      );
      expect(find.text('Hermes'), findsWidgets);
      expect(find.text('Own adapter'), findsWidgets);
      await pick(tester, 'Claude Code runner');
      expect(canConfirm(tester), isTrue);
    });

    testWidgets('creates the agent after the step-up and opens it', (
      tester,
    ) async {
      await list(tester);
      await tap(tester, 'agents.add');
      await type(tester, 'addAgent.name', 'Lotse');
      await type(tester, 'addAgent.handle', 'lotse');
      await tap(tester, 'addAgent.kind');
      await pick(tester, 'Own adapter');
      await type(tester, 'addAgent.description', 'Answers support tickets');
      await tap(tester, 'confirm.submit');
      await stepUp(tester, 'Create the agent Lotse');

      expect(repository.calls, [
        'createAgent lotse Lotse custom Answers support tickets',
      ]);
      final created = repository.agentRecordsById['lotse']!;
      expect(created.mode, AgentBillingMode.organization);
      expect(created.toolClass, 'chat');
      expect(find.byKey(const ValueKey('agent.lotse')), findsOneWidget);
      expect(opened, ['lotse']);
      expect(changed, 1);
    });

    testWidgets('a cancelled dialog creates nothing', (tester) async {
      await list(tester);
      await tap(tester, 'agents.add');
      await tap(tester, 'confirm.cancel');
      expect(repository.calls, isEmpty);
      expect(opened, isEmpty);
    });

    testWidgets('a taken handle is said in words', (tester) async {
      await list(tester);
      await tap(tester, 'agents.add');
      await type(tester, 'addAgent.name', 'Texter two');
      await type(tester, 'addAgent.handle', 'texter');
      await tap(tester, 'addAgent.kind');
      await pick(tester, 'Hermes');
      await tap(tester, 'confirm.submit');
      await stepUp(tester, 'Create the agent Texter two');

      expect(
        find.text('This handle is taken. Change it and try again.'),
        findsOneWidget,
      );
      expect(opened, isEmpty);
      expect(changed, 0);
      // The button stays: the person tries another handle.
      expect(find.byKey(const ValueKey('agents.add')), findsOneWidget);
    });

    testWidgets('a refused creation hides the button', (tester) async {
      repository.refuse['createAgent'] = forbidden;
      await list(tester);
      await tap(tester, 'agents.add');
      await type(tester, 'addAgent.name', 'Lotse');
      await type(tester, 'addAgent.handle', 'lotse');
      await tap(tester, 'addAgent.kind');
      await pick(tester, 'Own adapter');
      await tap(tester, 'confirm.submit');

      expect(find.byKey(const ValueKey('agents.add')), findsNothing);
      expect(find.byKey(const ValueKey('self.notice')), findsOneWidget);
      expect(opened, isEmpty);
    });
  });

  group('the tool class of an adapter agent', () {
    testWidgets(
      'shows with what the runtime may do and changes with a reason',
      (tester) async {
        await page(tester, 'texter');
        expect(textOf(tester, 'agentPage.toolClass'), 'Chat');
        expect(
          find.text(
            'Replies and reads the thread. Nothing it does reaches beyond the '
            'thread.',
          ),
          findsOneWidget,
        );

        await tap(tester, 'agentPage.toolClassChange');
        expect(find.text('Tool class of Texter'), findsOneWidget);
        // The class the agent has is no change, and a change needs its reason.
        await type(tester, 'confirm.reason', 'Runs a coding agent');
        expect(canConfirm(tester), isFalse);
        await tap(tester, 'toolClass.host');
        expect(canConfirm(tester), isTrue);
        await type(tester, 'confirm.reason', '  ');
        expect(canConfirm(tester), isFalse);
        await type(tester, 'confirm.reason', 'Runs a coding agent');
        await tap(tester, 'confirm.submit');
        await stepUp(tester, 'Set the tool class of Texter');

        expect(repository.calls, [
          'setToolClass texter host Runs a coding agent',
        ]);
        expect(textOf(tester, 'agentPage.toolClass'), 'Host');
        expect(
          find.text('Texter now has the tool class Host.'),
          findsOneWidget,
        );
        expect(find.textContaining('Only owners and people'), findsOneWidget);
        expect(changed, 1);
      },
    );

    testWidgets('a change of another device is refused and changes nothing', (
      tester,
    ) async {
      await page(tester, 'texter');
      // Another owner changed the agent since the page loaded.
      repository.agentRecordsById['texter'] = fakeAgentRecord(
        'texter',
        name: 'Texter',
        version: 2,
      );
      await tap(tester, 'agentPage.toolClassChange');
      await tap(tester, 'toolClass.scoped');
      await type(tester, 'confirm.reason', 'Narrow tools');
      await tap(tester, 'confirm.submit');
      await stepUp(tester, 'Set the tool class of Texter');

      expect(
        find.text('Someone else changed this in the meantime'),
        findsOneWidget,
      );
      expect(repository.agentRecordsById['texter']!.toolClass, 'chat');
      expect(changed, 0);
    });
  });

  group('the features of an adapter agent', () {
    testWidgets('are all allowed until one is switched off', (tester) async {
      await page(tester, 'texter');
      expect(
        textOf(tester, 'agentPage.features'),
        'Everything the adapter reports is allowed.',
      );
      expect(
        find.byKey(const ValueKey('agentPage.defaultModel')),
        findsNothing,
      );

      await tap(tester, 'agentPage.featuresEdit');
      expect(find.text('Features of Texter'), findsOneWidget);
      for (final key in agentFeatureKeys) {
        expect(
          tester
              .widget<SwitchListTile>(find.byKey(ValueKey('features.$key')))
              .value,
          isTrue,
          reason: key,
        );
      }
      await tap(tester, 'features.supports_files_out');
      await tap(tester, 'features.supports_followups');
      await type(tester, 'features.defaultModel', 'provider/model-large@1');
      await tap(tester, 'confirm.submit');
      await stepUp(tester, 'Change the features of Texter');

      expect(repository.calls, [
        'setOperatorConfig texter default_model=provider/model-large@1,'
            'supports_files_out=false,supports_followups=false',
      ]);
      expect(
        textOf(tester, 'agentPage.features'),
        'Switched off: Messages as a follow-up, Sending files',
      );
      expect(
        textOf(tester, 'agentPage.defaultModel'),
        'Default model: provider/model-large@1',
      );
      expect(find.text('The features of Texter are saved.'), findsOneWidget);
    });

    testWidgets('a stored configuration fills the dialog; allowing again and '
        'an empty model drop their keys', (tester) async {
      repository.agentRecordsById['texter'] = fakeAgentRecord(
        'texter',
        name: 'Texter',
        operatorConfig: const {
          'default_model': 'model-medium',
          'supports_voice': false,
          'supports_title': true,
        },
      );
      await page(tester, 'texter');
      expect(textOf(tester, 'agentPage.features'), 'Switched off: Voice input');

      await tap(tester, 'agentPage.featuresEdit');
      expect(
        tester
            .widget<SwitchListTile>(
              find.byKey(const ValueKey('features.supports_voice')),
            )
            .value,
        isFalse,
      );
      expect(
        tester
            .widget<TextField>(
              find.byKey(const ValueKey('features.defaultModel')),
            )
            .controller!
            .text,
        'model-medium',
      );
      await tap(tester, 'features.supports_voice');
      await type(tester, 'features.defaultModel', '');
      await tap(tester, 'confirm.submit');
      await stepUp(tester, 'Change the features of Texter');

      // An empty object: everything the adapter reports is allowed, and it picks the model.
      expect(repository.calls, ['setOperatorConfig texter']);
      expect(repository.agentRecordsById['texter']!.operatorConfig, isEmpty);
    });

    testWidgets(
      'a model with a character outside the pattern cannot be saved',
      (tester) async {
        await page(tester, 'texter');
        await tap(tester, 'agentPage.featuresEdit');
        await type(tester, 'features.defaultModel', 'model large');
        expect(
          find.text('Use letters, digits and the characters . _ : / @ - only.'),
          findsOneWidget,
        );
        expect(canConfirm(tester), isFalse);
      },
    );

    test('a member this build does not know stays in the saved object', () {
      // The dialog replaces only the members of 09 2.2.
      expect(
        agentFeaturesOff(const {'supports_voice': false, 'supports_x': false}),
        ['supports_voice'],
      );
    });
  });

  group('"Disable" and "Enable"', () {
    testWidgets('disabling says what ends, revokes the tokens and offers '
        '"Enable"', (tester) async {
      repository.tokensById['texter'] = [
        AgentToken(
          id: 'token-a',
          name: 'Adapter production',
          prefix: 'bothy_agt_EXAMPL',
          createdAt: DateTime.utc(2026, 10),
          dpopBound: true,
        ),
      ];
      await page(tester, 'texter');
      expect(textOf(tester, 'agentPage.status'), 'Active');
      expect(find.byKey(const ValueKey('agentPage.enable')), findsNothing);
      expect(
        find.byKey(const ValueKey('token.token-a.revoke')),
        findsOneWidget,
      );

      await tap(tester, 'agentPage.disable');
      expect(find.text('Disable Texter?'), findsOneWidget);
      expect(
        find.textContaining('its tokens are revoked, its connections close'),
        findsOneWidget,
      );
      await type(tester, 'confirm.reason', 'Adapter is replaced');
      await tap(tester, 'confirm.submit');
      await stepUp(tester, 'Disable the agent Texter');

      expect(repository.calls, [
        'setActive texter disable Adapter is replaced',
      ]);
      expect(textOf(tester, 'agentPage.status'), 'This agent is disabled.');
      expect(find.text('Texter is disabled.'), findsOneWidget);
      expect(find.byKey(const ValueKey('agentPage.disable')), findsNothing);
      expect(find.byKey(const ValueKey('agentPage.enable')), findsOneWidget);
      expect(repository.tokensById['texter']!.single.revokedAt, isNotNull);
      // The token list of the page is read again: the token shows as revoked.
      expect(
        find.descendant(
          of: find.byKey(const ValueKey('token.token-a')),
          matching: find.text('Revoked'),
        ),
        findsOneWidget,
      );
      expect(find.byKey(const ValueKey('token.token-a.revoke')), findsNothing);
      expect(changed, 1);
    });

    testWidgets('without a typed reason the default reason is sent', (
      tester,
    ) async {
      await page(tester, 'texter');
      await tap(tester, 'agentPage.disable');
      await tap(tester, 'confirm.submit');
      await stepUp(tester, 'Disable the agent Texter');
      expect(repository.calls, [
        'setActive texter disable Changed in the administration',
      ]);
    });

    testWidgets('enabling brings no token back and says so before', (
      tester,
    ) async {
      repository.agentRecordsById['texter'] = fakeAgentRecord(
        'texter',
        name: 'Texter',
        active: false,
      );
      await page(tester, 'texter');
      await tap(tester, 'agentPage.enable');
      expect(find.text('Enable Texter?'), findsOneWidget);
      expect(
        find.textContaining('Its earlier tokens stay revoked'),
        findsOneWidget,
      );
      await type(tester, 'confirm.reason', 'New adapter');
      await tap(tester, 'confirm.submit');
      await stepUp(tester, 'Enable the agent Texter');

      expect(repository.calls, ['setActive texter enable New adapter']);
      expect(textOf(tester, 'agentPage.status'), 'Active');
      expect(find.text('Texter is enabled.'), findsOneWidget);
    });

    testWidgets('an enable that breaks I1 is explained and changes nothing', (
      tester,
    ) async {
      repository.agentRecordsById['support'] = fakeAgentRecord(
        'support',
        name: 'Support',
        mode: AgentBillingMode.personal,
        holderId: 'alex',
        active: false,
      );
      repository.i1On['setActive'] = [
        i1Finding('support', holderId: 'alex', principalIds: ['alex', 'sam']),
      ];
      await page(tester, 'support');
      await tap(tester, 'agentPage.enable');
      await tap(tester, 'confirm.submit');
      await stepUp(tester, 'Enable the agent Support');

      expect(find.byKey(const ValueKey('i1.explanation')), findsOneWidget);
      expect(repository.agentRecordsById['support']!.active, isFalse);
      expect(find.byKey(const ValueKey('agentPage.enable')), findsOneWidget);
      expect(changed, 0);
    });
  });

  testWidgets('a refused owner action hides every owner control of the page', (
    tester,
  ) async {
    repository.refuse['setToolClass'] = forbidden;
    await page(tester, 'texter');
    await tap(tester, 'agentPage.toolClassChange');
    await tap(tester, 'toolClass.scoped');
    await type(tester, 'confirm.reason', 'Try');
    await tap(tester, 'confirm.submit');

    for (final key in [
      'agentPage.switch',
      'agentPage.toolClassChange',
      'agentPage.featuresEdit',
      'agentPage.disable',
    ]) {
      expect(find.byKey(ValueKey(key)), findsNothing, reason: key);
    }
    // What the server showed stays.
    expect(textOf(tester, 'agentPage.toolClass'), 'Chat');
    expect(textOf(tester, 'agentPage.status'), 'Active');
  });

  group('a built-in agent', () {
    AgentRecord builtin({bool active = true}) => AgentRecord(
      id: 'a',
      handle: 'notes',
      displayName: 'Notes',
      active: active,
      adapterKind: builtinAdapterKind,
      description: '',
      mode: AgentBillingMode.organization,
      toolClass: 'chat',
      runtime: const AgentRuntimeInfo(state: AgentLiveStateKind.online),
      version: 1,
    );

    NewAgentHost host(FakeNewAgentPort port) => NewAgentHost(
      port: port,
      newKey: () => 'key',
      announceMinInterval: const Duration(seconds: 10),
    );

    testWidgets('keeps its own section: no class, features or second '
        '"Disable" here', (tester) async {
      repository.agentRecordsById['a'] = builtin();
      final port = FakeNewAgentPort()..addAgent('a');
      await page(tester, 'a', builtin: host(port));

      expect(find.byKey(const ValueKey('builtinAgent')), findsOneWidget);
      for (final key in [
        'agentPage.toolClass',
        'agentPage.toolClassChange',
        'agentPage.features',
        'agentPage.featuresEdit',
        'agentPage.status',
        'agentPage.disable',
        'agentPage.enable',
      ]) {
        expect(find.byKey(ValueKey(key)), findsNothing, reason: key);
      }
    });

    testWidgets('that is disabled can be enabled again', (tester) async {
      repository.agentRecordsById['a'] = builtin(active: false);
      final port = FakeNewAgentPort()..addAgent('a');
      await page(tester, 'a', builtin: host(port));

      expect(find.byKey(const ValueKey('agentPage.disable')), findsNothing);
      await tap(tester, 'agentPage.enable');
      await type(tester, 'confirm.reason', 'Back in use');
      await tap(tester, 'confirm.submit');
      await stepUp(tester, 'Enable the agent Notes');

      expect(repository.calls, ['setActive a enable Back in use']);
      expect(repository.agentRecordsById['a']!.active, isTrue);
      expect(find.byKey(const ValueKey('agentPage.enable')), findsNothing);
    });

    testWidgets('without the host of its section can be disabled here', (
      tester,
    ) async {
      repository.agentRecordsById['a'] = builtin();
      await page(tester, 'a');
      expect(find.byKey(const ValueKey('builtinAgent')), findsNothing);
      expect(find.byKey(const ValueKey('agentPage.disable')), findsOneWidget);
      // Its class follows its definition, so the page offers no change of it.
      expect(
        find.byKey(const ValueKey('agentPage.toolClassChange')),
        findsNothing,
      );
    });
  });
}
