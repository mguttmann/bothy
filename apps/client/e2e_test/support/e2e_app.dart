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
import 'package:client/platform/device_key_signer.dart' show DeviceKeyStore;
import 'package:client/platform/push_platform.dart' show PushPlatform;
import 'package:client/platform/terminal_guard.dart' show TerminalPlatformGuard;
import 'package:client/share/share_android.dart' show AndroidShare;
import 'package:client/share/share_apple.dart' show AppleShare;
import 'package:domain/domain.dart' show AppConfig;
import 'package:flutter/gestures.dart' show kPrimaryButton;
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

import '../../test/support/fakes.dart';
import 'e2e_keys.dart';
import 'e2e_transport.dart';
import 'e2e_world.dart';
import 'totp.dart';

/// The bound of every wait of the suite: the patience of the repository's test kits
/// (`testkit.Patience` of the Go services, `pumpUntil` of the widget tests). A wait ends as soon
/// as its condition holds; the bound only turns a hang into a failure.
const Duration patience = Duration(minutes: 3);

/// The TOTP steps the run used, starting with the step of the world (08 2.5: once per step).
final Set<int> _usedSteps = {};

/// A TOTP code of Alex for a step no login of the run used yet; waits for the next step when
/// the current one is used (at most 30 s).
Future<String> freshTotp(E2eWorld world) async {
  _usedSteps.add(world.alex.lastStep);
  final watch = Stopwatch()..start();
  var step = totpStep(DateTime.now());
  while (_usedSteps.contains(step)) {
    if (watch.elapsed >= patience) fail('no fresh TOTP step');
    await Future<void>.delayed(const Duration(milliseconds: 200));
    step = totpStep(DateTime.now());
  }
  _usedSteps.add(step);
  return totpCode(world.alex.totpSecret!, step);
}

/// The data, keys, secrets and ports of one simulated device of the suite.
final class E2eDevice {
  E2eDevice._(
    this.world,
    this.root,
    this.ports,
    this.keys,
    this.secure,
    this.transport,
  );

  /// A device with a fresh data folder; [platform], [window] and [notifications] are the
  /// desktop ports of M4-30, [files] those of M4-22, [ticker] paces the run documents (the
  /// performance level uses the frames of the scheduler).
  ///
  /// The phone suites of brief M5-11 (e2e_test/phone) pass the ports of a real phone build:
  /// [lifecycle] and [network] (by default always in the foreground and online), [push] (by
  /// default none), [androidShare] (by default none) and the [deviceName] the registration
  /// carries.
  static Future<E2eDevice> create(
    E2eTarget world, {
    String platform = 'macos',
    DesktopWindow window = const NoDesktopWindow(),
    NotificationPresenter notifications = const NoNotificationPresenter(),
    FilePorts? files,
    FrameTicker? ticker,
    Map<String, String> config = fastConfig,
    DeviceKeyStore? deviceKeys,
    AppleShare? appleShare,
    AndroidShare? androidShare,
    AppLifecycle? lifecycle,
    NetworkStatus? network,
    PushPlatform? push,
    String deviceName = 'Alex laptop',
    TerminalPlatformGuard? terminalGuard,
  }) async {
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
      deviceName: deviceName,
      secureBackend: secure,
      // ID-T16 (M5-06) uses the platform's keys and the share glue: the Share Extension signs
      // with the `dpop` key of the shared keychain group.
      deviceKeys: deviceKeys ?? keys,
      appleShare: appleShare,
      androidShare: androidShare,
      push: push,
      discovery: ApiDiscoveryClient(
        platform: platform,
        appVersion: testBranding.appVersion,
      ),
      dataRoot: () async => root,
      browser: RecordingBrowser(),
      redirects: SilentRedirects.new,
      transport: transport,
      lifecycle: lifecycle ?? ForegroundLifecycle(),
      network: network ?? SwitchNetwork(),
      terminalGuard: terminalGuard,
      ticker: ticker ?? MicrotaskTicker(),
      deepLinks: ManualDeepLinks(),
      config: AppConfig.defaults().withOverrides(config),
      openDatabase: (file, keys) =>
          openLocalStore(file: file, keys: keys, inBackground: false),
      files: files,
    );
    return E2eDevice._(world, root, ports, keys, secure, transport);
  }

  /// The stack of the run.
  final E2eTarget world;

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

  /// The folder of the instance in the data folder.
  Directory get instanceFolder =>
      Directory('${root.path}/instances/${world.instanceId}');

  /// Whether no file, key or secret of the instance is left (R-SY-14).
  bool get wiped =>
      !instanceFolder.existsSync() &&
      keys.scopes.isEmpty &&
      !secure.values.keys.any((k) => k.contains(world.instanceId));

  /// Deletes the data folder once the app's database is closed; Windows refuses to delete a
  /// file a handle still holds for a moment (D-M4-18-21).
  Future<void> deleteRoot() async {
    final watch = Stopwatch()..start();
    while (root.existsSync()) {
      try {
        await root.delete(recursive: true);
      } on FileSystemException {
        if (watch.elapsed >= patience) rethrow;
        await Future<void>.delayed(const Duration(milliseconds: 50));
      }
    }
  }
}

