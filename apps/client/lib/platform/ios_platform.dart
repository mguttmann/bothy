// SPDX-License-Identifier: Apache-2.0

/// The iOS adapters of the login ports and of the file protection over the Pigeon channel of
/// the runner (docs/spec/15-client-architecture.md 6.3 and 8; 08 2.5 and 14.1; brief M5-02
/// Design notes 3 and 6; docs/decisions.d/M5-02.md D-M5-02-4, D-M5-02-5, D-M5-02-7).
library;

import '../src/generated/ios_platform_api.g.dart';
import 'browser_session.dart';

export '../src/generated/ios_platform_api.g.dart' show IosPlatformApi;
export 'browser_session.dart' show oidcCallbackPath, oidcRedirectUri;

/// [BrowserSessionLogin] over one `ASWebAuthenticationSession` per login (15 8), never a web
/// view.
///
/// The session keeps the browser's cookies (`prefersEphemeralWebBrowserSession` off), so the
/// identity provider's own session can be reused as RFC 8252 intends (D-M5-02-4). When the
/// person closes the sheet the redirect carries `error=cancelled`.
///
/// Example:
/// ```dart
/// final login = AppleWebAuthentication(api: IosPlatformApi(), bundleId: 'com.example.chat');
/// ```
final class AppleWebAuthentication extends BrowserSessionLogin {
  /// Creates the adapter over [api] for the build with [bundleId].
  AppleWebAuthentication({
    required IosPlatformApi api,
    required String bundleId,
  }) : _api = api, // ignore: prefer_initializing_formals
       super(appId: bundleId);

  final IosPlatformApi _api;

  @override
  Future<String> runSession(String url, String callbackScheme) =>
      _api.authenticate(url, callbackScheme);
}

/// [DomainPasskeys] over `ASAuthorizationPlatformPublicKeyCredentialProvider` for the relying
/// party [associatedDomain] of the build (15 8: passkeys in native apps only with an
/// associated domain; D-M5-02-5).
final class ApplePasskeys extends DomainPasskeys {
  /// Creates the authenticator over [api] for [associatedDomain].
  ApplePasskeys({required IosPlatformApi api, required String associatedDomain})
    : _api = api, // ignore: prefer_initializing_formals
      super(domain: associatedDomain);

  final IosPlatformApi _api;

  /// The domain of the build's `webcredentials` entitlement.
  String get associatedDomain => domain;

  @override
  Future<String> assertion(String requestOptionsJson) =>
      _api.passkeyAssertion(requestOptionsJson);

  @override
  Future<String> registration(String creationOptionsJson) =>
      _api.passkeyRegistration(creationOptionsJson);
}

/// Whether passkeys of a build with [associatedDomain] can serve the instance at [host]: the
/// relying party ID defaults to the host of `server.public_url` (08 2.5), and the system
/// accepts a relying party that is the associated domain or a domain above the host.
///
/// Example: `passkeyDomainServes('example.com', 'chat.example.com')` is true.
bool passkeyDomainServes(String associatedDomain, String host) =>
    associatedDomain.isNotEmpty &&
    (host == associatedDomain || host.endsWith('.$associatedDomain'));
