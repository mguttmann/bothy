// SPDX-License-Identifier: Apache-2.0

// The built-in agent on its agent card (brief A1-09 Tests and requirements 12 to 16; 01 6.20
// item 6; 26 3.1, 4.1, 8.3, 8.4; R-BA-06; 10 10.3), in the dialog of the desktop and the
// console and in the full-screen dialog of the phone: Edit opens the preview with the current
// definition, names the changed parts and saves with putDefinition; a 412 offers Reload;
// Versions lists newest first, Show is read-only, Restore saves an older version again; Change
// runner says that the connections restart and shows host_tool_class_ceiling,
// hosted_holder_mismatch and host_not_capable as sentences; a lowered class with a kept stdio
// server shows definition_needs_host with the way to remove the server; an added stdio server
// with host sets the class first, a removed one with chat sets it after the definition;
// Disable asks a reason; Continue setup resumes at the first missing step.

import 'package:admin_ui/admin_ui.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../admin/support.dart';
import '../new_agent/fake_new_agent_port.dart';

const Map<String, Object?> _plain = {
  'instructions': 'Keep the notes of Sam.',
  'skills': <Object?>[],
  'mcp_servers': <Object?>[],
  'settings': {'tool_class_wish': 'chat'},
  'origin': {'kind': 'manual'},
};

const Map<String, Object?> _withStdio = {
  'instructions': 'Sort the files Alex names.',
  'skills': <Object?>[],
  'mcp_servers': [
    {
      'name': 'files',
      'transport': 'stdio',
      'command': 'npx',
      'package': '@example/mcp-files@1.4.2',
    },
  ],
  'settings': {'tool_class_wish': 'host'},
  'origin': {'kind': 'manual'},
};

