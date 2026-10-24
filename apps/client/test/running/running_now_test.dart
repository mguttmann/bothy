// SPDX-License-Identifier: Apache-2.0

// Running now D10 (brief M4-23 Tests; 01 5.1 D10, 6.7): the rows of the active runs with
// state, agent, path and costs, the filters Space, Agent and "mine only", Stop only where the
// server's answers allow `run.cancel`, and the footer with the organization sum as an estimate
// and the note on personal runs.

import 'package:client/running/providers.dart';
import 'package:client/running/running_now_view.dart';
import 'package:client/running/running_sum.dart';
import 'package:client/sidebar/providers.dart';
import 'package:client/sync/message_ops_app.dart';
import 'package:design_system/design_system.dart' show dsIsolate;
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_riverpod/misc.dart' show Override;
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';

import '../support/column_host.dart';
import '../support/structure.dart';
import '../support/thread_fixtures.dart';
import '../support/until.dart';
import '../support/widgets.dart';

final PrincipalId scout = PrincipalId.parse(uuid(0xa4));

RunSummary active(
  int n,
  RunState state, {
  required int channelN,
  PrincipalId? triggeredBy,
  PrincipalId? agent,
  BillingMode mode = BillingMode.organization,
  int micros = 0,
}) => run(n, state, threadN: n, channelN: channelN, triggeredBy: triggeredBy)
    .copyWith(
      agentId: agent,
      billingModeExpected: mode,
      cost: Money(amountMicros: micros, currency: 'USD', isEstimate: true),
      startedAt: state == RunState.queued ? null : t0,
    );

// Channel 1 in space 1: Alex may trigger. Channel 2 in space 2: Alex may only write.
// Channel 3 in space 2: Alex may trigger and holds `manage`.
Future<void> fill(LocalDatabase db) async {
  await db.structureDao.upsertSpaceIfNewer(space(1, name: 'Engineering'));
  await db.structureDao.upsertSpaceIfNewer(space(2, name: 'Research'));
  final access = {
    1: const AccessView(level: AccessLevel.trigger, capabilities: []),
    2: const AccessView(level: AccessLevel.write, capabilities: []),
    3: const AccessView(
      level: AccessLevel.trigger,
      capabilities: [Capability.manage],
    ),
  };
  for (final MapEntry(key: n, value: view) in access.entries) {
    await db.structureDao.upsertChannelIfNewer(
      channel(n, spaceN: n == 1 ? 1 : 2, name: 'server-$n'),
    );
    await db.structureDao.putChannelAccess(channelId(n), view);
    await db.threadsDao.upsertIfNewer(
      thread(n, channelN: n, title: 'Gateway $n'),
    );
  }
  for (final p in [
    ...people,
    Principal(
      id: scout,
      kind: PrincipalKind.agent,
      handle: 'scout',
      displayName: 'Scout',
    ),
  ]) {
    await db.principalsDao.put(p);
  }
  for (final r in [
    // Alex's own run: Stop shows.
    active(1, RunState.running, channelN: 1, micros: 300000),
    // Sam's run in a channel where Alex may trigger but does not manage: no Stop.
    active(2, RunState.waitingInput, channelN: 1, triggeredBy: sam),
    // Alex may only write in channel 2: no Stop, even for an own run.
    active(
      3,
      RunState.queued,
      channelN: 2,
      agent: scout,
      mode: BillingMode.personal,
    ),
    // Sam's run where Alex holds `manage`: Stop shows.
    active(
      4,
      RunState.running,
      channelN: 3,
      triggeredBy: sam,
      agent: scout,
      micros: 500000,
    ),
  ]) {
    await db.runsDao.upsertIfNewer(r);
  }
}

final List<Override> overrides = [
  runningRefreshProvider.overrideWith((ref) async => true),
  runningClockProvider.overrideWith(
    (ref) => Stream.value(t0.add(const Duration(minutes: 4, seconds: 12))),
  ),
  ownPrincipalProvider.overrideWith((ref) async => people.first),
];

Finder row(int n) => find.byKey(ValueKey('running.row.${uuid(0x700 + n)}'));
Finder stop(int n) => find.byKey(ValueKey('running.stop.${uuid(0x700 + n)}'));

Future<ColumnHost> pumpView(WidgetTester tester) async {
  final host = await ColumnHost.open(tester);
  await tester.runAsync(() => fill(host.db));
  await host.pump(tester, const RunningNowView(), overrides: overrides);
  await pumpUntil(tester, () => row(4).evaluate().isNotEmpty, what: 'rows');
  return host;
}

