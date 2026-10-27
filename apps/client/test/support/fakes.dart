// SPDX-License-Identifier: Apache-2.0

// Ports of the app for the tests: an in-memory secure store, a scripted discovery, deep links
// by hand, a fixed lifecycle and network, a controllable clock and a ticker without frames.

import 'dart:async';
import 'dart:io';

import 'package:auth/auth.dart';
import 'package:client/app/bootstrap.dart';
import 'package:client/app/branding.dart';
import 'package:client/app/deep_links.dart';
import 'package:client/app/discovery.dart';
import 'package:client/desktop/start_at_login.dart';
import 'package:client/desktop/tray.dart';
import 'package:client/desktop/window_behavior.dart';
import 'package:client/files/file_ports.dart';
import 'package:client/lock/app_lock_port.dart';
import 'package:client/notifications/notification_presenter.dart';
import 'package:client/platform/device_key_signer.dart';
import 'package:client/platform/push_platform.dart';
import 'package:client/platform/secure_store.dart';
import 'package:client/platform/terminal_guard.dart';
import 'package:client/share/share_android.dart';
import 'package:client/share/share_apple.dart';
import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

import 'software_keys.dart';

/// The neutral branding of the tests.
const BuildBranding testBranding = BuildBranding(
  appName: 'ExampleChat',
  urlScheme: 'chat-example',
  bundleId: 'com.example.chat',
  defaultServer: 'https://chat.example.com',
  appVersion: '1.0.0',
);

/// [SecureBackend] in memory.
class MemorySecureBackend implements SecureBackend {
  /// The stored values.
  final Map<String, String> values = {};

  @override
  Future<String?> read(String key) async => values[key];

  @override
  Future<void> write(String key, String value) async => values[key] = value;

  @override
  Future<void> delete(String key) async => values.remove(key);

  @override
  Future<Set<String>> keys() async => values.keys.toSet();
}

/// [DiscoveryClient] that answers what the test sets.
class ScriptedDiscovery implements DiscoveryClient {
  /// Creates the discovery with its first answer.
  ScriptedDiscovery([this.answer = const DiscoveryNoInstance()]);

  /// The next answer.
  DiscoveryOutcome answer;

  /// The addresses asked for.
  final List<Uri> asked = [];

  @override
  Future<DiscoveryOutcome> discover(Uri address) async {
    asked.add(address);
    return answer;
  }
}

/// [DeepLinkSource] the test feeds by hand.
class ManualDeepLinks implements DeepLinkSource {
  /// Links that were there before the app listened.
  final List<Uri> pending = [];
  final StreamController<Uri> _links = StreamController<Uri>.broadcast();

  /// Delivers [link] as if the runner forwarded it.
  void send(Uri link) => _links.add(link);

  @override
  Stream<Uri> get links => _links.stream;

  @override
  Future<List<Uri>> takePending() async {
    final out = List.of(pending);
    pending.clear();
    return out;
  }
}

/// Always in the foreground.
class ForegroundLifecycle implements AppLifecycle {
  @override
  Stream<AppActivity> get changes => const Stream.empty();

  @override
  AppActivity get current => AppActivity.foreground;
}

/// A lifecycle the test switches between foreground and background.
class SwitchLifecycle implements AppLifecycle {
  final StreamController<AppActivity> _changes =
      StreamController<AppActivity>.broadcast(sync: true);
  AppActivity _current = AppActivity.foreground;

  /// Moves the app to [activity].
  void set(AppActivity activity) {
    _current = activity;
    _changes.add(activity);
  }

  @override
  Stream<AppActivity> get changes => _changes.stream;

  @override
  AppActivity get current => _current;
}

/// A network status the test switches.
class SwitchNetwork implements NetworkStatus {
  final StreamController<bool> _changes = StreamController<bool>.broadcast();
  bool _online = true;

  /// Switches the network.
  void set(bool online) {
    _online = online;
    _changes.add(online);
  }

  @override
  Stream<bool> get changes => _changes.stream;

  @override
  bool get online => _online;
}

/// A clock the test moves; delays complete at once.
class TestClock implements Clock {
  /// Creates the clock at [start].
  TestClock(this.start);

