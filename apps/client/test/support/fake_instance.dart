// SPDX-License-Identifier: Apache-2.0

// The app's onboarding controller against the fake server of M4-17 on 127.0.0.1, headless:
// the flows of E1 to E7 with the real auth session, local database and sync engine.

import 'dart:async';
import 'dart:io';

import 'package:auth/auth.dart' show StepUpPrompt;
import 'package:client/app/bootstrap.dart';
import 'package:client/app/discovery.dart';
import 'package:client/app/providers/ports.dart';
import 'package:client/desktop/window_behavior.dart';
import 'package:client/files/file_ports.dart';
import 'package:client/notifications/notification_presenter.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:client/platform/push_apple.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:sync_engine/sync_engine.dart' show AppLifecycle;
import 'package:sync_engine/testing.dart';

import 'fakes.dart';
import 'software_keys.dart';

/// Waits until [condition] holds, polling; throws a [StateError] naming [what] after
/// [timeout].
Future<void> eventually(
  String what,
  FutureOr<bool> Function() condition, {
  Duration timeout = const Duration(seconds: 15),
}) async {
  final end = DateTime.now().add(timeout);
  while (!await condition()) {
    if (DateTime.now().isAfter(end)) throw StateError('timed out: $what');
    await Future<void>.delayed(const Duration(milliseconds: 10));
  }
}

/// One fake server, its world and the app's ports and controller.
class FakeInstance {
  FakeInstance._(
    this.server,
    this.seed,
    this.root,
    this.ports,
    this.keys,
    this.secure,
  );

  /// Starts a fake with the standard world and a container over test ports; [discovery]
  /// wraps the discovery client of the app (a network the test switches); [platform],
  /// [window] and [notifications] are the desktop ports of M4-30.
  static Future<FakeInstance> start({
    Map<String, String> config = fastConfig,
    DiscoveryClient Function(DiscoveryClient real)? discovery,
    StepUpPrompt? stepUpPrompt,
    FilePorts? files,
    String platform = 'macos',
    DesktopWindow window = const NoDesktopWindow(),
    NotificationPresenter notifications = const NoNotificationPresenter(),
    AppLifecycle? lifecycle,
    FileProtector? protectFiles,
    ApplePush? applePush,
  }) async {
    final server = await FakeServer.start(
      options: const FakeServerOptions(
        timings: GatewayTimings(runPatchCoalesce: Duration(milliseconds: 40)),
      ),
    );
    final seed = WorldSeed.standard(server.world);
    return _on(
      server,
      seed,
      config,
      discovery,
      stepUpPrompt,
      files,
      platform: platform,
      window: window,
      notifications: notifications,
      lifecycle: lifecycle,
      protectFiles: protectFiles,
      applePush: applePush,
    );
  }

  /// A second app of Alex on the same fake: its own data folder, keys and secrets (a second
  /// simulated device, brief M4-25 Tests). The caller stops it with [stopSibling].
  Future<FakeInstance> sibling({
    Map<String, String> config = fastConfig,
    StepUpPrompt? stepUpPrompt,
  }) => _on(server, seed, config, null, stepUpPrompt, null);

  static Future<FakeInstance> _on(
    FakeServer server,
    WorldSeed seed,
    Map<String, String> config,
    DiscoveryClient Function(DiscoveryClient real)? discovery,
    StepUpPrompt? stepUpPrompt,
    FilePorts? files, {
    String platform = 'macos',
    DesktopWindow window = const NoDesktopWindow(),
    NotificationPresenter notifications = const NoNotificationPresenter(),
    AppLifecycle? lifecycle,
    FileProtector? protectFiles,
    ApplePush? applePush,
  }) async {
    final root = await Directory.systemTemp.createTemp('client-flow');
    final keys = SoftwareKeyStore();
    final secure = MemorySecureBackend();
    final DiscoveryClient real = ApiDiscoveryClient(
      platform: platform,
      appVersion: testBranding.appVersion,
      allowPlainHttp: true,
    );
    final ports = testPorts(
      dataRoot: root,
      config: config,
      secureBackend: secure,
      deviceKeys: keys,
      discovery: discovery == null ? real : discovery(real),
      stepUpPrompt: stepUpPrompt,
      files: files,
      platform: platform,
      window: window,
      notifications: notifications,
      lifecycle: lifecycle,
      protectFiles: protectFiles,
      applePush: applePush,
    );
    return FakeInstance._(server, seed, root, ports, keys, secure);
  }

  /// The fake.
  final FakeServer server;

  /// Alex, Sam, Robin and their channels.
  final WorldSeed seed;

  /// The data folder of the app.
  final Directory root;

  /// The ports of the app.
  final AppPorts ports;

  /// The device keys.
  final SoftwareKeyStore keys;

  /// The secure storage.
  final MemorySecureBackend secure;

  /// The folder of the instance.
  Directory get instanceFolder =>
      Directory('${root.path}/instances/${server.world.instanceId}');

  /// Whether no file, key or secret of the instance is left (R-SY-14).
  Future<bool> wiped() async =>
      !instanceFolder.existsSync() &&
      keys.scopes.isEmpty &&
      !secure.values.keys.any((k) => k.contains(server.world.instanceId));

  /// Deletes the data folder of a [sibling]; the fake keeps running. The folder goes as in
  /// [stop], once the sibling's database is closed.
  Future<void> stopSibling({
    Duration closeTimeout = const Duration(seconds: 10),
  }) => _deleteRoot(closeTimeout);

  /// Stops the fake and deletes the data folder.
  ///
  /// When a test removes the app, the app closes its database without waiting, as it does
  /// when it quits (`OnboardingController` on dispose). Windows refuses to delete an open
  /// file, so the caller first waits for that close (`InstanceServices.close`, as
  /// `TestApp.stop` does) and stops the fake only then. The bounded retry of the delete, at
  /// most [closeTimeout], stays as the last resort for a handle the system itself holds for a
  /// moment (docs/DECISIONS.md D-M4-18-18, D-M4-18-21).
  Future<void> stop({
    Duration closeTimeout = const Duration(seconds: 10),
  }) async {
    await server.stop();
    await _deleteRoot(closeTimeout);
  }

  Future<void> _deleteRoot(Duration closeTimeout) async {
    final end = DateTime.now().add(closeTimeout);
    while (root.existsSync()) {
      try {
        await root.delete(recursive: true);
      } on FileSystemException {
        if (DateTime.now().isAfter(end)) rethrow;
        await Future<void>.delayed(const Duration(milliseconds: 50));
      }
    }
  }
}

/// A provider container of the app over [instance]'s ports.
ProviderContainer appContainer(FakeInstance instance) => ProviderContainer(
  overrides: [appPortsProvider.overrideWithValue(instance.ports)],
);

/// Connects [c] to the fake and waits for E2.
Future<OnboardingController> connected(
  ProviderContainer c,
  FakeInstance f,
) async {
  c.read(onboardingControllerProvider);
  await Future<void>.delayed(Duration.zero);
  final control = c.read(onboardingControllerProvider.notifier);
  await control.connect(f.server.baseUri.toString());
  return control;
}
