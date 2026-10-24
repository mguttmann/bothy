// SPDX-License-Identifier: Apache-2.0

// Slash commands (brief M4-21 Tests; 01 D5; components.md 9; 09 10.3, 10.5): the operator's
// commands first and equal names of other agents qualified with the handle; option suggestions
// after `ui.autocomplete_debounce` with an older request cancelled; incomplete options block
// send with the reason, a complete command goes through `invokeCommand`.

import 'dart:async';

import 'package:client/agents/commands/autocomplete_client.dart';
import 'package:client/agents/commands/builtin_commands.dart';
import 'package:client/agents/commands/command_invoke.dart';
import 'package:client/agents/commands/command_options.dart';
import 'package:client/agents/commands/command_suggestions.dart';
import 'package:client/agents/providers.dart';
import 'package:client/sync/agent_ops.dart';
import 'package:client/thread/history_column.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';

import '../support/agent_fakes.dart';
import '../support/card_host.dart' show entriesOf;
import '../support/column_host.dart';
import '../support/structure.dart';
import '../support/thread_fixtures.dart';
import '../support/thread_host.dart';
import '../support/widgets.dart';

/// Nova, a second agent with a command of the same name as Robin's.
final PrincipalId nova = PrincipalId.parse(uuid(0xa4));

Command command(
  int n,
  String name, {
  PrincipalId? owner,
  bool control = false,
  List<CommandOption> options = const [],
}) => Command(
  id: CommandId.parse(uuid(0x900 + n)),
  ownerPrincipalId: owner ?? robin,
  name: name,
  description: 'About $name',
  scopeKind: TargetKind.instance,
  options: options,
  requiredLevel: CommandLevel.trigger,
  control: control,
  version: const ResourceVersion(1),
);

const CommandOption modelOption = CommandOption(
  name: 'name',
  description: 'Model',
  type: CommandOptionType.string,
  required: true,
  choices: [],
  autocomplete: true,
);

final Map<PrincipalId, Principal> principals = {
  for (final p in people) p.id: p,
  nova: Principal(
    id: nova,
    kind: PrincipalKind.agent,
    handle: 'nova',
    displayName: 'Nova',
  ),
};

