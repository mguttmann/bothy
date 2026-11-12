// SPDX-License-Identifier: Apache-2.0

// Stop of a run through the app's outbox against the Dart fake server on 127.0.0.1, headless
// (01 6.2; docs/decisions.d/AUD-client-misc-01.md, ruling AUD-CM01/3): an accepted Stop ends
// the run and stays taken; a Stop for a run that ended meanwhile (409) and a Stop without the
// right (403) are refused for good, leave no entry in the outbox, report their problem code
// and are offered again; without a connection the Stop waits in the outbox and goes out when
// the connection is back. `stop_run_test.dart` holds what the views show.

import 'dart:io';

import 'package:client/app/providers/ports.dart';
import 'package:client/app/providers/session.dart';
import 'package:client/running/providers.dart';
import 'package:client/sync/outbox_outcome.dart';
import 'package:domain/domain.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/testing.dart';

import '../support/app_scenario_client.dart';
import '../support/fake_instance.dart';
import '../support/fakes.dart';

void main() {
  Future<void> withApp(
    Future<void> Function(
      FakeInstance f,
      ProviderContainer c,
      LocalDatabase db,
      AppScenarioClient client,
    )
    body,
  ) async {
    final saved = HttpOverrides.current;
    HttpOverrides.global = null;
    final f = await FakeInstance.start();
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
      await body(f, c, c.read(localDatabaseProvider)!, client);
    } finally {
      await client.stop();
      c.dispose();
      await f.stop();
      HttpOverrides.global = saved;
    }
  }

  /// An active run of Robin in [thread], triggered by [by], as the device stores it.
  Future<RunSummary> activeRun(
    FakeInstance f,
    LocalDatabase db, {
    required String thread,
    required FakePerson by,
  }) async {
    final trigger = f.seed.post(by, thread, '@robin run the tests');
    final run = f.server.world.createRun(
      agentId: f.seed.robin.id,
      triggeringMessageId: trigger['id']! as String,
      triggeredBy: by.id,
    );
    final id = RunId.parse(run['id']! as String);
    RunSummary? stored;
    await eventually('the run is stored', () async {
      stored = await db.runsDao.byId(id);
      return stored != null;
    });
    return stored!;
  }

  String stateOf(FakeInstance f, RunSummary run) =>
      f.server.world.runs[run.id.value]!['state']! as String;

  test('an accepted Stop cancels the run and stays taken', () async {
    await withApp((f, c, db, client) async {
      final run = await activeRun(
        f,
        db,
        thread: f.seed.releaseNotes,
        by: f.seed.alex,
      );
      final refusals = <OutboxRefusal>[];
      final stops = c.read(runStopsProvider.notifier);
      // Two activations at once (a double click) and one more while the Stop is on its way:
      // one entry.
      await Future.wait([
        stops.stop(run, onRefused: refusals.add),
        stops.stop(run, onRefused: refusals.add),
      ]);
      await stops.stop(run, onRefused: refusals.add);
      await eventually(
        'the run is cancelled',
        () => stateOf(f, run) == 'cancelled',
      );
      await eventually(
        'the outbox is empty',
        () async => (await client.outbox()).isEmpty,
      );
      expect(f.server.requestsOf('cancelRun'), hasLength(1));
      expect(c.read(runStopsProvider), contains(run.id));
      expect(refusals, isEmpty);
      expect(f.server.violations, isEmpty);
    });
  });

  test('a Stop for a run that ended meanwhile is refused with 409, discarded '
      'and offered again', () async {
    await withApp((f, c, db, client) async {
      final run = await activeRun(
        f,
        db,
        thread: f.seed.releaseNotes,
        by: f.seed.alex,
      );
      // The run ends on the server; the device sends the Stop of the run it still shows.
      f.server.world.setRunState(run.id.value, 'completed');
      final refusals = <OutboxRefusal>[];
      await c
          .read(runStopsProvider.notifier)
          .stop(run, onRefused: refusals.add);
      await eventually('the refusal arrived', () => refusals.isNotEmpty);
      expect(refusals.single.code, 'invalid_state_transition');
      expect(f.server.requestsOf('cancelRun').single.status, 409);
      expect(await client.outbox(), isEmpty, reason: 'no failed entry stays');
      expect(c.read(runStopsProvider), isNot(contains(run.id)));
      expect(f.server.violations, isEmpty);
    });
  });

  test('a Stop without the right is refused with 403, discarded and offered '
      'again', () async {
    await withApp((f, c, db, client) async {
      // Alex may only write in Support, and Sam started the run there.
      final run = await activeRun(
        f,
        db,
        thread: f.seed.printer,
        by: f.seed.sam,
      );
      final refusals = <OutboxRefusal>[];
      await c
          .read(runStopsProvider.notifier)
          .stop(run, onRefused: refusals.add);
      await eventually('the refusal arrived', () => refusals.isNotEmpty);
      expect(refusals.single.code, 'forbidden');
      expect(f.server.requestsOf('cancelRun').single.status, 403);
      expect(await client.outbox(), isEmpty, reason: 'no failed entry stays');
      expect(c.read(runStopsProvider), isNot(contains(run.id)));
      expect(stateOf(f, run), isNot('cancelled'));
      expect(f.server.violations, isEmpty);
    });
  });

  test('without a connection the Stop waits in the outbox and goes out when '
      'the connection is back', () async {
    await withApp((f, c, db, client) async {
      final run = await activeRun(
        f,
        db,
        thread: f.seed.releaseNotes,
        by: f.seed.alex,
      );
      await client.goOffline();
      final refusals = <OutboxRefusal>[];
      await c
          .read(runStopsProvider.notifier)
          .stop(run, onRefused: refusals.add);
      final waiting = await client.outbox();
      expect(waiting, hasLength(1));
      expect(waiting.single.failed, isFalse);
      expect(c.read(runStopsProvider), contains(run.id));
      expect(f.server.requestsOf('cancelRun'), isEmpty);
      await client.goOnline();
      await eventually(
        'the run is cancelled',
        () => stateOf(f, run) == 'cancelled',
      );
      await eventually(
        'the outbox is empty',
        () async => (await client.outbox()).isEmpty,
      );
      expect(c.read(runStopsProvider), contains(run.id));
      expect(refusals, isEmpty);
      expect(f.server.violations, isEmpty);
    });
  });
}
