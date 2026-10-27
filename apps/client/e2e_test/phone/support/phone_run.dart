// SPDX-License-Identifier: Apache-2.0

// The shared set-up of the mobile E2E suites (brief M5-11 Design note 2; docs/spec/
// 15-client-architecture.md 20 row "E2E on devices"): the world of the Compose stack from the
// defines E2E_CONFIG and E2E_CA, the network of the app process, Alex's other device and Sam as
// API actors, and the app with the ports of a real phone build (Flutter's lifecycle, the network
// status of a release build, push and sharing of the platform where a suite needs them) on the
// iOS simulator or the Android emulator it runs on. tool/mobile_e2e_suite.sh runs one suite file
// per `patrol test`; every suite is its own app process against the same stack.

import 'dart:convert';
import 'dart:io';
import 'dart:math';

import 'package:auth/auth.dart' show DeviceKeys, LoginFailed;
import 'package:client/platform/app_lifecycle.dart';
import 'package:client/platform/device_key_signer.dart';
import 'package:client/platform/network_status.dart';
import 'package:client/platform/notification_channels.dart';
import 'package:client/platform/push_android.dart'
    show AndroidPush, PushAndroidHostApi;
import 'package:client/platform/push_apple.dart'
    show ApplePush, PushAppleHostApi;
import 'package:client/platform/push_platform.dart' show NativePush;
import 'package:client/share/share_android.dart';
import 'package:client/share/share_apple.dart';
import 'package:client/telemetry/client_metrics.dart';
import 'package:crypto/crypto.dart';
import 'package:device_key/device_key.dart' as dk;
import 'package:domain/domain.dart' show Uuid7Generator;
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:patrol/patrol.dart';

import '../../../integration_test/share/support/share_flow.dart'
    show nativeWait;
import '../../../test/support/widgets.dart' show en;
import '../../support/actors.dart';
import '../../support/e2e_app.dart';
import '../../support/e2e_keys.dart';
import '../../support/e2e_network.dart';
import '../../support/e2e_world.dart';
import '../../support/totp.dart';

/// The native automation of the mobile suites: every native step waits up to [nativeWait] (the
/// system apps answer slowly on a loaded machine), so the connection to patrol's automation
/// server must outlast it.
final PlatformAutomatorConfig phoneAutomatorConfig =
    PlatformAutomatorConfig.fromOptions(
      findTimeout: nativeWait,
      connectionTimeout: nativeWait + const Duration(minutes: 1),
    );

/// The platform of the phone build under test: `android` on the emulator, `ios` on the simulator.
String get phonePlatform => Platform.isAndroid ? 'android' : 'ios';

/// TOTP codes of Alex for the password logins of one suite process (08 2.5).
///
/// The server accepts a code only for a step above the last step it accepted. Every suite is
/// its own process against the same stack, so a process cannot know the steps an earlier one
/// used; those were at most the step current when this process started (the suites run one
/// after the other). So the first code of a process is for a later step than that, and every
/// further code for a later step than the one before: [next] waits for the next step when
/// needed, at most one period of 30 s.
final class PhoneTotp {
  /// Creates the codes of [world]; create it when the suite starts.
  PhoneTotp(this._world)
    : _floor = max(totpStep(DateTime.now()), _world.alex.lastStep);

  final E2eWorld _world;
  int _floor;

  /// A code of a step above every step used so far; fails after [patience].
  Future<String> next() async {
    final watch = Stopwatch()..start();
    var step = totpStep(DateTime.now());
    while (step <= _floor) {
      if (watch.elapsed >= patience) fail('no TOTP step after $_floor');
      await Future<void>.delayed(const Duration(milliseconds: 200));
      step = totpStep(DateTime.now());
    }
    _floor = step;
    return totpCode(_world.alex.totpSecret!, step);
  }
}

/// A device ID in the UUIDv7 form that is the same for [seed] in every process: the
/// timestamp of the instance ID [instanceId] and random bits from the SHA-256 of [seed].
String stableDeviceId(String instanceId, String seed) {
  final bytes = sha256.convert(utf8.encode(seed)).bytes;
  var at = 0;
  final millis = int.parse(
    instanceId.replaceAll('-', '').substring(0, 12),
    radix: 16,
  );
  return Uuid7Generator(
    nowMillis: () => millis,
    random: (bits) {
      var value = 0;
      for (var i = 0; i < 4; i++) {
        value = (value << 8) | bytes[at++ % bytes.length];
      }
      return value & ((1 << bits) - 1);
    },
  ).next();
}

