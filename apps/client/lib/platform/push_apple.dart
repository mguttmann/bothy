// SPDX-License-Identifier: Apache-2.0

/// Push on iOS and macOS: permission, APNs token, `registerPush`, the shared keychain of the
/// Notification Service Extension, taps and the extension's display counters (brief M5-04
/// Design note 5; docs/spec/13-push-notifications.md 5.1, 6.4 and 13; docs/spec/
/// 15-client-architecture.md 10.1 and 11).
library;

import 'dart:async';

import '../src/generated/push_apple_api.g.dart';
import '../telemetry/client_metrics.dart';
import 'push_platform.dart';

export '../src/generated/push_apple_api.g.dart'
    show PushAppleHostApi, PushDisplayCounters, PushRegistrationRecord, PushTap;
export 'push_platform.dart';

/// Push on iOS and macOS for the app (brief M5-04 Design note 5).
///
/// - Permission: [threadOpened] asks once per run, after the login, when the person opens a
///   thread, the first moment they can understand why; never before the login. Without the
///   permission nothing registers.
/// - Registration: with the permission the APNs token is registered with `registerPush`
///   (provider `apns`, the environment and bundle identifier of the build); the runner keeps the
///   preview token and the registration ID in the shared keychain at once, Dart never persists
///   them. A token that differs from the one last registered registers again (the server takes
///   the old one over).
/// - [logout] calls `unregisterPush` best effort; [forget] deletes the keychain items on every
///   end of the account (logout, `4012`, a revoked device).
/// - [previewModeChanged] writes `notify.preview_mode` into the App Group for the extension.
/// - [signedIn] takes the extension's display counters into the client metrics (attributes only
///   platform and app version, M4-31).
/// - [targets] gives the app path of every tapped notification, `/t/<thread>?m=<message>`.
///
/// Every platform failure is swallowed: push is an addition, the app works without it, and the
/// next start tries again.
///
/// Example:
/// ```dart
/// final push = ApplePush(host: PushAppleHostApi(), metrics: metrics)..listen();
/// await push.signedIn(account, previewMode: 'full');
/// ```
final class ApplePush implements PushPlatform, PushAppleFlutterApi {
  /// Creates the push over [host]; [metrics] receives the display counters.
  ApplePush({required PushAppleHostApi host, required ClientMetrics metrics})
    : _host = host, // ignore: prefer_initializing_formals
      _metrics = metrics; // ignore: prefer_initializing_formals

  /// The secret key of the APNs token last registered for an instance.
  static const String tokenKey = 'push/apns_token';

  final PushAppleHostApi _host;
  final ClientMetrics _metrics;
  final StreamController<String> _targets = StreamController<String>();
  PushAccount? _account;
  String _previewMode = 'full';
  bool _asked = false;
  Future<void>? _registering;

  /// The app path of every tapped notification; one listener (the app), which also gets the
  /// taps that arrived before it listened.
  @override
  Stream<String> get targets => _targets.stream;

  /// Connects the runner's tap calls and takes the taps that started the app.
  @override
  Future<void> listen() async {
    PushAppleFlutterApi.setUp(this);
    try {
      for (final tap in await _host.takePendingTaps()) {
        onTap(tap);
      }
    } on Object {
      // A host without the runner's channel (widget tests): no taps.
    }
  }

  /// The app path of [tap] (15 11): `/t/<thread>?m=<message>`, or `/t/<thread>` without a
  /// message.
  static String pathOf(PushTap tap) =>
      pushTargetPath(tap.threadId, tap.messageId);

  @override
  void onTap(PushTap tap) {
    if (tap.threadId.isEmpty) return;
    _targets.add(pathOf(tap));
  }

  /// The account signed in (at start or after a login) with [previewMode] of its synced
  /// settings: writes the App Group values, submits the display counters and, when the person
  /// allowed notifications already, registers.
  @override
  Future<void> signedIn(
    PushAccount account, {
    required String previewMode,
  }) async {
    _account = account;
    _previewMode = previewMode;
    if (!await _available()) return;
    await _guard(_writeSettings);
    await _guard(_takeCounters);
    if (await _guardValue(_host.isAuthorized) ?? false) {
      await _register();
    }
  }

  /// The person opened a thread: asks for the permission once per run and registers when it
  /// is given.
  @override
  Future<void> threadOpened() async {
    if (_account == null || _asked) return;
    _asked = true;
    if (!await _available()) return;
    if (await _guardValue(_host.isAuthorized) ?? false) return;
    if (await _guardValue(_host.requestAuthorization) ?? false) {
      await _register();
    }
  }

  /// `notify.preview_mode` changed in the synced settings.
  @override
  Future<void> previewModeChanged(String previewMode) async {
    if (previewMode == _previewMode) return;
    _previewMode = previewMode;
    if (_account == null || !await _available()) return;
    await _guard(_writeSettings);
  }

  /// The person logs out: `unregisterPush` best effort while the session still exists, then
  /// [forget]. The server disables the registrations of the session on logout anyway (13 6.4).
  @override
  Future<void> logout() async {
    final account = _account;
    if (account == null) return;
    if (!await _available()) return;
    final id = await _guardValue(
      () => _host.registrationId(account.instanceId),
    );
    if (id != null) {
      await _guard(() => account.registrations.unregister(id));
    }
    await forget(account.instanceId);
  }

  /// Every end of the account of [instanceId]: deletes the keychain items, the App Group
  /// values and the remembered APNs token.
  @override
  Future<void> forget(String instanceId) async {
    final account = _account;
    if (account?.instanceId == instanceId) {
      _account = null;
      _asked = false;
      await _guard(() => account!.secrets.delete(tokenKey));
    }
    if (!await _available()) return;
    await _guard(() => _host.deleteRegistration(instanceId));
  }

  @override
  Future<bool> isAvailable() => _available();

  Future<bool> _available() async =>
      await _guardValue(_host.isAvailable) ?? false;

  Future<void> _writeSettings() async {
    final account = _account;
    if (account == null) return;
    await _host.writeSettings(
      account.instanceId,
      _previewMode,
      account.fetchTimeout.inMilliseconds,
    );
  }

  Future<void> _takeCounters() async {
    final counters = await _host.takeDisplayCounters();
    recordPushCounters(
      _metrics,
      bounds: counters.bucketBounds,
      counts: counters.bucketCounts,
      placeholders: counters.placeholders,
    );
  }

  Future<void> _register() =>
      _registering ??= _registerOnce().whenComplete(() => _registering = null);

  Future<void> _registerOnce() async {
    final account = _account;
    if (account == null) return;
    await _guard(() async {
      final token = await _host.deviceToken();
      final known = await account.secrets.read(tokenKey);
      final stored = await _host.registrationId(account.instanceId);
      if (known == token && stored != null) return;
      final created = await account.registrations.register(
        provider: 'apns',
        token: token,
        environment: await _host.environment(),
        appId: await _host.appId(),
      );
      // The account may have ended while the call ran: then nothing is stored.
      if (!identical(_account, account)) return;
      await _host.saveRegistration(
        PushRegistrationRecord(
          instanceId: account.instanceId,
          serverUrl: account.serverUrl.toString(),
          registrationId: created.id,
          previewToken: created.previewToken,
        ),
      );
      await account.secrets.write(tokenKey, token);
    });
  }

  static Future<void> _guard(FutureOr<void> Function() body) async {
    try {
      await body();
    } on Object {
      // Push is an addition; the next start tries again (see the class comment).
    }
  }

  static Future<T?> _guardValue<T>(Future<T> Function() body) async {
    try {
      return await body();
    } on Object {
      return null;
    }
  }
}
