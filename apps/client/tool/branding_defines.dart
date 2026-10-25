// SPDX-License-Identifier: Apache-2.0

/// Turns a build branding file into the inputs of a desktop build (docs/DECISIONS.md
/// D-M4-18-7, ADR 0044).
///
/// Usage, from `apps/client`:
///
/// ```sh
/// dart run tool/branding_defines.dart [--input branding.yaml] [--check]
/// flutter build macos --dart-define-from-file=build/branding/dart_defines.json
/// ```
///
/// It writes `build/branding/dart_defines.json` (read by `BuildBranding.fromEnvironment`),
/// `macos/Runner/Configs/Branding.xcconfig` (name, bundle identifier and URL scheme of the
/// macOS bundle), `windows/runner/branding.h` (window title and URL scheme of the Windows
/// runner), `ios/Runner/Configs/Branding.xcconfig` (name, bundle identifier, URL scheme and
/// signing team of the iOS app, and the App Group and keychain group it shares with its
/// Notification Service Extension), `ios/Runner/Branding.entitlements` (the associated domain
/// for passkeys, only when the branding names one, docs/decisions.d/M5-02.md D-M5-02-2; with a
/// team the push entitlement and the shared groups, M5-04),
/// `ios/NotificationService/Branding.entitlements` (the shared groups of the extension) and
/// `android/branding.properties` (application ID, name, URL scheme and App Links host of the
/// Android app, read by `android/app/build.gradle.kts`; docs/decisions.d/M5-03.md D-M5-03-1).
/// With `--check` it writes nothing and exits with 1 when a native file differs from what the
/// input gives, so the checked-in files always match branding.example.yaml.
library;

import 'dart:convert';
import 'dart:io';

import 'package:yaml/yaml.dart';

/// The keys of a branding file this tool reads, each required.
const List<String> brandingKeys = [
  'app_name',
  'url_scheme',
  'bundle_id',
  'default_server',
];

/// The optional keys of a branding file: the public domain of the phone builds (the
/// associated domain of passkeys on iOS, the App Links host on Android), the Apple signing
/// team (both empty in a neutral build) and the Android application ID (the bundle identifier
/// when absent).
const List<String> optionalBrandingKeys = [
  'associated_domain',
  'development_team',
  'application_id',
];

/// A build branding, validated.
final class Branding {
  /// Creates the branding.
  const Branding({
    required this.appName,
    required this.urlScheme,
    required this.bundleId,
    required this.defaultServer,
    required this.appVersion,
    this.associatedDomain = '',
    this.developmentTeam = '',
    String? applicationId,
  }) : applicationId = applicationId ?? bundleId;

  /// Parses [yamlText] and takes [appVersion] from the app's pubspec.
  ///
  /// Throws a [FormatException] for a missing key, an app name with characters other than
  /// letters, digits and spaces, a URL scheme that is not RFC 3986 (lower case only), a bundle
  /// identifier that is not reverse DNS, a default server that is not an `https` origin, an
  /// associated domain that is not a host name, or a signing team that is not ten upper-case
  /// letters or digits, or an application ID (given, or else the bundle identifier) that is
  /// not lower-case letters and digits in dot-separated segments that start with a letter.
  factory Branding.parse(String yamlText, {required String appVersion}) {
    final doc = loadYaml(yamlText);
    if (doc is! YamlMap) throw const FormatException('the branding is no map');
    String value(String key) {
      final v = doc[key];
      if (v is! String || v.isEmpty) {
        throw FormatException('the branding lacks $key');
      }
      return v;
    }

    String optional(String key) {
      final v = doc[key];
      if (v == null) return '';
      if (v is! String) throw FormatException('$key: not a text');
      return v;
    }

    final branding = Branding(
      appName: value('app_name'),
      urlScheme: value('url_scheme'),
      bundleId: value('bundle_id'),
      defaultServer: value('default_server'),
      appVersion: appVersion,
      associatedDomain: optional('associated_domain'),
      developmentTeam: optional('development_team'),
      applicationId: doc['application_id'] == null
          ? null
          : optional('application_id'),
    );
    if (!RegExp(r'^[A-Za-z0-9 ]+$').hasMatch(branding.appName)) {
      throw const FormatException('app_name: letters, digits and spaces only');
    }
    if (!RegExp(r'^[a-z][a-z0-9+.-]*$').hasMatch(branding.urlScheme)) {
      throw const FormatException('url_scheme: not a lower-case URI scheme');
    }
    if (!RegExp(r'^[a-z0-9-]+(\.[a-z0-9-]+)+$').hasMatch(branding.bundleId)) {
      throw const FormatException('bundle_id: not a reverse DNS name');
    }
    final server = Uri.tryParse(branding.defaultServer);
    if (server == null || server.scheme != 'https' || server.host.isEmpty) {
      throw const FormatException('default_server: not an https origin');
    }
    if (branding.associatedDomain.isNotEmpty &&
        !RegExp(r'^[a-z0-9-]+(\.[a-z0-9-]+)+$')
            .hasMatch(branding.associatedDomain)) {
      throw const FormatException('associated_domain: not a host name');
    }
    if (branding.developmentTeam.isNotEmpty &&
        !RegExp(r'^[A-Z0-9]{10}$').hasMatch(branding.developmentTeam)) {
      throw const FormatException('development_team: not an Apple team ID');
    }
    // An Android package name that is also an RFC 3986 scheme (the OIDC redirect): no
    // hyphen, no underscore, no capital letter, no segment that starts with a digit.
    if (!RegExp(r'^[a-z][a-z0-9]*(\.[a-z][a-z0-9]*)+$')
        .hasMatch(branding.applicationId)) {
      throw const FormatException(
        'application_id: not an Android application ID; set application_id when bundle_id '
        'has a hyphen',
      );
    }
    return branding;
  }