/// One running app of the suite.
final class E2eApp {
  E2eApp._(this.tester, this.device);

  /// Starts the app on a fresh [E2eDevice] (see [E2eDevice.create] for the ports). The caller
  /// calls [stop].
  ///
  /// With [deviceView] the app keeps the view of the device it runs on (the phone suites of
  /// brief M5-11 on a simulator or emulator, whose native steps see the real screen); without,
  /// it gets a fixed window of its form factor.
  static Future<E2eApp> start(
    WidgetTester tester,
    E2eTarget world, {
    String platform = 'macos',
    DesktopWindow window = const NoDesktopWindow(),
    NotificationPresenter notifications = const NoNotificationPresenter(),
    FilePorts? files,
    FrameTicker? ticker,
    Map<String, String> config = fastConfig,
    DeviceKeyStore? deviceKeys,
    AppleShare? appleShare,
    AndroidShare? androidShare,
    AppLifecycle? lifecycle,
    NetworkStatus? network,
    PushPlatform? push,
    String deviceName = 'Alex laptop',
    bool deviceView = false,
    TerminalPlatformGuard? terminalGuard,
  }) async {
    if (!deviceView) {
      // A phone build (ID-T16 on the iOS simulator, M5-06) gets a phone window.
      final phone = platform == 'ios' || platform == 'android';
      tester.view
        ..physicalSize = phone
            ? const Size(390, 844) * 3
            : const Size(1440, 1000)
        ..devicePixelRatio = phone ? 3 : 1;
    }
    final device = await E2eDevice.create(
      world,
      platform: platform,
      window: window,
      notifications: notifications,
      files: files,
      ticker: ticker,
      config: config,
      deviceKeys: deviceKeys,
      appleShare: appleShare,
      androidShare: androidShare,
      lifecycle: lifecycle,
      network: network,
      push: push,
      deviceName: deviceName,
      terminalGuard: terminalGuard,
    );
    // The integration binding leaves the platform's text input in place; the suite types through
    // the test text input, which also works in a profile build (the performance level).
    if (!tester.testTextInput.isRegistered) tester.testTextInput.register();
    final app = E2eApp._(tester, device);
    await app.pumpApp();
    return app;
  }

  /// The tester.
  final WidgetTester tester;

  /// The data, keys and ports of the app.
  final E2eDevice device;

  /// The stack of the run.
  E2eTarget get world => device.world;

  /// The world of the chat flows, for the password login of Alex.
  E2eWorld get _chatWorld => switch (world) {
    final E2eWorld w => w,
    _ => throw StateError('the password login needs the world of e2e.json'),
  };

