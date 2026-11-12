// SPDX-License-Identifier: Apache-2.0

// Stop of a run as an action with an outcome (01 6.2; docs/decisions.d/AUD-client-misc-01.md,
// ruling AUD-CM01/3): one activation puts one cancelRun into the outbox, the Stop of every view
// of the run is busy or gone while it is on its way and after the server accepted it, and a
// Stop the server refuses for good is discarded from the outbox, says why and is offered again.
// The rows and cards of "Running now", the run graph, the live box of a thread and `/stop`
// share the state. `stop_run_fake_server_test.dart` runs the refusals against the fake server.

import 'package:client/agents/providers.dart';
import 'package:client/running/providers.dart';
import 'package:client/running/running_now_view.dart';
import 'package:client/sidebar/providers.dart';
import 'package:client/sync/message_ops_app.dart';
import 'package:client/thread/history_column.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/misc.dart' show Override;
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';
import 'package:local_store/local_store.dart';

import '../support/agent_fakes.dart';
import '../support/column_host.dart';
import '../support/structure.dart';
import '../support/thread_fixtures.dart';
import '../support/thread_host.dart';
import '../support/until.dart';
import '../support/widgets.dart';

final List<Override> _overrides = [
  runningRefreshProvider.overrideWith((ref) async => true),
  runningClockProvider.overrideWith((ref) => Stream.value(t0)),
  ownPrincipalProvider.overrideWith((ref) async => people.first),
];

Finder _row(int n) => find.byKey(ValueKey('running.row.${runId(n).value}'));
Finder _rowStop(int n) =>
    find.byKey(ValueKey('running.stop.${runId(n).value}'));
Finder _cardStop(int n) =>
    find.byKey(ValueKey('runCard.stop.${runId(n).value}'));
Finder _cardOpen(int n) =>
    find.byKey(ValueKey('runCard.open.${runId(n).value}'));
Finder _refused(int n) =>
    find.byKey(ValueKey('running.stopRefused.${runId(n).value}'));

// Alex may trigger in channel 1 and started run 1 there.
Future<void> _fill(LocalDatabase db) async {
  await db.structureDao.upsertSpaceIfNewer(space(1, name: 'Engineering'));
  await db.structureDao.upsertChannelIfNewer(
    channel(1, spaceN: 1, name: 'server'),
  );
  await db.structureDao.putChannelAccess(
    channelId(1),
    const AccessView(level: AccessLevel.trigger, capabilities: []),
  );
  await db.threadsDao.upsertIfNewer(thread(1, channelN: 1, title: 'Gateway'));
  for (final p in people) {
    await db.principalsDao.put(p);
  }
  await db.runsDao.upsertIfNewer(
    run(1, RunState.running, threadN: 1, channelN: 1).copyWith(startedAt: t0),
  );
}

/// "Running now" with run 1 of Alex, as cards (the default) or as the list.
Future<ColumnHost> _running(WidgetTester tester, {required bool list}) async {
  final host = await ColumnHost.open(tester);
  await tester.runAsync(() => _fill(host.db));
  await host.pump(tester, const RunningNowView(), overrides: _overrides);
  if (list) {
    await tester.tap(find.byKey(const ValueKey('running.form.list')));
    await host.settle(tester);
    await pumpUntil(tester, () => _row(1).evaluate().isNotEmpty, what: 'rows');
  } else {
    await pumpUntil(
      tester,
      () => _cardStop(1).evaluate().isNotEmpty,
      what: 'cards',
    );
  }
  return host;
}

Future<List<OutboxEntry>> _stops(WidgetTester tester, ColumnHost host) async {
  final all = await host.run(tester, host.db.outboxDao.allEntries);
  return [
    for (final e in all)
      if (e.kind == AppMessageOps.cancelRun) e,
  ];
}

/// The server refuses the Stop [entry] for good, as the outbox records it.
Future<void> _refuse(
  WidgetTester tester,
  ColumnHost host,
  OutboxEntry entry, {
  required String code,
  String? reason,
}) async {
  await host.run(
    tester,
    () => host.db.outboxDao.recordFailure(entry.id, code: code, reason: reason),
  );
  await host.settle(tester);
}

bool _busy(WidgetTester tester, Finder button) {
  final b = tester.widget<DsButton>(button);
  return b.loading && b.onPressed == null;
}

