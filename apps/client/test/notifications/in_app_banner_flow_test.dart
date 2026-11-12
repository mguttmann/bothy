// SPDX-License-Identifier: Apache-2.0

// A hint of the gateway in the phone app that is in front (13 4.6 item 1: a device with a
// gateway connection gets `notification.created` instead of a push, and "the app shows its own
// notification or an in-app banner"; 15 10; docs/decisions.d/AUD-client-app-01.md AUD-CA1/1).
//
// The whole path of the app: the phone app of an iOS and of an Android build over the real
// services (auth session, encrypted database, sync engine) and a real gateway socket to the
// fake server. Sam's message makes the server send `notification.created` to the connection of
// Alex's phone, the engine decodes the frame, and the app shows it as a banner over its
// content; a tap opens the thread at the message. Before this ticket the phone showed nothing:
// the hint only moved the counters, and no push followed, because the server had delivered it.

import 'dart:io';

import 'package:client/app/providers/session.dart';
import 'package:client/notifications/providers.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/sync_engine.dart'
    show ConnectionPhase, NotificationCreated;
import 'package:sync_engine/testing.dart';

import '../phone/support/phone_device.dart';
import '../support/fake_instance.dart';

/// Another device of Alex, the one a read comes from.
const String _desktop = '01890a5d-ac96-774b-bcce-b302099a8d01';

void main() {
  HttpOverrides? saved;
  FakeInstance? fake;
  PhoneDevice? device;

  final banner = find.byKey(const ValueKey('notification.banner'));

  Future<FakeInstance> start(String platform) async {
    final instance = await FakeInstance.start(platform: platform);
    final seed = instance.seed;
    instance.server.world
      ..notifyOnMessages = true
      ..setNotificationPref(seed.alex.id, 'channel', seed.general, 'all')
      ..setNotificationPref(seed.alex.id, 'channel', seed.support, 'all');
    return instance;
  }

  setUp(() {
    saved = HttpOverrides.current;
    HttpOverrides.global = null;
    fake = null;
    device = null;
  });

  tearDown(() async {
    device?.dispose();
    await fake?.stop();
    HttpOverrides.global = saved;
  });

  for (final platform in ['ios', 'android']) {
    testWidgets('$platform in front: a hint of the gateway shows as a banner '
        'and a tap opens the thread at the message', (tester) async {
      final instance = fake = (await tester.runAsync(() => start(platform)))!;
      final seed = instance.seed;
      final world = instance.server.world;
      final app = device = await PhoneDevice.start(tester, instance);
      final own = app.container
          .read(signedInServicesProvider)!
          .auth
          .session!
          .deviceId
          .value;
      expect(banner, findsNothing);

      final message = seed.post(seed.sam, seed.releaseNotes, 'Build is green');

      // The server delivered the hint over the gateway connection of this phone, so by 13 4.6
      // no push goes to it: what the app shows now is all the person gets.
      final hint = world.notifications.single;
      expect(hint.recipient, seed.alex.id);
      expect(hint.devices, {own});

      await app.until('the banner', () => banner.evaluate().isNotEmpty);
      expect(
        find.descendant(
          of: banner,
          matching: find.text('general · Release notes'),
        ),
        findsOneWidget,
      );
      expect(
        find.descendant(of: banner, matching: find.text('Sam: Build is green')),
        findsOneWidget,
      );

      await tester.tap(find.byKey(const ValueKey('notification.banner.open')));
      final target = '/t/${seed.releaseNotes}?m=${message['id']}';
      await app.until(
        'the thread at the message',
        () => app.location == target,
      );
      await tester.pump();
      expect(banner, findsNothing, reason: 'the opened banner is gone');
      expect(instance.server.violations, isEmpty);
      await app.end();
    });
  }

  test('the banner of an account goes with the account', () async {
    final instance = fake = await start('ios');
    final seed = instance.seed;
    final c = appContainer(instance);
    addTearDown(c.dispose);
    // The app widget listens to the provider; a listener keeps it active the same way.
    final hints = c.listen(desktopNotificationsProvider, (_, _) {});
    addTearDown(hints.close);
    final banners = c.read(inAppBannersProvider)!;
    final control = await connected(c, instance);
    await control.loginWithPassword(
      WorldSeed.alexLogin,
      WorldSeed.password,
      WorldSeed.totp,
    );
    await eventually(
      'live',
      () => c.read(syncEngineProvider)?.state.phase == ConnectionPhase.live,
    );
    seed.post(seed.sam, seed.releaseNotes, 'Build is green');
    await eventually('the banner', () => banners.current.value != null);
    expect(banners.current.value!.content.body, 'Sam: Build is green');

    await control.logout();
    // Nothing of the account that ended is kept for the next one of this app.
    await eventually('no banner', () => banners.current.value == null);
  });

  test('the banner of a thread goes when the thread is read on another '
      'device', () async {
    final instance = fake = await start('ios');
    final seed = instance.seed;
    final c = appContainer(instance);
    addTearDown(c.dispose);
    final hints = c.listen(desktopNotificationsProvider, (_, _) {});
    addTearDown(hints.close);
    final banners = c.read(inAppBannersProvider)!;
    final control = await connected(c, instance);
    await control.loginWithPassword(
      WorldSeed.alexLogin,
      WorldSeed.password,
      WorldSeed.totp,
    );
    await eventually(
      'live',
      () => c.read(syncEngineProvider)?.state.phase == ConnectionPhase.live,
    );
    final message = seed.post(seed.sam, seed.releaseNotes, 'Build is green');
    await eventually('the banner', () => banners.current.value != null);

    // Alex reads the thread on the desktop: `read_state.updated` reaches the phone through
    // the gateway, and the banner goes (01 10.3).
    instance.server.world.putReadState(
      seed.alex.id,
      seed.releaseNotes,
      message['thread_seq']! as int,
      deviceId: _desktop,
    );
    await eventually('no banner', () => banners.current.value == null);
  });

  testWidgets('no banner for the thread that is open on the phone', (
    tester,
  ) async {
    final instance = fake = (await tester.runAsync(() => start('ios')))!;
    final seed = instance.seed;
    final app = device = await PhoneDevice.start(tester, instance);
    app.router.go('/t/${seed.releaseNotes}');
    await app.until(
      'the history of the thread',
      () => find
          .byKey(ValueKey('historyColumn.${seed.releaseNotes}.null'))
          .evaluate()
          .isNotEmpty,
    );
    // The app reported the thread as on screen (04 6.2), as a history column does.
    await app.until(
      'the focus',
      () => app.container
          .read(syncEngineProvider)!
          .focus
          .threads
          .any((t) => t.value == seed.releaseNotes),
    );

    // The fake sends the hint whatever the focus says, so the rule of the app decides here: it
    // receives the hint and shows nothing for the thread the person is reading.
    final hints = <NotificationCreated>[];
    final received = app.container
        .read(syncEngineProvider)!
        .notifications
        .listen(hints.add);
    seed.post(seed.sam, seed.releaseNotes, 'Build is green');
    await app.until('the hint of the open thread', () => hints.length == 1);
    await app.settle();
    expect(banner, findsNothing);

    // A hint of another thread shows over the open one.
    seed.post(seed.sam, seed.printer, 'Paper jam');
    await app.until('the hint of the other thread', () => hints.length == 2);
    await app.until('the banner', () => banner.evaluate().isNotEmpty);
    expect(
      find.descendant(of: banner, matching: find.text('support · Printer')),
      findsOneWidget,
    );
    await tester.runAsync(received.cancel);
    await app.end();
  });
}
