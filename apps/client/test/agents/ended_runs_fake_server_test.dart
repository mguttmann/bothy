// SPDX-License-Identifier: Apache-2.0

// The line of a run that ended before this device knew it (01 3.2, 6.2; 09 5.7; ruling
// AUD-CA01/1): the local `runs` table holds a run only from the active runs of the snapshot and
// from `run.state_changed` since then, so a new device and a device after a resync know no run
// that ended before. The desktop app runs against the Dart fake server on 127.0.0.1 with the
// real engine, the generated decoder and the local database: the anchor of such a run shows its
// line and "Retry", again after `resync_required`, and Retry reaches the server. The loader
// stores every page of `listThreadRuns` and nothing of a thread the device lost meanwhile.

import 'dart:async';
import 'dart:io';

import 'package:client/agents/providers.dart';
import 'package:client/agents/thread_runs_loader.dart';
import 'package:client/app/app.dart';
import 'package:client/app/providers/ports.dart';
import 'package:client/app/providers/session.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart' hide ConnectionState;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:sync_engine/testing.dart';

import '../support/app_scenario_client.dart';
import '../support/fake_instance.dart';
import '../support/fakes.dart';
import '../support/until.dart';
import '../support/widgets.dart';

/// Lets real IO run and advances the fake clock of the test zone a little each round, until
/// [condition] holds; bounded by the patience of the test kits.
Future<void> _until(
  WidgetTester tester,
  String what,
  bool Function() condition,
) async {
  final watch = Stopwatch()..start();
  while (!condition()) {
    if (watch.elapsed >= testPatience) {
      fail('$what did not happen within $testPatience');
    }
    await tester.runAsync(
      () => Future<void>.delayed(const Duration(milliseconds: 5)),
    );
    await tester.pump(const Duration(milliseconds: 5));
  }
}

/// A run of Robin for Alex in "Release notes" with its anchor, ended `failed`; returns its ID.
String failedRun(FakeInstance f, {String text = '@robin run the tests'}) {
  final world = f.server.world;
  final w = f.seed;
  final trigger = w.post(w.alex, w.releaseNotes, text);
  final run = world.createRun(
    agentId: w.robin.id,
    triggeringMessageId: trigger['id']! as String,
    triggeredBy: w.alex.id,
  );
  final id = run['id']! as String;
  FakeAgentInterface.of(world).postAnchor(w.robin.id, id);
  world
    ..setRunState(id, 'running')
    ..setRunState(
      id,
      'failed',
      triggerState: 'failed',
      reason: 'adapter_error',
    );
  return id;
}