void main() {
  testWidgets('one activation sends one cancelRun; the row is busy while the '
      'Stop is on its way', (tester) async {
    final host = await _running(tester, list: true);
    expect(_busy(tester, _rowStop(1)), isFalse);
    await tester.tap(_rowStop(1));
    await host.settle(tester);
    // The idle engine sends nothing: the entry waits, as it does without a connection.
    await tester.tap(_rowStop(1), warnIfMissed: false);
    await host.settle(tester);
    final sent = await _stops(tester, host);
    expect(sent, hasLength(1));
    expect(sent.single.payload, contains(runId(1).value));
    expect(sent.single.threadId, isNull);
    expect(_busy(tester, _rowStop(1)), isTrue);
    expect(_refused(1), findsNothing);
    await host.dispose(tester);
  });

  testWidgets('a Stop the server accepted stays taken and says nothing', (
    tester,
  ) async {
    final host = await _running(tester, list: true);
    await tester.tap(_rowStop(1));
    await host.settle(tester);
    final entry = (await _stops(tester, host)).single;
    // The outbox removes an entry the server confirmed; the run ends with its event.
    await host.run(tester, () => host.db.outboxDao.delete(entry.id));
    await host.settle(tester);
    expect(_busy(tester, _rowStop(1)), isTrue);
    expect(_refused(1), findsNothing);
    await host.dispose(tester);
  });

  testWidgets('a refused Stop is discarded, says why and is offered again', (
    tester,
  ) async {
    final host = await _running(tester, list: true);
    await tester.tap(_rowStop(1));
    await host.settle(tester);
    // 409: the run ended before the Stop arrived (runs/app/cancel.go).
    await _refuse(
      tester,
      host,
      (await _stops(tester, host)).single,
      code: 'invalid_state_transition',
    );
    expect(
      await _stops(tester, host),
      isEmpty,
      reason: 'no failed entry stays',
    );
    expect(_refused(1), findsOneWidget);
    expect(
      find.text(
        en.runningStopRefused(problemText(en, 'invalid_state_transition')),
      ),
      findsOneWidget,
    );
    expect(_busy(tester, _rowStop(1)), isFalse);
    // Stop works again: a new entry.
    await tester.tap(_rowStop(1));
    await host.settle(tester);
    expect(await _stops(tester, host), hasLength(1));
    await host.dispose(tester);
  });

  testWidgets('a refusal with a reason code shows the text of the reason', (
    tester,
  ) async {
    final host = await _running(tester, list: true);
    await tester.tap(_rowStop(1));
    await host.settle(tester);
    await _refuse(
      tester,
      host,
      (await _stops(tester, host)).single,
      code: 'agent_invocation_denied',
      reason: 'operator_not_in_channel',
    );
    expect(
      find.text(en.runningStopRefused(en.triggerReasonOperatorNotInChannel)),
      findsOneWidget,
    );
    await host.dispose(tester);
  });

  testWidgets('a card offers no Stop while one is on its way and offers it '
      'again after a refusal', (tester) async {
    final host = await _running(tester, list: false);
    await tester.tap(_cardStop(1));
    await host.settle(tester);
    expect(await _stops(tester, host), hasLength(1));
    expect(_cardStop(1), findsNothing);
    expect(_cardOpen(1), findsOneWidget);
    // The list shows the same Stop as on its way: the state belongs to the run.
    await tester.tap(find.byKey(const ValueKey('running.form.list')));
    await host.settle(tester);
    await pumpUntil(tester, () => _row(1).evaluate().isNotEmpty, what: 'rows');
    expect(_busy(tester, _rowStop(1)), isTrue);
    await _refuse(
      tester,
      host,
      (await _stops(tester, host)).single,
      code: 'forbidden',
    );
    expect(_refused(1), findsOneWidget);
    expect(_busy(tester, _rowStop(1)), isFalse);
    await tester.tap(find.byKey(const ValueKey('running.form.cards')));
    await host.settle(tester);
    await pumpUntil(
      tester,
      () => _cardStop(1).evaluate().isNotEmpty,
      what: 'the Stop of the card',
    );
    await host.dispose(tester);
  });

  testWidgets('the live box of a thread offers no second Stop and shows a '
      'refusal', (tester) async {
    final host = await ColumnHost.open(tester);
    await host.run(tester, () async {
      await seedThread(
        host.db,
        messages: [msg(1, author: alex, text: '@robin run the tests')],
        level: AccessLevel.trigger,
      );
      await host.db.runsDao.upsertIfNewer(run(1, RunState.running));
    });
    await host.pump(
      tester,
      HistoryColumn(threadId: threadId(1)),
      location: '/t/${threadId(1).value}',
      overrides: [
        ...threadOverrides(FakeThreadReads(host.db)),
        runSourceProvider.overrideWithValue(StoreRunSource()),
        agentReadsProvider.overrideWithValue(FakeAgentReads()),
      ],
    );
    final stop = find.text(en.liveBoxStop);
    await pumpUntil(tester, () => stop.evaluate().isNotEmpty, what: 'Stop');
    await tester.tap(stop);
    await host.settle(tester);
    expect(await _stops(tester, host), hasLength(1));
    expect(
      stop,
      findsNothing,
      reason: 'no second Stop while one is on its way',
    );
    await _refuse(
      tester,
      host,
      (await _stops(tester, host)).single,
      code: 'invalid_state_transition',
    );
    expect(_refused(1), findsOneWidget);
    expect(stop, findsOneWidget);
    expect(await _stops(tester, host), isEmpty);
    await host.dispose(tester);
  });
}
