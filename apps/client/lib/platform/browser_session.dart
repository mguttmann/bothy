// SPDX-License-Identifier: Apache-2.0

/// The login of the phone builds in the system's browser session and their passkeys for one
/// domain, shared by iOS (`ASWebAuthenticationSession`, M5-02) and Android (Custom Tabs, M5-03)
/// (docs/spec/15-client-architecture.md 8; 08 2.2 and 2.5; docs/decisions.d/M5-02.md D-M5-02-4
/// and D-M5-02-5; docs/decisions.d/M5-03.md D-M5-03-3).
library;

import 'dart:async';
import 'dart:convert';

import 'package:auth/auth.dart';
import 'package:flutter/services.dart';

/// The path of the OIDC redirect below the private-use scheme of the build.
const String oidcCallbackPath = '/oidc/callback';

/// The OIDC redirect URI of a phone build whose bundle identifier (iOS) or application ID
/// (Android) is [appId].
///
/// 08 2.2 step 2 allows a private-use scheme only in reverse domain form (RFC 8252 7.1), so
/// the scheme is the app's identifier (lower-case reverse DNS, checked by
/// `tool/branding_defines.dart`), not the deep link scheme of 15 11; the provider lists this
/// URI in `app_redirect_uris`.
///
/// Example: `oidcRedirectUri('com.example.chat')` is `com.example.chat:/oidc/callback`.
Uri oidcRedirectUri(String appId) => Uri(scheme: appId, path: oidcCallbackPath);

/// [BrowserLauncher] and [RedirectReceiver] in one over a browser session of the system that
/// returns the redirect itself: one session per login (15 8), never a web view.
///
/// [open] starts the session and returns at once (the browser has the URL);
/// [waitForRedirect] waits for the redirect the session returns. When the person closes the
/// browser the redirect carries `error=cancelled`, any other failure `error=login_failed`; the
/// login then fails and E2 shows its error.
abstract base class BrowserSessionLogin
    implements BrowserLauncher, RedirectReceiver {
  /// Creates the login of the build with the identifier [appId].
  BrowserSessionLogin({required String appId})
    : _redirect = oidcRedirectUri(appId);

  final Uri _redirect;
  Future<Uri>? _session;

  /// Runs the system's session for [url] until the redirect with the scheme [callbackScheme]
  /// arrives; returns the redirect URL. Fails with a [PlatformException] whose code is
  /// `cancelled` when the person closed the browser.
  Future<String> runSession(String url, String callbackScheme);

  @override
  Future<Uri> redirectUri() async {
    _session = null;
    return _redirect;
  }

  @override
  Future<void> open(Uri url) async {
    final session = runSession(
      url.toString(),
      _redirect.scheme,
    ).then(Uri.parse, onError: _failed);
    // A failure before the caller waits must not surface as an unhandled error.
    unawaited(session.then<void>((_) {}, onError: (Object _) {}));
    _session = session;
  }

  Uri _failed(Object error) {
    final code = error is PlatformException ? error.code : 'failed';
    return _redirect.replace(
      queryParameters: {
        'error': code == 'cancelled' ? 'cancelled' : 'login_failed',
      },
    );
  }

  @override
  Future<Uri> waitForRedirect(Uri expectedPrefix, Duration timeout) {
    final session = _session;
    if (session == null) throw StateError('open was not called');
    return session.timeout(timeout);
  }
}

/// [PasskeyAuthenticator] of the system for the one relying party [domain] of the build.
///
/// Options for another relying party are refused with a [StateError] before the system is
/// asked, because the system would refuse them anyway without the matching association (the
/// associated domain on iOS, the Digital Asset Links of the App Links host on Android).
abstract base class DomainPasskeys implements PasskeyAuthenticator {
  /// Creates the authenticator for [domain].
  DomainPasskeys({required this.domain});

  /// The relying party the build is associated with.
  final String domain;

  /// Runs the system's assertion for the request options JSON; returns the credential JSON.
  Future<String> assertion(String requestOptionsJson);

  /// Runs the system's registration for the creation options JSON; returns the credential
  /// JSON.
  Future<String> registration(String creationOptionsJson);

  @override
  Future<Map<String, Object?>> getAssertion(
    Map<String, Object?> requestOptions,
  ) async {
    _check(requestOptions['rpId']);
    return _decode(await assertion(jsonEncode(requestOptions)));
  }

  @override
  Future<Map<String, Object?>> createCredential(
    Map<String, Object?> creationOptions,
  ) async {
    final rp = creationOptions['rp'];
    _check(rp is Map ? rp['id'] : null);
    return _decode(await registration(jsonEncode(creationOptions)));
  }

  void _check(Object? rpId) {
    if (rpId != domain) {
      throw StateError('the relying party is not the domain of the build');
    }
  }

  static Map<String, Object?> _decode(String json) =>
      (jsonDecode(json) as Map).cast<String, Object?>();
}
