// SPDX-License-Identifier: Apache-2.0

// The flow of integration_test/notifications_windows_test.dart headless (brief M4-30 Tests):
// the app's providers against the fake server with a Windows window that hides to the
// notification area. With the window hidden, a message from Sam makes the fake send
// `notification.created` and the presenter shows one toast; a click opens the thread at the
// message; after Quit the device has no connection and the fake creates nothing.

import 'dart:io';

import 'package:client/app/providers/session.dart';
import 'package:client/desktop/providers.dart';
import 'package:client/desktop/window_behavior.dart';
import 'package:client/notifications/providers.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:sync_engine/testing.dart';

import '../support/desktop_fakes.dart';
import '../support/fake_instance.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  HttpOverrides? saved;
  late FakeInstance fake;
  late ProviderContainer c;
  late FakeWindow window;
  late RecordingPresenter presenter;

  setUp(() async {
    saved = HttpOverrides.current;
    HttpOverrides.global = null;
    window = FakeWindow();
    presenter = RecordingPresenter();
    fake = await FakeInstance.start(
      platform: 'windows',
      window: window,
      notifications: presenter,
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
    await c.read(desktopShellProvider.future);
  });

  tearDown(() async {
    c.dispose();
    await fake.stop();
    HttpOverrides.global = saved;
  });

  test(
    'hidden: one toast with title and text; a click opens the thread',
    () async {
      final targets = <String>[];
      final sub = c
          .read(desktopNotificationsProvider)
          .targets
          .listen(targets.add);
      final world = fake.server.world;
      final seed = fake.seed;
      world
        ..notifyOnMessages = true
        ..setNotificationPref(seed.alex.id, 'channel', seed.general, 'all');

      window.send(DesktopWindowEvent.close);
      await eventually('hidden', () => window.calls.contains('hide'));
      final message = seed.post(seed.sam, seed.releaseNotes, 'Build is green');
      await eventually('a toast', () => presenter.shown.isNotEmpty);
      final toast = presenter.shown.single;
      expect(toast.content.title, 'general · Release notes');
      expect(toast.content.body, 'Sam: Build is green');
      expect(toast.silent, isFalse);
      expect(toast.payload, '/t/${seed.releaseNotes}?m=${message['id']}');

      presenter.taps.add(toast.payload);
      await eventually('the click', () => targets.isNotEmpty);
      expect(targets, [toast.payload]);
      await sub.cancel();
    },
  );

  test('after Quit: no connection of the device and no notification', () async {
    final world = fake.server.world;
    final seed = fake.seed;
    world
      ..notifyOnMessages = true
      ..setNotificationPref(seed.alex.id, 'channel', seed.general, 'all');
    c.read(desktopNotificationsProvider);
    final device = c
        .read(signedInServicesProvider)!
        .auth
        .session!
        .deviceId
        .value;

    await c.read(windowBehaviorProvider).quit();
    expect(window.calls.last, 'destroy');
    await eventually(
      'no connection of the device',
      () => !fake.server.gateway.connections.any(
        (conn) => conn.open && conn.deviceId == device,
      ),
    );
    seed.post(seed.sam, seed.releaseNotes, 'Deploy is done');
    expect(
      world.notifications.where((n) => n.recipient == seed.alex.id),
      isEmpty,
    );
    expect(world.notificationSkips.where((s) => s.userId == seed.alex.id), [
      (userId: seed.alex.id, reason: 'no_route'),
    ]);
    await Future<void>.delayed(const Duration(milliseconds: 100));
    expect(presenter.shown, isEmpty);
  });
}
