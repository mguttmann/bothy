// SPDX-License-Identifier: Apache-2.0

/// The build branding of the apps before the distribution of M7 (docs/DECISIONS.md
/// D-M4-18-7, docs/decisions.d/M5-02.md D-M5-02-2, ADR 0044).
library;

/// What the build knows about its branding: the app name the system shows, the URL scheme of
/// deep links, the bundle identifier, the address E1 suggests and the app version.
///
/// `tool/branding_defines.dart` writes these values from `branding.example.yaml` (or the
/// `branding.yaml` of an instance) into `--dart-define-from-file` input; a build without it
/// gets the neutral values of `branding.example.yaml`. The interface shows
/// `branding.product_name` of the instance after discovery and [appName] only where the system
/// needs a name before that (01 12, fixed requirement 20).
///
/// Example: `final branding = BuildBranding.fromEnvironment;`
final class BuildBranding {
  /// Creates a branding from explicit values (tests).
  const BuildBranding({
    required this.appName,
    required this.urlScheme,
    required this.bundleId,
    required this.defaultServer,
    required this.appVersion,
    this.crashDsn = '',
    this.crashEnvironment = 'production',
    this.associatedDomain = '',
  });

  /// The branding the build was compiled with.
  static const BuildBranding fromEnvironment = BuildBranding(
    appName: String.fromEnvironment(
      'BRANDING_APP_NAME',
      defaultValue: 'ExampleChat',
    ),
    urlScheme: String.fromEnvironment(
      'BRANDING_URL_SCHEME',
      defaultValue: 'chat-example',
    ),
    bundleId: String.fromEnvironment(
      'BRANDING_BUNDLE_ID',
      defaultValue: 'com.example.chat',
    ),
    defaultServer: String.fromEnvironment(
      'BRANDING_DEFAULT_SERVER',
      defaultValue: 'https://chat.example.com',
    ),
    appVersion: String.fromEnvironment(
      'BRANDING_APP_VERSION',
      defaultValue: '0.0.0',
    ),
    crashDsn: String.fromEnvironment('CRASH_DSN'),
    crashEnvironment: String.fromEnvironment(
      'CRASH_ENVIRONMENT',
      defaultValue: 'production',
    ),
    associatedDomain: String.fromEnvironment('BRANDING_ASSOCIATED_DOMAIN'),
  );

  /// The name of the app in the system (window title before discovery).
  final String appName;

  /// The URL scheme of deep links (15 11), for example `chat-example`.
  final String urlScheme;

  /// The bundle identifier.
  final String bundleId;

  /// The address E1 suggests.
  final String defaultServer;

  /// The version of this build, compared with `min_client_versions` on E1.
  final String appVersion;

  /// The crash report target of the build (the define `CRASH_DSN`), or "" for none. 15 16
  /// allows the build configuration only; there is no instance setting (docs/DECISIONS.md
  /// D-M4-31-3).
  final String crashDsn;

  /// The `environment` of every crash report (the define `CRASH_ENVIRONMENT`).
  final String crashEnvironment;

  /// The associated domain of an iOS build (`associated_domain` of the branding file), or ""
  /// without one: only with it the build has the `webcredentials` entitlement and offers
  /// passkeys (15 8; D-M5-02-5).
  final String associatedDomain;

  /// Whether the build names a crash report target (15 16): only then Settings offers
  /// `privacy.crash_reports` and the crash path can start (briefs M4-25 Design note 4, M4-31).
  bool get hasCrashTarget => crashDsn.isNotEmpty;
}
