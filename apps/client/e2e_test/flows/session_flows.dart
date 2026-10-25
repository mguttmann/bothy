// SPDX-License-Identifier: Apache-2.0

// Notification and revocation flows (brief M4-34 product requirement 13): a notification while
// the app runs hidden in the notification area (the Windows behavior of M4-30; on Windows through
// the real presenter, elsewhere a recording one), and the owner revoking Alex's device, which
// closes the gateway with 4012 and ends the app in E7 with a wiped store (08 5.4, R-SY-14).

import 'dart:async';
import 'dart:io';

import 'package:client/app/branding.dart';
import 'package:client/desktop/window_behavior.dart';
import 'package:client/notifications/notification_presenter.dart';
import 'package:client/notifications/toast_content.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../test/support/desktop_fakes.dart';
import '../support/e2e_app.dart';
import '../support/e2e_context.dart';
import '../support/e2e_keys.dart';

/// Records every toast and hands it to [inner].
final class RecordingPresenter implements NotificationPresenter {
  /// Records in front of [inner].
  RecordingPresenter(this.inner);

  /// The presenter of the platform, or none.
  final NotificationPresenter inner;

  /// The shown toasts: content, slot and the app path a click opens.
  final List<({ToastContent content, String tag, String payload})> shown = [];

  @override
  Future<void> show(
    ToastContent content, {
    required String tag,
    required bool silent,
    required String payload,
  }) async {
    shown.add((content: content, tag: tag, payload: payload));
    // The real path runs; a system that asks for a permission must not block the test.
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

/// Registers the notification and revocation flows.
void sessionFlows(E2eContext c) {
  final world = c.world;

  testWidgets('a notification while hidden in the notification area', (
    tester,
  ) async {
    const branding = BuildBranding.fromEnvironment;
    final presenter = RecordingPresenter(
      Platform.isWindows
          ? LocalNotificationPresenter(
              appName: branding.appName,
              appUserModelId: branding.bundleId,
              guid: LocalNotificationPresenter.guidFor(branding.bundleId),
            )
          : const NoNotificationPresenter(),
    );
    final window = FakeWindow();
    final nonce = randomNonce().substring(0, 8);
    final app = await E2eApp.start(
      tester,
      world,
      platform: 'windows',
      window: window,
      notifications: presenter,
    );
    try {
      // A post of Alex's: its messages reach Alex at level all (13 3.1).
      final post = (await tester.runAsync(
        () => c.owner.createPost(world.forumId, 'Build $nonce', 'Status'),
      ))!;
      await tester.runAsync(() => c.owner.setLevel('thread', post, 'all'));
      await app.signIn(await c.alexCode());
      window.send(DesktopWindowEvent.close);
      await app.until('hidden', () => window.calls.contains('hide'));
      // The server learns the app's connection from gateway_presence, which every api process
      // writes once per cluster.presence_flush_interval (M7-17); a message before that finds no
      // route and gets no hint (13 4.6). So Sam writes again until the hint of one arrives.
      var n = 0;
      final watch = Stopwatch()..start();
      while (!presenter.shown.any((t) => t.tag == 'thread:$post')) {
        if (watch.elapsed >= patience) {
          fail('no toast: ${app.device.transport.describe()}');
        }
        await tester.runAsync(
          () => c.sam.post(post, 'Build is green $nonce ${n++}'),
        );
        await app.untilOr(
          () => presenter.shown.any((t) => t.tag == 'thread:$post'),
          const Duration(seconds: 3),
        );
      }
      final toast = presenter.shown.firstWhere((t) => t.tag == 'thread:$post');
      // A click opens the thread of the message. The texts of the toast are not checked: a hint
      // that crosses from the worker to the api carries no preview fields, so the toast shows
      // the placeholder (docs/decisions.d/M4-34.md).
      expect(toast.payload, startsWith('/t/$post'));
      expect(toast.content.title, isNotEmpty);
    } finally {
      await app.stop();
    }
  });

  testWidgets('revocation by the owner: 4012, E7 and a wiped store', (
    tester,
  ) async {
    final app = await E2eApp.start(tester, world);
    try {
      await app.signIn(await c.alexCode());
      final device = app.deviceId;
      await tester.runAsync(() => c.owner.revokeDevice(device));
      await app.waitFor(find.byKey(const ValueKey('screen.e7')));
      await app.until(
        'the close code 4012',
        () => app.device.transport.closeCodes.contains(4012),
        detail: () => app.device.transport.describe(),
      );
      await app.until('the wiped store', () => app.device.wiped);
    } finally {
      await app.stop();
    }
  });
}
