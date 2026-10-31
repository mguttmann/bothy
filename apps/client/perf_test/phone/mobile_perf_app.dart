// SPDX-License-Identifier: Apache-2.0

// The phone app of the mobile performance level (brief M5-11 Design note 5) against the fake
// server of perf_fake.dart. The ports are those of a release build of the phone wherever a port
// does work the targets count: the platform's device keys sign the DPoP proofs (the software keys
// of the integration tests would sign in Dart on the app's isolate), the platform's secure
// storage holds the tokens and the database key, the database is encrypted on its own isolate,
// and the run documents follow the frames of the scheduler. What stays from the tests: the fake's
// plain HTTP on 127.0.0.1, a fresh data folder, a network switch for the catch-up, no deep links,
// browser or app lock, and the gateway transport of the E2E apps, which records when a status
// line arrives and counts dispatched events. The view keeps the size of the phone it runs on.
//
// On a host that is no phone (the smoke run of the harness with `flutter test` on the Flutter
// tester, which has no platform plugins) the keys and secrets are those of the integration
// tests; such a run checks the flow, its values are never evidence (mobile_perf_summary.dart).

import 'dart:developer' show Timeline;
import 'dart:io';
import 'dart:math' show Random;

import 'package:client/app/app.dart';
import 'package:client/app/bootstrap.dart';
import 'package:client/app/discovery.dart';
import 'package:client/app/providers/ports.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:client/platform/device_key_signer.dart';
import 'package:client/platform/frame_ticker.dart';
import 'package:client/platform/secure_store.dart';
import 'package:device_key/device_key.dart' as dk;
import 'package:domain/domain.dart' show AppConfig;
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:sync_engine/testing.dart' show WorldSeed;

import '../../e2e_test/support/e2e_transport.dart';
import '../../test/support/fakes.dart';
import '../../test/support/software_keys.dart';
import 'perf_fake.dart';

/// The bound of every wait of the level: the patience of the repository's test kits. A wait
/// ends as soon as its condition holds; the bound only turns a hang into a failure.
const Duration patience = Duration(minutes: 3);

/// The platform of the app's form factor: the phone's own, `ios` on another host (a run there
/// checks the harness and is never evidence).
String get phonePlatform => Platform.isAndroid ? 'android' : 'ios';

/// One running app of the level and the data it keeps between its starts.
final class MobilePerfApp {
  MobilePerfApp._(
    this.tester,
    this.fake,
    this.root,
    this.ports,
    this.transport,
    this.network,
    this._keys,
    this._secrets,
  );

  /// The first part of the names of every run's secrets.
  static const String _runPrefix = 'perf-run-';

  /// Starts the app against [fake] on a fresh data folder; the caller calls [stop]. Throws the
  /// error of the platform's secure storage when it cannot be read or written.
  static Future<MobilePerfApp> start(WidgetTester tester, PerfFake fake) async {
    final platform = phonePlatform;
    final root = await Directory.systemTemp.createTemp('client-perf');
    final transport = PerfTransport();
    final network = SwitchNetwork();
    final phone = Platform.isIOS || Platform.isAndroid;
    final DeviceKeyStore keys = phone
        ? PlatformDeviceKeyStore(
            dk.DeviceKeyClient(),
            () => const dk.StepUpPrompt(
              title: 'Confirm',
              negativeButton: 'Cancel',
            ),
          )
        : SoftwareKeyStore();
    // A namespace of this run in the platform's store, so nothing of another install or run is
    // read or overwritten and [stop] removes exactly what the run wrote. The fake's instance ID
    // is the same in every run, so the name has a random part, and what an aborted run left in
    // the store (the iOS keychain outlives the app) is removed first.
    final store = phone
        ? FlutterSecureBackend(appId: testBranding.bundleId)
        : MemorySecureBackend();
    await _RunSecrets(store, _runPrefix).clear();
    final run = Random.secure().nextInt(1 << 32).toRadixString(16);
    final secrets = _RunSecrets(store, '$_runPrefix$run/');
    final ports = AppPorts(
      branding: testBranding,
      platform: platform,
      deviceName: 'Alex phone',
      secureBackend: secrets,
      deviceKeys: keys,
      discovery: ApiDiscoveryClient(
        platform: platform,
        appVersion: testBranding.appVersion,
        allowPlainHttp: true,
      ),
      dataRoot: () async => root,
      browser: RecordingBrowser(),
      redirects: SilentRedirects.new,
      transport: transport,
      lifecycle: ForegroundLifecycle(),
      network: network,
      ticker: SchedulerFrameTicker(),
      deepLinks: ManualDeepLinks(),
      config: AppConfig.defaults().withOverrides(fastConfig),
      allowPlainHttpForTests: true,
    );
    // The integration binding leaves the platform's text input in place; typing goes through
    // the test text input, which also works in a profile build.
    if (!tester.testTextInput.isRegistered) tester.testTextInput.register();
    final app = MobilePerfApp._(
      tester,
      fake,
      root,
      ports,
      transport,
      network,
      keys,
      secrets,
    );
    await app.pumpApp();
    return app;
  }

  /// The tester.
  final WidgetTester tester;

  /// The fake server of the run.
  final PerfFake fake;

  /// The data folder of the app.
  final Directory root;

  /// The ports of the app.
  final AppPorts ports;

