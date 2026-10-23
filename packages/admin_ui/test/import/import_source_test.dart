// SPDX-License-Identifier: Apache-2.0

// The page of one import source (brief M8-08 Design notes 1, 3, 4; fixed requirements 11 to
// 16): the state buttons follow the state, every step shows its confirmation and then the
// step-up, the 409 reasons read in plain language, "Lead here" names the order of 22 13.3 and
// leads into the operator step with the planned agent, "Discord leads" offers removing the
// operator, completion is offered only when every imported channel is led here and warns about
// the retention class, the page reads the server again on the refresh control and when the tab
// becomes visible, and no content, file name or person name reaches the screen.

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

import 'support.dart';

void main() {
  late GlobalKey<NavigatorState> navigator;
  late FakeAdminRepository repository;

  setUp(() {
    navigator = GlobalKey<NavigatorState>();
    repository = instanceRepository(navigator);
  });

  List<String> actions() => [
    for (final a in ImportStateAction.values)
      if (find.byKey(ValueKey('import.action.${a.name}')).evaluate().isNotEmpty)
        a.name,
  ];

  testWidgets('the state buttons follow the state', (tester) async {
    final draft = repository.seedImport('Old');
    await pumpSource(tester, repository, draft, navigator);
    expect(actions(), ['start', 'cancel']);
    expect(find.byKey(const ValueKey('import.next')), findsOneWidget);
    expect(
      find.textContaining('bothy import discord --guild 100 --map <file>'),
      findsNWidgets(2),
    );

    for (final (state, expected) in [
      ('running', ['pause', 'cancel', 'complete']),
      ('paused', ['start', 'cancel', 'complete']),
      ('completed', <String>[]),
      ('failed', <String>[]),
    ]) {
      final id = repository.seedImport('Old $state', state: state);
      await pumpSource(tester, repository, id, navigator);
      expect(actions(), expected, reason: state);
      expect(find.byKey(const ValueKey('import.next')), findsNothing);
    }
  });

  testWidgets(
    'start asks the confirmation, then the step-up, then reads again',
    (tester) async {
      final id = repository.seedImport('Old community server');
      await pumpSource(tester, repository, id, navigator);
      await tapKey(tester, 'import.action.start');
      expect(find.text('Start Old community server?'), findsOneWidget);
      await confirmChange(tester);
      await confirmStepUp(tester, 'Start the import Old community server');
      expect(repository.calls.last, 'startImport $id');
      expect(find.text('Old community server is started.'), findsOneWidget);
      expect(textOf(tester, 'import.state'), 'discord  |  Started');
      expect(actions(), ['pause', 'cancel', 'complete']);
    },
  );

  testWidgets('a cancelled confirmation or step-up changes nothing', (
    tester,
  ) async {
    final id = repository.seedImport('Old');
    await pumpSource(tester, repository, id, navigator);
    await tapKey(tester, 'import.action.cancel');
    await tester.tap(find.byKey(const ValueKey('confirm.cancel')));
    await tester.pumpAndSettle();
    expect(repository.calls, isEmpty);

    await tapKey(tester, 'import.action.cancel');
    await confirmChange(tester);
    await tester.tap(find.byKey(const ValueKey('stepUp.cancel')));
    await tester.pumpAndSettle();
    expect(repository.importsById[id]!['state'], 'draft');
    expect(find.byKey(const ValueKey('self.notice')), findsNothing);
  });

  testWidgets('cancel ends the source as cancelled', (tester) async {
    final id = repository.seedImport('Old', state: 'paused');
    repository.stepUpFresh = true;
    await pumpSource(tester, repository, id, navigator);
    await tapKey(tester, 'import.action.cancel');
    expect(find.textContaining('cannot be started again'), findsOneWidget);
    await confirmChange(tester);
    expect(textOf(tester, 'import.state'), 'discord  |  Cancelled');
    expect(actions(), isEmpty);
  });

  testWidgets('a state that changed meanwhile reads in plain language', (
    tester,
  ) async {
    final id = repository.seedImport('Old', state: 'running');
    repository.stepUpFresh = true;
    await pumpSource(tester, repository, id, navigator);
    // Another owner paused it after the page loaded; pause twice is no error, cancel then
    // complete is.
    repository.importsById[id]!['state'] = 'completed';
    await tapKey(tester, 'import.action.pause');
    await confirmChange(tester);
    expect(
      find.text(
        'The source is in another state now. The page shows the current state.',
      ),
      findsOneWidget,
    );
    expect(textOf(tester, 'import.state'), 'discord  |  Completed');
    expect(actions(), isEmpty);
  });

  testWidgets(
    'complete is offered only when every imported channel is led here',
    (tester) async {
      final id = repository.seedImport('Old', state: 'running');
      repository
        ..postChannels(id, ['301'])
        ..stepUpFresh = true;
      await pumpSource(tester, repository, id, navigator);
      expect(actions(), ['pause', 'cancel']);
      expect(find.byKey(const ValueKey('import.completeHint')), findsOneWidget);

      await repository.setImportLead(
        id,
        discordChannel: '301',
        lead: ImportLead.here,
        purpose: 'p',
      );
      await tapKey(tester, 'import.refresh');
      expect(actions(), ['pause', 'cancel', 'complete']);

      // The lead went back meanwhile: the server's 409 reads in plain language.
      await repository.setImportLead(
        id,
        discordChannel: '301',
        lead: ImportLead.discord,
        purpose: 'p',
      );
      await tapKey(tester, 'import.action.complete');
      expect(find.textContaining('retention class import'), findsOneWidget);
      expect(find.textContaining('no longer idempotent'), findsOneWidget);
      await confirmChange(tester);
      expect(
        find.text(
          'Not every imported channel is led here yet. Switch each channel over first.',
        ),
        findsOneWidget,
      );
      expect(repository.importsById[id]!['state'], 'running');
      expect(actions(), ['pause', 'cancel']);
    },
  );

  testWidgets(
    'the channel table shows Discord ID, channel here, phase and plan',
    (tester) async {
      final id = repository.seedImport('Old', state: 'running');
      repository.postChannels(id, ['301']);
      await pumpSource(tester, repository, id, navigator);
      expect(textOf(tester, 'import.channel.301.name'), 'tickets');
      expect(find.text('Discord ID 301'), findsOneWidget);
      expect(textOf(tester, 'import.channel.301.phase'), 'Mirror, read-only');
      expect(textOf(tester, 'import.channel.301.readOnly'), 'Read-only');
      expect(
        textOf(tester, 'import.channel.301.planned'),
        'Planned operator: Texter',
      );
      expect(
        find.byKey(const ValueKey('import.channel.301.leadHere')),
        findsOneWidget,
      );
      // Not imported: no lead action (the server would answer channel_not_imported).
      expect(textOf(tester, 'import.channel.302.phase'), 'Not imported');
      expect(
        textOf(tester, 'import.channel.302.planned'),
        'No operator planned',
      );
      expect(
        find.byKey(const ValueKey('import.channel.302.leadHere')),
        findsNothing,
      );
      expect(find.byKey(const ValueKey('import.channel.304')), findsNothing);
    },
  );

  testWidgets(
    'Lead here: the order of 22 13.3, the lead, then the planned operator',
    (tester) async {
      final id = repository.seedImport('Old', state: 'running');
      repository.postChannels(id, ['301']);
      await pumpSource(tester, repository, id, navigator);
      await tapKey(tester, 'import.channel.301.leadHere');
      final body = textOf(tester, 'confirm.body');
      expect(body, contains('In Discord you set the channel read-only'));
      expect(
        body,
        contains(
          'bothy import discord switch --channel 301 --discord-locked succeeded',
        ),
      );
      expect(body, contains('Then you set the operator Texter.'));
      expect(
        find.byKey(const ValueKey('import.confirm.warning')),
        findsNothing,
      );
      await confirmChange(tester);
      await confirmStepUp(tester, 'Change the lead of tickets');
      expect(repository.calls.last, 'setImportLead $id 301 here');

      // The operator step opens with the planned agent.
      expect(
        find.byKey(const ValueKey('import.operator.dialog')),
        findsOneWidget,
      );
      expect(find.text('Texter'), findsWidgets);
      expect(
        find.byKey(const ValueKey('import.operator.warning')),
        findsNothing,
      );
      final submit = find.byKey(const ValueKey('import.operator.submit'));
      expect(tester.widget<FilledButton>(submit).onPressed, isNull);
      await tester.enterText(
        find.byKey(const ValueKey('import.operator.reason')),
        'Planned operator of tickets',
      );
      await tester.pumpAndSettle();
      await tester.tap(submit);
      await tester.pumpAndSettle();
      expect(
        repository.calls.last,
        'setImportChannelOperator ch-301 texter Planned operator of tickets',
      );
      expect(find.text('Texter is the operator of tickets.'), findsOneWidget);
      expect(
        textOf(tester, 'import.channel.301.phase'),
        'Led here, operator ok',
      );
      expect(textOf(tester, 'import.channel.301.readOnly'), 'Writable');
    },
  );

  testWidgets('another agent than planned warns; skipping leaves no operator', (
    tester,
  ) async {
    final id = repository.seedImport('Old', state: 'running');
    repository
      ..postChannels(id, ['301'])
      ..stepUpFresh = true;
    await pumpSource(tester, repository, id, navigator);
    await tapKey(tester, 'import.channel.301.leadHere');
    await confirmChange(tester);
    await pickOption(tester, 'import.operator.agent', 'Support');
    expect(
      find.textContaining('Support is not the planned operator Texter'),
      findsOneWidget,
    );
    await tester.tap(find.byKey(const ValueKey('import.operator.skip')));
    await tester.pumpAndSettle();
    expect(textOf(tester, 'import.channel.301.phase'), 'Led here, no operator');
    // The row offers the operator step again.
    await tapKey(tester, 'import.channel.301.setOperator');
    expect(
      find.byKey(const ValueKey('import.operator.dialog')),
      findsOneWidget,
    );
  });

  testWidgets('a channel with another operator warns before it is led here', (
    tester,
  ) async {
    final id = repository.seedImport('Old', state: 'running');
    repository
      ..postChannels(id, ['301'])
      ..operatorElsewhere(id, '301', 'support');
    await pumpSource(tester, repository, id, navigator);
    await tapKey(tester, 'import.channel.301.leadHere');
    expect(
      find.byKey(const ValueKey('import.confirm.warning')),
      findsOneWidget,
    );
    expect(find.textContaining('Imported threads keep Texter'), findsOneWidget);
  });

  testWidgets('the operator step meets I1 and takes the guided option', (
    tester,
  ) async {
    final id = repository.seedImport(
      'Old',
      state: 'running',
      config: fakeMapping(plannedOperator: 'agent:support'),
    );
    repository
      ..postChannels(id, ['301'])
      ..stepUpFresh = true
      ..i1On['setImportChannelOperator'] = [
        i1Finding('support', holderId: 'alex', principalIds: const ['sam']),
      ];
    await pumpSource(tester, repository, id, navigator);
    await tapKey(tester, 'import.channel.301.leadHere');
    await confirmChange(tester);
    await tester.enterText(
      find.byKey(const ValueKey('import.operator.reason')),
      'Planned operator',
    );
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('import.operator.submit')));
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('guided.explain')), findsOneWidget);
    await tester.tap(find.byKey(const ValueKey('guided.offer')));
    await tester.pumpAndSettle();
    await tester.enterText(
      find.byKey(const ValueKey('confirm.reason')).last,
      'Support runs for the organization',
    );
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('confirm.submit')).last);
    await tester.pumpAndSettle();
    expect(
      repository.resolutions.single,
      contains('setImportChannelOperator support'),
    );
    expect(find.byKey(const ValueKey('guided.result')), findsOneWidget);
    await tester.tap(find.byKey(const ValueKey('guided.close')));
    await tester.pumpAndSettle();
    expect(textOf(tester, 'import.channel.301.phase'), 'Led here, operator ok');
  });

  testWidgets('Discord leads: read-only again, then removing the operator', (
    tester,
  ) async {
    final id = repository.seedImport('Old', state: 'running');
    repository
      ..postChannels(id, ['301'])
      ..stepUpFresh = true;
    await repository.setImportLead(
      id,
      discordChannel: '301',
      lead: ImportLead.here,
      purpose: 'p',
    );
    repository.operatorElsewhere(id, '301', 'texter');
    repository.stepUpFresh = false;
    await pumpSource(tester, repository, id, navigator);
    await tapKey(tester, 'import.channel.301.leadDiscord');
    // A destructive confirmation shows its body in the danger banner.
    expect(
      find.textContaining('read-only for everyone, owners too'),
      findsOneWidget,
    );
    expect(
      find.textContaining(
        'bothy import discord revert --channel 301 counts them',
      ),
      findsOneWidget,
    );
    await confirmChange(tester);
    await confirmStepUp(tester, 'Change the lead of tickets');
    expect(repository.calls.last, 'setImportLead $id 301 discord');
    expect(find.text('Remove the operator of tickets?'), findsOneWidget);
    await tester.enterText(
      find.byKey(const ValueKey('import.operator.reason')),
      'Discord leads again',
    );
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('import.operator.submit')));
    await tester.pumpAndSettle();
    expect(
      repository.calls.last,
      'setImportChannelOperator ch-301 - Discord leads again',
    );
    expect(find.text('tickets has no operator now.'), findsOneWidget);
    expect(textOf(tester, 'import.channel.301.phase'), 'Mirror, read-only');
    expect(textOf(tester, 'import.channel.301.readOnly'), 'Read-only');
  });

  testWidgets('no lead action on a completed source', (tester) async {
    final id = repository.seedImport('Old', state: 'running');
    repository.postChannels(id, ['301']);
    repository.importsById[id]!['state'] = 'completed';
    await pumpSource(tester, repository, id, navigator);
    expect(
      find.byKey(const ValueKey('import.channel.301.leadHere')),
      findsNothing,
    );
    expect(
      find.byKey(const ValueKey('import.channel.301.leadDiscord')),
      findsNothing,
    );
  });

  testWidgets(
    'the page reads again on refresh and when the tab becomes visible',
    (tester) async {
      final id = repository.seedImport('Old', state: 'running');
      await pumpSource(tester, repository, id, navigator);
      expect(
        find.byKey(const ValueKey('import.counter.server.channel.posted')),
        findsNothing,
      );

      repository.postChannels(id, ['301']);
      await tapKey(tester, 'import.refresh');
      expect(
        textOf(tester, 'import.counter.server.channel.posted'),
        'channel.posted: 1',
      );

      repository.postChannels(id, ['302']);
      await setLifecycle(tester, [
        AppLifecycleState.inactive,
        AppLifecycleState.hidden,
      ]);
      expect(
        textOf(tester, 'import.counter.server.channel.posted'),
        'channel.posted: 1',
      );
      // The tab becomes visible again.
      await setLifecycle(tester, [
        AppLifecycleState.inactive,
        AppLifecycleState.resumed,
      ]);
      expect(
        textOf(tester, 'import.counter.server.channel.posted'),
        'channel.posted: 2',
      );
    },
  );

  testWidgets('no content, file name or person name reaches the screen', (
    tester,
  ) async {
    final id = repository.seedImport(
      'Old',
      state: 'running',
      stats: const {
        'server': {
          'message': {'posted': 3},
          'note': 'Hello from the old server',
        },
        'tool': {
          'last_message': 'Secret plans for Friday',
          'attachment': 'contract.pdf',
          'author': 'oldfriend42',
          'people': ['oldfriend42'],
          'messages': {'taken_over': 3},
        },
      },
    );
    repository.importItemsById[id]!.addAll(const [
      ImportItemEntry(
        id: 'item-a',
        originKey: 'forum_tag:302:9',
        itemKind: 'forum_tag',
        state: 'failed',
        error: 'name_collision',
      ),
      ImportItemEntry(
        id: 'item-b',
        originKey: 'message:500:0',
        itemKind: 'message',
        state: 'failed',
        error: 'could not post "Secret plans for Friday" by oldfriend42',
      ),
    ]);
    await pumpSource(tester, repository, id, navigator);
    final texts = [
      for (final e in find.byType(Text).evaluate())
        (e.widget as Text).data ??
            (e.widget as Text).textSpan?.toPlainText() ??
            '',
    ].join('\n');
    for (final secret in [
      'Hello from the old server',
      'Secret plans',
      'contract.pdf',
      'oldfriend42',
      'user:alex',
    ]) {
      expect(texts, isNot(contains(secret)), reason: secret);
    }
    expect(
      textOf(tester, 'import.counter.tool.messages.taken_over'),
      'messages.taken_over: 3',
    );
    expect(
      textOf(tester, 'import.failed.item-a'),
      'forum_tag forum_tag:302:9: name_collision',
    );
    expect(textOf(tester, 'import.failed.item-b'), 'message message:500:0: ');
  });

  testWidgets('a refused source shows the reason and nothing else', (
    tester,
  ) async {
    final id = repository.seedImport('Old');
    repository.refuse['importSource'] = const ApiProblem(
      status: 403,
      code: 'forbidden',
      reason: 'owner_only',
    );
    await pumpSource(tester, repository, id, navigator);
    expect(find.byKey(const ValueKey('admin.denied')), findsOneWidget);
    expect(actions(), isEmpty);
    expect(find.byKey(const ValueKey('import.channels')), findsNothing);
  });

  testWidgets('German texts', (tester) async {
    final id = repository.seedImport('Alt', state: 'running');
    repository.postChannels(id, ['301']);
    await pumpSource(
      tester,
      repository,
      id,
      navigator,
      locale: const Locale('de'),
    );
    final de = lookupAppLocalizations(const Locale('de'));
    expect(textOf(tester, 'import.channel.301.phase'), de.importPhaseMirror);
    expect(find.text(de.importLeadHereAction), findsOneWidget);
    expect(
      de.importPhaseMirror,
      isNot(lookupAppLocalizations(const Locale('en')).importPhaseMirror),
    );
  });
}
