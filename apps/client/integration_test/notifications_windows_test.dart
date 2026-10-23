// SPDX-License-Identifier: Apache-2.0

// Local notifications in the notification area (brief M4-30 Tests; 13 4.6 and 5.3, 15 10.3):
// with the window hidden to the notification area, a message from Sam in a channel of Alex makes
// the fake send `notification.created`, and the presenter shows one toast with the expected title
// and text (a recording wrapper around the real presenter). After Quit the fake sees no
// connection for the device and creates nothing; after a restart the unread counters show the
// message. On Windows the whole flow runs; on macOS the run checks the presenter path only.

import 'dart:async';
import 'dart:io';

import 'package:client/app/branding.dart';
import 'package:client/desktop/providers.dart';
import 'package:client/desktop/window_behavior.dart';
import 'package:client/notifications/notification_presenter.dart';
import 'package:client/notifications/toast_content.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';
import 'package:l10n/l10n.dart';

import '../test/support/desktop_fakes.dart';
import 'support/test_app.dart';

final AppLocalizations en = lookupAppLocalizations(const Locale('en'));

/// Records every toast and hands it to the real presenter of the platform.
class RecordingWrapper implements NotificationPresenter {
  RecordingWrapper(this.inner);

  final NotificationPresenter inner;
  final List<({ToastContent content, String tag, bool silent})> shown = [];

  @override
  Future<void> show(
    ToastContent content, {
    required String tag,
    required bool silent,
    required String payload,
  }) async {
    shown.add((content: content, tag: tag, silent: silent));
    // The real path runs, but a system that asks for a permission must not block the test.
    await inner
        .show(content, tag: tag, silent: silent, payload: payload)
        .timeout(const Duration(seconds: 5), onTimeout: () {});
  }

  @override
  Future<void> cancelThread(String tag) => inner.cancelThread(tag);

  @override
  Stream<String> get clicks => inner.clicks;

  @override
  Future<String?> launchPayload() => inner.launchPayload();

  @override
  Future<void> dispose() => inner.dispose();
}

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets(
    'a toast while running in the notification area, none after Quit',
    (tester) async {
      const branding = BuildBranding.fromEnvironment;
      final presenter = RecordingWrapper(
        LocalNotificationPresenter(
          appName: branding.appName,
          appUserModelId: branding.bundleId,
          guid: LocalNotificationPresenter.guidFor(branding.bundleId),
        ),
      );
      final window = FakeWindow();
      final windows = Platform.isWindows;
      final app = await TestApp.start(
        tester,
        platform: windows ? 'windows' : 'macos',
        window: window,
        notifications: presenter,
      );
      final world = app.fake.server.world;
      final seed = app.fake.seed;
      try {
        await app.connect();
        await app.loginWithPassword();
        await app.waitFor(find.byKey(const ValueKey('column.sidebar')));
        await app.waitLive();
        world
          ..notifyOnMessages = true
          // Messages of the channel reach Alex at level `all` (13 3.1).
          ..setNotificationPref(seed.alex.id, 'channel', seed.general, 'all');

        if (windows) {
          window.send(DesktopWindowEvent.close);
          await tester.pump(const Duration(milliseconds: 100));
          expect(window.calls, contains('hide'));
        }
        seed.post(seed.sam, seed.releaseNotes, 'Build is green');
        final end = DateTime.now().add(const Duration(seconds: 20));
        while (presenter.shown.isEmpty) {
          if (DateTime.now().isAfter(end)) {
            fail('timed out waiting for a toast');
          }
          await tester.pump(const Duration(milliseconds: 50));
        }
        final toast = presenter.shown.single;
        expect(toast.content.title, 'general · Release notes');
        expect(toast.content.body, 'Sam: Build is green');
        expect(toast.silent, isFalse);
        if (!windows) return;

        // Quit: the services close, the device has no connection, the fake creates nothing.
        final device = app.deviceId;
        await app.container.read(windowBehaviorProvider).quit();
        expect(window.calls.last, 'destroy');
        await eventually(
          'no connection of the device',
          () => !app.fake.server.gateway.connections.any(
            (c) => c.open && c.deviceId == device,
          ),
        );
        final created = world.notifications.length;
        seed.post(seed.sam, seed.releaseNotes, 'Deploy is done');
        expect(world.notifications, hasLength(created));
        expect(
          world.notificationSkips.where((s) => s.userId == seed.alex.id).last,
          (userId: seed.alex.id, reason: 'no_route'),
        );
        expect(presenter.shown, hasLength(1));

        // Restart: the counters show both messages of Sam.
        await app.restart();
        await app.waitFor(find.byKey(const ValueKey('column.sidebar')));
        await app.waitLive();
        final semantics = tester.ensureSemantics();
        await app.waitFor(
          find.bySemanticsLabel('general, ${en.threadRowUnread(2)}'),
        );
        semantics.dispose();
      } finally {
        await app.stop();
      }
    },
  );
}
