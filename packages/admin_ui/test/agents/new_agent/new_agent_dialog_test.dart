// SPDX-License-Identifier: Apache-2.0

// "New agent" in the shared screens of the desktop and the console (brief A1-08 Tests;
// docs/spec/01-product.md 6.20; docs/spec/26-builtin-agents.md 8.1 to 8.3): the entry is hidden
// without an offer (an older server, no runner that hosts agents) and shown otherwise; a
// built-in agent without a definition is "Not set up"; the template, interview and form paths
// lead to the same preview; the interview shows its questions, a draft with field errors, the
// cost and each fallback; personal is offered only under 10 10.3; the hosting runner is
// read-only after `agent.create`; a failed step shows one sentence, "Try again" resumes and
// "Close" keeps what exists.

import 'package:admin_ui/admin_ui.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../admin/support.dart';
import 'fake_new_agent_port.dart';

const NewAgentOffer _oneRunner = NewAgentOffer(
  templates: [writingTemplate, filesTemplate],
  runners: [devRunner],
);

void main() {
  late FakeNewAgentPort port;
  late List<NewAgentResult?> results;
  var keys = 0;

  setUp(() {
    keys = 0;
    port = FakeNewAgentPort(offerValue: _oneRunner);
    results = [];
  });

  Future<void> tap(WidgetTester tester, String key) async {
    final f = find.byKey(ValueKey(key));
    await tester.ensureVisible(f);
    await tester.pumpAndSettle();
    await tester.tap(f);
    await tester.pumpAndSettle();
  }

  Future<void> type(WidgetTester tester, String key, String text) async {
    final f = find.byKey(ValueKey('$key.field'));
    await tester.ensureVisible(f);
    await tester.enterText(f, text);
    await tester.pumpAndSettle();
  }

  Future<void> open(WidgetTester tester, {NewAgentOffer? offer}) async {
    await pumpInApp(
      tester,
      Builder(
        builder: (context) => TextButton(
          key: const ValueKey('open'),
          onPressed: () async => results.add(
            await showNewAgentDialog(
              context,
              port: port,
              offer: offer ?? port.offerValue!,
              newKey: () => 'key-${++keys}',
              announceMinInterval: const Duration(seconds: 10),
            ),
          ),
          child: const Text('open'),
        ),
      ),
    );
    await tap(tester, 'open');
  }

  String? errorOf(WidgetTester tester, String key) => tester
      .widget<TextField>(find.byKey(ValueKey('$key.field')))
      .decoration
      ?.errorText;

  group('the entry in the agent list', () {
    Future<FakeAdminRepository> list(WidgetTester tester) async {
      final repository = FakeAdminRepository(
        prompt: StepUpDialogPrompt(() => null),
      );
      repository.agentRecordsById['notes'] = const AgentRecord(
        id: 'notes',
        handle: 'notes',
        displayName: 'Notes',
        active: true,
        adapterKind: builtinAdapterKind,
        description: '',
        mode: AgentBillingMode.organization,
        toolClass: 'chat',
        runtime: AgentRuntimeInfo(state: AgentLiveStateKind.offline),
        version: 1,
      );
      await pumpInApp(
        tester,
        AgentsList(
          repository: repository,
          onOpen: (_) {},
          newAgent: NewAgentHost(
            port: port,
            newKey: () => 'key-${++keys}',
            announceMinInterval: const Duration(seconds: 10),
            onCreated: results.add,
          ),
        ),
      );
      await tester.pumpAndSettle();
      return repository;
    }

    testWidgets(
      'is hidden without an offer (older server, no hosting runner)',
      (tester) async {
        port.offerValue = null;
        await list(tester);
        expect(find.byKey(const ValueKey('agents.new')), findsNothing);
      },
    );

    testWidgets('is shown with an offer and opens the dialog', (tester) async {
      await list(tester);
      await tap(tester, 'agents.new');
      expect(find.byKey(const ValueKey('newAgent.dialog')), findsOneWidget);
      expect(
        find.byKey(const ValueKey('newAgent.way.template')),
        findsOneWidget,
      );
      expect(
        find.byKey(const ValueKey('newAgent.way.interview')),
        findsOneWidget,
      );
      expect(find.byKey(const ValueKey('newAgent.way.empty')), findsOneWidget);
    });

    testWidgets('marks a built-in agent without a definition as Not set up', (
      tester,
    ) async {
      await list(tester);
      expect(
        find.byKey(const ValueKey('agent.notes.notSetUp')),
        findsOneWidget,
      );
      expect(find.text('Not set up'), findsOneWidget);
      expect(find.byKey(const ValueKey('agent.texter.notSetUp')), findsNothing);
    });

    testWidgets(
      'Close after a failed step keeps the agent, which the list shows as Not set up',
      (tester) async {
        await list(tester);
        await tap(tester, 'agents.new');
        await tap(tester, 'newAgent.way.template');
        await tap(tester, 'newAgent.template.writing-helper');
        port.failNext['putDefinition'] = const NewAgentFailure.offline();
        await tap(tester, 'newAgent.create');
        expect(find.byKey(const ValueKey('newAgent.failure')), findsOneWidget);
        await tap(tester, 'newAgent.close');
        expect(find.byKey(const ValueKey('newAgent.dialog')), findsNothing);
        expect(port.agentCount, 1);
        expect(results, isEmpty);
        // The list loaded again and asked the definition of the new agent.
        expect(port.calls.where((c) => c == 'definition'), isNotEmpty);
      },
    );
  });

  group('the paths', () {
    testWidgets('a template leads to the preview and Create runs the steps', (
      tester,
    ) async {
      await open(tester);
      await tap(tester, 'newAgent.way.template');
      expect(find.text('Needs these secrets: FILES_TOKEN'), findsOneWidget);
      await tap(tester, 'newAgent.template.file-helper');
      expect(find.byKey(const ValueKey('newAgent.preview')), findsOneWidget);
      final chip = tester.widget<ChoiceChip>(
        find.byKey(const ValueKey('newAgent.class.host')),
      );
      expect(chip.selected, isTrue);
      Finder secret(String name) => find.descendant(
        of: find.byKey(const ValueKey('newAgent.secrets')),
        matching: find.text(name),
      );
      expect(secret('env:FILES_TOKEN'), findsOneWidget);
      expect(secret('FILES_TOKEN'), findsOneWidget);
      await tap(tester, 'newAgent.create');
      expect(find.byKey(const ValueKey('newAgent.dialog')), findsNothing);
      expect(results.single!.agentId, port.onlyAgent);
      expect(port.writes, [
        'createAgent',
        'setToolClass host',
        'putDefinition',
      ]);
    });

    testWidgets('the empty form checks its fields before anything is sent', (
      tester,
    ) async {
      await open(tester);
      await tap(tester, 'newAgent.way.empty');
      await tap(tester, 'newAgent.create');
      expect(errorOf(tester, 'newAgent.name'), 'Enter a name.');
      expect(errorOf(tester, 'newAgent.handle'), isNotNull);
      expect(port.calls, isEmpty);

      await type(tester, 'newAgent.name', 'Notes');
      await type(tester, 'newAgent.handle', 'notes');
      await type(tester, 'newAgent.instructions', 'Keep the notes of Robin.');
      await tap(tester, 'newAgent.create');
      expect(results.single, isNotNull);
      expect(port.putBodies.single['instructions'], 'Keep the notes of Robin.');
    });

    testWidgets(
      'the interview shows questions, then a draft with its field errors and the cost',
      (tester) async {
        port.interviewReplies.addAll([
          const InterviewReply(
            outcome: InterviewOutcome.questions,
            questions: ['What should the agent do?'],
            costMicros: 20000,
            currency: 'USD',
          ),
          InterviewReply(
            outcome: InterviewOutcome.draft,
            draft: {
              ...writingTemplate.definition,
              'name': 'Notes',
              'handle': 'notes',
              'description': 'Keeps notes',
            },
            errors: const [FieldIssue('/instructions', 'too_long')],
            costMicros: 30000,
            currency: 'USD',
          ),
        ]);
        await open(tester);
        await tap(tester, 'newAgent.way.interview');
        expect(find.text('What should the agent do?'), findsOneWidget);
        expect(
          find.byKey(const ValueKey('newAgent.interview.cost')),
          findsOneWidget,
        );
        await tester.enterText(
          find.byKey(const ValueKey('newAgent.interview.input')),
          'Keep the notes of our meetings',
        );
        await tap(tester, 'newAgent.interview.send');

        expect(port.interviewRequests, hasLength(2));
        final (turns, wantDraft) = port.interviewRequests.last;
        expect(
          [for (final t in turns) (t.role, t.text)],
          [
            (InterviewRole.builder, 'What should the agent do?'),
            (InterviewRole.owner, 'Keep the notes of our meetings'),
          ],
        );
        expect(wantDraft, isFalse);
        expect(find.byKey(const ValueKey('newAgent.preview')), findsOneWidget);
        expect(errorOf(tester, 'newAgent.instructions'), 'Too long.');
        // The cost of both turns.
        expect(find.byKey(const ValueKey('newAgent.cost')), findsOneWidget);
        expect(find.textContaining('0.05'), findsOneWidget);
      },
    );

    testWidgets('"Use what we have" asks for the draft', (tester) async {
      port.interviewReplies.add(
        const InterviewReply(
          outcome: InterviewOutcome.questions,
          questions: ['Which channel?'],
        ),
      );
      await open(tester);
      await tap(tester, 'newAgent.way.interview');
      await tap(tester, 'newAgent.interview.useWhatWeHave');
      expect(port.interviewRequests.last.$2, isTrue);
    });

    for (final (reason, text) in [
      ('builder_not_configured', 'No builder agent is set up on this server.'),
      ('builder_offline', 'The builder agent is offline.'),
      ('builder_timeout', 'The builder agent did not answer in time.'),
      ('builder_invalid', 'The answer of the builder agent could not be used.'),
    ]) {
      testWidgets('the fallback $reason says why and offers the other ways', (
        tester,
      ) async {
        port.interviewReplies.add(
          InterviewReply(
            outcome: InterviewOutcome.fallback,
            fallbackReason: reason,
          ),
        );
        await open(tester);
        await tap(tester, 'newAgent.way.interview');
        expect(find.text(text), findsOneWidget);
        expect(
          find.byKey(const ValueKey('newAgent.interview.input')),
          findsNothing,
        );
        await tap(tester, 'newAgent.interview.toEmpty');
        expect(find.byKey(const ValueKey('newAgent.preview')), findsOneWidget);
      });
    }

    testWidgets('the fallback leads to the templates', (tester) async {
      await open(tester);
      await tap(tester, 'newAgent.way.interview');
      await tap(tester, 'newAgent.interview.toTemplates');
      expect(find.byKey(const ValueKey('newAgent.templates')), findsOneWidget);
    });
  });

  group('the preview', () {
    testWidgets('offers personal only to the only owner who holds the runner', (
      tester,
    ) async {
      port.offerValue = const NewAgentOffer(
        templates: [writingTemplate],
        runners: [alexRunner],
      );
      await open(tester);
      await tap(tester, 'newAgent.way.empty');
      expect(
        find.byKey(const ValueKey('newAgent.billing.personal')),
        findsOneWidget,
      );

      await tap(tester, 'newAgent.cancel');
      port
        ..offerValue = _oneRunner
        ..contextValue = const NewAgentContext(meId: 'alex', onlyOwner: true);
      await tap(tester, 'open');
      await tap(tester, 'newAgent.way.empty');
      expect(
        find.byKey(const ValueKey('newAgent.billing.personal')),
        findsNothing,
      );
      expect(
        find.text(
          'Personal needs a hosting runner on a personal subscription.',
        ),
        findsOneWidget,
      );
    });

    testWidgets('a server without the class says what it does not load', (
      tester,
    ) async {
      await open(tester);
      await tap(tester, 'newAgent.way.template');
      await tap(tester, 'newAgent.template.file-helper');
      await tap(tester, 'newAgent.class.chat');
      expect(
        find.byKey(const ValueKey('newAgent.server.0.notLoaded')),
        findsOneWidget,
      );
      expect(
        find.text('Answers in the chat. Loads no MCP server; skills load.'),
        findsOneWidget,
      );
    });

    testWidgets(
      'a failed step: one sentence, the runner fixed, Try again resumes',
      (tester) async {
        await open(tester);
        await tap(tester, 'newAgent.way.template');
        await tap(tester, 'newAgent.template.file-helper');
        await tap(tester, 'newAgent.class.chat');
        await tap(tester, 'newAgent.create');

        expect(
          find.text(
            'A local (stdio) MCP server needs the tool class Host. Raise the class or remove '
            'the server, then try again.',
          ),
          findsOneWidget,
        );
        expect(
          find.byKey(const ValueKey('newAgent.hostFixed')),
          findsOneWidget,
        );
        final host = tester.widget<DsDropdownField<HostingRunner>>(
          find.byWidgetPredicate((w) => w is DsDropdownField<HostingRunner>),
        );
        expect(host.enabled, isFalse);
        final name = tester.widget<TextField>(
          find.byKey(const ValueKey('newAgent.name.field')),
        );
        expect(name.enabled, isFalse);
        expect(
          find.bySemanticsLabel(RegExp('Store the definition: failed')),
          findsOneWidget,
        );

        await tap(tester, 'newAgent.class.host');
        await tap(tester, 'newAgent.retry');
        expect(results.single, isNotNull);
        expect(port.agentCount, 1);
        expect(port.writes, [
          'createAgent',
          'putDefinition',
          'setToolClass host',
          'putDefinition',
        ]);
      },
    );
  });
}
