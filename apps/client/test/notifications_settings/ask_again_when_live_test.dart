// SPDX-License-Identifier: Apache-2.0

// The loads that run once per session are asked again when the connection is live again after
// they failed (docs/decisions.d/AUD-client-misc-01.md, ruling AUD-CM01/4), through the app's
// engine against the Dart fake server on 127.0.0.1, headless: an owner whose first `getMe`
// failed is an owner once the connection is back, and the notification levels the first load
// could not fetch are stored then. A load that succeeded is not repeated at a reconnect. A
// resync empties the levels with the rest of the mirror, and the load brings them back.

import 'dart:io';

import 'package:client/app/providers/ports.dart';
import 'package:client/app/providers/session.dart';
import 'package:client/notifications_settings/providers.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

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
      await body(f, c, c.read(localDatabaseProvider)!, client);
    } finally {
      await client.stop();
      c.dispose();
      await f.stop();
      HttpOverrides.global = saved;
    }
  }

  test('an owner whose first read failed without a connection is an owner '
      'once the connection is live again', () async {
    await withApp((f, c, db, client) async {
      expect(f.seed.alex.isOwner, isTrue);
      await client.goOffline();
      // The one read of the session fails (the read retry does not repeat a 500).
      f.server.faults.onRequest('getMe', status: 500, code: 'internal_error');
      expect(await c.read(isOwnerProvider.future), isFalse);
      expect(f.server.requestsOf('getMe').last.status, 500);
      await client.goOnline();
      await eventually(
        'the owner is known',
        () => c.read(isOwnerProvider).value ?? false,
      );
      // The answer stands: another reconnect asks nothing.
      final asked = f.server.requestsOf('getMe').length;
      await client.goOffline();
      await client.goOnline();
      expect(await c.read(isOwnerProvider.future), isTrue);
      expect(f.server.requestsOf('getMe'), hasLength(asked));
      expect(f.server.violations, isEmpty);
    });
  });

  test('the notification levels a first load could not fetch are stored once '
      'the connection is live again', () async {
    await withApp(
      // A level from before this session: only the load brings it, the snapshot has none.
      before: (f) => f.server.world.putNotificationPref(
        f.seed.alex.id,
        'channel',
        f.seed.general,
        {'level': 'all'},
      ),
      (f, c, db, client) async {
        await client.goOffline();
        f.server.faults.onRequest(
          'listMyNotificationPreferences',
          status: 500,
          code: 'internal_error',
        );
        await c.read(notifyRefreshProvider.future);
        expect(await db.personalDao.notificationPreferences(), isEmpty);
        await client.goOnline();
        await eventually('the level is stored', () async {
          // A view that shows the levels watches the load; reading it here does the same.
          c.read(notifyRefreshProvider);
          return (await db.personalDao.notificationPreferences()).isNotEmpty;
        });
        final stored = await db.personalDao.notificationPreferences();
        expect(stored.single.targetId, f.seed.general);
        expect(f.server.violations, isEmpty);
      },
    );
  });

  test('the notification levels are loaded again after a resync emptied '
      'them', () async {
    await withApp(
      before: (f) => f.server.world.putNotificationPref(
        f.seed.alex.id,
        'channel',
        f.seed.general,
        {'level': 'all'},
      ),
      (f, c, db, client) async {
        await c.read(notifyRefreshProvider.future);
        expect(await db.personalDao.notificationPreferences(), hasLength(1));
        final loads = f.server
            .requestsOf('listMyNotificationPreferences')
            .length;
        // `resync_required` replaces the mirror with a snapshot, which carries no levels.
        final resynced = c
            .read(syncEngineProvider)!
            .notices
            .firstWhere((n) => n is Resynced);
        f.server.faults.resyncRequired('retention');
        f.server.disconnect();
        await resynced;
        await eventually('the level is stored again', () async {
          // A view that shows the levels watches the load; reading it here does the same.
          c.read(notifyRefreshProvider);
          return (await db.personalDao.notificationPreferences()).isNotEmpty &&
              f.server.requestsOf('listMyNotificationPreferences').length >
                  loads;
        });
        expect(f.server.violations, isEmpty);
      },
    );
  });
}