  /// The app name the system shows (window title before discovery, bundle name).
  final String appName;

  /// The URL scheme of deep links, for example `chat-example`.
  final String urlScheme;

  /// The bundle identifier on macOS (and the package identity of M7).
  final String bundleId;

  /// The address E1 suggests.
  final String defaultServer;

  /// The app version, from `version:` of the pubspec without the build number.
  final String appVersion;

  /// The domain of the `webcredentials` entitlement on iOS (passkeys, 15 8), or "".
  final String associatedDomain;

  /// The Apple team that signs the iOS app, or "" for an unsigned build.
  final String developmentTeam;

  /// The application ID of the Android app, also the scheme of its OIDC redirect.
  final String applicationId;

  /// The JSON of `--dart-define-from-file`.
  String dartDefines() => const JsonEncoder.withIndent('  ').convert({
    'BRANDING_APP_NAME': appName,
    'BRANDING_URL_SCHEME': urlScheme,
    'BRANDING_BUNDLE_ID': bundleId,
    'BRANDING_DEFAULT_SERVER': defaultServer,
    'BRANDING_APP_VERSION': appVersion,
    'BRANDING_ASSOCIATED_DOMAIN': associatedDomain,
    'BRANDING_APPLICATION_ID': applicationId,
  });

  /// The xcconfig that `AppInfo.xcconfig` includes.
  String xcconfig() =>
      '// SPDX-License-Identifier: Apache-2.0\n'
      '// Generated by tool/branding_defines.dart from the build branding; do not edit.\n'
      'BRANDING_APP_NAME = $appName\n'
      'BRANDING_BUNDLE_ID = $bundleId\n'
      'BRANDING_URL_SCHEME = $urlScheme\n';

  /// The xcconfig that `ios/Runner/Configs/AppInfo.xcconfig` includes.
  String iosXcconfig() =>
      '// SPDX-License-Identifier: Apache-2.0\n'
      '// Generated by tool/branding_defines.dart from the build branding; do not edit.\n'
      'BRANDING_APP_NAME = $appName\n'
      'BRANDING_BUNDLE_ID = $bundleId\n'
      'BRANDING_URL_SCHEME = $urlScheme\n'
      // No trailing space without a team: the text hooks would change the checked-in file.
      'BRANDING_DEVELOPMENT_TEAM =${_value(developmentTeam)}\n'
      // M5-04: the App Group and the keychain group the app shares with its Notification
      // Service Extension, read by both through their Info.plist; empty without a team.
      'CLIENTKIT_APP_GROUP =${_value(appGroup)}\n'
      'CLIENTKIT_KEYCHAIN_GROUP =${_value(sharedKeychainGroup)}\n';

  static String _value(String v) => v.isEmpty ? '' : ' $v';

  /// The App Group of a signed iOS build (`group.<bundle id>`), or "" without a team: App
  /// Groups and keychain sharing need a team signature (docs/decisions.d/M5-04.md D-M5-04-1).
  String get appGroup => developmentTeam.isEmpty ? '' : 'group.$bundleId';

  /// The keychain group the app shares with its extensions (`<team>.<bundle id>.shared`), or
  /// "" without a team.
  String get sharedKeychainGroup =>
      developmentTeam.isEmpty ? '' : '$developmentTeam.$bundleId.shared';

  /// The entitlements of the iOS app: the associated domain of passkeys when the branding
  /// names one, and with a team the push entitlement, the App Group and the keychain groups
  /// (the app's own group first, so its own secrets stay out of the shared one; M5-04).
  String iosEntitlements() => _plist(
    (associatedDomain.isEmpty
            ? ''
            : _array('com.apple.developer.associated-domains', [
                'webcredentials:$associatedDomain',
              ])) +
        _push(appKeychainGroups: true),
  );

  /// The entitlements of the iOS Notification Service Extension: the App Group and the shared
  /// keychain group with a team, none without (M5-04).
  String iosExtensionEntitlements() => _plist(_push(appKeychainGroups: false));

