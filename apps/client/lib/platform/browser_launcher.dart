// SPDX-License-Identifier: Apache-2.0

/// The system browser for the OIDC login (docs/spec/15-client-architecture.md 8; 08 2.2 item 3).
library;

import 'package:auth/auth.dart';
import 'package:url_launcher/url_launcher.dart';

/// [BrowserLauncher] on `url_launcher` (flutter.dev): opens the default browser of the
/// system, never a web view (RFC 8252 8.12).
///
/// [opener] replaces `launchUrl` in tests.
final class SystemBrowserLauncher implements BrowserLauncher {
  /// Creates the launcher.
  SystemBrowserLauncher({Future<bool> Function(Uri url)? opener})
    : _opener = opener ?? _external;

  final Future<bool> Function(Uri url) _opener;

  static Future<bool> _external(Uri url) =>
      launchUrl(url, mode: LaunchMode.externalApplication);

  /// Opens [url]; throws a [StateError] when the system could not open it.
  @override
  Future<void> open(Uri url) async {
    if (!await _opener(url)) {
      throw StateError('the system browser did not open the login page');
    }
  }
}
