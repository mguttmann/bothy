// SPDX-License-Identifier: Apache-2.0

/// The composition root of the desktop app (brief M4-18 Design note 1; docs/spec/
/// 15-client-architektur.md 4; docs/DECISIONS.md D-M4-18-1).
///
/// [AppPorts] is everything the app takes from the outside world; [productionPorts] builds the
/// ports of a real desktop build and the integration test entry builds its own. Per server
/// address [InstanceServices.open] builds the device keys, the secret store, the encrypted
/// local database, the auth session and the sync engine, and [AccountWiper] removes them again.
library;

import 'dart:io' as io;
import 'dart:io' show Directory, Platform;
import 'dart:math';

import 'package:api_client/api_client.dart' hide File, InstanceInfo;
import 'package:auth/auth.dart';
import 'package:device_key/device_key.dart' as dk;
import 'package:domain/domain.dart' hide InstanceInfo;
import 'package:flutter/widgets.dart';
import 'package:l10n/l10n.dart';
import 'package:local_store/local_store.dart';
import 'package:path_provider/path_provider.dart';
import 'package:sync_engine/sync_engine.dart';

import '../platform/app_lifecycle.dart';
import '../platform/browser_launcher.dart';
import '../platform/device_key_signer.dart';
import '../platform/frame_ticker.dart';
import '../platform/loopback_redirect.dart';
import '../platform/network_status.dart';
import '../platform/secure_store.dart';
import 'account_wiper.dart';
import 'branding.dart';
import 'deep_links.dart';
import 'discovery.dart';

/// Opens the local database of an instance; the production value is `openLocalStore`.
typedef DatabaseOpener = Future<LocalDatabase> Function(
  io.File file,
  DatabaseKeySource keys,
);

/// The outside world of the app. Riverpod hands it out through `appPortsProvider`, and tests
/// override it (15 4: "Tests replace providers").
final class AppPorts {
  /// Creates the ports.
  AppPorts({
    required this.branding,
    required this.platform,
    required this.deviceName,
    required this.secureBackend,
    required this.deviceKeys,
    required this.discovery,
    required this.dataRoot,
    required this.browser,
    required this.redirects,
    required this.transport,
    required this.lifecycle,
    required this.network,
    required this.ticker,
    required this.deepLinks,
    AppConfig? config,
    this.passkeys,
    this.clock = const SystemClock(),
    this.openDatabase = _openEncrypted,
    this.allowPlainHttpForTests = false,
    this.stepUpPrompt = const ConfirmingStepUpPrompt(),
  }) : config = config ?? AppConfig.defaults();

  /// The build branding.
  final BuildBranding branding;

  /// `macos` or `windows`, the `DevicePlatform` of this build.
  final String platform;

  /// How this device names itself in the registration (08 5.1).
  final String deviceName;

  /// The secure storage of the platform.
  final SecureBackend secureBackend;

  /// The device keys.
  final DeviceKeyStore deviceKeys;

  /// `getDiscovery` for E1.
  final DiscoveryClient discovery;

  /// The folder under which every instance gets its own folder.
  final Future<Directory> Function() dataRoot;

  /// The system browser of the OIDC login.
  final BrowserLauncher browser;

  /// A fresh loopback receiver per OIDC attempt.
  final RedirectReceiver Function() redirects;

  /// The WebSocket transport of the gateway.
  final GatewayTransport transport;

  /// Foreground and background.
  final AppLifecycle lifecycle;

  /// The network hint of the engine.
  final NetworkStatus network;

  /// The frame pacing of the run documents.
  final FrameTicker ticker;

  /// Deep links from outside.
  final DeepLinkSource deepLinks;

  /// The client values of D-W-115 (and `auth.device_pending_ttl`, D-W-116).
  final AppConfig config;

  /// WebAuthn of the platform, or null where the build has none (docs/DECISIONS.md
  /// D-M4-18-9); E2 and E3 offer passkeys only with it.
  final PasskeyAuthenticator? passkeys;

  /// The clock of the auth session, the remaining times and the announcements.
  final Clock clock;

  /// Opens the local database.
  final DatabaseOpener openDatabase;

  /// Whether `http` servers are accepted: only the integration test entry sets it, so no
  /// release build can talk plain HTTP (D-M4-17-6).
  final bool allowPlainHttpForTests;

  /// Explains a step-up before the system asks for the confirmation.
  final StepUpPrompt stepUpPrompt;

  static Future<LocalDatabase> _openEncrypted(
    io.File file,
    DatabaseKeySource keys,
  ) => openLocalStore(file: file, keys: keys);
}

/// The [StepUpPrompt] of the desktop shell: it continues at once, because the step-up key asks
/// the system for the confirmation itself (08 6.1); the screens that start a step-up (M4-25)
/// explain it before they call.
final class ConfirmingStepUpPrompt implements StepUpPrompt {
  /// Creates the prompt.
  const ConfirmingStepUpPrompt();

  @override
  Future<bool> ask(String purposeText) async => true;
}

