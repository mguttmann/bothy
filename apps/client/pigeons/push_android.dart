// SPDX-License-Identifier: Apache-2.0

/// The Pigeon source of the push channel between the app and the Kotlin push code of its
/// Android runner (brief M5-05 Design note 4; docs/decisions.d/M5-05.md).
///
/// Regenerate with `dart run pigeon --input pigeons/push_android.dart` in `apps/client`;
/// `make codegen-check` regenerates and compares. The generated files are
/// `lib/src/generated/push_android_api.g.dart` and
/// `android/app/src/main/kotlin/com/example/client/push/PushAndroidApi.g.kt`.
library;

import 'package:pigeon/pigeon.dart';

/// What `registerPush` returned, for the encrypted store of the push service.
class AndroidPushRegistration {
  /// Creates the record.
  AndroidPushRegistration({
    required this.instanceId,
    required this.serverUrl,
    required this.registrationId,
    required this.previewToken,
  });

  /// The `instance_id` the push payload names in `i`.
  String instanceId;

  /// The server address of the instance.
  String serverUrl;

  /// The ID of the push registration.
  String registrationId;

  /// The preview token, shown once by the server; only the Kotlin store keeps it.
  String previewToken;
}

/// The display counters of the push service since the last call.
class AndroidPushCounters {
  /// Creates the counters.
  AndroidPushCounters({
    required this.bucketBounds,
    required this.bucketCounts,
    required this.placeholders,
  });

  /// The upper bounds of the duration buckets in seconds.
  List<double> bucketBounds;

  /// The count per bucket, aligned with [bucketBounds].
  List<int> bucketCounts;

  /// The pushes whose placeholder stayed.
  int placeholders;
}

/// A tapped notification: the instance, the thread and the message of its payload.
class AndroidPushTap {
  /// Creates the tap.
  AndroidPushTap({
    required this.instanceId,
    required this.threadId,
    this.messageId,
  });

  /// `i`.
  String instanceId;

  /// `t`.
  String threadId;

  /// `m`, absent without a message.
  String? messageId;
}

/// The push services of Android the app needs and Flutter does not offer. Every error leaves the Kotlin side with the code `unavailable` (no Firebase configuration,
/// no activity, no token) or `failed`, never with a message that names local details.
@ConfigurePigeon(
  PigeonOptions(
    dartOut: 'lib/src/generated/push_android_api.g.dart',
    dartPackageName: 'client',
    kotlinOut: 'android/app/src/main/kotlin/com/example/client/push/PushAndroidApi.g.kt',
    kotlinOptions: KotlinOptions(
      package: 'com.example.client.push',
      errorClassName: 'PushAndroidError',
    ),
    copyrightHeader: 'pigeons/copyright_push_android.txt',
  ),
)
@HostApi()
abstract class PushAndroidHostApi {
  /// Whether this build can receive pushes: it was built with a Firebase configuration.
  bool isAvailable();

  /// The package name, the `app_id` of the registration.
  String appId();

  /// Whether notifications are allowed: the runtime permission from API 33 and the app's
  /// notifications not turned off.
  bool isAuthorized();

  /// Asks the person for the notification permission (API 33 and later); returns whether
  /// notifications are allowed afterwards.
  @async
  bool requestAuthorization();

  /// The FCM registration token.
  @async
  String token();

  /// Stores [registration] encrypted, replacing the earlier one of its instance.
  void saveRegistration(AndroidPushRegistration registration);

  /// The registration ID stored for [instanceId], or null.
  String? registrationId(String instanceId);

  /// Deletes the stored registration of [instanceId], its settings and the shown push
  /// notifications.
  void deleteRegistration(String instanceId);

  /// Writes what the push service reads: the signed-in instance, `notify.preview_mode` and
  /// `client.push.fetch_timeout` in milliseconds.
  void writeSettings(
    String instanceId,
    String previewMode,
    int fetchTimeoutMillis,
  );

  /// Returns the push service's display counters and resets them.
  AndroidPushCounters takeDisplayCounters();

  /// Returns the taps that arrived before the app listened; afterwards taps go to
  /// [PushAndroidFlutterApi.onTap].
  List<AndroidPushTap> takePendingTaps();
}

/// Calls from the runner into the app.
@FlutterApi()
abstract class PushAndroidFlutterApi {
  /// The person tapped a notification.
  void onTap(AndroidPushTap tap);

  /// FCM issued a new registration token (`onNewToken`).
  void onNewToken(String token);
}