  /// The start time.
  DateTime start;

  @override
  DateTime now() => start;

  @override
  Future<void> delay(Duration duration) => Future<void>.delayed(Duration.zero);
}

/// Runs the frame callback in the next microtask.
class MicrotaskTicker implements FrameTicker {
  @override
  void requestFrame(void Function() onFrame) => scheduleMicrotask(onFrame);
}

/// A browser that records the URLs it was asked to open.
class RecordingBrowser implements BrowserLauncher {
  /// The opened URLs.
  final List<Uri> opened = [];

  @override
  Future<void> open(Uri url) async => opened.add(url);
}

/// A redirect receiver that never receives anything.
class SilentRedirects implements RedirectReceiver {
  @override
  Future<Uri> redirectUri() async => Uri.parse('http://127.0.0.1:1/callback');

  @override
  Future<Uri> waitForRedirect(Uri expectedPrefix, Duration timeout) =>
      Completer<Uri>().future.timeout(timeout);
}

/// Short client values, so polling and reconnects take milliseconds.
const Map<String, String> fastConfig = {
  'sync.reconnect_backoff_base': '10ms',
  'sync.reconnect_backoff_cap': '100ms',
  'client.outbox_retry_base': '20ms',
  'client.outbox_retry_cap': '200ms',
};

/// The ports of a test: memory secrets, software keys, a temporary data folder, plain HTTP
/// allowed. [dataRoot] is a folder the caller deletes.
AppPorts testPorts({
  required Directory dataRoot,
  DiscoveryClient? discovery,
  SecureBackend? secureBackend,
  DeviceKeyStore? deviceKeys,
  DeepLinkSource? deepLinks,
  NetworkStatus? network,
  Clock? clock,
  PasskeyAuthenticator? passkeys,
  String? passkeyDomain,
  FileProtector? protectFiles,
  AppLifecycle? lifecycle,
  BrowserLauncher? browser,
  DatabaseOpener? openDatabase,
  StepUpPrompt? stepUpPrompt,
  BuildBranding branding = testBranding,
  Map<String, String> config = fastConfig,
  FilePorts? files,
  String platform = 'macos',
  NotificationPresenter notifications = const NoNotificationPresenter(),
  DesktopWindow window = const NoDesktopWindow(),
  TrayPort tray = const NoTray(),
  Future<StartAtLogin> Function()? startAtLogin,
  PushPlatform? push,
  AppLockPort appLock = const NoAppLockPort(),
  AppleShare? appleShare,
  AndroidShare? androidShare,
  TerminalPlatformGuard? terminalGuard,
}) => AppPorts(
  terminalGuard: terminalGuard,
  push: push,
  appLock: appLock,
  appleShare: appleShare,
  androidShare: androidShare,
  branding: branding,
  stepUpPrompt: stepUpPrompt,
  platform: platform,
  notifications: notifications,
  window: window,
  tray: tray,
  startAtLogin: startAtLogin ?? () async => const NoStartAtLogin(),
  deviceName: 'Alex laptop',
  secureBackend: secureBackend ?? MemorySecureBackend(),
  deviceKeys: deviceKeys ?? SoftwareKeyStore(),
  discovery: discovery ?? ScriptedDiscovery(),
  dataRoot: () async => dataRoot,
  browser: browser ?? RecordingBrowser(),
  redirects: SilentRedirects.new,
  transport: IoGatewayTransport(),
  lifecycle: lifecycle ?? ForegroundLifecycle(),
  network: network ?? SwitchNetwork(),
  ticker: MicrotaskTicker(),
  deepLinks: deepLinks ?? ManualDeepLinks(),
  config: AppConfig.defaults().withOverrides(config),
  passkeys: passkeys,
  passkeyDomain: passkeyDomain,
  protectFiles: protectFiles ?? (_) async {},
  clock: clock ?? const SystemClock(),
  openDatabase:
      openDatabase ??
      (file, keys) =>
          openLocalStore(file: file, keys: keys, inBackground: false),
  allowPlainHttpForTests: true,
  files: files,
);