/// Alex's other device of the mobile suites, one per stack, which every suite process signs in
/// again ([ApiActor.signIn] with the same keys and device ID).
///
/// The first process of a stack redeems the enrollment code of the world. Every suite issues
/// new codes for Alex, which end that code (08 3.2), so in a later process the server refuses
/// it and the device signs in with Alex's password and a TOTP code of [totp] as the known
/// active device it is (08 5.2). Only a refusal the server answered leads to the password; a
/// failure without an answer (no network) ends the set-up, so the device never logs in with
/// the password before its code was redeemed, which would leave it waiting for an approval.
Future<ApiActor> stableOwner(
  E2eWorld world,
  E2eNetwork network,
  PhoneTotp totp,
) {
  final seed = 'mobile-e2e/alex-desk/${world.instanceId}';
  return ApiActor.signIn(
    world,
    network,
    name: 'Alex desk',
    keys: keysFor(seed),
    deviceId: stableDeviceId(world.instanceId, seed),
    login: (auth) async {
      final byCode = await auth.login.redeemEnrollmentCode(world.alex.code);
      if (byCode is! LoginFailed || byCode.problem == null) return byCode;
      return auth.login.loginWithPassword(
        loginName: world.alex.handle,
        password: world.alex.password!,
        totpCode: await totp.next(),
      );
    },
  );
}

/// The device keys of an app whose native parts sign with the `dpop` key: the Notification
/// Service Extension and the Share Extension on iOS, the preview worker of the push service on
/// Android. The `dpop` key lives in the platform's keystore (the shared keychain group on iOS),
/// where those parts find it by the instance ID; the `step_up` key is a software key as in the
/// other E2E apps, because the platform's step-up key needs a lock screen and a confirmation
/// that the emulator and the suites do not have, and no suite signs a step-up in the app.
final class NativeDpopKeyStore implements DeviceKeyStore {
  /// Creates the store over the platform's device keys.
  NativeDpopKeyStore([dk.DeviceKeyClient? client])
    : _client = client ?? dk.DeviceKeyClient();

  final dk.DeviceKeyClient _client;
  final String _nonce = randomNonce();

  @override
  Future<DeviceKeys> ensure(String scope) async {
    final dpop = await _client.ensure(scope, dk.KeyPurpose.dpop);
    return DeviceKeys(
      dpop: DeviceKeySignerAdapter(_client, scope, dk.KeyPurpose.dpop, dpop),
      stepUp: keysFor('$_nonce/$scope').stepUp,
    );
  }

  @override
  Future<void> delete(String scope) => _client.delete(scope);
}

/// One suite's run: the world, the network of the app process, Alex's other device ([owner]),
/// Sam ([sam]) and the app; [stop] removes what the run created. A suite runs inside [run].
final class PhoneRun {
  PhoneRun._(this.$, this.world, this.network, this.totp, this.owner, this.sam);

  /// Runs [body] with a new run ([start]) and always ends it ([stop]) inside the test body,
  /// while the app's widgets still exist (flutter_test replaces the tree before the tear-downs of
  /// `addTearDown` run). A failure of the end fails the test only when [body] passed, so it never
  /// hides the failure of a step.
  static Future<void> run(
    PatrolIntegrationTester $,
    Future<void> Function(PhoneRun run) body,
  ) async {
    final run = await start($);
    var passed = false;
    try {
      await body(run);
      passed = true;
    } finally {
      final failure = await run.stop();
      if (passed && failure != null) {
        Error.throwWithStackTrace(failure.error, failure.trace);
      }
    }
  }

  /// Sets the run up: the world of the defines, the network with the tunnel for the server's
  /// connections only (patrol's automation server stays reachable), Alex's other device and a
  /// new device of Sam with a code Alex issues. Throws what the set-up throws, after removing
  /// what it created.
  static Future<PhoneRun> start(PatrolIntegrationTester $) async {
    final world = E2eWorld.fromEnvironment();
    final totp = PhoneTotp(world);
    final network = await E2eNetwork.start(world.server, world.caPem);
    network.installForServer();
    ApiActor? owner;
    try {
      owner = await stableOwner(world, network, totp);
      final sam = await ApiActor.enroll(
        world,
        network,
        await owner.issueCode(world.sam.id),
        name: 'Sam phone',
      );
      return PhoneRun._($, world, network, totp, owner, sam);
    } on Object {
      await owner?.dispose();
      await network.stop();
      HttpOverrides.global = null;
      rethrow;
    }
  }

  /// The patrol tester.
  final PatrolIntegrationTester $;

  /// The world of the stack.
  final E2eWorld world;

  /// The network of the app process.
  final E2eNetwork network;

  /// The TOTP codes of this process.
  final PhoneTotp totp;

  /// Alex's other device: issues codes, approves and revokes devices, sets levels.
  final ApiActor owner;

  /// Sam's device of this run.
  final ApiActor sam;

  E2eApp? _app;
  FlutterAppLifecycle? _lifecycle;
  LifecycleNetworkStatus? _status;
  NativePush? _push;
  NativeDpopKeyStore? _nativeKeys;
  final Set<String> _appDevices = {};

  /// The app of [startApp].
  E2eApp get app => _app!;

  /// The push of the app, when [startApp] was asked for it.
  NativePush? get push => _push;

