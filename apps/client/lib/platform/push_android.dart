// SPDX-License-Identifier: Apache-2.0

/// Push on Android: permission, FCM token, `registerPush`, the encrypted store of the Kotlin
/// push service, taps and the service's display counters (brief M5-05 Design note 4;
/// docs/spec/13-push-notifications.md 5.2, 6.4 and 13; docs/spec/15-client-architecture.md
/// 10.2 and 11).
library;

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

/// Push on Android for the app (brief M5-05 Design note 4; docs/decisions.d/M5-05.md): the
/// flow of [NativePush] over the Kotlin push host.
///
/// The permission is the runtime permission `POST_NOTIFICATIONS` from API 33, asked at the same
/// moment as on iOS (D-M5-05-6); below API 33 the app's notifications only must not be turned
/// off. The FCM token is registered with provider `fcm` and the package name; a token FCM
/// rotates while the app runs ([onNewToken]) registers again. [forget] also removes the shown
/// push notifications. A build without a Firebase configuration has no push: every call does
/// nothing.
///
/// Example:
/// ```dart
/// final push = AndroidPush(host: PushAndroidHostApi(), metrics: metrics)..listen();
/// await push.signedIn(account, previewMode: 'full');
/// ```
final class AndroidPush extends NativePush implements PushAndroidFlutterApi {
  /// Creates the push over [host]; [metrics] receives the display counters.
  AndroidPush({
    required PushAndroidHostApi host,
    required ClientMetrics metrics,
  }) : _host = host, // ignore: prefer_initializing_formals
       super(metrics);

  /// The secret key of the FCM token last registered for an instance.
  static const String fcmTokenKey = 'push/fcm_token';

  final PushAndroidHostApi _host;

  @override
  String get tokenKey => fcmTokenKey;

  @override
  void onTap(AndroidPushTap tap) => tapped(tap.threadId, tap.messageId);

  @override
  void onNewToken(String token) => tokenChanged();

  @override
  Future<List<(String, String?)>> connectHost() async {
    PushAndroidFlutterApi.setUp(this);
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
  Future<String> hostToken() => _host.token();

  @override
  Future<RegisteredPush> registerToken(
    PushRegistrations registrations,
    String token,
  ) async => registrations.register(
    provider: 'fcm',
    token: token,
    appId: await _host.appId(),
  );

  @override
  Future<void> saveRegistration(PushAccount account, RegisteredPush created) =>
      _host.saveRegistration(
        AndroidPushRegistration(
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