/// The ports of a real desktop build on macOS or Windows.
///
/// The secure storage, the device keys of the platform, the application support folder, the
/// system browser, a loopback receiver per login, the `dart:io` WebSocket transport, the
/// lifecycle of Flutter with the derived network status (D-M4-18-8), the scheduler's frames
/// and the deep link channel of the runner.
AppPorts productionPorts({
  BuildBranding branding = BuildBranding.fromEnvironment,
}) {
  final lifecycle = FlutterAppLifecycle();
  AppLocalizations texts() => lookupAppLocalizations(
    resolveAppLocale(
      systemLocales: WidgetsBinding.instance.platformDispatcher.locales,
    ),
  );
  return AppPorts(
    branding: branding,
    platform: Platform.isWindows ? 'windows' : 'macos',
    deviceName: Platform.localHostname,
    secureBackend: FlutterSecureBackend(),
    deviceKeys: PlatformDeviceKeyStore(
      dk.DeviceKeyClient(),
      () => dk.StepUpPrompt(
        title: texts().shellStepUpTitle,
        negativeButton: texts().onboardingCancel,
      ),
    ),
    discovery: ApiDiscoveryClient(
      platform: Platform.isWindows ? 'windows' : 'macos',
      appVersion: branding.appVersion,
    ),
    dataRoot: getApplicationSupportDirectory,
    browser: SystemBrowserLauncher(),
    redirects: () => LoopbackRedirectReceiver(
      doneText: () => texts().onboardingLoopbackDone,
    ),
    transport: IoGatewayTransport(),
    lifecycle: lifecycle,
    network: LifecycleNetworkStatus(lifecycle),
    ticker: SchedulerFrameTicker(),
    deepLinks: MethodChannelDeepLinks(),
  );
}

/// Everything the app holds for one signed-in or signing-in instance.
final class InstanceServices {
  InstanceServices._({
    required this.instance,
    required this.auth,
    required this.database,
    required this.engine,
    required this.wiper,
    required this.secrets,
  });

  /// Builds the services of [instance] (Design note 1): the device keys with the instance ID as
  /// scope, the secrets prefixed with the instance ID, the database key (32 random bytes at the
  /// first start, kept in the secret store), the encrypted database in
  /// `<dataRoot>/instances/<instance_id>/local.db`, the auth session with its login flows and
  /// the sync engine (not started).
  ///
  /// Throws what the device keys or the database throw; the caller shows E1 again.
  static Future<InstanceServices> open(
    AppPorts ports,
    InstanceInfo instance,
  ) async {
    final secrets = InstanceSecrets(ports.secureBackend, instance.instanceId);
    final keys = await ports.deviceKeys.ensure(instance.instanceId);
    final root = await ports.dataRoot();
    final folder = Directory(
      '${root.path}${Platform.pathSeparator}instances'
      '${Platform.pathSeparator}${instance.instanceId}',
    );
    await folder.create(recursive: true);
    final database = await ports.openDatabase(
      io.File('${folder.path}${Platform.pathSeparator}local.db'),
      SecureDatabaseKeySource(secrets),
    );
    final random = Random.secure();
    final ids = Uuid7Generator(
      nowMillis: () => ports.clock.now().millisecondsSinceEpoch,
      random: (bits) => _bits(random, bits),
    );
    final auth = AuthSession(
      serverUrl: instance.address,
      keys: keys,
      secrets: secrets,
      clock: ports.clock,
      ids: ids,
      config: ports.config,
      device: DeviceDescription(
        name: ports.deviceName,
        platform: ports.platform,
        appVersion: ports.branding.appVersion,
      ),
      stepUpPrompt: ports.stepUpPrompt,
      buildClient: (interceptors) => createApiClient(
        baseUrl: instance.address,
        traceContext: newTraceContext,
        readRetry: ReadRetryPolicy.fromBackoff(
          base: ports.config.outboxRetryBase,
          cap: ports.config.outboxRetryCap,
        ),
        authInterceptors: interceptors,
      ),
      browser: ports.browser,
      redirects: ports.redirects(),
      passkeys: ports.passkeys,
    );
    final engine = SyncEngine.create(
      baseUrl: instance.address,
      auth: auth,
      store: database,
      transport: ports.transport,
      network: ports.network,
      lifecycle: ports.lifecycle,
      ticker: ports.ticker,
      config: ports.config,
      client: GatewayClientInfo(
        (b) => b
          ..platform = DevicePlatform.valueOf(ports.platform)
          ..version = ports.branding.appVersion,
      ),
      clock: ports.clock,
      ids: ids,
      keepConnectedInBackground: true,
      allowPlainHttpForTests: ports.allowPlainHttpForTests,
    );
    var stopped = false;
    final wiper = AccountWiper(
      instanceId: instance.instanceId,
      folder: folder,
      stop: () async {
        if (stopped) return;
        stopped = true;
        await engine.dispose();
      },
      wipeDatabase: database.wipe,
      keys: ports.deviceKeys,
      secrets: secrets,
    );
    return InstanceServices._(
      instance: instance,
      auth: auth,
      database: database,
      engine: engine,
      wiper: wiper,
      secrets: secrets,
    );
  }

  /// The instance.
  final InstanceInfo instance;

  /// The login session of the instance.
  final AuthSession auth;

  /// The local database.
  final LocalDatabase database;

  /// The sync engine; started after the login.
  final SyncEngine engine;

  /// Deletes every local datum of the account.
  final AccountWiper wiper;

  /// The secrets of the instance.
  final InstanceSecrets secrets;

  /// Whether a refresh token of an earlier login is stored, so the app can start signed in.
  ///
  /// The key is the one `AuthSession` writes (`auth/<origin>/refresh_token`, M4-09).
  Future<bool> hasStoredSession() async =>
      await secrets.read('auth/${instance.address.origin}/refresh_token') !=
      null;

  /// The ID of this device at the instance, kept by `AuthSession` (`auth/<origin>/device_id`,
  /// M4-09), or null before the first registration.
  Future<String?> storedDeviceId() =>
      secrets.read('auth/${instance.address.origin}/device_id');

  /// Stops the engine and closes the database without deleting anything (the app quits or
  /// switches to another address).
  Future<void> close() async {
    await engine.dispose();
    await database.close();
    await auth.dispose();
  }

  static int _bits(Random random, int bits) {
    var value = 0;
    for (var left = bits; left > 0; left -= 30) {
      final take = left > 30 ? 30 : left;
      value = (value << take) | random.nextInt(1 << take);
    }
    return value;
  }
}
