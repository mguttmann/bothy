// SPDX-License-Identifier: Apache-2.0

// The control room (brief M4-39 Tests; 01 D10; 09 5.6; 04 6.2 `background_thread_ids`): the
// switch Cards and List, Stop only with the capability, "live details paused" beyond the live
// budget, the stall notice of the last load, and the focus of the visible cards: a card whose
// thread the app does not show goes into `background_thread_ids`, a card whose thread is open
// stays in `thread_ids` only, and `idle` stays the window state.

import 'dart:io';

import 'package:client/app/providers/session.dart';
import 'package:client/running/providers.dart';
import 'package:client/running/running_now_view.dart';
import 'package:client/thread/providers.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:sync_engine/testing.dart';

import '../support/agent_fakes.dart';
import '../support/column_host.dart';
import '../support/fake_instance.dart';
import '../support/fakes.dart' show fastConfig;
import '../support/structure.dart';
import '../support/until.dart';
import '../support/widgets.dart';
import 'running_now_test.dart' show fill, overrides;

/// A focus whose live set the test fixes.
final class _FixedFocus extends ControlRoomFocus {
  _FixedFocus(this.live, this.shown);

  final Set<RunId> live;

  /// The cards each [show] reported, shared by every instance of one test.
  final List<List<RunId>> shown;

  @override
  Set<RunId> build() => live;

  @override
  void show(List<({RunId run, ThreadId thread})> cards) =>
      shown.add([for (final c in cards) c.run]);
}

Finder card(int n) => find.byKey(ValueKey('running.card.${uuid(0x700 + n)}'));

Future<(ColumnHost, List<List<RunId>>)> pumpCards(
  WidgetTester tester, {
  Set<RunId>? live,
}) async {
  final shown = <List<RunId>>[];
  final host = await ColumnHost.open(tester);
  await tester.runAsync(() => fill(host.db));
  await host.pump(
    tester,
    const RunningNowView(),
    overrides: [
      ...overrides,
      controlRoomFocusProvider.overrideWith(
        () => _FixedFocus(live ?? {runId(1)}, shown),
      ),
      cardDocumentProvider.overrideWith(
        (ref, id) => Stream.value(
          id == runId(1).value
              ? RunDocument.fromJson(
                  runDoc(
                    1,
                    steps: {
                      's1': step('s1', 1, name: 'Bash'),
                      's2': step('s2', 2, kind: 'subagent', name: 'tests'),
                    },
                  ),
                )
              : null,
        ),
      ),
      activeRunStallsProvider.overrideWith(() => _Stalls()),
    ],
  );
  await pumpUntil(tester, () => card(4).evaluate().isNotEmpty, what: 'cards');
  return (host, shown);
}

final class _Stalls extends ActiveRunStalls {
  @override
  Map<RunId, DateTime> build() => {
    runId(4): t0.add(const Duration(minutes: 1)),
  };
}

