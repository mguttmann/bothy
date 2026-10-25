// SPDX-License-Identifier: Apache-2.0

/// The Android adapters of the login ports over the Pigeon channel of the runner
/// (docs/spec/15-client-architecture.md 8 and 9; 08 2.2, 2.5 and 18; brief M5-03 Design notes 2
/// and 3; docs/decisions.d/M5-03.md D-M5-03-3).
library;

import '../src/generated/android_platform_api.g.dart';
import 'browser_session.dart';

export '../src/generated/android_platform_api.g.dart' show AndroidPlatformApi;

/// [BrowserSessionLogin] over a Custom Tab (`androidx.browser`), or the default browser where
/// no Custom Tabs provider exists; never a web view (15 8).
///
/// The redirect `<application ID>:/oidc/callback` reaches the runner's link activity, which
/// hands it to the waiting call; coming back to the app without it ends the login with
/// `error=cancelled`.
///
/// Example:
/// ```dart
/// final login = AndroidCustomTabsLogin(api: AndroidPlatformApi(), applicationId: 'com.example.chat');
/// ```
final class AndroidCustomTabsLogin extends BrowserSessionLogin {
  /// Creates the adapter over [api] for the build with [applicationId].
  AndroidCustomTabsLogin({
    required AndroidPlatformApi api,
    required String applicationId,
  }) : _api = api, // ignore: prefer_initializing_formals
       super(appId: applicationId);

  final AndroidPlatformApi _api;

  @override
  Future<String> runSession(String url, String callbackScheme) =>
      _api.authenticate(url, callbackScheme);
}

/// [DomainPasskeys] over the Credential Manager (`androidx.credentials`) for the relying party
/// [appLinksHost] of the build (15 8: passkeys in native apps only with a public domain).
///
/// The instance lists the app's origin `android:apk-key-hash:<hash>` in
/// `auth.webauthn_origins` (08 18), and the host serves the Digital Asset Links of the app.
final class AndroidPasskeys extends DomainPasskeys {
  /// Creates the authenticator over [api] for [appLinksHost].
  AndroidPasskeys({
    required AndroidPlatformApi api,
    required String appLinksHost,
  }) : _api = api, // ignore: prefer_initializing_formals
       super(domain: appLinksHost);

  final AndroidPlatformApi _api;

  @override
  Future<String> assertion(String requestOptionsJson) =>
      _api.passkeyAssertion(requestOptionsJson);

  @override
  Future<String> registration(String creationOptionsJson) =>
      _api.passkeyRegistration(creationOptionsJson);
}

/// Whether passkeys of a build with [appLinksHost] can serve the instance at [host]: only the
/// same host, because the Credential Manager checks the relying party against the Digital
/// Asset Links of exactly that host (brief M5-03 Design note 2).
///
/// Example: `appLinksHostServes('chat.example.com', 'chat.example.com')` is true.
bool appLinksHostServes(String appLinksHost, String host) =>
    appLinksHost.isNotEmpty && host == appLinksHost;

/// The passkeys of an Android build: [AndroidPasskeys] for [appLinksHost] when the device
/// [supported] them (the Credential Manager's platform provider, API 34 and later) and the
/// build names an App Links host; null otherwise, so E2 hides the passkey button.
AndroidPasskeys? androidPasskeys(
  AndroidPlatformApi api, {
  required bool supported,
  required String appLinksHost,
}) => supported && appLinksHost.isNotEmpty
    ? AndroidPasskeys(api: api, appLinksHost: appLinksHost)
    : null;

/// What the app reads from the runner once at start, before it builds its ports.
final class AndroidRunnerInfo {
  /// Creates the values.
  const AndroidRunnerInfo({
    required this.deviceName,
    required this.passkeysSupported,
  });

  /// Reads the values over [api]; throws the runner's `PlatformException`.
  static Future<AndroidRunnerInfo> load(AndroidPlatformApi api) async =>
      AndroidRunnerInfo(
        deviceName: await api.deviceName(),
        passkeysSupported: await api.passkeysSupported(),
      );

  /// The device's name for the registration (08 5.1).
  final String deviceName;

  /// Whether the Credential Manager serves passkeys on this device.
  final bool passkeysSupported;
}
