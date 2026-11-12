// SPDX-License-Identifier: Apache-2.0

// The built-in commands of 09 10.1 (brief M4-21 Tests): the six are suggested, never sent
// through `invokeCommand`, and each calls its own operation; the command loader fills the
// local table from `listChannelCommands` when the channel opens.

import 'package:client/agents/agent_reads.dart';
import 'package:client/agents/providers.dart';
import 'package:client/sync/agent_ops.dart';
import 'package:client/sync/message_ops_app.dart';
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

final Command remote = Command(
  id: CommandId.parse(uuid(0x901)),
  ownerPrincipalId: robin,
  name: 'deploy',
  description: 'Deploy the service',
  scopeKind: TargetKind.instance,
  options: const [],
  requiredLevel: CommandLevel.trigger,
  control: false,
  version: const ResourceVersion(1),
);

Future<(ColumnHost, FakeAgentReads)> open(
  WidgetTester tester, {
  bool running = true,
}) async {
  final host = await ColumnHost.open(tester);
  final reads = FakeAgentReads()
    ..channelCommands[channelId(1)] = [remote]
    ..sessionsOf[threadId(1)] = <AgentSessionView>[
      (
        id: uuid(0xb01),
        externalSessionRef: 'sess-now',
        current: true,
        model: 'model-large',
        cost: const Money(amountMicros: 0, currency: 'USD', isEstimate: true),
        createdAt: t0,
        endedAt: null,
      ),
      (
        id: uuid(0xb02),
        externalSessionRef: 'sess-earlier',
        current: false,
        model: null,
        cost: const Money(amountMicros: 0, currency: 'USD', isEstimate: true),
        createdAt: t0.subtract(const Duration(days: 1)),
        endedAt: t0,
      ),
    ];
  await host.run(tester, () async {
    await seedThread(
      host.db,
      th: thread(1, title: 'Deploy', operator: robin, lastSeq: 1),
      messages: [msg(1, author: alex)],
      level: AccessLevel.trigger,
      capabilities: const [Capability.manage],
    );
    if (running) await host.db.runsDao.upsertIfNewer(run(1, RunState.running));
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
  return (host, reads);
}

Future<void> send(WidgetTester tester, ColumnHost host, String text) async {
  await tester.enterText(composerField, text);
  await host.settle(tester);
  // Close the suggestion list first; Enter would accept its active row.
  await tester.sendKeyEvent(LogicalKeyboardKey.escape);
  await host.settle(tester);
  await tester.sendKeyEvent(LogicalKeyboardKey.enter);
  await host.settle(tester);
}

void main() {
  testWidgets('the six built-ins are suggested with the commands of the '
      'channel; the loader filled the table', (tester) async {
    final (host, _) = await open(tester);
    final stored = await host.run(
      tester,
      () => host.db.commandsDao.watchAll().first,
    );
    expect(stored.map((c) => c.name), ['deploy']);
    await tester.enterText(composerField, '/');
    await host.settle(tester);
    final list = tester.widget<DsSuggestionList>(find.byType(DsSuggestionList));
    expect(
      [for (final s in list.suggestions) s.command],
      ['/stop', '/status', '/new', '/resume', '/title', '/close', '/deploy'],
    );
    expect(list.suggestions.take(6).every((s) => s.control), isTrue);
    expect(list.suggestions.first.description, en.commandBuiltinStop);
    await host.dispose(tester);
  });

  testWidgets('each built-in calls its own operation, never invokeCommand', (
    tester,
  ) async {
    final (host, _) = await open(tester);
    Future<List<String>> kinds() async => [
      for (final e in await host.run(
        tester,
        () => host.db.outboxDao.allEntries(),
      ))
        if (e.kind != 'putThreadDraft' && e.kind != 'putReadState') e.kind,
    ];

    await send(tester, host, '/stop');
    expect(await kinds(), [AppMessageOps.cancelRun]);

    await send(tester, host, '/new');
    expect((await kinds()).last, AgentOps.resetAgentSession);

    await send(tester, host, '/title Release checklist');
    final titles = await entriesOf(tester, host, AppMessageOps.setThreadTitle);
    expect(titles.single.payload, contains('Release checklist'));

    await send(tester, host, '/close');
    expect((await kinds()).last, AppMessageOps.closeThread);

    await send(tester, host, '/resume');
    expect(find.byKey(const ValueKey('command.resume')), findsOneWidget);
    expect(find.text('sess-now'), findsNothing);
    await tester.tap(find.text('sess-earlier'));
    await host.settle(tester);
    final binds = await entriesOf(tester, host, AgentOps.bindAgentSession);
    expect(binds.single.payload, contains('sess-earlier'));
    expect(binds.single.payload, contains(uuid(0xb01)));

    await send(tester, host, '/status');
    expect(find.byKey(const ValueKey('command.status')), findsOneWidget);
    expect(find.textContaining('sess-now'), findsOneWidget);
    await tester.tap(find.text(en.threadCancel));
    await host.settle(tester);

    expect(await entriesOf(tester, host, AgentOps.invokeCommand), isEmpty);
    expect(await entriesOf(tester, host, 'createMessage'), isEmpty);
    await host.dispose(tester);
  });

  testWidgets('/resume and /status say that the sessions could not be loaded '
      'instead of showing none', (tester) async {
    final (host, reads) = await open(tester);
    reads.sessionsError = Exception('offline');

    await send(tester, host, '/resume');
    expect(find.byKey(const ValueKey('command.resume')), findsOneWidget);
    expect(find.text(en.commandSessionsFailed), findsOneWidget);
    expect(find.text(en.commandResumeEmpty), findsNothing);
    await tester.tapAt(const Offset(5, 5));
    await host.settle(tester);
    expect(find.byKey(const ValueKey('command.resume')), findsNothing);

    // /status still shows the run this device knows.
    await send(tester, host, '/status');
    final status = find.byKey(const ValueKey('command.status'));
    expect(status, findsOneWidget);
    expect(
      find.descendant(of: status, matching: find.byType(DsLiveBox)),
      findsOneWidget,
    );
    expect(find.text(en.commandSessionsFailed), findsOneWidget);
    await tester.tap(find.text(en.threadCancel));
    await host.settle(tester);

    // A thread without sessions still reads "No earlier sessions".
    reads
      ..sessionsError = null
      ..sessionsOf[threadId(1)] = const [];
    await send(tester, host, '/resume');
    expect(find.text(en.commandResumeEmpty), findsOneWidget);
    expect(find.text(en.commandSessionsFailed), findsNothing);
    await tester.tapAt(const Offset(5, 5));
    await host.settle(tester);
    await host.dispose(tester);
  });

  testWidgets('a built-in in another letter case is the built-in, never a '
      'message', (tester) async {
    final (host, _) = await open(tester);
    await send(tester, host, '/Stop');
    expect(await entriesOf(tester, host, 'createMessage'), isEmpty);
    expect(
      await entriesOf(tester, host, AppMessageOps.cancelRun),
      hasLength(1),
    );
    await send(tester, host, '/TITLE Release checklist');
    final titles = await entriesOf(tester, host, AppMessageOps.setThreadTitle);
    expect(titles.single.payload, contains('Release checklist'));
    expect(await entriesOf(tester, host, 'createMessage'), isEmpty);
    await host.dispose(tester);
  });

  testWidgets('a built-in in another letter case that is not allowed here is '
      'never sent as a message', (tester) async {
    final (host, _) = await open(tester, running: false);
    await send(tester, host, '/STOP');
    expect(await entriesOf(tester, host, 'createMessage'), isEmpty);
    expect(await entriesOf(tester, host, AppMessageOps.cancelRun), isEmpty);
    expect(find.text(en.commandNotAvailable('/stop')), findsOneWidget);
    await host.dispose(tester);
  });

  testWidgets(
    'a built-in that is not allowed here is never sent as a message',
    (tester) async {
      // No active run: /stop is not offered and must not reach the model as a prompt.
      final (host, _) = await open(tester, running: false);
      await send(tester, host, '/stop');
      expect(await entriesOf(tester, host, 'createMessage'), isEmpty);
      expect(await entriesOf(tester, host, AppMessageOps.cancelRun), isEmpty);
      expect(find.text(en.commandNotAvailable('/stop')), findsOneWidget);
      expect(tester.widget<TextField>(composerField).controller!.text, '/stop');
      await host.dispose(tester);
    },
  );
}
