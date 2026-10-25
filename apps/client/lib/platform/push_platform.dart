// SPDX-License-Identifier: Apache-2.0

/// What push on every phone and Mac platform shares: the registration with the server, the
/// signed-in account and the interface the app drives (docs/spec/13-push-notifications.md 6.4,
/// 13; docs/spec/15-client-architecture.md 10; briefs M5-04 and M5-05).
library;

import 'dart:async';

import 'package:api_client/api_client.dart';

import '../telemetry/client_metrics.dart' as telemetry;
import 'secure_store.dart';

/// What `registerPush` returned that the app keeps: the registration ID and the preview token.
///
/// [toString] names neither, so a log line never carries the token.
final class RegisteredPush {
  /// Creates the value.
  const RegisteredPush({required this.id, required this.previewToken});

  /// The ID of the registration, for `unregisterPush`.
  final String id;

  /// The preview token, shown once (13 6.4).
  final String previewToken;

  @override
  String toString() => 'RegisteredPush(<redacted>)';
}

/// The two REST operations of a push registration (13 13).
abstract interface class PushRegistrations {
  /// `registerPush` with [provider] (`apns` or `fcm`): [token] of the provider (hex for APNs),
  /// [appId] the bundle identifier or package name, [environment] `production` or `sandbox`
  /// (APNs only; absent means `production`). Throws what the API client throws.
  Future<RegisteredPush> register({
    required String provider,
    required String token,
    required String appId,
    String? environment,
  });

  /// `unregisterPush` of [registrationId]. Throws what the API client throws.
  Future<void> unregister(String registrationId);
}

/// [PushRegistrations] over the generated API client of a signed-in session; [idempotencyKey]
/// gives a fresh key per registration (the preview token is a one-time secret, so a replay
/// answers `409`).
final class ApiPushRegistrations implements PushRegistrations {
  /// Creates the adapter over [api].
  ApiPushRegistrations(this._api, this._idempotencyKey);

  final ApiClient _api;
  final String Function() _idempotencyKey;

  @override
  Future<RegisteredPush> register({
    required String provider,
    required String token,
    required String appId,
    String? environment,
  }) async {
    final response = await _api.getDevicesApi().registerPush(
      idempotencyKey: _idempotencyKey(),
      pushRegistrationCreate: PushRegistrationCreate(
        (b) => b
          ..provider = PushProvider.valueOf(provider)
          ..token = token
          ..environment = environment == null
              ? null
              : PushEnvironment.valueOf(environment)
          ..appId = appId,
      ),
    );
    final created = response.data;
    if (created == null) {
      throw StateError('registerPush answered without a body');
    }
    return RegisteredPush(id: created.id, previewToken: created.previewToken);
  }

  @override
  Future<void> unregister(String registrationId) =>
      _api.getDevicesApi().unregisterPush(registrationId: registrationId);
}

/// The signed-in account as push needs it.
final class PushAccount {
  /// Creates the account.
  const PushAccount({
    required this.instanceId,
    required this.serverUrl,
    required this.secrets,
    required this.registrations,
    required this.fetchTimeout,
  });

  /// The `instance_id`, the scope of the device keys and of the keychain items.
  final String instanceId;

  /// The server address.
  final Uri serverUrl;

  /// The secrets of the instance; they keep the provider token last registered, never the
  /// preview token.
  final InstanceSecrets secrets;

  /// The REST operations.
  final PushRegistrations registrations;

  /// `client.push.fetch_timeout` of `AppConfig`, written for the extension or the push service.
  final Duration fetchTimeout;
}

/// Push of one platform as the app drives it: `ApplePush` on iOS and macOS, `AndroidPush` on
/// Android (in their own libraries).
///
/// Every method swallows platform failures: push is an addition, the app works without it.
abstract interface class PushPlatform {
  /// Whether this build receives pushes on this device (a Firebase configuration on Android,
  /// the shared groups of a team signature on iOS and macOS); false when the runner fails.
  Future<bool> isAvailable();

  /// The app path of every tapped notification (`/t/<thread>?m=<message>`); one listener.
  Stream<String> get targets;

  /// Connects the runner's calls and takes the taps that started the app.
  Future<void> listen();

  /// The account signed in (at start or after a login) with [previewMode] of its settings.
  Future<void> signedIn(PushAccount account, {required String previewMode});

  /// The person opened a thread: the moment to ask for the permission.
  Future<void> threadOpened();

  /// `notify.preview_mode` changed in the synced settings.
  Future<void> previewModeChanged(String previewMode);

  /// The person logs out: `unregisterPush` best effort, then [forget].
  Future<void> logout();

  /// Every end of the account of [instanceId]: deletes what the platform keeps for it.
  Future<void> forget(String instanceId);
}

/// The app path of a tapped notification (15 11): `/t/<thread>?m=<message>`, or `/t/<thread>`
/// without a message.
String pushTargetPath(String threadId, String? messageId) {
  final thread = Uri.encodeComponent(threadId);
  return messageId == null || messageId.isEmpty
      ? '/t/$thread'
      : '/t/$thread?m=${Uri.encodeQueryComponent(messageId)}';
}

/// Submits display counters of a native push component as client metrics (M4-31): one
/// histogram value per counted push at its bucket's upper bound (the bound falls into its own
/// bucket of the app's collector) and the placeholders as a counter.
void recordPushCounters(
  telemetry.ClientMetrics collector, {
  required List<double> bounds,
  required List<int> counts,
  required int placeholders,
}) {
  for (var i = 0; i < bounds.length && i < counts.length; i++) {
    for (var n = 0; n < counts[i]; n++) {
      collector.record(telemetry.ClientMetric.pushDisplayDuration, bounds[i]);
    }
  }
  if (placeholders > 0) {
    collector.record(telemetry.ClientMetric.pushPlaceholder, placeholders);
  }
}

