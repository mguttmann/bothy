// SPDX-License-Identifier: Apache-2.0

/// What push on every phone and Mac platform shares: the registration with the server, the
/// signed-in account and the interface the app drives (docs/spec/13-push-notifications.md 6.4,
/// 13; docs/spec/15-client-architecture.md 10; briefs M5-04 and M5-05).
library;

import 'dart:async';

import 'package:api_client/api_client.dart';

import '../telemetry/client_metrics.dart' as metrics;
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
  metrics.ClientMetrics collector, {
  required List<double> bounds,
  required List<int> counts,
  required int placeholders,
}) {
  for (var i = 0; i < bounds.length && i < counts.length; i++) {
    for (var n = 0; n < counts[i]; n++) {
      collector.record(metrics.ClientMetric.pushDisplayDuration, bounds[i]);
    }
  }
  if (placeholders > 0) {
    collector.record(metrics.ClientMetric.pushPlaceholder, placeholders);
  }
}
