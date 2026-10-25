// SPDX-License-Identifier: Apache-2.0

/// The Pigeon source of the channel between the app and the Kotlin glue of its Android runner
/// (brief M5-03 Design notes 2 to 5; docs/decisions.d/M5-03.md).
///
/// Regenerate with `dart run pigeon --input pigeons/android_platform_api.dart` in `apps/client`;
/// `make codegen-check` regenerates and compares. The generated files are
/// `lib/src/generated/android_platform_api.g.dart` and
/// `android/app/src/main/kotlin/com/example/client/AndroidPlatformApi.g.kt`.
library;

import 'package:pigeon/pigeon.dart';

/// One notification channel of 01 10.3 as the app hands it to the system.
class AndroidNotificationChannel {
  /// Creates the channel description.
  AndroidNotificationChannel({
    required this.id,
    required this.name,
    required this.description,
    required this.high,
    required this.kinds,
  });

  /// The stable channel ID (`mentions`, `questions`, `messages`, `runs`).
  String id;

  /// The name in the device language (from `l10n`).
  String name;

  /// The description in the device language (from `l10n`).
  String description;

  /// Whether the importance is high (`IMPORTANCE_HIGH`); otherwise `IMPORTANCE_DEFAULT`.
  bool high;

  /// The notification kinds of 13 2 posted into this channel (wire names).
  List<String> kinds;
}

/// The system services of Android the app needs and Flutter does not offer: the login in a
/// Custom Tab, passkeys through the Credential Manager, the notification channels and the
/// device's name.
///
/// Every error leaves the Kotlin side as a `FlutterError` with one of the codes `cancelled`
/// (the person closed the tab or the sheet), `unavailable` (the system cannot run the request
/// here) or `failed` (anything else), never with a message that names local details.
@ConfigurePigeon(
  PigeonOptions(
    dartOut: 'lib/src/generated/android_platform_api.g.dart',
    dartPackageName: 'client',
    kotlinOut: 'android/app/src/main/kotlin/com/example/client/AndroidPlatformApi.g.kt',
    kotlinOptions: KotlinOptions(package: 'com.example.client'),
    copyrightHeader: 'pigeons/copyright.txt',
  ),
)
@HostApi()
abstract class AndroidPlatformApi {
  /// Opens [url] in a Custom Tab (the default browser when no Custom Tabs provider exists) and
  /// waits for the redirect whose scheme is [callbackScheme]; returns the redirect URL with
  /// its query. Returning to the app without the redirect ends with `cancelled`.
  @async
  String authenticate(String url, String callbackScheme);

  /// Whether the Credential Manager can serve passkeys on this device (API 34 and later, where
  /// the platform provider exists without Play services).
  bool passkeysSupported();

  /// Runs a Credential Manager `GetPublicKeyCredentialOption` for the
  /// `PublicKeyCredentialRequestOptions` JSON [requestOptionsJson]; returns the
  /// `PublicKeyCredential` JSON.
  @async
  String passkeyAssertion(String requestOptionsJson);

  /// Runs a Credential Manager `CreatePublicKeyCredentialRequest` for the
  /// `PublicKeyCredentialCreationOptions` JSON [creationOptionsJson]; returns the
  /// `PublicKeyCredential` JSON.
  @async
  String passkeyRegistration(String creationOptionsJson);

  /// Creates every channel of [channels] that is missing and updates the names and
  /// descriptions of the existing ones (the system keeps the importance the person chose);
  /// keeps the kind to channel mapping for the push service of M5-05.
  void registerNotificationChannels(List<AndroidNotificationChannel> channels);

  /// The device's name for the registration (08 5.1): the manufacturer and the model.
  String deviceName();

  /// `SystemClock.elapsedRealtime()`: milliseconds since boot, counting deep sleep, not
  /// settable by the person; the app lock counts its time with it (brief M5-08,
  /// D-M5-08-8).
  int sinceBootMillis();
}