void main() {
  test('the operator first, equal names qualified with the handle, control '
      'commands marked', () {
    final commands = [
      command(1, 'stats', owner: nova),
      command(2, 'model'),
      command(3, 'stats'),
      command(4, 'deploy', owner: nova, control: true),
    ];
    final items = commandSuggestions(
      '',
      builtins: const [],
      commands: commands,
      operatorId: robin,
      principals: principals,
    );
    expect(
      [for (final i in items) i.token],
      ['model', 'stats', 'deploy', 'stats@nova'],
    );
    final rows = [for (final i in items) suggestionOf(en, i, principals)];
    expect(rows[3].originName, 'Nova');
    expect(rows[2].control, isTrue);
    expect(rows[0].control, isFalse);
    // A prefix narrows the list; a qualified token resolves to its command.
    expect(
      commandSuggestions(
        'st',
        builtins: const [],
        commands: commands,
        operatorId: robin,
        principals: principals,
      ).map((i) => i.token),
      ['stats', 'stats@nova'],
    );
    expect(
      commandOfToken(
        'stats@nova',
        commands: commands,
        operatorId: robin,
        principals: principals,
      )!.id,
      commands[0].id,
    );
  });

  test('options parse by position and by name; the last takes the rest', () {
    final c = command(
      1,
      'escalate',
      options: const [
        CommandOption(
          name: 'ticket',
          description: 'Ticket',
          type: CommandOptionType.string,
          required: true,
          choices: [],
          autocomplete: false,
        ),
        CommandOption(
          name: 'note',
          description: 'Note',
          type: CommandOptionType.string,
          required: false,
          choices: [],
          autocomplete: false,
        ),
      ],
    );
    expect(optionValues(c, '47 urgent please'), ['47', 'urgent please']);
    expect(optionValues(c, 'note:hi ticket:9'), ['9', 'hi']);
    expect(optionValues(c, ''), [null, null]);
    expect(activeOption(c, '47 ')!.option.name, 'note');
    expect(composerCommand(c, '').options.first.missing, isTrue);
    expect(invokeOptions(c, '47'), [(name: 'ticket', value: '47')]);
  });

  testWidgets('suggestions wait for the debounce; a newer request cancels the '
      'older one', (tester) async {
    final reads = FakeAgentReads()
      ..suggest = (option, value) => [
        CommandChoice(name: 'answer $value', value: value),
      ];
    final client = AutocompleteClient(
      debounce: const Duration(milliseconds: 150),
      fetch: reads.autocomplete,
    );
    addTearDown(client.dispose);
    AutocompleteRequest ask(String value) => (
      channel: channelId(1),
      command: command(1, 'model').id,
      thread: threadId(1),
      option: 'name',
      value: value,
      options: const [],
    );
    client.query(ask('m'));
    await tester.pump(const Duration(milliseconds: 100));
    client.query(ask('mo'));
    await tester.pump(const Duration(milliseconds: 100));
    expect(reads.autocompleteRequests, isEmpty, reason: 'still typing');
    final hold = reads.hold = Completer<void>();
    await tester.pump(const Duration(milliseconds: 60));
    expect([for (final r in reads.autocompleteRequests) r.$1.value], ['mo']);

    client.query(ask('mod'));
    await tester.pump(const Duration(milliseconds: 150));
    expect(
      [for (final r in reads.autocompleteRequests) r.$1.value],
      ['mo', 'mod'],
    );
    expect(reads.autocompleteRequests.first.$2.isCancelled, isTrue);
    hold.complete();
    await tester.pump();
    expect([for (final c in client.choices) c.value], ['mod']);
  });

  testWidgets('incomplete options block send with the reason; the complete '
      'command goes through invokeCommand', (tester) async {
    final host = await ColumnHost.open(tester);
    final reads = FakeAgentReads()
      ..suggest = (option, value) => [
        for (final m in const ['model-large', 'model-fast'])
          if (m.startsWith(value)) CommandChoice(name: m, value: m),
      ];
    final model = command(1, 'model', options: const [modelOption]);
    await host.run(tester, () async {
      await seedThread(
        host.db,
        th: thread(1, title: 'Deploy', operator: robin),
        messages: [msg(1, author: alex)],
        level: AccessLevel.trigger,
      );
      await host.db.commandsDao.upsertIfNewer(model);
    });
    await host.pump(
      tester,
      HistoryColumn(threadId: threadId(1)),
      location: '/t/${threadId(1).value}',
      overrides: [
        ...threadOverrides(FakeThreadReads(host.db)),
        runSourceProvider.overrideWithValue(StoreRunSource()),
        agentReadsProvider.overrideWithValue(reads),
      ],
    );
    await tester.enterText(composerField, '/mo');
    await host.settle(tester);
    expect(find.byType(DsSuggestionList), findsOneWidget);
    expect(find.text('/model'), findsWidgets);

    await tester.enterText(composerField, '/model ');
    await host.settle(tester);
    expect(find.text(en.composerOptionMissing('name')), findsOneWidget);
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await host.settle(tester);
    expect(await entriesOf(tester, host, AgentOps.invokeCommand), isEmpty);

    // The agent's suggestions after the debounce (`ui.autocomplete_debounce`).
    await tester.enterText(composerField, '/model model-l');
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));
    expect(
      reads.autocompleteRequests.where((r) => r.$1.value == 'model-l'),
      isEmpty,
      reason: 'the debounce has not passed',
    );
    await tester.pump(const Duration(milliseconds: 100));
    await host.settle(tester);
    expect(reads.autocompleteRequests.last.$1.value, 'model-l');
    expect(find.text('model-large'), findsOneWidget);
    expect(find.text('model-fast'), findsNothing);
    await tester.tap(find.text('model-large'));
    await host.settle(tester);
    expect(find.text(en.composerOptionMissing('name')), findsNothing);

    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await host.settle(tester);
    final invoked = await entriesOf(tester, host, AgentOps.invokeCommand);
    expect(invoked, hasLength(1));
    expect(invoked.single.payload, contains(model.id.value));
    expect(invoked.single.payload, contains('"value":"model-large"'));
    // A command is no message.
    expect(await entriesOf(tester, host, 'createMessage'), isEmpty);
    expect(tester.widget<TextField>(composerField).controller!.text, isEmpty);
    await host.dispose(tester);
  });

  test('a built-in name is never resolved to a registered command', () {
    final outcome = resolveCommand(
      '/stop',
      builtins: const [BuiltinCommand.stop],
      commands: [command(1, 'stop')],
      operatorId: robin,
      principals: principals,
    );
    expect(outcome, isA<SendBuiltin>());
  });
}
