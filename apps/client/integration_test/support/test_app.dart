// SPDX-License-Identifier: Apache-2.0

// The test entry of the desktop app (brief M4-18 Scope; M4-17 Design note 8): the app with
// the ports of a test against the fake server on 127.0.0.1. Only this entry allows plain HTTP;
// it lives in a test folder, so no release build contains it. Secrets stay in memory and the
// device keys are software keys, so no test touches the keychain or the credential store.

import 'package:client/app/app.dart';
import 'package:client/app/bootstrap.dart' show FileProtector;
import 'package:client/app/providers/ports.dart';
import 'package:client/desktop/window_behavior.dart';
import 'package:client/files/file_ports.dart';
import 'package:client/lock/app_lock_port.dart';
import 'package:client/notifications/notification_presenter.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:client/platform/device_key_signer.dart' show DeviceKeyStore;
import 'package:client/share/share_android.dart';
import 'package:client/share/share_apple.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:sync_engine/testing.dart';

import '../../test/support/fake_instance.dart';
import '../../test/support/fakes.dart';
import '../../test/support/other_device.dart';
import '../../test/support/ready.dart';
import '../../test/support/until.dart' show testPatience;

export '../../test/support/fake_instance.dart' show FakeInstance, eventually;
export '../../test/support/other_device.dart' show OtherDevice;
export '../../test/support/ready.dart' show tapWhenEnabled;

/// The running test app and its fake.
class TestApp {
  TestApp._(this.tester, this.fake);

  /// The tester.
  final WidgetTester tester;

  /// The fake server, its world and the app's ports.
  final FakeInstance fake;

  /// Starts the fake and pumps the app; the caller calls [stop]. [platform], [window] and
  /// [notifications] are the desktop ports of M4-30; [appleShare], [androidShare] and
  /// [deviceKeys] are the platform ports of the share device tests (M5-06).
  static Future<TestApp> start(
    WidgetTester tester, {
    Map<String, String> config = fastConfig,
    FilePorts? files,
    String platform = 'macos',
    DesktopWindow window = const NoDesktopWindow(),
    NotificationPresenter notifications = const NoNotificationPresenter(),
    AppLifecycle? lifecycle,
    FileProtector? protectFiles,
    AppLockPort appLock = const NoAppLockPort(),
    AppleShare? appleShare,
    AndroidShare? androidShare,
    DeviceKeyStore? deviceKeys,
  }) async {
    // A phone build (brief M5-02) gets a phone window, the desktop a desktop window.
    final phone = platform == 'ios' || platform == 'android';
    tester.view
      ..physicalSize = phone ? const Size(390, 844) * 3 : const Size(1440, 1000)
      ..devicePixelRatio = phone ? 3 : 1;
    final fake = await FakeInstance.start(
      config: config,
      files: files,
      platform: platform,
      window: window,
      notifications: notifications,
      lifecycle: lifecycle,
      protectFiles: protectFiles,
      appLock: appLock,
      appleShare: appleShare,
      androidShare: androidShare,
      deviceKeys: deviceKeys,
    );
    final app = TestApp._(tester, fake);
    await app.pumpApp();
    return app;
  }

