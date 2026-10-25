// SPDX-License-Identifier: Apache-2.0

// The desktop app of the E2E suite against the real server of the Compose stack (brief M4-34
// Design note 2): the app with the ports of a test (a fresh data folder, memory secrets, software
// keys of e2e_keys.dart) and the discovery of a real build, which talks HTTPS only. Every network
// connection goes through the tunnel of e2e_network.dart. The flows of TestApp of the integration
// tests, with the waits bounded by [patience] instead of a fixed number of seconds.

import 'dart:io';

import 'package:client/app/app.dart';
import 'package:client/app/bootstrap.dart';
import 'package:client/app/discovery.dart';
import 'package:client/app/providers/ports.dart';
import 'package:client/desktop/start_at_login.dart';
import 'package:client/desktop/window_behavior.dart';
import 'package:client/files/file_ports.dart';
import 'package:client/notifications/notification_presenter.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:domain/domain.dart' show AppConfig;
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

import '../../test/support/fakes.dart';
import '../../test/support/ready.dart';
import 'e2e_keys.dart';
import 'e2e_transport.dart';
import 'e2e_world.dart';
import 'totp.dart';

export '../../test/support/ready.dart' show tapWhenEnabled;

/// The bound of every wait of the suite: the patience of the repository's test kits
/// (`testkit.Patience` of the Go services, `pumpUntil` of the widget tests). A wait ends as soon
/// as its condition holds; the bound only turns a hang into a failure.
const Duration patience = Duration(minutes: 3);

/// The TOTP steps the run used, starting with the step of the world (08 2.5: once per step).
final Set<int> _usedSteps = {};

/// One running app of the suite.
final class E2eApp {
  E2eApp._(
    this.tester,
    this.world,
    this.root,
    this.ports,
    this.keys,
    this.secure,
    this.transport,
  );

  /// Starts the app with a fresh data folder; [platform], [window] and [notifications] are the
  /// desktop ports of M4-30, [files] those of M4-22. The caller calls [stop].
  static Future<E2eApp> start(
    WidgetTester tester,
    E2eWorld world, {
    String platform = 'macos',
    DesktopWindow window = const NoDesktopWindow(),
    NotificationPresenter notifications = const NoNotificationPresenter(),
    FilePorts? files,
    Map<String, String> config = fastConfig,
  }) async {
    tester.view
      ..physicalSize = const Size(1440, 1000)
      ..devicePixelRatio = 1;
    final root = await Directory.systemTemp.createTemp('client-e2e');
    final keys = E2eKeyStore();
    final secure = MemorySecureBackend();
    final transport = RecordingTransport();
    final ports = AppPorts(
      branding: testBranding,
      platform: platform,
      notifications: notifications,
      window: window,
      startAtLogin: () async => const NoStartAtLogin(),
      deviceName: 'Alex laptop',
      secureBackend: secure,
      deviceKeys: keys,
      discovery: ApiDiscoveryClient(
        platform: platform,
        appVersion: testBranding.appVersion,
      ),
      dataRoot: () async => root,
      browser: RecordingBrowser(),
      redirects: SilentRedirects.new,
      transport: transport,
      lifecycle: ForegroundLifecycle(),
      network: SwitchNetwork(),
      ticker: MicrotaskTicker(),
      deepLinks: ManualDeepLinks(),
      config: AppConfig.defaults().withOverrides(config),
      openDatabase: (file, keys) =>
          openLocalStore(file: file, keys: keys, inBackground: false),
      files: files,
    );
    final app = E2eApp._(tester, world, root, ports, keys, secure, transport);
    await app.pumpApp();
    return app;
  }

  /// The tester.
  final WidgetTester tester;

  /// The world of the run.
  final E2eWorld world;

  /// The data folder.
  final Directory root;

  /// The ports of the app.
  final AppPorts ports;

  /// The device keys.
  final E2eKeyStore keys;

  /// The secure storage.
  final MemorySecureBackend secure;

  /// The gateway transport with its record.
  final RecordingTransport transport;