void main() {
  group('the desktop app', () {
    HttpOverrides? saved;
    late FakeInstance fake;
    late ProviderContainer c;
    var disposed = false;

    setUp(() async {
      saved = HttpOverrides.current;
      HttpOverrides.global = null;
      fake = await FakeInstance.start();
      c = appContainer(fake);
      disposed = false;
    });

    tearDown(() async {
      if (!disposed) c.dispose();
      await fake.stop();
      HttpOverrides.global = saved;
    });

    testWidgets('a new device shows the line and Retry of a run that ended '
        'before its sign-in, again after a resync, and Retry reaches the '
        'server', (tester) async {
      desktopWindow(tester);
      final world = fake.server.world;
      final seed = fake.seed;
      // The run ends before the device exists.
      final runId = failedRun(fake);
      final id = RunId.parse(runId);

      await tester.runAsync(() async {
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
      final engine = c.read(syncEngineProvider)!;
      final db = c.read(localDatabaseProvider)!;
      // The premise: the snapshot names active runs only, so the device does not hold the run.
      expect(await tester.runAsync(() => db.runsDao.byId(id)), isNull);
      final resyncs = <String>[];
      final notices = engine.notices.listen((n) {
        if (n is Resynced) resyncs.add(n.reason);
      });
      // Whether the local table held the run, at each of its changes. (A database read of the
      // test would wait behind a transaction of the app that only a pumped frame continues.)
      final held = <bool>[];
      final table = c.listen(threadRunsProvider(seed.releaseNotes), (_, next) {
        final runs = next.value;
        if (runs != null) held.add(runs.any((r) => r.id == id));
      }, fireImmediately: true);

      await tester.pumpWidget(
        UncontrolledProviderScope(container: c, child: const ClientApp()),
      );
      final sidebar = find.byKey(const ValueKey('column.sidebar'));
      await _until(
        tester,
        'the desktop shell',
        () => sidebar.evaluate().isNotEmpty,
      );
      GoRouter.of(tester.element(sidebar)).go('/t/${seed.releaseNotes}');

      final line = find.byKey(ValueKey('runline.$runId'));
      final retry = find.byKey(ValueKey('runline.retry.$runId'));
      bool shown() => line.evaluate().isNotEmpty && retry.evaluate().isNotEmpty;
      await _until(tester, 'the line and Retry of the ended run', shown);
      expect(held.last, isTrue);

      // `resync_required` empties the mirror; the open thread gets the run and its line back.
      final before = held.length;
      fake.server.faults.resyncRequired('retention');
      fake.server.disconnect();
      await _until(tester, 'the resync', () => resyncs.contains('retention'));
      await _until(
        tester,
        'the table without the run after the resync',
        () => held.skip(before).contains(false),
      );
      await _until(
        tester,
        'the run stored again after the resync',
        () => held.last,
      );
      await _until(tester, 'the line and Retry after the resync', shown);

      // Retry goes through the outbox to `retryRun`: the server starts the next run.
      await tester.tap(
        find.descendant(of: retry, matching: find.byType(InkWell)).first,
      );
      await _until(
        tester,
        'the run the retry started',
        () => world.runs.length == 2,
      );
      expect(fake.server.violations, isEmpty);

      table.close();
      // Not awaited: the future of this cancel completes outside the fake zone of the test.
      unawaited(notices.cancel());
      await tester.pumpWidget(const SizedBox());
      c.dispose();
      disposed = true;
      await tester.pump(const Duration(minutes: 5));
    });
  });

  group('the loader', () {
    /// The app of Alex signed in on a fake where [before] ran first.
    Future<void> withApp(
      Future<void> Function(
        FakeInstance f,
        SyncEngine engine,
        LocalDatabase db,
        AppScenarioClient client,
      )
      body, {
      void Function(FakeInstance f)? before,
    }) async {
      final saved = HttpOverrides.current;
      HttpOverrides.global = null;
      final f = await FakeInstance.start();
      before?.call(f);
      final c = ProviderContainer(
        overrides: [appPortsProvider.overrideWithValue(f.ports)],
      );
      final client = AppScenarioClient(
        c,
        f.server,
        f.ports.network as SwitchNetwork,
      );
      try {
        await client.start();
        await body(
          f,
          c.read(syncEngineProvider)!,
          c.read(localDatabaseProvider)!,
          client,
        );
      } finally {
        await client.stop();
        c.dispose();
        await f.stop();
        HttpOverrides.global = saved;
      }
    }

    Future<List<RunSummary>> runsOf(LocalDatabase db, String thread) =>
        db.runsDao.watchForThread(ThreadId.parse(thread)).first;

    /// The summaries of the runs of the thread of [r], newest first, as the server lists them.
    List<JsonMap> newestFirst(FakeInstance f, FakeRequest r) {
      final world = f.server.world;
      final thread = world.visibleThread(r.person.id, r.param('thread_id'));
      return [
        for (final run in world.runs.values)
          if (run['thread_id'] == thread['id']) world.runSummary(run),
      ].reversed.toList();
    }

    test(
      'every page of listThreadRuns is stored by the version rule',
      () async {
        late List<String> ids;
        await withApp(
          before: (f) => ids = [
            failedRun(f, text: '@robin first'),
            failedRun(f, text: '@robin second'),
            failedRun(f, text: '@robin third'),
          ],
          (f, engine, db, client) async {
            final w = f.seed;
            // The fake answers one page; this handler pages like the server: one run per
            // page, `next_cursor` until the last.
            final cursors = <String?>[];
            f.server.rest.handle('listThreadRuns', (r) {
              final items = newestFirst(f, r);
              cursors.add(r.query('cursor'));
              final at = int.parse(r.query('cursor') ?? '0');
              return FakeResponse(200, {
                'items': [items[at]],
                if (at + 1 < items.length) 'next_cursor': '${at + 1}',
              });
            });
            await client.focusThread(w.releaseNotes);
            expect(await runsOf(db, w.releaseNotes), isEmpty);
            final loader = EngineThreadRunsLoader(engine, db);
            await loader.load(ThreadId.parse(w.releaseNotes));
            expect(cursors, [null, '1', '2']);
            final stored = await runsOf(db, w.releaseNotes);
            expect([for (final r in stored) r.id.value], unorderedEquals(ids));
            expect([
              for (final r in stored) r.state,
            ], everyElement(RunState.failed));
            // A second load stores no version again.
            await loader.load(ThreadId.parse(w.releaseNotes));
            expect(await runsOf(db, w.releaseNotes), stored);
            expect(f.server.violations, isEmpty);
          },
        );
      },
    );

    test('an answer that arrives after the device lost the thread is not '
        'stored', () async {
      await withApp(before: failedRun, (f, engine, db, client) async {
        final world = f.server.world;
        final w = f.seed;
        await client.focusThread(w.releaseNotes);
        // The server answers, then Alex loses the channel, and only then the answer arrives.
        final removed = Completer<void>();
        f.server.rest.handle('listThreadRuns', (r) async {
          final body = {'items': newestFirst(f, r)};
          world.setAccess(w.alex.id, w.general, null);
          await removed.future;
          return FakeResponse(200, body);
        });
        final load = EngineThreadRunsLoader(
          engine,
          db,
        ).load(ThreadId.parse(w.releaseNotes));
        await eventually(
          'the removal',
          () => client.notices.contains('removed:channel:${w.general}'),
        );
        removed.complete();
        await load;
        expect(await runsOf(db, w.releaseNotes), isEmpty);
      });
    });
  });
}
