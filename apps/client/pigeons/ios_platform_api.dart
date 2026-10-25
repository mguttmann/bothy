// SPDX-License-Identifier: Apache-2.0

/// The Pigeon source of the channel between the app and the Swift glue of its iOS runner
/// (brief M5-02 Design notes 3 and 6; docs/decisions.d/M5-02.md D-M5-02-4 to D-M5-02-7).
///
/// Regenerate with `dart run pigeon --input pigeons/ios_platform_api.dart` in `apps/client`;
/// `make codegen-check` regenerates and compares. The generated files are
/// `lib/src/generated/ios_platform_api.g.dart` and `ios/Runner/IosPlatformApi.g.swift`.
library;

import 'package:pigeon/pigeon.dart';

/// The system services of iOS the app needs and Flutter does not offer: the login in the
/// system browser, passkeys and the protection class of files.
///
/// Every error leaves the Swift side as a `PigeonError` with one of the codes `cancelled`
/// (the person closed the sheet), `unavailable` (the system cannot run the request here) or
/// `failed` (anything else), never with a message that names local details.
@ConfigurePigeon(
  PigeonOptions(
    dartOut: 'lib/src/generated/ios_platform_api.g.dart',
    dartPackageName: 'client',
    swiftOut: 'ios/Runner/IosPlatformApi.g.swift',
    copyrightHeader: 'pigeons/copyright.txt',
  ),
)
@HostApi()
abstract class IosPlatformApi {
  /// Runs one `ASWebAuthenticationSession` for [url] whose callback has the scheme
  /// [callbackScheme], without an ephemeral browser session, presented from the key window;
  /// returns the callback URL with its query.
  @async
  String authenticate(String url, String callbackScheme);

  /// Runs an `ASAuthorizationController` assertion with the platform provider for the
  /// `PublicKeyCredentialRequestOptions` JSON [requestOptionsJson]; returns the
  /// `PublicKeyCredential` JSON.
  @async
  String passkeyAssertion(String requestOptionsJson);

  /// Runs an `ASAuthorizationController` registration with the platform provider for the
  /// `PublicKeyCredentialCreationOptions` JSON [creationOptionsJson]; returns the
  /// `PublicKeyCredential` JSON.
  @async
  String passkeyRegistration(String creationOptionsJson);

  /// Sets the protection class `NSFileProtectionComplete` on every existing file or folder of
  /// [paths]; a missing path is skipped.
  void protectFiles(List<String> paths);

  /// The protection class of [path] (`complete`, `completeUnlessOpen`,
  /// `completeUntilFirstUserAuthentication`, `none`), or null when the path is missing.
  String? fileProtection(String path);

  /// `CLOCK_MONOTONIC` of Darwin in milliseconds: the time since boot, counting sleep, not
  /// settable by the person; the app lock counts its time with it (brief M5-08,
  /// D-M5-08-8).
  int sinceBootMillis();
}