  /// Starts the app with the ports of a phone build on this device: Flutter's lifecycle (the
  /// background closes the gateway, 15 6.3) and the network status of a release build; with
  /// [push] the platform's push (and on Android the notification channels the release entry
  /// registers); with [share] the platform's sharing; with [nativeKeys] the `dpop` key in the
  /// platform's keystore ([NativeDpopKeyStore]), else software keys.
  Future<E2eApp> startApp({
    bool push = false,
    bool share = false,
    bool nativeKeys = false,
  }) async {
    final lifecycle = _lifecycle = FlutterAppLifecycle();
    final status = _status = LifecycleNetworkStatus(lifecycle);
    NativePush? pushPort;
    if (push) {
      final metrics = ClientMetrics();
      if (Platform.isAndroid) {
        await registerNotificationChannels(AndroidPlatformApi(), en);
        pushPort = AndroidPush(host: PushAndroidHostApi(), metrics: metrics);
      } else {
        pushPort = ApplePush(host: PushAppleHostApi(), metrics: metrics);
      }
    }
    _push = pushPort;
    final keys = nativeKeys ? _nativeKeys = NativeDpopKeyStore() : null;
    return _app = await E2eApp.start(
      $.tester,
      world,
      platform: phonePlatform,
      deviceView: true,
      deviceName: 'Alex phone',
      lifecycle: lifecycle,
      network: status,
      push: pushPort,
      deviceKeys: keys,
      appleShare: share && Platform.isIOS
          ? AppleShare(host: ShareAppleHostApi())
          : null,
      androidShare: share && Platform.isAndroid
          ? AndroidShare(host: ShareAndroidHostApi())
          : null,
    );
  }

  /// The ID of the push registration the runner keeps for the instance after `registerPush`
  /// succeeded (15 10.1, 10.2), or null.
  Future<String?> pushRegistrationId() => Platform.isAndroid
      ? PushAndroidHostApi().registrationId(world.instanceId)
      : PushAppleHostApi().registrationId(world.instanceId);

  /// Remembers the app's device [deviceId], which [stop] revokes.
  void trackDevice(String deviceId) => _appDevices.add(deviceId);

  /// E1 and E3: the app connects to the stack and redeems a code Alex's other device issued;
  /// waits for the phone shell with a live engine.
  Future<void> signInWithCode() async {
    final app = this.app;
    await app.connect();
    await app.enroll(await owner.issueCode(world.alex.id));
    await app.waitFor(find.byKey(const ValueKey('phone.shell')));
    trackDevice(app.deviceId);
    await app.waitLive();
  }

  /// E2: types Alex's login name, password and a TOTP code of [totp] and submits.
  Future<void> loginWithPassword() async {
    final app = this.app;
    final tester = app.tester;
    final code = await totp.next();
    await app.waitFor(find.byKey(const ValueKey('e2.loginName')));
    await tester.enterText(
      find.byKey(const ValueKey('e2.loginName')),
      world.alex.handle,
    );
    await app.tap('e2.password.open');
    await tester.enterText(
      find.byKey(const ValueKey('e2.password')),
      world.alex.password!,
    );
    await tester.enterText(find.byKey(const ValueKey('e2.code')), code);
    await app.tap('e2.submit');
  }

  /// Whether Flutter reports the app in the foreground and active.
  bool get resumed =>
      WidgetsBinding.instance.lifecycleState == AppLifecycleState.resumed;

  /// Removes what the run created, also after a failed step: it opens the network again, brings
  /// the app to the foreground (only there it draws the frames its end needs), ends the app and
  /// deletes its data folder, its push registration and its `dpop` key on the device, revokes
  /// its devices and Sam's on the server, then ends the actors and the network. Every step runs;
  /// returns the first failure, or null.
  Future<({Object error, StackTrace trace})?> stop() async {
    ({Object error, StackTrace trace})? first;
    Future<void> step(Future<void> Function() body) async {
      try {
        await body();
      } on Object catch (e, trace) {
        first ??= (error: e, trace: trace);
      }
    }

    // A suite that failed while the network was cut leaves the app nothing to end through.
    network.restore();
    final app = _app;
    if (app != null) {
      await step(() async {
        if (!resumed) {
          await $.platform.mobile.openApp();
          await untilInBackgroundAsync(
            'the app in the foreground',
            () async => resumed,
          );
        }
        await app.stop();
      });
    }
    final push = _push;
    if (push != null) await step(() => push.forget(world.instanceId));
    final keys = _nativeKeys;
    if (keys != null) await step(() => keys.delete(world.instanceId));
    for (final id in {..._appDevices, sam.deviceId}) {
      await step(() => owner.revokeDevice(id));
    }
    await step(sam.dispose);
    await step(owner.dispose);
    await step(() async => _status?.dispose());
    await step(() async => _lifecycle?.dispose());
    await step(network.stop);
    HttpOverrides.global = null;
    return first;
  }
}

/// Polls [condition] without pumping until it completes with true: while the app is in the
/// background the binding draws no frames, so a pump would wait for a frame that never comes.
/// Fails naming [what] and the [detail] of that moment after [patience].
Future<void> untilInBackgroundAsync(
  String what,
  Future<bool> Function() condition, {
  String Function()? detail,
}) async {
  final watch = Stopwatch()..start();
  while (!await condition()) {
    if (watch.elapsed >= patience) {
      fail('timed out waiting for $what ${detail?.call() ?? ''}');
    }
    await Future<void>.delayed(const Duration(milliseconds: 200));
  }
}