  /// Pumps the app on its ports and data, as a start of the app does.
  Future<void> pumpApp() async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [appPortsProvider.overrideWithValue(ports)],
        child: const ClientApp(),
      ),
    );
    await tester.pump();
  }

  /// The app's providers.
  ProviderContainer get container =>
      ProviderScope.containerOf(tester.element(find.byType(ClientApp)));

  /// The onboarding state.
  OnboardingState get state => container.read(onboardingControllerProvider);

  /// The device ID of this app's session.
  String get deviceId => state.services!.auth.session!.deviceId.value;

  /// Pumps until [finder] finds something; fails after [patience].
  Future<void> waitFor(Finder finder) =>
      until('$finder', () => finder.evaluate().isNotEmpty);

  /// Pumps until [condition] holds; fails naming [what] and the [detail] of that moment after
  /// [patience].
  Future<void> until(
    String what,
    bool Function() condition, {
    String Function()? detail,
  }) async {
    final watch = Stopwatch()..start();
    while (!condition()) {
      if (watch.elapsed >= patience) {
        fail('timed out waiting for $what ${detail?.call() ?? ''}');
      }
      await tester.pump(const Duration(milliseconds: 50));
    }
  }

  /// Taps the widget with [key] once it is shown and enabled and settles.
  Future<void> tap(String key) =>
      tapWhenEnabled(tester, find.byKey(ValueKey(key)));

  /// E1: types the server address and connects; waits for E2.
  Future<void> connect() async {
    await waitFor(find.byKey(const ValueKey('e1.address')));
    await tester.enterText(
      find.byKey(const ValueKey('e1.address')),
      world.server.toString(),
    );
    await tester.tap(find.byKey(const ValueKey('e1.connect')));
    await waitFor(find.byKey(const ValueKey('screen.e2')));
  }

  /// E2 "I have an enrollment code", E3: redeems [code] with this device's keys (08 3.2).
  Future<void> enroll(String code) async {
    await tap('e2.enroll');
    await waitFor(find.byKey(const ValueKey('screen.e3')));
    await tester.enterText(find.byKey(const ValueKey('e3.code')), code);
    await tester.pump();
    await tap('e3.continue');
  }

  /// E2: password and one-time code as Alex, with a TOTP step no login of the run used yet.
  Future<void> loginWithPassword() async {
    final alex = world.alex;
    _usedSteps.add(alex.lastStep);
    var step = totpStep(DateTime.now());
    // A step is valid once; the next one starts at most 30 s later.
    await until('a fresh TOTP step', () {
      step = totpStep(DateTime.now());
      return !_usedSteps.contains(step);
    });
    _usedSteps.add(step);
    await tester.enterText(
      find.byKey(const ValueKey('e2.loginName')),
      alex.handle,
    );
    await tester.tap(find.byKey(const ValueKey('e2.password.open')));
    await tester.pump();
    await tester.enterText(
      find.byKey(const ValueKey('e2.password')),
      alex.password!,
    );
    await tester.enterText(
      find.byKey(const ValueKey('e2.code')),
      totpCode(alex.totpSecret!, step),
    );
    await tester.tap(find.byKey(const ValueKey('e2.submit')));
  }

  /// Waits until the engine is live; a timeout names the engine's last state.
  Future<void> waitLive() => until(
    'live',
    () => state.services?.engine.state.phase == ConnectionPhase.live,
    detail: () =>
        '(engine ${state.services?.engine.state}, ${transport.describe()})',
  );

  /// Connects, redeems [code] and waits for the main view and the live engine.
  Future<void> signIn(String code) async {
    await connect();
    await enroll(code);
    await waitFor(find.byKey(const ValueKey('column.sidebar')));
    await waitLive();
  }

  /// Opens the channel [channelId] in the sidebar.
  Future<void> openChannel(String channelId) async {
    await waitFor(find.byKey(ValueKey('sidebar.channel.$channelId')));
    await tester.tap(find.byKey(ValueKey('sidebar.channel.$channelId')));
    await tester.pump();
  }

  /// Opens the forum post [threadId] of the forum [forumId].
  Future<void> openPost(String forumId, String threadId) async {
    await openChannel(forumId);
    await waitFor(find.byKey(ValueKey('postList.post.$threadId')));
    await tester.tap(find.byKey(ValueKey('postList.post.$threadId')));
    await waitFor(find.byKey(ValueKey('history.$threadId')));
  }

  /// Opens the settings section [segment] from the shell.
  Future<void> openSettings(String segment) async {
    await tester.tap(find.byKey(const ValueKey('sidebar.settings')));
    await waitFor(find.byKey(const ValueKey('screen.settings')));
    await tester.tap(find.byKey(ValueKey('settings.nav.$segment')));
    await tester.pumpAndSettle();
  }

  /// The folder of the instance in the data folder.
  Directory get instanceFolder =>
      Directory('${root.path}/instances/${world.instanceId}');

  /// Whether no file, key or secret of the instance is left (R-SY-14).
  Future<bool> wiped() async =>
      !instanceFolder.existsSync() &&
      keys.scopes.isEmpty &&
      !secure.values.keys.any((k) => k.contains(world.instanceId));

  /// Removes the app, waits until its services are closed and deletes the data folder.
  Future<void> stop() async {
    final services = state.services;
    await tester.pumpWidget(const SizedBox());
    await services?.close();
    final watch = Stopwatch()..start();
    // Windows refuses to delete a file a handle still holds for a moment (D-M4-18-21).
    while (root.existsSync()) {
      try {
        await root.delete(recursive: true);
      } on FileSystemException {
        if (watch.elapsed >= patience) rethrow;
        await Future<void>.delayed(const Duration(milliseconds: 50));
      }
    }
    tester.view.reset();
  }
}
