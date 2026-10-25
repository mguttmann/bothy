// SPDX-License-Identifier: Apache-2.0

/// The Pigeon source of the push channel between the app and the Swift glue of its iOS and macOS
/// runners (brief M5-04 Design note 5; docs/decisions.d/M5-04.md).
///
/// Regenerate with `dart run pigeon --input pigeons/push_apple.dart` in `apps/client`;
/// `make codegen-check` regenerates and compares. The generated files are
/// `lib/src/generated/push_apple_api.g.dart` and `ios/Runner/PushAppleApi.g.swift`; the macOS
/// runner compiles the same Swift file.
library;

import 'package:pigeon/pigeon.dart';

/// What `registerPush` returned, for the shared keychain of the app and its extension.
class PushRegistrationRecord {
  /// Creates the record.
  PushRegistrationRecord({
    required this.instanceId,
    required this.serverUrl,
    required this.registrationId,
    required this.previewToken,
  });

  /// The `instance_id` the push payload names in `b.i`.
  String instanceId;

  /// The server address of the instance.
  String serverUrl;

  /// The ID of the push registration.
  String registrationId;

  /// The preview token, shown once by the server; only the keychain keeps it.
  String previewToken;
}

/// The display counters of the Notification Service Extension since the last call.
class PushDisplayCounters {
  /// Creates the counters.
  PushDisplayCounters({
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
class PushTap {
  /// Creates the tap.
  PushTap({required this.instanceId, required this.threadId, this.messageId});

  /// `b.i`.
  String instanceId;

  /// `b.t`.
  String threadId;

  /// `b.m`, absent without a message.
  String? messageId;
}

/// The notification services of iOS and macOS the app needs and Flutter does not offer.
///
/// Every error leaves the Swift side as a `PushApplePigeonError` with the code `unavailable`
/// (no permission, no shared groups, no APNs token) or `failed`, never with a message that
/// names local details.
@ConfigurePigeon(
  PigeonOptions(
    dartOut: 'lib/src/generated/push_apple_api.g.dart',
    dartPackageName: 'client',
    swiftOut: 'ios/Runner/PushAppleApi.g.swift',
    swiftOptions: SwiftOptions(errorClassName: 'PushApplePigeonError'),
    copyrightHeader: 'pigeons/copyright_push_apple.txt',
  ),
)
@HostApi()
abstract class PushAppleHostApi {
  /// Whether this build can receive pushes: it has the App Group and the shared keychain group
  /// (a build signed by a team).
  bool isAvailable();

  /// The APNs environment of the build's `aps-environment` entitlement: `production` or
  /// `sandbox` (the value `development`).
  String environment();

  /// The bundle identifier, the `app_id` of the registration.
  String appId();

  /// Whether the person allowed notifications already, without asking.
  @async
  bool isAuthorized();

  /// Asks the person for alerts, sounds and badges; returns whether they allowed them.
  @async
  bool requestAuthorization();

  /// Registers with APNs and returns the device token in lower-case hex.
  @async
  String deviceToken();

  /// Stores [record] in the shared keychain, replacing the earlier one of its instance.
  void saveRegistration(PushRegistrationRecord record);

  /// The registration ID stored for [instanceId], or null.
  String? registrationId(String instanceId);

  /// Deletes the keychain items of [instanceId] and the account values of the App Group.
  void deleteRegistration(String instanceId);

  /// Writes the App Group values the extension reads: the signed-in instance,
  /// `notify.preview_mode` and `client.push.fetch_timeout` in milliseconds.
  void writeSettings(String instanceId, String previewMode, int fetchTimeoutMillis);

  /// Returns the extension's display counters and resets them.
  PushDisplayCounters takeDisplayCounters();

  /// Returns the taps that arrived before the app listened; afterwards taps go to
  /// [PushAppleFlutterApi.onTap].
  List<PushTap> takePendingTaps();
}

/// Calls from the runner into the app.
@FlutterApi()
abstract class PushAppleFlutterApi {
  /// The person tapped a notification.
  void onTap(PushTap tap);
}