void main() {
  testWidgets('cards are the default; the switch shows the rows and back', (
    tester,
  ) async {
    final (host, shown) = await pumpCards(tester);
    for (var n = 1; n <= 4; n++) {
      expect(card(n), findsOneWidget, reason: 'run $n');
    }
    expect(find.byKey(const ValueKey('running.list')), findsNothing);
    await tester.tap(find.byKey(const ValueKey('running.form.list')));
    await host.settle(tester);
    expect(card(1), findsNothing);
    expect(find.byKey(ValueKey('running.row.${uuid(0x701)}')), findsOneWidget);
    await tester.tap(find.byKey(const ValueKey('running.form.cards')));
    await host.settle(tester);
    expect(card(1), findsOneWidget);
    // The grid reported the cards on screen in on-screen order.
    expect(shown.last, [for (var n = 1; n <= 4; n++) runId(n)]);
    await host.dispose(tester);
  });

  testWidgets('Stop only with the capability; Graph opens the run graph', (
    tester,
  ) async {
    final (host, _) = await pumpCards(tester);
    Finder stop(int n) =>
        find.byKey(ValueKey('runCard.stop.${uuid(0x700 + n)}'));
    expect(stop(1), findsOneWidget, reason: 'own run with trigger');
    expect(stop(2), findsNothing, reason: 'foreign run without manage');
    expect(stop(3), findsNothing, reason: 'no trigger in the channel');
    expect(stop(4), findsOneWidget, reason: 'manage in the channel');
    expect(find.byKey(ValueKey('runCard.open.${uuid(0x702)}')), findsOneWidget);
    await tester.tap(find.byKey(ValueKey('runCard.graph.${uuid(0x704)}')));
    await host.settle(tester);
    expect(host.visited.last, '/running/${uuid(0x704)}?t=${threadId(4).value}');
    await host.dispose(tester);
  });

  testWidgets('a live card shows the document; the others are paused with '
      'the stall of the last load', (tester) async {
    final (host, _) = await pumpCards(tester);
    Finder inCard(int n, String text) =>
        find.descendant(of: card(n), matching: find.text(text));
    expect(inCard(1, en.vizStepToolCall('Bash')), findsOneWidget);
    expect(inCard(1, en.runningSubagents(1)), findsOneWidget);
    expect(inCard(1, en.runningLivePaused), findsNothing);
    for (final n in [2, 3, 4]) {
      expect(inCard(n, en.runningLivePaused), findsOneWidget, reason: '$n');
    }
    expect(inCard(4, en.liveBoxStall(3)), findsOneWidget);
    await host.dispose(tester);
  });

  testWidgets('on the phone Running now shows the cards by default and offers '
      'the switch as on the desktop (M5-14, M11)', (tester) async {
    final host = await ColumnHost.open(tester);
    await tester.runAsync(() => fill(host.db));
    await host.pump(
      tester,
      const RunningNowView(),
      platform: 'ios',
      overrides: [
        ...overrides,
        cardDocumentProvider.overrideWith(
          (ref, id) => Stream<RunDocument?>.value(null),
        ),
      ],
    );
    await pumpUntil(tester, () => card(1).evaluate().isNotEmpty, what: 'cards');
    expect(find.byKey(ValueKey('running.row.${uuid(0x701)}')), findsNothing);
    expect(find.byKey(const ValueKey('running.form.cards')), findsOneWidget);
    expect(find.byKey(const ValueKey('running.form.list')), findsOneWidget);
    await host.dispose(tester);
  });

  group('the focus of the visible cards', () {
    HttpOverrides? saved;
    late FakeInstance fake;
    late ProviderContainer c;

    setUp(() async {
      saved = HttpOverrides.current;
      HttpOverrides.global = null;
      fake = await FakeInstance.start(
        config: {...fastConfig, 'sync.max_focused_threads': '2'},
      );
      c = appContainer(fake);
      final control = await connected(c, fake);
      await control.loginWithPassword(
        WorldSeed.alexLogin,
        WorldSeed.password,
        WorldSeed.totp,
      );
      await eventually(
        'live',
        () => c.read(syncEngineProvider)?.state.phase == ConnectionPhase.live,
      );
    });

    tearDown(() async {
      c.dispose();
      await fake.stop();
      HttpOverrides.global = saved;
    });

    Map<String, Object?>? lastFocus() {
      final frames = fake.server.gateway.frames.where(
        (f) => !f.fromServer && f.op == 'focus',
      );
      return frames.isEmpty ? null : frames.last.data;
    }

    test(
      'an open thread stays in thread_ids, the next card goes into '
      'background_thread_ids, the rest waits; idle stays the window',
      () async {
        final world = fake.server.world;
        final seed = fake.seed;
        final forum = world.addForum(seed.space, 'deploys', {
          seed.alex.id: 'trigger',
        });
        final threads = [
          seed.releaseNotes,
          world.addThread(forum, 'First', createdBy: seed.alex.id),
          world.addThread(forum, 'Second', createdBy: seed.alex.id),
        ];
        final cards = <({RunId run, ThreadId thread})>[
          for (final t in threads)
            (run: RunId.parse(world.nextId()), thread: ThreadId.parse(t)),
        ];
        final open = ThreadId.parse(seed.releaseNotes);
        c.read(visibleThreadsProvider.notifier).add(open);
        await eventually(
          'the open thread in focus',
          () =>
              (lastFocus()?['thread_ids'] as List?)?.contains(open.value) ??
              false,
        );
        final sub = c.listen(controlRoomFocusProvider, (_, _) {});
        c.read(controlRoomFocusProvider.notifier).show(cards);
        await eventually(
          'the background focus',
          () => lastFocus()?['background_thread_ids'] != null,
        );
        final frame = lastFocus()!;
        expect(frame['thread_ids'], [open.value]);
        expect(frame['background_thread_ids'], [threads[1]]);
        expect(frame['idle'], isFalse, reason: 'the window is in front');
        expect(c.read(controlRoomFocusProvider), {cards[0].run, cards[1].run});
        sub.close();
        await eventually(
          'the places freed after the view',
          () => lastFocus()?['background_thread_ids'] == null,
        );
      },
    );
  });
}
