// SPDX-License-Identifier: Apache-2.0

/// Push on iOS and macOS: permission, APNs token, `registerPush`, the shared keychain of the
/// Notification Service Extension, taps and the extension's display counters (brief M5-04
/// Design note 5; docs/spec/13-push-notifications.md 5.1, 6.4 and 13; docs/spec/
/// 15-client-architecture.md 10.1 and 11).
library;

import '../src/generated/push_apple_api.g.dart';
import '../telemetry/client_metrics.dart';
import 'push_platform.dart';

export '../src/generated/push_apple_api.g.dart'
    show PushAppleHostApi, PushDisplayCounters, PushRegistrationRecord, PushTap;
export 'push_platform.dart';

/// Push on iOS and macOS for the app (brief M5-04 Design note 5): the flow of [NativePush] over
/// the Swift glue of the runners.
///
/// The APNs token is registered with provider `apns`, the environment and the bundle identifier
/// of the build; the runner keeps the preview token and the registration ID in the shared
/// keychain, and [forget] deletes the keychain items. [previewModeChanged] writes
/// `notify.preview_mode` into the App Group for the extension, and [signedIn] takes the
/// extension's display counters into the client metrics (attributes only platform and app
/// version, M4-31).
///
/// Example:
/// ```dart
/// final push = ApplePush(host: PushAppleHostApi(), metrics: metrics)..listen();
/// await push.signedIn(account, previewMode: 'full');
/// ```
final class ApplePush extends NativePush implements PushAppleFlutterApi {
  /// Creates the push over [host]; [metrics] receives the display counters.
  ApplePush({required PushAppleHostApi host, required ClientMetrics metrics})
    : _host = host, // ignore: prefer_initializing_formals
      super(metrics);

  /// The secret key of the APNs token last registered for an instance.
  static const String apnsTokenKey = 'push/apns_token';

  final PushAppleHostApi _host;

  /// The app path of [tap] (15 11): `/t/<thread>?m=<message>`, or `/t/<thread>` without a
  /// message.
  static String pathOf(PushTap tap) =>
      pushTargetPath(tap.threadId, tap.messageId);

  @override
  String get tokenKey => apnsTokenKey;

  @override
  void onTap(PushTap tap) => tapped(tap.threadId, tap.messageId);

  @override
  Future<List<(String, String?)>> connectHost() async {
    PushAppleFlutterApi.setUp(this);
    return [
      for (final tap in await _host.takePendingTaps())
        (tap.threadId, tap.messageId),
    ];
  }

  @override
  Future<bool> hostAvailable() => _host.isAvailable();

  @override
  Future<bool> hostAuthorized() => _host.isAuthorized();

  @override
  Future<bool> hostRequestAuthorization() => _host.requestAuthorization();

  @override
  Future<String> hostToken() => _host.deviceToken();

  @override
  Future<RegisteredPush> registerToken(
    PushRegistrations registrations,
    String token,
  ) async => registrations.register(
    provider: 'apns',
    token: token,
    environment: await _host.environment(),
    appId: await _host.appId(),
  );

  @override
  Future<void> saveRegistration(PushAccount account, RegisteredPush created) =>
      _host.saveRegistration(
        PushRegistrationRecord(
          instanceId: account.instanceId,
          serverUrl: account.serverUrl.toString(),
          registrationId: created.id,
          previewToken: created.previewToken,
        ),
      );

  @override
  Future<String?> hostRegistrationId(String instanceId) =>
      _host.registrationId(instanceId);

  @override
  Future<void> hostDeleteRegistration(String instanceId) =>
      _host.deleteRegistration(instanceId);

  @override
  Future<void> hostWriteSettings(
    String instanceId,
    String previewMode,
    int fetchTimeoutMillis,
  ) => _host.writeSettings(instanceId, previewMode, fetchTimeoutMillis);

  @override
  Future<void> takeCounters() async {
    final counters = await _host.takeDisplayCounters();
    recordPushCounters(
      metrics,
      bounds: counters.bucketBounds,
      counts: counters.bucketCounts,
      placeholders: counters.placeholders,
    );
  }
}
