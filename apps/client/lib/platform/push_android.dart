// SPDX-License-Identifier: Apache-2.0

/// Push on Android: permission, FCM token, `registerPush`, the encrypted store of the Kotlin
/// push service, taps and the service's display counters (brief M5-05 Design note 4;
/// docs/spec/13-push-notifications.md 5.2, 6.4 and 13; docs/spec/15-client-architecture.md
/// 10.2 and 11).
library;

import 'dart:async';

import '../src/generated/push_android_api.g.dart';
import '../telemetry/client_metrics.dart';
import 'push_platform.dart';

export '../src/generated/push_android_api.g.dart'
    show
        AndroidPushCounters,
        AndroidPushRegistration,
        AndroidPushTap,
        PushAndroidHostApi;
export 'push_platform.dart';

/// Push on Android for the app (brief M5-05 Design note 4; docs/decisions.d/M5-05.md).
///
/// - Permission: [threadOpened] asks once per run, after the login, when the person opens a
///   thread, as on iOS (D-M5-04-5); from API 33 that is the runtime permission
///   `POST_NOTIFICATIONS`, below it the notifications only must not be turned off. Without it
///   nothing registers.
/// - Registration: with the permission the FCM token is registered with `registerPush`
///   (provider `fcm`, the package name); the runner keeps the preview token and the
///   registration ID in its Keystore-encrypted store at once, Dart never persists them. A token
///   that differs from the one last registered, also one FCM rotates while the app runs
///   ([onNewToken]), registers again.
/// - [logout] calls `unregisterPush` best effort; [forget] deletes the stored registration and
///   the shown push notifications on every end of the account.
/// - [previewModeChanged] hands `notify.preview_mode` to the push service.
/// - [signedIn] takes the service's display counters into the client metrics.
/// - [targets] gives the app path of every tapped notification, `/t/<thread>?m=<message>`.
///
/// A build without a Firebase configuration has no push: every call does nothing. Every
/// platform failure is swallowed: push is an addition, and the next start tries again.
///
/// Example:
/// ```dart
/// final push = AndroidPush(host: PushAndroidHostApi(), metrics: metrics)..listen();
/// await push.signedIn(account, previewMode: 'full');
/// ```
final class AndroidPush implements PushPlatform, PushAndroidFlutterApi {
  /// Creates the push over [host]; [metrics] receives the display counters.
  AndroidPush({
    required PushAndroidHostApi host,
    required ClientMetrics metrics,
  }) : _host = host, // ignore: prefer_initializing_formals
       _metrics = metrics; // ignore: prefer_initializing_formals

  /// The secret key of the FCM token last registered for an instance.
  static const String tokenKey = 'push/fcm_token';

  final PushAndroidHostApi _host;
  final ClientMetrics _metrics;
  final StreamController<String> _targets = StreamController<String>();
  PushAccount? _account;
  String _previewMode = 'full';
  bool _asked = false;
  Future<void>? _registering;

  @override
  Stream<String> get targets => _targets.stream;

  @override
  Future<void> listen() async {
    PushAndroidFlutterApi.setUp(this);
    try {
      for (final tap in await _host.takePendingTaps()) {
        onTap(tap);
      }
    } on Object {
      // A host without the runner's channel (widget tests): no taps.
    }
  }

  @override
  void onTap(AndroidPushTap tap) {
    if (tap.threadId.isEmpty) return;
    _targets.add(pushTargetPath(tap.threadId, tap.messageId));
  }

  /// FCM issued a new token: registers it when the account is signed in and the permission
  /// is given; otherwise the next start compares the token and registers then.
  @override
  void onNewToken(String token) {
    if (_account == null) return;
    unawaited(_registerIfAuthorized());
  }

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
    await _registerIfAuthorized();
  }

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

  @override
  Future<void> previewModeChanged(String previewMode) async {
    if (previewMode == _previewMode) return;
    _previewMode = previewMode;
    if (_account == null || !await _available()) return;
    await _guard(_writeSettings);
  }

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

  Future<void> _registerIfAuthorized() async {
    if (!await _available()) return;
    if (await _guardValue(_host.isAuthorized) ?? false) {
      await _register();
    }
  }

  Future<void> _register() =>
      _registering ??= _registerOnce().whenComplete(() => _registering = null);

  Future<void> _registerOnce() async {
    final account = _account;
    if (account == null) return;
    await _guard(() async {
      final token = await _host.token();
      final known = await account.secrets.read(tokenKey);
      final stored = await _host.registrationId(account.instanceId);
      if (known == token && stored != null) return;
      final created = await account.registrations.register(
        provider: 'fcm',
        token: token,
        appId: await _host.appId(),
      );
      // The account may have ended while the call ran: then nothing is stored.
      if (!identical(_account, account)) return;
      await _host.saveRegistration(
        AndroidPushRegistration(
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
