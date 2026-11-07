// SPDX-License-Identifier: Apache-2.0

// Autonomous runs through the app's engine against the Dart fake server on 127.0.0.1, headless,
// for each platform build of the desktop and the phone (brief AR-08 Tests; 01 6.21, 09 5.8):
// the thread's wake-ups load into the local table when the view asks, `run.wake_changed` brings
// an end and its note, a fired wake-up names its wake run, the Cancel of the outbox reaches the
// server and comes back, and the channel switch is stored with its version; a channel of a
// server before stage AR stays without the switch. integration_test/autonomous_runs_test.dart
// drives the same in the desktop app and the phone suite on the devices.

import 'dart:io';

import 'package:client/agents/wakes/wake_notes.dart';
import 'package:client/app/providers/ports.dart';
import 'package:client/app/providers/session.dart';
import 'package:client/sync/wake_ops.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:sync_engine/testing.dart';

import '../support/app_scenario_client.dart';
import '../support/fake_instance.dart';
import '../support/fakes.dart';

void main() {
  final en = lookupAppLocalizations(const Locale('en'));

  Future<void> withApp(
    String platform,
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
    final f = await FakeInstance.start(platform: platform);
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

  /// A run of Robin for Alex in Release notes and a pending wake-up of it.
  ({JsonMap run, JsonMap wake}) pendingWake(FakeInstance f) {
    final world = f.server.world;
    final w = f.seed;
    final trigger = w.post(w.alex, w.releaseNotes, '@robin watch the checks');
    final run = world.createRun(
      agentId: w.robin.id,
      triggeringMessageId: trigger['id']! as String,
      triggeredBy: w.alex.id,
    );
    final wake = world.addWake(
      threadId: w.releaseNotes,
      agentId: w.robin.id,
      sourceRunId: run['id']! as String,
      triggeredBy: w.alex.id,
      kind: 'pr_checks',
      label: 'CI of #123',
      condition: {'repository': 'example/app', 'number': 123},
    );
    return (run: run, wake: wake);
  }

  Future<List<RunWake>> stored(LocalDatabase db, String thread) =>
      db.runsDao.watchWakesForThread(ThreadId.parse(thread)).first;

  for (final platform in ['macos', 'windows', 'ios', 'android']) {
    test('$platform: load, live end with its note, wake run, cancel and '
        'switch', () async {
      await withApp(platform, (f, engine, db, client) async {
        final world = f.server.world;
        final w = f.seed;
        final (:run, :wake) = pendingWake(f);
        await client.focusThread(w.releaseNotes);
        await engine.catchUp.loadThreadWakes(ThreadId.parse(w.releaseNotes));
        await eventually(
          'the pending wake-up is stored',
          () async => (await stored(
            db,
            w.releaseNotes,
          )).any((x) => x.id.value == wake['id'] && x.isPending),
        );

        // run.wake_changed with the end; the note arrives as a message of the server.
        world.endWake(
          wake['id']! as String,
          'expired',
          reason: 'deadline_passed',
        );
        await eventually(
          'the end is stored',
          () async =>
              (await stored(db, w.releaseNotes)).single.state ==
              RunWakeState.expired,
        );
        final notes = [
          for (final m in await db.messagesDao.pageBefore(
            ThreadId.parse(w.releaseNotes),
            null,
            50,
          ))
            if (isThreadNote(m)) m,
        ];
        expect(notes, hasLength(1));
        expect(
          threadNoteText(en, notes.single),
          en.noteWakeExpired(en.wakesNoteName(dsIsolate('CI of #123'))),
        );

        // A fired wake-up names its wake run.
        final second = world.addWake(
          threadId: w.releaseNotes,
          agentId: w.robin.id,
          sourceRunId: run['id']! as String,
          triggeredBy: w.alex.id,
          label: 'nightly',
        );
        final wakeRun = world.fireWake(second['id']! as String);
        await eventually(
          'the wake run is named',
          () async => (await stored(
            db,
            w.releaseNotes,
          )).any((x) => x.firedRunId?.value == wakeRun['id']),
        );

        // Cancel through the outbox reaches the server and comes back.
        final third = world.addWake(
          threadId: w.releaseNotes,
          agentId: w.robin.id,
          sourceRunId: run['id']! as String,
          triggeredBy: w.alex.id,
          label: 'later',
        );
        await eventually(
          'the third wake-up is stored',
          () async => (await stored(
            db,
            w.releaseNotes,
          )).any((x) => x.id.value == third['id']),
        );
        await enqueueWakeCancel(engine, WakeId.parse(third['id']! as String));
        await eventually(
          'the cancel is stored',
          () async => (await stored(db, w.releaseNotes)).any(
            (x) =>
                x.id.value == third['id'] && x.state == RunWakeState.cancelled,
          ),
        );
        expect(world.wakes[third['id']]!['state'], 'cancelled');

        // The channel switch with manage and the stored version.
        world.channels[w.general]!.capabilities[w.alex.id] = {'manage'};
        final general = ChannelId.parse(w.general);
        final before = (await db.structureDao.channelById(general))!;
        expect(before.allowAutonomousRuns, isFalse);
        await enqueueAutonomousRuns(
          engine,
          general,
          before.version,
          allowed: true,
        );
        await eventually(
          'the switch is stored',
          () async =>
              (await db.structureDao.channelById(general))
                  ?.allowAutonomousRuns ==
              true,
        );
        expect(
          world.channels[w.general]!.json['allow_autonomous_runs'],
          isTrue,
        );
      });
    });
  }

  test('a server before stage AR: no switch and no wake-ups', () async {
    await withApp(
      'macos',
      (f, engine, db, client) async {
        final w = f.seed;
        final general = await db.structureDao.channelById(
          ChannelId.parse(w.general),
        );
        expect(general!.allowAutonomousRuns, isNull);
        await engine.catchUp.loadThreadWakes(ThreadId.parse(w.releaseNotes));
        expect(await stored(db, w.releaseNotes), isEmpty);
      },
      before: (f) {
        f.server.world.withoutAutonomousRunsSwitch();
        f.server.faults.onRequest(
          'listThreadWakes',
          status: 404,
          code: 'not_found',
        );
      },
    );
  });
}
