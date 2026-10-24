// SPDX-License-Identifier: Apache-2.0

/// The iOS adapters of the login ports and of the file protection over the Pigeon channel of
/// the runner (docs/spec/15-client-architecture.md 6.3 and 8; 08 2.5 and 14.1; brief M5-02
/// Design notes 3 and 6; docs/decisions.d/M5-02.md D-M5-02-4, D-M5-02-5, D-M5-02-7).
library;

import 'dart:async';
import 'dart:convert';

import 'package:auth/auth.dart';
import 'package:flutter/services.dart';

import '../src/generated/ios_platform_api.g.dart';

export '../src/generated/ios_platform_api.g.dart' show IosPlatformApi;

/// The path of the OIDC redirect below the private-use scheme of the build.
const String oidcCallbackPath = '/oidc/callback';

/// The OIDC redirect URI of an iOS build whose bundle identifier is [bundleId].
///
/// 08 2.2 step 2 allows a private-use scheme only in reverse domain form (RFC 8252 7.1), so
/// the scheme is the bundle identifier (lower-case reverse DNS, checked by
/// `tool/branding_defines.dart`), not the deep link scheme of 15 11; the provider lists this
/// URI in `app_redirect_uris`.
///
/// Example: `oidcRedirectUri('com.example.chat')` is `com.example.chat:/oidc/callback`.
Uri oidcRedirectUri(String bundleId) =>
    Uri(scheme: bundleId, path: oidcCallbackPath);

/// [BrowserLauncher] and [RedirectReceiver] in one: one `ASWebAuthenticationSession` per
/// login (15 8), never a web view.
///
/// [open] starts the session and returns at once (the browser has the URL);
/// [waitForRedirect] waits for the callback the session returns. The session keeps the
/// browser's cookies (`prefersEphemeralWebBrowserSession` off), so the identity provider's own
/// session can be reused as RFC 8252 intends (D-M5-02-4). When the person closes the sheet the
/// redirect carries `error=cancelled`; the login then fails and E2 shows its error.
///
/// Example:
/// ```dart
/// final login = AppleWebAuthentication(api: IosPlatformApi(), bundleId: 'com.example.chat');
/// ```
final class AppleWebAuthentication
    implements BrowserLauncher, RedirectReceiver {
  /// Creates the adapter over [api] for the build with [bundleId].
  AppleWebAuthentication({
    required IosPlatformApi api,
    required String bundleId,
  }) : _api = api, // ignore: prefer_initializing_formals
       _redirect = oidcRedirectUri(bundleId);

  final IosPlatformApi _api;
  final Uri _redirect;
  Future<Uri>? _session;

  @override
  Future<Uri> redirectUri() async {
    _session = null;
    return _redirect;
  }

  @override
  Future<void> open(Uri url) async {
    final session = _api
        .authenticate(url.toString(), _redirect.scheme)
        .then(Uri.parse, onError: _cancelled);
    // A failure before the caller waits must not surface as an unhandled error.
    unawaited(session.then<void>((_) {}, onError: (Object _) {}));
    _session = session;
  }

  Uri _cancelled(Object error) {
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

/// [PasskeyAuthenticator] over `ASAuthorizationPlatformPublicKeyCredentialProvider` for the
/// relying party [associatedDomain] of the build (15 8: passkeys in native apps only with an
/// associated domain; D-M5-02-5).
///
/// Options for another relying party are refused with a [StateError] before the system is
/// asked, because the system would refuse them anyway without the matching entitlement.
final class ApplePasskeys implements PasskeyAuthenticator {
  /// Creates the authenticator over [api] for [associatedDomain].
  ApplePasskeys({required IosPlatformApi api, required this.associatedDomain})
    : _api = api; // ignore: prefer_initializing_formals

  final IosPlatformApi _api;

  /// The domain of the build's `webcredentials` entitlement.
  final String associatedDomain;

  @override
  Future<Map<String, Object?>> getAssertion(
    Map<String, Object?> requestOptions,
  ) async {
    _check(requestOptions['rpId']);
    return _decode(await _api.passkeyAssertion(jsonEncode(requestOptions)));
  }

  @override
  Future<Map<String, Object?>> createCredential(
    Map<String, Object?> creationOptions,
  ) async {
    final rp = creationOptions['rp'];
    _check(rp is Map ? rp['id'] : null);
    return _decode(await _api.passkeyRegistration(jsonEncode(creationOptions)));
  }

  void _check(Object? rpId) {
    if (rpId != associatedDomain) {
      throw StateError('the relying party is not the associated domain');
    }
  }

  static Map<String, Object?> _decode(String json) =>
      (jsonDecode(json) as Map).cast<String, Object?>();
}

/// Whether passkeys of a build with [associatedDomain] can serve the instance at [host]: the
/// relying party ID defaults to the host of `server.public_url` (08 2.5), and the system
/// accepts a relying party that is the associated domain or a domain above the host.
///
/// Example: `passkeyDomainServes('example.com', 'chat.example.com')` is true.
bool passkeyDomainServes(String associatedDomain, String host) =>
    associatedDomain.isNotEmpty &&
    (host == associatedDomain || host.endsWith('.$associatedDomain'));