  /// Pumps the app on its ports and data, as a start of the app does.
  Future<void> pumpApp() async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [appPortsProvider.overrideWithValue(device.ports)],
        child: const ClientApp(),
      ),
    );
    await tester.pump();
  }

  /// Ends the running app, waits until its services are closed and starts it again on the
  /// same data, as a start after Quit does (brief M4-30); the caller waits for what it needs.
  Future<void> restart() async {
    await shutDown();
    await pumpApp();
  }

  /// Ends the running app and waits until its services are closed; [pumpApp] starts it again.
  Future<void> shutDown() async {
    final services = state.services;
    await tester.pumpWidget(const SizedBox());
    await services?.close();
  }

  /// The app's providers.
  ProviderContainer get container =>
      ProviderScope.containerOf(tester.element(find.byType(ClientApp)));

  /// The onboarding state.
  OnboardingState get state => container.read(onboardingControllerProvider);

  /// The device ID of this app's session.
  String get deviceId => state.services!.auth.session!.deviceId.value;

  /// Pumps until [finder] finds something; fails after [patience]. The description names the
  /// finder without evaluating it, so it never reads elements of a tree that has changed.
  Future<void> waitFor(Finder finder) => until(
    finder.toString(describeSelf: true),
    () => finder.evaluate().isNotEmpty,
  );

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
        fail(
          'timed out waiting for $what ${detail?.call() ?? ''}\n'
          'keys on screen: ${_keysOnScreen()}',
        );
      }
      await tester.pump(const Duration(milliseconds: 50));
    }
  }

  /// The string keys of the widgets on screen, for a failure message.
  String _keysOnScreen() {
    final keys = <String>{
      for (final e
          in find
              .byWidgetPredicate((w) => w.key is ValueKey<String>)
              .evaluate())
        (e.widget.key! as ValueKey<String>).value,
    };
    return keys.take(120).join(' ');
  }

  /// Pumps until [condition] holds or [interval] passed, whichever comes first; the poll of a
  /// step that repeats an action until the server reacts (never the bound of a test).
  Future<void> untilOr(bool Function() condition, Duration interval) async {
    final watch = Stopwatch()..start();
    while (!condition() && watch.elapsed < interval) {
      await tester.pump(const Duration(milliseconds: 50));
    }
  }

  /// Pumps until [condition] completes with true; for conditions that ask the server.
  Future<void> untilAsync(
    String what,
    Future<bool> Function() condition,
  ) async {
    final watch = Stopwatch()..start();
    while (!(await tester.runAsync(condition) ?? false)) {
      if (watch.elapsed >= patience) fail('timed out waiting for $what');
      await tester.pump(const Duration(milliseconds: 100));
    }
  }

  /// Taps the widget with [key] once it is shown and enabled.
  Future<void> tap(String key) => tapFinder(find.byKey(ValueKey(key)));

  /// Taps the first widget of [finder] once it is shown and its button is enabled, then pumps
  /// one frame. It never waits for the app to settle: against a live server a progress
  /// indicator may run for as long as the server takes, so the next step waits for what it
  /// needs instead.
  Future<void> tapFinder(Finder finder, {int buttons = kPrimaryButton}) async {
    await until(
      '${finder.toString(describeSelf: true)} shown and enabled',
      () => _enabled(finder),
    );
    await tester.ensureVisible(finder.first);
    await tester.tap(finder.first, buttons: buttons);
    await tester.pump();
  }

  /// Taps the first widget of [finder] like [tapFinder], after the route that shows it (a menu,
  /// a dialog) finished its entrance: a tap during the animation of a popup menu lands outside
  /// the item while it still grows, and the menu closes without a choice.
  Future<void> tapSettled(Finder finder) async {
    await until(
      '${finder.toString(describeSelf: true)} shown in a settled route',
      () {
        if (!_enabled(finder)) return false;
        final route = ModalRoute.of(finder.evaluate().first);
        return route == null ||
            route.animation == null ||
            route.animation!.status == AnimationStatus.completed;
      },
    );
    await tester.tap(finder.first);
    await tester.pump();
  }

  /// E1: types the server address and connects; waits for E2.
  Future<void> connect() async {
    await waitFor(find.byKey(const ValueKey('e1.address')));
    await tester.enterText(
      find.byKey(const ValueKey('e1.address')),
      world.server.toString(),
    );
    await tap('e1.connect');
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
    final world = _chatWorld;
    final totp = (await tester.runAsync(() => freshTotp(world)))!;
    await tester.enterText(
      find.byKey(const ValueKey('e2.loginName')),
      world.alex.handle,
    );
    await tap('e2.password.open');
    await tester.enterText(
      find.byKey(const ValueKey('e2.password')),
      world.alex.password!,
    );
    await tester.enterText(find.byKey(const ValueKey('e2.code')), totp);
    await tap('e2.submit');
  }

  /// Waits until the engine is live; a timeout names the engine's last state.
  Future<void> waitLive() => until(
    'live',
    () => state.services?.engine.state.phase == ConnectionPhase.live,
    detail: () =>
        '(engine ${state.services?.engine.state}, '
        '${device.transport.describe()})',
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
    await tap('sidebar.channel.$channelId');
  }

  /// Opens the forum post [threadId] of the forum [forumId].
  Future<void> openPost(String forumId, String threadId) async {
    await openChannel(forumId);
    await tap('postList.post.$threadId');
    await waitFor(find.byKey(ValueKey('history.$threadId')));
  }

  /// Opens the settings section [segment] from the shell.
  Future<void> openSettings(String segment) async {
    await tap('sidebar.settings');
    await waitFor(find.byKey(const ValueKey('screen.settings')));
    await tap('settings.nav.$segment');
  }

  /// Removes the app, waits until its services are closed and deletes the data folder.
  Future<void> stop() async {
    final services = state.services;
    await tester.pumpWidget(const SizedBox());
    await services?.close();
    await device.deleteRoot();
    tester.view.reset();
    if (tester.testTextInput.isRegistered) tester.testTextInput.unregister();
  }
}

