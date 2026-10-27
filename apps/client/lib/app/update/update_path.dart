// SPDX-License-Identifier: Apache-2.0

/// The update path of a build: where a person gets a newer version of this app (docs/spec/
/// 18-release.md 5 and 8, 15 15; brief M7-13 requirement 16).
library;

import 'package:flutter/services.dart';
import 'package:url_launcher/url_launcher.dart';

/// How this build is updated, from its build configuration (`distribution.yaml` values the
/// release steps compile in as `BRANDING_UPDATE_URL`).
///
/// iOS: the TestFlight or App Store page; Android: the Play page; Windows: `stable.appinstaller`
/// of the update address; macOS: Sparkle's own update check inside the app ([inApp]), with
/// the download address of the appcast's archives as [url] for a build that carries no
/// Sparkle. A build without an address and without Sparkle has no path ([available] is
/// false); the app then only names the required version.
///
/// Example: `UpdatePath.of(platform: 'ios', updateUrl: 'https://apps.example.com/chat')`.
final class UpdatePath {
  /// Creates the path.
  const UpdatePath({this.url, this.inApp = false});

  /// The path of a build of [platform] (`ios`, `android`, `macos`, `windows`) with the update
  /// address [updateUrl] ("" for none). Anything but an `https` address counts as none.
  factory UpdatePath.of({required String platform, required String updateUrl}) {
    final parsed = Uri.tryParse(updateUrl);
    final url =
        parsed != null && parsed.scheme == 'https' && parsed.host.isNotEmpty
        ? parsed
        : null;
    return UpdatePath(url: url, inApp: platform == 'macos');
  }

  /// No path.
  static const UpdatePath none = UpdatePath();

  /// The page or file of the update, or null.
  final Uri? url;

  /// Whether the app checks for the update itself (Sparkle on macOS).
  final bool inApp;

  /// Whether the app can offer an "Update" action.
  bool get available => inApp || url != null;

  @override
  bool operator ==(Object other) =>
      other is UpdatePath && other.url == url && other.inApp == inApp;

  @override
  int get hashCode => Object.hash(url, inApp);
}

/// Opens the update path; tests replace it.
abstract interface class UpdateLauncher {
  /// Opens [path]: Sparkle's update check when [UpdatePath.inApp] and the build carries
  /// Sparkle, else [UpdatePath.url] in the system (TestFlight, the store, the browser).
  /// Returns whether anything opened; never throws.
  Future<bool> open(UpdatePath path);
}

/// An [UpdateLauncher] that opens nothing (tests, builds without a path).
final class NoUpdateLauncher implements UpdateLauncher {
  /// Creates the launcher.
  const NoUpdateLauncher();

  @override
  Future<bool> open(UpdatePath path) async => false;
}

/// [UpdateLauncher] of a real build: the method channel `bothy/updater` of the macOS runner
/// (`AppUpdater.checkForUpdates`, which answers false when the bundle carries no Sparkle) and
/// `url_launcher` for the address. [channel] and [opener] are replaced in tests.
final class PlatformUpdateLauncher implements UpdateLauncher {
  /// Creates the launcher.
  PlatformUpdateLauncher({
    MethodChannel? channel,
    Future<bool> Function(Uri url)? opener,
  }) : _channel = channel ?? const MethodChannel('bothy/updater'),
       _launch =
           opener ??
           ((url) => launchUrl(url, mode: LaunchMode.externalApplication));

  final MethodChannel _channel;
  final Future<bool> Function(Uri url) _launch;

  @override
  Future<bool> open(UpdatePath path) async {
    if (path.inApp) {
      try {
        if (await _channel.invokeMethod<bool>('checkForUpdates') ?? false) {
          return true;
        }
      } on Object {
        // No runner channel (a test or another platform): the address is the way.
      }
    }
    final url = path.url;
    if (url == null) return false;
    try {
      return await _launch(url);
    } on Object {
      return false;
    }
  }
}