  /// The gateway transport with its record of status lines, dispatched events and the arrival
  /// of a catch-up.
  final PerfTransport transport;

  /// The network status the catch-up switches off and on.
  final SwitchNetwork network;

  final DeviceKeyStore _keys;
  final _RunSecrets _secrets;

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

  /// Ends the running app and waits until its services are closed; [pumpApp] starts it again
  /// on the same data, as a start after the app was ended does.
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

  /// Signs Alex in with password and one-time code through the onboarding controller (the
  /// screens of E1 and E2 are not measured) and waits for the phone shell with a live engine.
  Future<void> signIn() async {
    await waitFor(find.byKey(const ValueKey('e1.address')));
    final control = container.read(onboardingControllerProvider.notifier);
    await control.connect(fake.baseUri.toString());
    if (state.step != OnboardingStep.login) {
      fail(
        'the app did not reach E2 (${state.serverStatus}): '
        '${await _whyNotOpened()}',
      );
    }
    await control.loginWithPassword(
      WorldSeed.alexLogin,
      WorldSeed.password,
      WorldSeed.totp,
    );
    await waitFor(find.byKey(const ValueKey('phone.shell')));
    await until(
      'live',
      () => state.services?.engine.state.phase == ConnectionPhase.live,
      detail: () =>
          '(engine ${state.services?.engine.state}, ${transport.record.describe()})',
    );
  }

  /// Why the instance of [fake] does not open: the controller shows only "no instance" when
  /// opening the services fails, so this opens them once more with the same ports and returns
  /// the error, or what discovery answered.
  Future<String> _whyNotOpened() async {
    final outcome = await ports.discovery.discover(fake.baseUri);
    if (outcome is! DiscoveryFound) return 'discovery answered $outcome';
    try {
      final services = await InstanceServices.open(ports, outcome.instance);
      await services.close();
      return 'the services open on a second try';
    } on Object catch (e, stack) {
      return '$e\n$stack';
    }
  }

  /// Pumps until [finder] finds something; fails after [patience].
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
        fail('timed out waiting for $what ${detail?.call() ?? ''}');
      }
      await tester.pump(const Duration(milliseconds: 50));
    }
  }

  /// Removes the app, waits until its services are closed and removes what the run left on
  /// the phone: the device keys and secrets of the fake's instance and the data folder.
  Future<void> stop() async {
    // After a failure between [shutDown] and [pumpApp] no app is mounted and nothing is open.
    final mounted = find.byType(ClientApp).evaluate().isNotEmpty;
    final services = mounted ? state.services : null;
    await tester.pumpWidget(const SizedBox());
    await services?.close();
    await _keys.delete(fake.instanceId);
    await _secrets.clear();
    final watch = Stopwatch()..start();
    while (root.existsSync()) {
      try {
        await root.delete(recursive: true);
      } on FileSystemException {
        if (watch.elapsed >= patience) rethrow;
        await Future<void>.delayed(const Duration(milliseconds: 50));
      }
    }
    if (tester.testTextInput.isRegistered) tester.testTextInput.unregister();
  }
}

/// The platform's secure storage under a [_prefix] of one run.
final class _RunSecrets implements SecureBackend {
  _RunSecrets(this._inner, this._prefix);

  final SecureBackend _inner;
  final String _prefix;

  @override
  Future<String?> read(String key) => _inner.read('$_prefix$key');

  @override
  Future<void> write(String key, String value) =>
      _inner.write('$_prefix$key', value);

  @override
  Future<void> delete(String key) => _inner.delete('$_prefix$key');

  @override
  Future<Set<String>> keys() async => {
    for (final k in await _inner.keys())
      if (k.startsWith(_prefix)) k.substring(_prefix.length),
  };

  /// Deletes every value of the run.
  Future<void> clear() async {
    for (final k in await _inner.keys()) {
      if (k.startsWith(_prefix)) await _inner.delete(k);
    }
  }
}

/// The gateway transport of the level: the [RecordingTransport] of the E2E apps (status line
/// arrivals, dispatched events) plus the moments the frames that open and close a catch-up
/// arrived, on the clock of the timeline, so the summary tells the delivery of a catch-up from
/// its application.
final class PerfTransport implements GatewayTransport {
  /// The record of the E2E transport.
  final RecordingTransport record = RecordingTransport();

  /// When the last `resumed` or `ready` frame arrived, or null.
  int? sessionAt;

  /// When the last `catchup.complete` frame arrived, or null.
  int? completeAt;

  @override
  Future<GatewayChannel> connect(Uri url) async =>
      _TimedChannel(await record.connect(url), this);
}

final class _TimedChannel implements GatewayChannel {
  _TimedChannel(this._inner, PerfTransport times)
    : frames = _inner.frames.map((text) {
        if (text.contains('"op":"resumed"') || text.contains('"op":"ready"')) {
          times.sessionAt = Timeline.now;
        } else if (text.contains('"op":"catchup.complete"')) {
          times.completeAt = Timeline.now;
        }
        return text;
      });

  final GatewayChannel _inner;

  @override
  final Stream<String> frames;

  @override
  void send(String text) => _inner.send(text);

  @override
  Future<void> close(int code, [String? reason]) => _inner.close(code, reason);

  @override
  int? get closeCode => _inner.closeCode;

  @override
  String? get closeReason => _inner.closeReason;
}