/// A second app of Alex, headless: its own data, keys and engine (brief M4-25 Tests).
final class E2eSecondApp {
  E2eSecondApp._(this.world, this.device, this.container);

  /// Starts the providers of a second app on a fresh device. The caller calls [stop].
  static Future<E2eSecondApp> start(E2eWorld world) async {
    final device = await E2eDevice.create(world);
    final container = ProviderContainer(
      overrides: [appPortsProvider.overrideWithValue(device.ports)],
    );
    return E2eSecondApp._(world, device, container);
  }

  /// The world of the run.
  final E2eWorld world;

  /// Its data, keys and ports.
  final E2eDevice device;

  /// Its providers.
  final ProviderContainer container;

  /// Its onboarding state.
  OnboardingState get state => container.read(onboardingControllerProvider);

  /// Connects and logs in with password and one-time code as Alex.
  Future<void> loginWithPassword() async {
    container.read(onboardingControllerProvider);
    await Future<void>.delayed(Duration.zero);
    final control = container.read(onboardingControllerProvider.notifier);
    await control.connect(world.server.toString());
    await control.loginWithPassword(
      world.alex.handle,
      world.alex.password!,
      await freshTotp(world),
    );
  }

  /// Releases the providers, waits until its services are closed and deletes the data folder.
  Future<void> stop() async {
    final services = state.services;
    container.dispose();
    await services?.close();
    await device.deleteRoot();
  }
}

/// Whether the first widget of [finder] is shown and its button (the first
/// [ButtonStyleButton], [IconButton] or [InkResponse] at or below it) is enabled; a widget
/// without one counts as enabled once it is shown (the rule of `tapWhenEnabled` of the
/// integration tests).
bool _enabled(Finder finder) {
  final found = finder.evaluate();
  if (found.isEmpty) return false;
  bool? enabled;
  void visit(Element e) {
    if (enabled != null) return;
    enabled = switch (e.widget) {
      ButtonStyleButton(:final enabled) => enabled,
      IconButton(:final onPressed) => onPressed != null,
      InkResponse(:final onTap) => onTap != null,
      _ => null,
    };
    if (enabled == null) e.visitChildren(visit);
  }

  visit(found.first);
  return enabled ?? true;
}
