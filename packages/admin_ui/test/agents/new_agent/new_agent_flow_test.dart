// SPDX-License-Identifier: Apache-2.0

// BA-T16 for the creation flow (docs/spec/26-builtin-agents.md 8.2, 8.3, 12; brief A1-08
// Tests): a failure at each step after `agent.create` leaves exactly the earlier steps and their
// audit rows; "Try again" resumes without a second create; the class rises before and falls
// after the definition, so the `stdio` cases are stored without `tool_class_too_low` or
// `definition_needs_host`; the hosting runner is fixed once the agent exists; `personal` is
// offered only under 10 10.3.

import 'package:admin_ui/admin_ui.dart';
import 'package:flutter_test/flutter_test.dart';

import 'fake_new_agent_port.dart';

void main() {
  late FakeNewAgentPort port;
  late NewAgentFlow flow;
  var keys = 0;

  NewAgentFlow newFlow({NewAgentContext? context}) => NewAgentFlow(
    port: port,
    offer: standardOffer,
    newKey: () => 'key-${++keys}',
    context: context ?? port.contextValue,
  );

  setUp(() {
    keys = 0;
    port = FakeNewAgentPort();
    flow = newFlow();
  });

  /// A writing helper on Dev runner with a channel #writing in Team.
  void writingWithChannel() {
    flow
      ..useTemplate(writingTemplate)
      ..runner = devRunner
      ..edit(
        (d) => d
          ..channelWanted = true
          ..channelSpaceId = 'space-1'
          ..channelName = 'writing',
      );
  }

  group('the steps', () {
    test(
      'a template without servers runs create, definition, channel, operator',
      () async {
        writingWithChannel();
        expect(flow.steps, [
          NewAgentStep.createAgent,
          NewAgentStep.storeDefinition,
          NewAgentStep.createChannel,
          NewAgentStep.setOperator,
        ]);
        await flow.run();
        expect(flow.finished, isTrue);
        expect(port.audit, [
          'agent.create',
          'agent.update',
          'channel.create',
          'channel.operator.set',
        ]);
        expect(port.channels.values.single, port.onlyAgent);
        expect(flow.result!.channelId, 'channel-1');
      },
    );

    test(
      'a stdio template with host sets the class before the definition',
      () async {
        flow
          ..useTemplate(filesTemplate)
          ..runner = devRunner;
        expect(flow.targetClass, AgentToolClass.host);
        expect(flow.steps, [
          NewAgentStep.createAgent,
          NewAgentStep.setClass,
          NewAgentStep.storeDefinition,
        ]);
        await flow.run();
        expect(flow.failure, isNull);
        expect(port.writes, [
          'createAgent',
          'setToolClass host',
          'putDefinition',
        ]);
        expect(port.definitionsOf(port.onlyAgent), hasLength(1));
      },
    );

    test('each step that fails leaves exactly the earlier steps', () async {
      const order = [
        'setToolClass',
        'putDefinition',
        'setBilling',
        'createChannel',
        'setOperator',
      ];
      const rows = [
        'agent.create',
        'agent.tool_class.set',
        'agent.update',
        'agent.billing_mode.set',
        'channel.create',
        'channel.operator.set',
      ];
      for (final (i, op) in order.indexed) {
        port = FakeNewAgentPort();
        flow = newFlow()
          ..useTemplate(writingTemplate)
          ..runner = alexRunner
          ..billing = 'personal'
          ..edit(
            (d) => d
              ..toolClass = AgentToolClass.scoped
              ..channelWanted = true
              ..channelSpaceId = 'space-1'
              ..channelName = 'writing',
          );
        port.failNext[op] = const NewAgentFailure.offline();
        await flow.run();
        expect(flow.failedStep, flow.steps[i + 1], reason: op);
        expect(flow.stateOf(flow.steps[i + 1]), NewAgentStepState.failed);
        expect(port.audit, rows.sublist(0, i + 1), reason: op);
        for (final done in flow.steps.take(i + 1)) {
          expect(
            flow.stateOf(done),
            NewAgentStepState.done,
            reason: '$op $done',
          );
        }

        // "Try again" runs the rest and creates nothing twice.
        await flow.run();
        expect(flow.finished, isTrue, reason: op);
        expect(port.agentCount, 1);
        expect(port.audit, rows, reason: op);
      }
    });

    test('a lost answer of create is repeated with the same key', () async {
      flow
        ..useTemplate(writingTemplate)
        ..runner = devRunner;
      port.failNext['putDefinition'] = const NewAgentFailure.offline();
      await flow.run();
      final created = port.agentCount;
      await flow.run();
      expect(port.agentCount, created);
      expect(port.calls.where((c) => c == 'createAgent'), hasLength(1));
    });

    test('a refused definition, the class raised, Try again: class then definition', () async {
      flow
        ..useTemplate(filesTemplate)
        ..runner = devRunner
        ..edit((d) => d.toolClass = AgentToolClass.chat);
      await flow.run();
      expect(flow.failedStep, NewAgentStep.storeDefinition);
      expect(flow.failure!.code, 'validation_failed');
      expect(flow.errorsAt('/mcp_servers/0/transport'), ['tool_class_too_low']);

      port.calls.clear();
      flow.edit((d) => d.toolClass = AgentToolClass.host);
      expect(flow.steps.indexOf(NewAgentStep.setClass), 1);
      await flow.run();
      expect(flow.finished, isTrue);
      expect(port.writes, ['setToolClass host', 'putDefinition']);
      expect(port.agentCount, 1);
    });

    test('a failed definition after host, the class back to chat: the agent does not stay at host', () async {
      flow
        ..useTemplate(writingTemplate)
        ..runner = devRunner
        ..edit((d) => d.toolClass = AgentToolClass.host);
      port.failNext['putDefinition'] = const NewAgentFailure.offline();
      await flow.run();
      expect(port.classOf(port.onlyAgent), AgentToolClass.host);

      port.calls.clear();
      flow.edit((d) => d.toolClass = AgentToolClass.chat);
      await flow.run();
      expect(flow.finished, isTrue);
      expect(port.classOf(port.onlyAgent), AgentToolClass.chat);
      expect(port.writes, ['putDefinition', 'setToolClass chat']);
    });

    test('a stored stdio definition, the server removed and chat: definition before class', () async {
      flow
        ..useTemplate(filesTemplate)
        ..runner = devRunner
        ..edit(
          (d) => d
            ..channelWanted = true
            ..channelSpaceId = 'space-1'
            ..channelName = 'files',
        );
      port.failNext['createChannel'] = const NewAgentFailure.offline();
      await flow.run();
      expect(flow.failedStep, NewAgentStep.createChannel);
      expect(port.definitionsOf(port.onlyAgent), hasLength(1));

      port.calls.clear();
      flow.edit(
        (d) => d
          ..servers.clear()
          ..toolClass = AgentToolClass.chat,
      );
      expect(
        flow.steps.indexOf(NewAgentStep.storeDefinition) <
            flow.steps.indexOf(NewAgentStep.setClass),
        isTrue,
      );
      await flow.run();
      expect(flow.failure, isNull);
      expect(flow.finished, isTrue);
      expect(port.writes, [
        'putDefinition',
        'setToolClass chat',
        'createChannel',
        'setOperator',
      ]);
      expect(port.classOf(port.onlyAgent), AgentToolClass.chat);
    });

    test(
      'the ceiling of the runner fails the class step with its reason',
      () async {
        flow
          ..useTemplate(filesTemplate)
          ..runner = alexRunner;
        await flow.run();
        expect(flow.failedStep, NewAgentStep.setClass);
        expect(flow.failure!.reason, 'host_tool_class_ceiling');
        expect(port.audit, ['agent.create']);
      },
    );

    test(
      'a cancelled step-up fails the step and keeps the earlier ones',
      () async {
        flow
          ..useTemplate(writingTemplate)
          ..runner = devRunner;
        port.failNext['putDefinition'] =
            const NewAgentFailure.stepUpCancelled();
        await flow.run();
        expect(flow.failure!.kind, NewAgentFailureKind.stepUpCancelled);
        expect(flow.stateOf(NewAgentStep.createAgent), NewAgentStepState.done);
      },
    );
  });

  group('the preview', () {
    test('the hosting runner is read-only once the agent exists', () async {
      flow
        ..useTemplate(writingTemplate)
        ..runner = devRunner;
      port.failNext['putDefinition'] = const NewAgentFailure.offline();
      await flow.run();
      expect(flow.canEditHost, isFalse);
      flow.runner = alexRunner;
      expect(flow.runner, devRunner);
      flow.edit((d) => d.handle = 'other');
      expect(flow.draft!.handle, 'writing-helper');
      await flow.run();
      expect(port.calls.where((c) => c == 'createAgent'), hasLength(1));
    });

    test('edits of the preview reach the request', () async {
      flow
        ..useEmpty()
        ..runner = devRunner
        ..edit(
          (d) => d
            ..name = 'Notes'
            ..handle = 'notes'
            ..instructions = 'Keep the notes of Sam.'
            ..model = 'claude-sonnet-4-5'
            ..costCap = '2.5'
            ..skills.add(
              DraftSkill(
                name: 'summary',
                description: 'Sums up',
                body: 'Sum up.',
              ),
            )
            ..servers.add(
              DraftMcpServer(
                name: 'docs',
                url: 'https://mcp.example.com/v1',
                headers: [
                  DraftEntry(
                    name: 'Authorization',
                    secretRef: 'env:DOCS_TOKEN',
                    isSecret: true,
                  ),
                ],
              ),
            ),
        );
      expect(flow.draft!.secretNames, ['env:DOCS_TOKEN']);
      await flow.run();
      expect(port.putBodies.single, {
        'instructions': 'Keep the notes of Sam.',
        'skills': [
          {'name': 'summary', 'description': 'Sums up', 'body': 'Sum up.'},
        ],
        'mcp_servers': [
          {
            'name': 'docs',
            'transport': 'http',
            'url': 'https://mcp.example.com/v1',
            'headers': [
              {'name': 'Authorization', 'secret_ref': 'env:DOCS_TOKEN'},
            ],
          },
        ],
        'settings': {
          'model': 'claude-sonnet-4-5',
          'tool_class_wish': 'chat',
          'max_cost_usd_per_run': 2.5,
        },
        'origin': {'kind': 'manual'},
      });
    });

    test('template, interview and form lead to the same preview', () {
      final fromTemplate = newFlow()..useTemplate(filesTemplate);
      final fromInterview = newFlow()
        ..useInterview(
          InterviewReply(
            outcome: InterviewOutcome.draft,
            draft: {
              ...filesTemplate.definition,
              'name': 'File helper',
              'handle': 'file-helper',
              'description': 'Reads and sorts the files of a folder',
              'channel_suggestion': {
                'space_name': 'Team',
                'channel_name': 'files',
              },
            },
          ),
          costMicros: 4000,
          currency: 'USD',
        );
      final fromForm = newFlow()..useEmpty();
      for (final f in [fromTemplate, fromInterview, fromForm]) {
        expect(f.draft, isNotNull);
      }
      expect(
        fromInterview.draft!.definitionJson()['mcp_servers'],
        fromTemplate.draft!.definitionJson()['mcp_servers'],
      );
      expect(fromInterview.draft!.channelSpaceId, 'space-1');
      expect(fromInterview.costMicros, 4000);
      expect(fromForm.draft!.definitionJson()['origin'], {'kind': 'manual'});
    });

    test('personal is offered only to the only owner who holds the personal runner', () {
      flow
        ..useEmpty()
        ..runner = devRunner;
      expect(flow.personalOffer, PersonalOffer.runnerNotPersonal);
      flow.billing = 'personal';
      expect(flow.billing, 'organization');

      flow.runner = alexRunner;
      expect(flow.personalOffer, PersonalOffer.offered);

      final two = newFlow(
        context: const NewAgentContext(meId: 'alex', onlyOwner: false),
      )..runner = alexRunner;
      expect(two.personalOffer, PersonalOffer.notOnlyOwner);

      final sam = newFlow(
        context: const NewAgentContext(meId: 'sam', onlyOwner: true),
      )..runner = alexRunner;
      expect(sam.personalOffer, PersonalOffer.notHolder);

      final unsupported = newFlow()
        ..runner = const HostingRunner(
          id: 'runner-3',
          name: 'Old laptop',
          ceiling: AgentToolClass.chat,
          billingMode: 'personal',
          holderId: 'alex',
          modesSupported: ['organization'],
        );
      expect(unsupported.personalOffer, PersonalOffer.notSupported);
    });

    test(
      'a refused handle shows at its field and a new handle is a new request',
      () async {
        flow
          ..useEmpty()
          ..runner = devRunner
          ..edit(
            (d) => d
              ..name = 'Notes'
              ..handle = 'notes',
          );
        port.failNext['createAgent'] = const NewAgentFailure.problem(
          'validation_failed',
          errors: [FieldIssue('/handle', 'pattern_mismatch')],
        );
        await flow.run();
        expect(flow.errorsAt(PreviewField.handle), ['pattern_mismatch']);
        expect(flow.canEditIdentity, isTrue);
        flow.edit((d) => d.handle = 'notes-2');
        await flow.run();
        expect(flow.finished, isTrue);
        expect(keys, 2);
      },
    );

    test('invalid fields block Create', () async {
      flow.useEmpty();
      expect(flow.invalidFields, {
        PreviewField.name,
        PreviewField.handle,
        PreviewField.host,
      });
      await flow.run();
      expect(port.calls, isEmpty);
      flow.edit((d) => d.costCap = 'two');
      expect(flow.invalidFields, contains(PreviewField.costCap));
      // A cap that parses to infinity is marked too; JSON has no such number.
      flow.edit((d) => d.costCap = 'Infinity');
      expect(flow.invalidFields, contains(PreviewField.costCap));
      flow.edit((d) => d.costCap = '1e999');
      expect(flow.invalidFields, contains(PreviewField.costCap));
    });
  });

  group('the interview', () {
    test('an error other than a failure shows as internal_error and returns the answer', () async {
      port.interviewReplies.add(
        const InterviewReply(
          outcome: InterviewOutcome.questions,
          questions: ['What should the agent do?'],
        ),
      );
      final interview = AgentInterview(port);
      await interview.start();
      port.failNext['interview'] = StateError('malformed answer');
      await interview.answer('Keep the notes');
      expect(interview.error?.code, 'internal_error');
      expect(interview.pending, isFalse);
      expect(
        [for (final t in interview.turns) t.role],
        [InterviewRole.builder],
      );
    });
  });
}
