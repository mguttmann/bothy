// SPDX-License-Identifier: Apache-2.0

// The import in the browser (brief M8-08 Design note 5): the owner adds a source from a
// white-label mapping (a wrong handle is shown at its place first), starts it with step-up, the
// flow posts a structure batch and the tool's counters through the API as the import tool
// would, the counters and two mirrored rows appear, this server and then Discord lead in a
// channel with the operator step each, completion waits until every channel is led here and warns about the
// retention class, and a second source is paused and cancelled. Every step runs under the
// enforced CSP of make console-e2e.

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:patrol/patrol.dart';

import 'support.dart';

const String _name = 'Old community server';

/// The mapping: a new space for category 900 with the text channel 901 (planned operator
/// [operator]) and the forum 902 without one.
String _mapping({String operator = 'agent:texter', String guild = '800'}) =>
    '''
version: 1
guild_id: "$guild"
defaults:
  unmapped_human: placeholder
  unmapped_bot: placeholder
  uncategorized_space: skip
spaces:
  - discord_category: "9$guild"
    target: {create: {name: "Imported $guild"}}
channels:
  - discord_channel: "91$guild"
    target: {create: {name: "general-$guild"}}
    planned_operator: "$operator"
  - discord_channel: "92$guild"
    target: {create: {name: "questions-$guild"}}
''';

/// The ID of the source named [name], from listImports.
Future<String> _sourceId(String name) async {
  final list = itemsOf(await consoleApi('GET', '/imports'), 'listImports');
  final s = list.firstWhere(
    (s) => s['name'] == name,
    orElse: () => throw TestFailure('no import source $name'),
  );
  return s['id']! as String;
}

/// Adds the source [name] with [mapping] through the dialog and waits for its page.
Future<void> _create(
  PatrolIntegrationTester $,
  String name,
  String mapping,
) async {
  await tapKey($, 'import.add');
  await typeKey($, 'import.create.name', name);
  await chooseMapping($, mapping);
  await waitFor($, byKey('import.create.file'));
  await tapKey($, 'import.create.submit');
  await tapKey($, 'confirm.submit');
}

/// Runs the state step [action] of the open source with its confirmation and waits for
/// [done].
Future<void> _step(
  PatrolIntegrationTester $,
  String action,
  String done,
) async {
  await tapKey($, 'import.action.$action');
  await tapKey($, 'confirm.submit');
  await waitFor($, find.text(done));
}

/// The phase text of the row of [discordId].
String _phase(PatrolIntegrationTester $, String discordId) =>
    $.tester.widget<Text>(byKey('import.channel.$discordId.phase')).data!;