  String _push({required bool appKeychainGroups}) {
    if (developmentTeam.isEmpty) return '';
    // `development`, as Xcode writes it: a development profile grants only that value, and
    // the export for TestFlight, the App Store or ad hoc re-signs with the profile's
    // `production` (D-M5-04-10). The app reads the environment from its embedded profile.
    final aps = appKeychainGroups
        ? '\t<key>aps-environment</key>\n'
              '\t<string>development</string>\n'
        : '';
    return aps +
        _array('com.apple.security.application-groups', [appGroup]) +
        _array('keychain-access-groups', [
          if (appKeychainGroups) '$developmentTeam.$bundleId',
          sharedKeychainGroup,
        ]);
  }

  static String _array(String key, List<String> values) =>
      '\t<key>$key</key>\n'
      '\t<array>\n'
      '${values.map((v) => '\t\t<string>$v</string>\n').join()}'
      '\t</array>\n';

  static String _plist(String body) =>
      '<?xml version="1.0" encoding="UTF-8"?>\n'
      '<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" '
      '"http://www.apple.com/DTDs/PropertyList-1.0.dtd">\n'
      '<!-- SPDX-License-Identifier: Apache-2.0 -->\n'
      '<!-- Generated by tool/branding_defines.dart from the build branding; do not edit. -->\n'
      '<plist version="1.0">\n'
      '<dict>\n'
      '$body'
      '</dict>\n'
      '</plist>\n';

  /// The properties that `android/app/build.gradle.kts` reads: the application ID, the label,
  /// the URL scheme of deep links and the App Links host ("" for none, so the build declares
  /// no verified link).
  String androidProperties() =>
      '# SPDX-License-Identifier: Apache-2.0\n'
      '# Generated by tool/branding_defines.dart from the build branding; do not edit.\n'
      'applicationId=$applicationId\n'
      'appName=$appName\n'
      'urlScheme=$urlScheme\n'
      'appLinksHost=$associatedDomain\n';

  /// The header of the Windows runner.
  String windowsHeader() =>
      '// SPDX-License-Identifier: Apache-2.0\n'
      '// Generated by tool/branding_defines.dart from the build branding; do not edit.\n'
      '#ifndef RUNNER_BRANDING_H_\n'
      '#define RUNNER_BRANDING_H_\n'
      '\n'
      '#define BRANDING_APP_NAME_W L"$appName"\n'
      '#define BRANDING_URL_SCHEME "$urlScheme"\n'
      '#define BRANDING_URL_SCHEME_W L"$urlScheme"\n'
      '\n'
      '#endif  // RUNNER_BRANDING_H_\n';
}

/// The `version:` of [pubspecText] without a `+build` suffix.
///
/// Throws a [FormatException] when the pubspec has no version.
String pubspecVersion(String pubspecText) {
  final version = (loadYaml(pubspecText) as YamlMap)['version'];
  if (version is! String) {
    throw const FormatException('pubspec without version');
  }
  return version.split('+').first;
}

/// Runs the tool in [appDir]; returns the exit code. [out] receives the messages.
int run(
  List<String> args, {
  required Directory appDir,
  required StringSink out,
}) {
  var input = 'branding.example.yaml';
  var check = false;
  for (var i = 0; i < args.length; i++) {
    switch (args[i]) {
      case '--check':
        check = true;
      case '--input' when i + 1 < args.length:
        input = args[++i];
      default:
        out.writeln('usage: branding_defines.dart [--input <file>] [--check]');
        return 2;
    }
  }
  final Branding branding;
  try {
    branding = Branding.parse(
      File('${appDir.path}/$input').readAsStringSync(),
      appVersion: pubspecVersion(
        File('${appDir.path}/pubspec.yaml').readAsStringSync(),
      ),
    );
  } on FormatException catch (e) {
    out.writeln('branding_defines: ${e.message}');
    return 1;
  }
  final native = {
    'macos/Runner/Configs/Branding.xcconfig': branding.xcconfig(),
    'windows/runner/branding.h': branding.windowsHeader(),
    'ios/Runner/Configs/Branding.xcconfig': branding.iosXcconfig(),
    'ios/Runner/Branding.entitlements': branding.iosEntitlements(),
    'ios/NotificationService/Branding.entitlements': branding
        .iosExtensionEntitlements(),
    'android/branding.properties': branding.androidProperties(),
  };
  if (check) {
    var ok = true;
    for (final MapEntry(key: path, value: text) in native.entries) {
      final file = File('${appDir.path}/$path');
      if (!file.existsSync() || file.readAsStringSync() != text) {
        out.writeln('branding_defines: $path differs from $input');
        ok = false;
      }
    }
    return ok ? 0 : 1;
  }
  for (final MapEntry(key: path, value: text) in {
    ...native,
    'build/branding/dart_defines.json': '${branding.dartDefines()}\n',
  }.entries) {
    File('${appDir.path}/$path')
      ..createSync(recursive: true)
      ..writeAsStringSync(text);
    out.writeln('wrote $path');
  }
  return 0;
}

void main(List<String> args) {
  exitCode = run(args, appDir: Directory.current, out: stdout);
}
