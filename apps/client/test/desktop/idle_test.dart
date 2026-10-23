// SPDX-License-Identifier: Apache-2.0

// `idle` in the focus op (brief M4-30 Tests and Design note 3; 13 4.4, 04 6.2): the app reports
// `idle: true` while its window is hidden to the notification area, minimized or behind another
// window, and `idle: false` as soon as it is the foreground window again. Headless against the
// fake server with the app's providers; the window is a fake that the test drives.

import 'dart:io';

import 'package:client/app/providers/session.dart';
import 'package:client/desktop/providers.dart';
import 'package:client/desktop/window_behavior.dart';
import 'package:client/thread/providers.dart';
import 'package:domain/domain.dart';
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

  setUp(() async {
    saved = HttpOverrides.current;
    HttpOverrides.global = null;
    window = FakeWindow();
    fake = await FakeInstance.start(platform: 'windows', window: window);
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

  /// The `idle` of the last `focus` op the app sent.
  bool? lastIdle() {
    final focus = fake.server.gateway.frames.where(
      (f) => !f.fromServer && f.op == 'focus',
    );
    return focus.isEmpty ? null : focus.last.data['idle'] as bool?;
  }

  Future<void> expectIdle(bool idle, String why) =>
      eventually('idle $idle after $why', () => lastIdle() == idle);

  test(
    'hidden, minimized or in the background: idle; foreground: not',
    () async {
      // The window state reaches the focus only while the app listens to it.
      final sub = c.listen(windowFocusedProvider, (_, _) {});
      c
          .read(visibleThreadsProvider.notifier)
          .add(ThreadId.parse(fake.seed.releaseNotes));
      await expectIdle(false, 'the start in the foreground');

      window.send(DesktopWindowEvent.close);
      await expectIdle(true, 'closing to the notification area');
      expect(window.calls, contains('hide'));
      expect(
        c.read(syncEngineProvider)?.state.phase,
        ConnectionPhase.live,
        reason: 'the connection stays while hidden',
      );

      await c.read(windowBehaviorProvider).open();
      await expectIdle(false, 'Open from the tray');

      window.send(DesktopWindowEvent.minimize);
      await expectIdle(true, 'minimizing');
      window
        ..send(DesktopWindowEvent.restore)
        ..send(DesktopWindowEvent.focus);
      await expectIdle(false, 'restoring');

      window.send(DesktopWindowEvent.blur);
      await expectIdle(true, 'another window in the foreground');
      window.send(DesktopWindowEvent.focus);
      await expectIdle(false, 'the foreground again');
      sub.close();
    },
  );
}