/// The flow push shares on every platform with a native receiver (briefs M5-04 Design note 5
/// and M5-05 Design note 4); a subclass connects it to its runner's Pigeon host.
///
/// - Permission: [threadOpened] asks once per run, after the login, when the person opens a
///   thread, the first moment they can understand why; never before the login. Without the
///   permission nothing registers.
/// - Registration: with the permission the provider's token is registered with `registerPush`
///   ([registerToken]); the runner keeps the preview token and the registration ID at once
///   ([saveRegistration]), Dart never persists them. A token that differs from the one last
///   registered (kept under [tokenKey] in the instance's secrets) registers again.
/// - [logout] calls `unregisterPush` best effort; [forget] deletes what the runner keeps on every
///   end of the account (logout, `4012`, a revoked device).
/// - [previewModeChanged] hands `notify.preview_mode` to the runner.
/// - [signedIn] takes the runner's display counters into the client metrics.
/// - [targets] gives the app path of every tapped notification ([tapped]).
///
/// Every platform failure is swallowed: push is an addition, the app works without it, and the
/// next start tries again.
abstract base class NativePush implements PushPlatform {
  /// Creates the flow; [metrics] receives the display counters.
  NativePush(this.metrics);

  /// The collector of the client metrics.
  final telemetry.ClientMetrics metrics;

  final StreamController<String> _targets = StreamController<String>();
  PushAccount? _account;
  String _previewMode = 'full';
  bool _asked = false;
  Future<void>? _registering;

  /// The secret key of the provider token last registered for an instance.
  String get tokenKey;

  /// Whether the runner can receive pushes in this build.
  Future<bool> hostAvailable();

  /// Whether the person allowed notifications already, without asking.
  Future<bool> hostAuthorized();

  /// Asks the person; returns whether notifications are allowed afterwards.
  Future<bool> hostRequestAuthorization();

  /// The provider token of this device.
  Future<String> hostToken();

  /// `registerPush` of [token] through [registrations] with the provider's parameters.
  Future<RegisteredPush> registerToken(
    PushRegistrations registrations,
    String token,
  );

  /// Hands what `registerPush` returned for [account] to the runner's store.
  Future<void> saveRegistration(PushAccount account, RegisteredPush created);

  /// The registration ID the runner keeps for [instanceId], or null.
  Future<String?> hostRegistrationId(String instanceId);

  /// Deletes what the runner keeps for [instanceId].
  Future<void> hostDeleteRegistration(String instanceId);

  /// Writes the signed-in instance, `notify.preview_mode` and `client.push.fetch_timeout`.
  Future<void> hostWriteSettings(
    String instanceId,
    String previewMode,
    int fetchTimeoutMillis,
  );

  /// Takes the runner's display counters into [metrics] and resets them.
  Future<void> takeCounters();

  /// Connects the runner's calls and returns the taps that arrived before (thread and
  /// message of each).
  Future<List<(String, String?)>> connectHost();

  @override
  Stream<String> get targets => _targets.stream;

  @override
  Future<bool> isAvailable() => _available();

  @override
  Future<void> listen() async {
    try {
      for (final (thread, message) in await connectHost()) {
        tapped(thread, message);
      }
    } on Object {
      // A host without the runner's channel (widget tests): no taps.
    }
  }

  /// A notification of [threadId] at [messageId] was tapped; an empty thread is ignored.
  void tapped(String threadId, String? messageId) {
    if (threadId.isEmpty) return;
    _targets.add(pushTargetPath(threadId, messageId));
  }

  /// The provider issued a new token: registers it when an account is signed in and the
  /// permission is given; otherwise the next start compares the token and registers then.
  void tokenChanged() {
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
    await _guard(takeCounters);
    await _registerIfAuthorized();
  }

  @override
  Future<void> threadOpened() async {
    if (_account == null || _asked) return;
    _asked = true;
    if (!await _available()) return;
    if (await _guardValue(hostAuthorized) ?? false) return;
    if (await _guardValue(hostRequestAuthorization) ?? false) {
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
    final id = await _guardValue(() => hostRegistrationId(account.instanceId));
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
    await _guard(() => hostDeleteRegistration(instanceId));
  }

  Future<bool> _available() async => await _guardValue(hostAvailable) ?? false;

  Future<void> _writeSettings() async {
    final account = _account;
    if (account == null) return;
    await hostWriteSettings(
      account.instanceId,
      _previewMode,
      account.fetchTimeout.inMilliseconds,
    );
  }

  Future<void> _registerIfAuthorized() async {
    if (!await _available()) return;
    if (await _guardValue(hostAuthorized) ?? false) {
      await _register();
    }
  }

  Future<void> _register() =>
      _registering ??= _registerOnce().whenComplete(() => _registering = null);

  Future<void> _registerOnce() async {
    final account = _account;
    if (account == null) return;
    await _guard(() async {
      final token = await hostToken();
      final known = await account.secrets.read(tokenKey);
      final stored = await hostRegistrationId(account.instanceId);
      if (known == token && stored != null) return;
      final created = await registerToken(account.registrations, token);
      // The account may have ended while the call ran: then nothing is stored.
      if (!identical(_account, account)) return;
      await saveRegistration(account, created);
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