void main() {
  late FakeNewAgentPort port;
  late List<String> switched;
  var keys = 0;
  var changed = 0;

  setUp(() {
    keys = 0;
    changed = 0;
    port = FakeNewAgentPort();
    switched = [];
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

  for (final phone in [false, true]) {
    final layout = phone ? 'phone' : 'desktop and console';

    Future<void> section(WidgetTester tester) async {
      await pumpInApp(
        tester,
        BuiltinAgentSection(
          host: NewAgentHost(
            port: port,
            newKey: () => 'key-${++keys}',
            announceMinInterval: const Duration(seconds: 10),
            fullScreen: phone,
          ),
          agentId: 'a',
          onChanged: () => changed++,
          onSwitchBilling: () => switched.add('a'),
        ),
      );
      await tester.pumpAndSettle();
    }

    group(layout, () {
      testWidgets('shows the runner, the version and the actions', (
        tester,
      ) async {
        port.addAgent('a', definitions: [_plain]);
        await section(tester);
        expect(textOf(tester, 'builtinAgent.host'), 'Runner: Dev runner');
        expect(textOf(tester, 'builtinAgent.version'), 'Definition version 1');
        for (final k in ['edit', 'versions', 'changeRunner', 'disable']) {
          expect(find.byKey(ValueKey('builtinAgent.$k')), findsOneWidget);
        }
        expect(
          find.byKey(const ValueKey('builtinAgent.continueSetup')),
          findsNothing,
        );
      });

      testWidgets('Edit saves a new version with the changed parts named', (
        tester,
      ) async {
        port.addAgent('a', definitions: [_plain]);
        await section(tester);
        await tap(tester, 'builtinAgent.edit');
        expect(find.byKey(const ValueKey('newAgent.dialog')), findsOneWidget);
        expect(find.text('Edit Notes'), findsOneWidget);
        expect(
          find.byKey(const ValueKey('agentEdit.noChanges')),
          findsOneWidget,
        );
        expect(
          tester
              .widget<FilledButton>(
                find.byKey(const ValueKey('agentEdit.save')),
              )
              .onPressed,
          isNull,
        );
        // The identity and the runner are the agent's; the channel is no part of an edit.
        expect(
          tester
              .widget<TextField>(
                find.byKey(const ValueKey('newAgent.name.field')),
              )
              .enabled,
          isFalse,
        );
        expect(
          find.byKey(const ValueKey('newAgent.channel.wanted')),
          findsNothing,
        );
        await type(tester, 'newAgent.instructions', 'Keep the notes of Robin.');
        expect(
          textOf(tester, 'agentEdit.changes'),
          contains('Instructions changed'),
        );
        await tap(tester, 'agentEdit.save');
        expect(find.byKey(const ValueKey('newAgent.dialog')), findsNothing);
        expect(port.writes, ['putDefinition']);
        expect(port.changes, ['putDefinition edit']);
        expect(port.putIfMatches, [1]);
        expect(
          port.definitionsOf('a').last['instructions'],
          'Keep the notes of Robin.',
        );
        expect(textOf(tester, 'builtinAgent.version'), 'Definition version 2');
        expect(
          textOf(tester, 'builtinAgent.notice'),
          'Notes is saved as version 2.',
        );
        expect(changed, 1);
      });

      testWidgets(
        'a 412 says another device changed it and Reload starts again',
        (tester) async {
          port.addAgent('a', definitions: [_plain]);
          await section(tester);
          await tap(tester, 'builtinAgent.edit');
          await type(tester, 'newAgent.instructions', 'Mine.');
          port.changeBehind('a', {..._plain, 'instructions': 'Theirs.'});
          await tap(tester, 'agentEdit.save');
          expect(
            textOf(tester, 'newAgent.failure'),
            startsWith('Another device changed this agent.'),
          );
          expect(find.byKey(const ValueKey('newAgent.retry')), findsNothing);
          await tap(tester, 'agentEdit.reload');
          expect(find.byKey(const ValueKey('newAgent.failure')), findsNothing);
          expect(
            tester
                .widget<TextField>(
                  find.byKey(const ValueKey('newAgent.instructions.field')),
                )
                .controller!
                .text,
            'Theirs.',
          );
          expect(
            find.byKey(const ValueKey('agentEdit.noChanges')),
            findsOneWidget,
          );
          expect(port.definitionsOf('a').length, 2);
        },
      );

      testWidgets(
        'Versions: newest first, Show is read-only, Restore saves again',
        (tester) async {
          port.addAgent(
            'a',
            definitions: [
              _plain,
              {..._plain, 'instructions': 'B.'},
            ],
          );
          await section(tester);
          await tap(tester, 'builtinAgent.versions');
          expect(
            find.byKey(const ValueKey('agentVersions.dialog')),
            findsOneWidget,
          );
          final rows = [
            for (final w in tester.widgetList(
              find.byWidgetPredicate(
                (w) =>
                    w.key is ValueKey<String> &&
                    (w.key! as ValueKey<String>).value.startsWith(
                      'agentVersions.row.',
                    ),
              ),
            ))
              (w.key! as ValueKey<String>).value,
          ];
          expect(rows, ['agentVersions.row.2', 'agentVersions.row.1']);
          expect(textOf(tester, 'agentVersions.row.2'), contains('by Alex'));
          expect(
            find.byKey(const ValueKey('agentVersions.current.2')),
            findsOneWidget,
          );
          expect(
            find.byKey(const ValueKey('agentVersions.restore.2')),
            findsNothing,
          );

          await tap(tester, 'agentVersions.show.1');
          expect(find.text('Notes, version 1'), findsOneWidget);
          expect(
            tester
                .widget<TextField>(
                  find.byKey(const ValueKey('newAgent.instructions.field')),
                )
                .enabled,
            isFalse,
          );
          expect(find.byKey(const ValueKey('agentEdit.save')), findsNothing);
          await tap(tester, 'newAgent.close');
          expect(port.writes, isEmpty);

          await tap(tester, 'agentVersions.restore.1');
          expect(find.text('Restore version 1 of Notes'), findsOneWidget);
          expect(
            textOf(tester, 'agentEdit.changes'),
            contains('Instructions changed'),
          );
          await tap(tester, 'agentEdit.save');
          expect(port.writes, ['putDefinition']);
          expect(port.putIfMatches, [2]);
          expect(port.definitionsOf('a').length, 3);
          expect(port.definitionsOf('a').last, _plain);
          expect(
            textOf(tester, 'builtinAgent.version'),
            'Definition version 3',
          );
        },
      );

      testWidgets(
        'Change runner moves the agent and says the connections restart',
        (tester) async {
          port.addAgent('a', definitions: [_plain]);
          await section(tester);
          await tap(tester, 'builtinAgent.changeRunner');
          expect(
            find.text(
              'The connections of the agent restart on the new runner; an answer in progress stops.',
            ),
            findsOneWidget,
          );
          expect(
            find.byKey(const ValueKey('agentHost.runner.runner-1')),
            findsNothing,
          );
          await tap(tester, 'agentHost.runner.runner-2');
          await tap(tester, 'agentHost.confirm');
          expect(find.byKey(const ValueKey('agentHost.dialog')), findsNothing);
          expect(port.hostOf('a'), 'runner-2');
          expect(port.changes, ['setHost host']);
          expect(textOf(tester, 'builtinAgent.host'), 'Runner: Alex laptop');
          expect(
            textOf(tester, 'builtinAgent.notice'),
            'Notes is now on Alex laptop.',
          );
        },
      );

      testWidgets('Change runner: the class ceiling is a sentence', (
        tester,
      ) async {
        port.addAgent(
          'a',
          toolClass: AgentToolClass.host,
          definitions: [_withStdio],
        );
        await section(tester);
        await tap(tester, 'builtinAgent.changeRunner');
        await tap(tester, 'agentHost.runner.runner-2');
        await tap(tester, 'agentHost.confirm');
        expect(
          textOf(tester, 'agentHost.error'),
          'Alex laptop loads at most the tool class Scoped, below the class of '
          'the agent. Choose another runner, or lower the class of the agent first.',
        );
        expect(find.byKey(const ValueKey('agentHost.dialog')), findsOneWidget);
        expect(port.hostOf('a'), 'runner-1');
      });

      testWidgets(
        'Change runner: the billing holder is a sentence with Switch billing',
        (tester) async {
          port
            ..extraRunners.add(
              const HostingRunner(
                id: 'runner-3',
                name: 'Build host',
                ceiling: AgentToolClass.host,
              ),
            )
            ..offerValue = const NewAgentOffer(
              templates: [],
              runners: [
                alexRunner,
                HostingRunner(
                  id: 'runner-3',
                  name: 'Build host',
                  ceiling: AgentToolClass.host,
                ),
              ],
            )
            ..addAgent(
              'a',
              hostedBy: 'runner-2',
              billing: 'personal',
              holder: 'alex',
              definitions: [_plain],
            );
          await section(tester);
          await tap(tester, 'builtinAgent.changeRunner');
          await tap(tester, 'agentHost.runner.runner-3');
          await tap(tester, 'agentHost.confirm');
          expect(
            textOf(tester, 'agentHost.error'),
            startsWith('The agent bills Personal'),
          );
          await tap(tester, 'agentHost.switchBilling');
          expect(find.byKey(const ValueKey('agentHost.dialog')), findsNothing);
          expect(switched, ['a']);
          expect(port.hostOf('a'), 'runner-2');
        },
      );

      testWidgets(
        'Change runner: a runner that hosts no agents is a sentence',
        (tester) async {
          port.addAgent('a', definitions: [_plain]);
          port.failNext['setHost'] = const NewAgentFailure.problem(
            'validation_failed',
            errors: [FieldIssue('/hosted_by', 'host_not_capable')],
          );
          await section(tester);
          await tap(tester, 'builtinAgent.changeRunner');
          await tap(tester, 'agentHost.runner.runner-2');
          await tap(tester, 'agentHost.confirm');
          expect(
            textOf(tester, 'agentHost.error'),
            'This runner does not take built-in agents right now. Choose another runner.',
          );
        },
      );

      testWidgets(
        'a lowered class with the stdio server kept: definition_needs_host, '
        'Remove the local servers, Try again',
        (tester) async {
          port.addAgent(
            'a',
            toolClass: AgentToolClass.host,
            definitions: [_withStdio],
          );
          await section(tester);
          await tap(tester, 'builtinAgent.edit');
          await tap(tester, 'newAgent.class.chat');
          expect(
            textOf(tester, 'agentEdit.changes'),
            contains('Tool class from Host to Chat'),
          );
          await tap(tester, 'agentEdit.save');
          expect(
            textOf(tester, 'newAgent.failure'),
            startsWith('The stored definition still has a local MCP server'),
          );
          expect(port.classOf('a'), AgentToolClass.host);
          await tap(tester, 'agentEdit.removeLocal');
          expect(
            find.byKey(const ValueKey('newAgent.server.0.name')),
            findsNothing,
          );
          port.calls.clear();
          await tap(tester, 'newAgent.retry');
          expect(find.byKey(const ValueKey('newAgent.dialog')), findsNothing);
          expect(port.writes, ['putDefinition', 'setToolClass chat']);
          expect(port.classOf('a'), AgentToolClass.chat);
        },
      );

      testWidgets(
        'an added stdio server with host: the class step before the definition',
        (tester) async {
          port.addAgent('a', definitions: [_plain]);
          await section(tester);
          await tap(tester, 'builtinAgent.edit');
          await tap(tester, 'newAgent.server.add');
          await type(tester, 'newAgent.server.0.name', 'files');
          await tap(tester, 'newAgent.server.0.transport.stdio');
          await type(tester, 'newAgent.server.0.command', 'npx');
          await type(
            tester,
            'newAgent.server.0.package',
            '@example/mcp-files@1.4.2',
          );
          await tap(tester, 'newAgent.class.host');
          await tap(tester, 'agentEdit.save');
          expect(find.byKey(const ValueKey('newAgent.dialog')), findsNothing);
          expect(port.writes, ['setToolClass host', 'putDefinition']);
          expect(port.audit, ['agent.tool_class.set', 'agent.update']);
          expect(port.classOf('a'), AgentToolClass.host);
        },
      );

      testWidgets(
        'a removed stdio server with chat: the definition before the class step',
        (tester) async {
          port.addAgent(
            'a',
            toolClass: AgentToolClass.host,
            definitions: [_withStdio],
          );
          await section(tester);
          await tap(tester, 'builtinAgent.edit');
          await tap(tester, 'newAgent.server.0.remove');
          await tap(tester, 'newAgent.class.chat');
          await tap(tester, 'agentEdit.save');
          expect(find.byKey(const ValueKey('newAgent.dialog')), findsNothing);
          expect(port.writes, ['putDefinition', 'setToolClass chat']);
          expect(port.classOf('a'), AgentToolClass.chat);
        },
      );

      testWidgets('Disable asks a reason and disables the agent', (
        tester,
      ) async {
        port.addAgent('a', definitions: [_plain]);
        await section(tester);
        await tap(tester, 'builtinAgent.disable');
        expect(find.text('Disable Notes?'), findsOneWidget);
        await tester.enterText(
          find.byKey(const ValueKey('confirm.reason')),
          'Not needed any more',
        );
        await tap(tester, 'confirm.submit');
        expect(port.activeOf('a'), isFalse);
        expect(port.disableReasons, ['Not needed any more']);
        expect(
          textOf(tester, 'builtinAgent.disabled'),
          'This agent is disabled.',
        );
        expect(find.byKey(const ValueKey('builtinAgent.edit')), findsNothing);
      });

      testWidgets('Continue setup resumes at the first missing step', (
        tester,
      ) async {
        port.addAgent('a');
        await section(tester);
        expect(textOf(tester, 'builtinAgent.notSetUp'), 'Not set up');
        expect(find.byKey(const ValueKey('builtinAgent.edit')), findsNothing);
        expect(
          find.byKey(const ValueKey('builtinAgent.versions')),
          findsNothing,
        );
        await tap(tester, 'builtinAgent.continueSetup');
        expect(find.text('Set up Notes'), findsOneWidget);
        await tap(tester, 'newAgent.way.template');
        await tap(tester, 'newAgent.template.file-helper');
        expect(
          tester
              .widget<TextField>(
                find.byKey(const ValueKey('newAgent.name.field')),
              )
              .controller!
              .text,
          'Notes',
        );
        await tap(tester, 'agentEdit.continue');
        expect(find.byKey(const ValueKey('newAgent.dialog')), findsNothing);
        expect(port.writes, ['setToolClass host', 'putDefinition']);
        expect(port.changes, ['setToolClass setup', 'putDefinition setup']);
        expect(port.agentCount, 1);
        expect(textOf(tester, 'builtinAgent.version'), 'Definition version 1');
      });
    });
  }
}