  /// Pumps the app on the same ports and data, as a new start of the app does.
  Future<void> pumpApp() async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [appPortsProvider.overrideWithValue(fake.ports)],
        child: const ClientApp(),
      ),
    );
    await tester.pump();
  }

  /// Ends the running app and starts it again on the same ports and data (brief M4-30: a
  /// restart after Quit).
  Future<void> restart() async {
    await tester.pumpWidget(const SizedBox());
    await tester.pump();
    await pumpApp();
  }

  /// The app's providers.
  ProviderContainer get container =>
      ProviderScope.containerOf(tester.element(find.byType(ClientApp)));

  /// The onboarding state.
  OnboardingState get state => container.read(onboardingControllerProvider);

  /// The device ID of this app's session.
  String get deviceId => state.services!.auth.session!.deviceId.value;

  /// Pumps until [finder] finds something; fails after [timeout].
  Future<void> waitFor(Finder finder, {Duration timeout = testPatience}) async {
    final end = DateTime.now().add(timeout);
    while (finder.evaluate().isEmpty) {
      if (DateTime.now().isAfter(end)) fail('timed out waiting for $finder');
      await tester.pump(const Duration(milliseconds: 50));
    }
  }

  /// Taps the widget with [key] once it is shown and enabled ([tapWhenEnabled]) and settles.
  Future<void> tap(String key) =>
      tapWhenEnabled(tester, find.byKey(ValueKey(key)));

  /// E1: types the fake's address and connects; waits for E2.
  Future<void> connect() async {
    await waitFor(find.byKey(const ValueKey('e1.address')));
    await tester.enterText(
      find.byKey(const ValueKey('e1.address')),
      fake.server.baseUri.toString(),
    );
    await tester.tap(find.byKey(const ValueKey('e1.connect')));
    await waitFor(find.byKey(const ValueKey('screen.e2')));
  }

  /// E2: password and one-time code as Alex.
  Future<void> loginWithPassword() async {
    await tester.enterText(
      find.byKey(const ValueKey('e2.loginName')),
      WorldSeed.alexLogin,
    );
    await tester.tap(find.byKey(const ValueKey('e2.password.open')));
    await tester.pump();
    await tester.enterText(
      find.byKey(const ValueKey('e2.password')),
      WorldSeed.password,
    );
    await tester.enterText(
      find.byKey(const ValueKey('e2.code')),
      WorldSeed.totp,
    );
    await tester.tap(find.byKey(const ValueKey('e2.submit')));
  }

  /// Waits until the engine is live.
  Future<void> waitLive() async {
    final end = DateTime.now().add(testPatience);
    while (state.services?.engine.state.phase != ConnectionPhase.live) {
      if (DateTime.now().isAfter(end)) fail('timed out waiting for live');
      await tester.pump(const Duration(milliseconds: 50));
    }
  }

  /// An active second device of Alex, signed in.
  Future<OtherDevice> otherDevice() async {
    final other = OtherDevice(fake.server);
    await other.signIn();
    return other;
  }

  /// A second app of Alex on the same fake, headless (its own data, keys and engine; brief
  /// M4-25 Tests). The caller calls [SecondApp.stop].
  Future<SecondApp> secondApp() async {
    final instance = await fake.sibling();
    return SecondApp._(instance, appContainer(instance));
  }

  /// Opens the settings section [segment] from the shell.
  Future<void> openSettings(String segment) async {
    await tester.tap(find.byKey(const ValueKey('sidebar.settings')));
    await waitFor(find.byKey(const ValueKey('screen.settings')));
    await tester.tap(find.byKey(ValueKey('settings.nav.$segment')));
    await tester.pumpAndSettle();
  }

  /// The address the app remembers.
  Uri get address => fake.server.baseUri;

  /// Removes the app, waits until its services are closed and stops the fake.
  ///
  /// Removing the app begins the close of its services without waiting (as when it quits);
  /// the data folder can only go once the database is closed, so this waits for that one
  /// close before the fake stops and the folder is deleted.
  Future<void> stop() async {
    final services = state.services;
    await tester.pumpWidget(const SizedBox());
    await services?.close();
    await fake.stop();
    tester.view.reset();
  }
}

/// The second, headless app of Alex.
class SecondApp {
  SecondApp._(this.instance, this.container);

  /// Its ports and data folder.
  final FakeInstance instance;

  /// Its providers.
  final ProviderContainer container;

  /// Its onboarding state.
  OnboardingState get state => container.read(onboardingControllerProvider);

  /// Connects and logs in with password and one-time code as Alex.
  Future<void> signIn() async {
    final control = await connected(container, instance);
    await control.loginWithPassword(
      WorldSeed.alexLogin,
      WorldSeed.password,
      WorldSeed.totp,
    );
  }

  /// Releases the providers, waits until its services are closed and deletes the data folder.
  Future<void> stop() async {
    final services = state.services;
    container.dispose();
    await services?.close();
    await instance.stopSibling();
  }
}
