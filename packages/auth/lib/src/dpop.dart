// SPDX-License-Identifier: Apache-2.0

/// DPoP proofs after RFC 9449 for REST requests and the gateway (08 7.3, 04 6.3 item 3).
library;

import 'package:auth/src/jose.dart';
import 'package:auth/src/nonce_store.dart';
import 'package:auth/src/ports.dart';
import 'package:domain/domain.dart';

/// Builds DPoP proofs with the device key (brief M4-09 Design note 2).
///
/// Header members in the order `typ` (`dpop+jwt`), `alg`, `jwk` (public part only); payload
/// `jti` (a fresh UUIDv7), `htm`, `htu` (normalized, [normalizeHtu]), `iat` (whole seconds of
/// the clock), `ath` only when an access token is sent (RFC 9449 4.2, 4.3), and `nonce` when the
/// server issued one (RFC 9449 8). The JSON has no whitespace.
///
/// Example:
/// ```dart
/// final proofs = DpopProofFactory(keys.dpop, nonces, clock, ids);
/// final proof = await proofs.forRequest(
///   method: 'GET',
///   url: Uri.parse('https://chat.example.com/api/v1/me'),
///   accessToken: token,
/// );
/// ```
final class DpopProofFactory {
  /// Creates the factory for [key], taking nonces from [nonces], time from [clock] and the
  /// `jti` values from [ids].
  DpopProofFactory(this._key, this._nonces, this._clock, this._ids);

  final DeviceKeySigner _key;
  final NonceStore _nonces;
  final Clock _clock;
  final Uuid7Generator _ids;

  /// A proof for a REST request with [method] to [url].
  ///
  /// [accessToken] adds `ath`; leave it `null` for the login calls, which bind the new tokens
  /// to this key (06 3.1 item 3). The current nonce of the store is included when present.
  /// Throws an [ArgumentError] for a URL that is not absolute `http` or `https`, and a
  /// [StateError] when the signer returns no 64-byte `R || S` signature.
  Future<String> forRequest({
    required String method,
    required Uri url,
    String? accessToken,
  }) => _build(
    htm: method.toUpperCase(),
    htu: normalizeHtu(url),
    accessToken: accessToken,
    nonce: _nonces.current,
  );

  /// A proof for `identify`, `resume` or `reauth` of the gateway (04 6.3 items 3, 6, 7;
  /// R-SY-01): `htm` = `GET`, `htu` = the HTTPS form of [gatewayUrl] (`wss` becomes `https`,
  /// `ws` becomes `http`), `ath` of [accessToken] and the [nonce] of `hello` or
  /// `reauth_required`.
  ///
  /// Throws like [forRequest].
  Future<String> forGateway({
    required Uri gatewayUrl,
    required String accessToken,
    required String nonce,
  }) => _build(
    htm: 'GET',
    htu: normalizeHtu(gatewayUrl, gateway: true),
    accessToken: accessToken,
    nonce: nonce,
  );

  Future<String> _build({
    required String htm,
    required String htu,
    required String? accessToken,
    required String? nonce,
  }) => signCompactJws(
    signer: _key,
    header: {'typ': 'dpop+jwt', 'alg': _key.alg, 'jwk': _key.publicJwk},
    payload: {
      'jti': _ids.next(),
      'htm': htm,
      'htu': htu,
      'iat': _clock.now().millisecondsSinceEpoch ~/ 1000,
      'ath': ?(accessToken == null ? null : accessTokenHash(accessToken)),
      'nonce': ?nonce,
    },
  );

  /// The `htu` form of [url] (RFC 9449 4.2, 08 7.3): scheme and host in lower case, the
  /// default port removed (443 for `https`, 80 for `http`), query and fragment dropped, the
  /// path kept as sent (`/` when empty). With [gateway] the WebSocket schemes map to their
  /// HTTP form first (`wss` to `https`, `ws` to `http`).
  ///
  /// Throws an [ArgumentError] for any other scheme or a URL without a host.
  ///
  /// Example: `HTTPS://Chat.Example.com:443/api/v1/threads?x=1#y` gives
  /// `https://chat.example.com/api/v1/threads`.
  static String normalizeHtu(Uri url, {bool gateway = false}) {
    var scheme = url.scheme.toLowerCase();
    if (gateway) {
      scheme = switch (scheme) {
        'wss' => 'https',
        'ws' => 'http',
        _ => scheme,
      };
    }
    if ((scheme != 'https' && scheme != 'http') || url.host.isEmpty) {
      throw ArgumentError.value(
        '${url.scheme}://${url.host}',
        'url',
        'must be an absolute http or https URL',
      );
    }
    final host = url.host.toLowerCase();
    final authority = host.contains(':') ? '[$host]' : host;
    final defaultPort = scheme == 'https' ? 443 : 80;
    final port = url.hasPort && url.port != defaultPort ? ':${url.port}' : '';
    final path = url.path.isEmpty ? '/' : url.path;
    return '$scheme://$authority$port$path';
  }
}