void main() {
  group('sumRunningCosts', () {
    test('sums organization runs per currency and notes personal ones', () {
      final sum = sumRunningCosts([
        active(1, RunState.running, channelN: 1, micros: 300000),
        active(2, RunState.running, channelN: 1, micros: 500000),
        active(
          3,
          RunState.running,
          channelN: 1,
          micros: 900000,
          mode: BillingMode.personal,
        ),
      ]);
      expect(sum.organizationMicros, {'USD': 800000});
      expect(sum.hasOrganization, isTrue);
      expect(sum.hasPersonal, isTrue);
    });

    test('a mode this version does not know counts as information only', () {
      final sum = sumRunningCosts([
        active(1, RunState.running, channelN: 1, mode: BillingMode.unknown),
      ]);
      expect(sum.hasOrganization, isFalse);
      expect(sum.hasPersonal, isTrue);
      expect(sum.organizationMicros, isEmpty);
    });

    test('the reported mode wins over the expected one', () {
      final r = active(
        1,
        RunState.running,
        channelN: 1,
        micros: 10,
      ).copyWith(billingModeUsed: BillingMode.personal);
      expect(sumRunningCosts([r]).hasOrganization, isFalse);
    });
  });

  group('mayStop', () {
    final mine = active(1, RunState.running, channelN: 1);
    final other = active(2, RunState.running, channelN: 1, triggeredBy: sam);
    const trigger = AccessView(level: AccessLevel.trigger, capabilities: []);
    const manage = AccessView(
      level: AccessLevel.trigger,
      capabilities: [Capability.manage],
    );
    const write = AccessView(
      level: AccessLevel.write,
      capabilities: [Capability.manage],
    );
    test('the starter with trigger may stop', () {
      expect(mayStop(mine, trigger, alex), isTrue);
    });
    test('another person needs manage', () {
      expect(mayStop(other, trigger, alex), isFalse);
      expect(mayStop(other, manage, alex), isTrue);
    });
    test('without trigger or without a reported access nobody may stop', () {
      expect(mayStop(mine, write, alex), isFalse);
      expect(mayStop(mine, null, alex), isFalse);
    });
  });

  testWidgets(
    'rows with state, agent, path and costs; Stop only where allowed',
    (tester) async {
      final host = await pumpView(tester);
      for (var n = 1; n <= 4; n++) {
        expect(row(n), findsOneWidget, reason: 'run $n');
      }
      expect(find.text(en.runningTitle(4)), findsOneWidget);
      expect(stop(1), findsOneWidget, reason: 'own run with trigger');
      expect(stop(2), findsNothing, reason: 'foreign run without manage');
      expect(stop(3), findsNothing, reason: 'no trigger in the channel');
      expect(stop(4), findsOneWidget, reason: 'manage in the channel');
      expect(
        find.descendant(
          of: row(1),
          matching: find.text(
            ['Engineering', 'server-1', 'Gateway 1'].map(dsIsolate).join(' > '),
          ),
        ),
        findsOneWidget,
      );
      expect(
        find.descendant(of: row(1), matching: find.text('4:12')),
        findsOneWidget,
      );
      expect(
        find.descendant(of: row(3), matching: find.text(en.liveBoxStateQueued)),
        findsOneWidget,
      );
      expect(
        find.descendant(
          of: row(2),
          matching: find.text(en.liveBoxStateWaitingForYou),
        ),
        findsOneWidget,
      );
      expect(
        find.text(
          '${en.runningSumOrganization('USD 0.80', en.costEstimate)} '
          '${en.runningPersonalNote}',
        ),
        findsOneWidget,
      );
      await host.dispose(tester);
    },
  );

  testWidgets('filters by space, agent and mine only', (tester) async {
    final host = await pumpView(tester);
    await tester.tap(find.byKey(const ValueKey('running.filter.mine')));
    await host.settle(tester);
    expect(row(1), findsOneWidget);
    expect(row(3), findsOneWidget);
    expect(row(2), findsNothing);
    expect(row(4), findsNothing);
    await tester.tap(find.byKey(const ValueKey('running.filter.mine')));
    await host.settle(tester);

    await tester.tap(find.byKey(const ValueKey('running.filter.space')));
    await host.settle(tester);
    await tester.tap(
      find.byKey(ValueKey('running.space.${spaceId(2).value}')).last,
    );
    await host.settle(tester);
    expect(row(1), findsNothing);
    expect(row(3), findsOneWidget);
    expect(row(4), findsOneWidget);

    await tester.tap(find.byKey(const ValueKey('running.filter.agent')));
    await host.settle(tester);
    await tester.tap(find.byKey(ValueKey('running.agent.${scout.value}')).last);
    await host.settle(tester);
    expect(row(3), findsOneWidget);
    expect(row(4), findsOneWidget);
    expect(find.text(en.runningTitle(2)), findsOneWidget);
    // Only the personal run is left after "mine only": the footer drops the organization sum.
    await tester.tap(find.byKey(const ValueKey('running.filter.mine')));
    await host.settle(tester);
    expect(find.text(en.runningPersonalNote), findsOneWidget);
    await host.dispose(tester);
  });

  testWidgets(
    'Stop enqueues cancelRun in the run lane, Open goes to the thread',
    (tester) async {
      final host = await pumpView(tester);
      await tester.tap(stop(1));
      await host.settle(tester);
      final entries = (await tester.runAsync(host.db.outboxDao.allEntries))!;
      expect(entries.map((e) => e.kind), [AppMessageOps.cancelRun]);
      expect(entries.single.payload, contains(uuid(0x701)));
      await tester.tap(find.byKey(ValueKey('running.open.${uuid(0x704)}')));
      await host.settle(tester);
      expect(host.visited.last, '/t/${threadId(4).value}');
      await host.dispose(tester);
    },
  );

  testWidgets(
    'the counter of the sidebar counts the active runs of the table',
    (tester) async {
      final host = await pumpView(tester);
      final container = ProviderScope.containerOf(
        tester.element(find.byType(RunningNowView)),
      );
      final count = container.listen(activeRunCountProvider, (_, _) {});
      addTearDown(count.close);
      await pumpUntil(
        tester,
        () => count.read() == 4,
        what: 'four active runs',
      );
      await host.run(
        tester,
        () => host.db.runsDao.upsertIfNewer(
          active(
            1,
            RunState.completed,
            channelN: 1,
          ).copyWith(version: const ResourceVersion(2)),
        ),
      );
      await pumpUntil(
        tester,
        () => count.read() == 3,
        what: 'the ended run leaves the count',
      );
      await pumpUntil(
        tester,
        () => row(1).evaluate().isEmpty,
        what: 'the ended run leaves the list',
      );
      await host.dispose(tester);
    },
  );
}