void main() {
  patrolTest('Import add a source: a wrong handle at its place, then fixed', (
    $,
  ) async {
    await openConsole($, '/import');
    await _create($, _name, _mapping(operator: 'agent:nobody'));
    await waitFor(
      $,
      find.text(
        'channels[0].planned_operator: names no person or agent with this handle',
      ),
    );
    expectStepUp();
    await chooseMapping($, _mapping());
    await waitFor($, byKey('import.create.file'));
    await tapKey($, 'import.create.submit');
    await tapKey($, 'confirm.submit');
    await waitFor($, byKey('import.action.start'));
    expect(find.byKey(const ValueKey('import.next')), findsOneWidget);
  });

  patrolTest('Import start, then the tool writes and the page shows it', (
    $,
  ) async {
    final id = await _sourceId(_name);
    await openConsole($, '/import/$id');
    // The step-up of import.manage may still be fresh from adding the source (auth.step_up_max_age),
    // so this flow does not count it; the first flow proves the step-up of import.manage.
    await _step($, 'start', '$_name is started.');

    // The import tool's part, with the owner's session: structure, then cursor and counters.
    final batch = await consoleApi(
      'POST',
      '/imports/$id/batches',
      body: {
        'items': [
          {
            'origin_key': 'space:9800',
            'item_kind': 'space',
            'data': {'name': 'Imported 800', 'sort_key': 'a0'},
          },
          {
            'origin_key': 'channel:91800',
            'item_kind': 'channel',
            'data': {
              'space_origin_key': 'space:9800',
              'name': 'general-800',
              'kind': 'text',
              'sort_key': 'a0',
            },
          },
          {
            'origin_key': 'channel:92800',
            'item_kind': 'channel',
            'data': {
              'space_origin_key': 'space:9800',
              'name': 'questions-800',
              'kind': 'forum',
              'sort_key': 'a1',
            },
          },
        ],
      },
    );
    if (batch.status != 200 && batch.status != 201) {
      throw e2eFailure(
        'applyImportBatch answered ${batch.status}: ${batch.json}',
      );
    }
    final cursor = await consoleApi(
      'PUT',
      '/imports/$id/cursor',
      body: {
        'cursor': {'version': 1},
        'stats': {
          'threads': {'open': 2, 'done': 5},
        },
      },
    );
    if (cursor.status != 200) {
      throw e2eFailure(
        'setImportCursor answered ${cursor.status}: ${cursor.json}',
      );
    }

    await tapKey($, 'import.refresh');
    await waitFor($, find.text('channel.posted: 2'));
    await waitFor($, find.text('threads.done: 5'));
    await waitFor($, find.text('Mirror, read-only'));
    expect(_phase($, '91800'), 'Mirror, read-only');
    expect(_phase($, '92800'), 'Mirror, read-only');
  });

  patrolTest('Import lead here with the planned operator, then Discord leads', (
    $,
  ) async {
    final id = await _sourceId(_name);
    await openConsole($, '/import/$id');
    await tapKey($, 'import.channel.91800.leadHere');
    await waitFor($, find.textContaining('--channel 91800 --discord-locked'));
    await tapKey($, 'confirm.submit');
    await waitFor($, byKey('import.operator.dialog'));
    await typeKey($, 'import.operator.reason', 'Planned operator');
    await tapKey($, 'import.operator.submit');
    await waitFor($, find.text('Led here, operator ok'));
    expectStepUp();
    expect(
      find.byKey(const ValueKey('import.channel.91800.readOnly')),
      findsOneWidget,
    );
    expect(find.text('Writable'), findsOneWidget);

    await tapKey($, 'import.channel.91800.leadDiscord');
    await tapKey($, 'confirm.submit');
    await waitFor($, byKey('import.operator.dialog'));
    await typeKey($, 'import.operator.reason', 'Discord leads again');
    await tapKey($, 'import.operator.submit');
    await waitFor($, find.text('general-800 has no operator now.'));
    expect(_phase($, '91800'), 'Mirror, read-only');
    expect(find.text('Writable'), findsNothing);
  });

  patrolTest(
    'Import complete waits until every channel is led here and warns first',
    ($) async {
      final id = await _sourceId(_name);
      await openConsole($, '/import/$id');
      await waitFor($, byKey('import.completeHint'));
      expect(
        find.byKey(const ValueKey('import.action.complete')),
        findsNothing,
      );

      await tapKey($, 'import.channel.91800.leadHere');
      await tapKey($, 'confirm.submit');
      await waitFor($, byKey('import.operator.dialog'));
      await typeKey($, 'import.operator.reason', 'Planned operator');
      await tapKey($, 'import.operator.submit');
      await waitFor($, find.text('Texter is the operator of general-800.'));
      // No operator is planned for the forum, so no operator step follows.
      await tapKey($, 'import.channel.92800.leadHere');
      await tapKey($, 'confirm.submit');
      await waitFor($, find.text('questions-800 is led here.'));
      expectStepUp();

      await tapKey($, 'import.action.complete');
      await waitFor($, find.textContaining('retention class import'));
      expect(find.textContaining('no longer idempotent'), findsOneWidget);
      await tapKey($, 'confirm.submit');
      await waitFor($, find.text('$_name is completed.'));
      expect(find.byKey(const ValueKey('import.action.start')), findsNothing);
    },
  );

  patrolTest('Import pause and cancel a second source', ($) async {
    await openConsole($, '/import');
    await _create($, 'Second server', _mapping(guild: '700'));
    await waitFor($, byKey('import.action.start'));
    expectStepUp();
    await _step($, 'start', 'Second server is started.');
    await _step($, 'pause', 'Second server is paused.');
    await _step($, 'cancel', 'Second server is cancelled.');
    expect(find.text('discord  |  Cancelled'), findsOneWidget);
  });
}
