// SPDX-License-Identifier: Apache-2.0

/// The JOSE pieces the client needs, built by hand on `crypto` and base64url so that the output
/// matches the server's profile of ADR 0067 exactly: no JOSE library, no algorithm or header
/// form the server would reject (docs/DECISIONS.md D-M4-09-1).
library;

import 'dart:convert';
import 'dart:typed_data';

import 'package:auth/src/ports.dart';
import 'package:crypto/crypto.dart';

/// Encodes [bytes] as base64url without padding (RFC 7515 2, "Base64url Encoding").
///
/// Example: `base64UrlNoPad([0xfb, 0xff])` returns `-_8`.
String base64UrlNoPad(List<int> bytes) =>
    base64Url.encode(bytes).replaceAll('=', '');

/// Encodes [json] as compact JSON (no whitespace, members in insertion order) and then as
/// base64url without padding, the form of every JWS segment.
String encodeSegment(Map<String, Object?> json) =>
    base64UrlNoPad(utf8.encode(jsonEncode(json)));

/// Signs a compact JWS (RFC 7515 7.1) over [header] and [payload] with [signer].
///
/// The signing input is `BASE64URL(header) || '.' || BASE64URL(payload)` in ASCII. An ES256
/// signature must be the JOSE form `R || S` of 64 bytes (RFC 7518 3.4); any other length means
/// the signer returned DER or a wrong key and throws a [StateError] instead of sending a proof
/// the server would reject. Errors of [signer] (a cancelled OS prompt, a lost key) pass
/// through unchanged.
///
/// Example:
/// ```dart
/// final jws = await signCompactJws(
///   signer: keys.stepUp,
///   header: {'alg': 'ES256'},
///   payload: {'challenge': c, 'sid': sid, 'iat': iat},
/// );
/// ```
Future<String> signCompactJws({
  required DeviceKeySigner signer,
  required Map<String, Object?> header,
  required Map<String, Object?> payload,
}) async {
  final input = '${encodeSegment(header)}.${encodeSegment(payload)}';
  final signature = await signer.sign(Uint8List.fromList(ascii.encode(input)));
  if (signer.alg == 'ES256' && signature.length != 64) {
    throw StateError(
      'ES256 signer returned ${signature.length} bytes, expected R || S of 64 '
      '(RFC 7518 3.4)',
    );
  }
  return '$input.${base64UrlNoPad(signature)}';
}

/// The required members per key type of RFC 7638 3.2, in lexicographic order.
const Map<String, List<String>> _thumbprintMembers = {
  'EC': ['crv', 'kty', 'x', 'y'],
  'OKP': ['crv', 'kty', 'x'],
  'RSA': ['e', 'kty', 'n'],
};

/// The JWK thumbprint of [jwk] after RFC 7638: SHA-256 over the JSON of the required members
/// in lexicographic order without whitespace, as base64url without padding.
///
/// This is the `jkt` the server stores per key (08 5.1) and binds the tokens to (`cnf.jkt`).
/// Throws an [ArgumentError] for an unknown `kty` or a missing required member.
///
/// Example: for the RSA key of RFC 7638 3.1 the result is
/// `NzbLsXh8uDCcd-6MNwXF4W_7noWXFZAfHkxZsRGC9Xs`.
String jwkThumbprint(Map<String, Object?> jwk) {
  final members = _thumbprintMembers[jwk['kty']];
  if (members == null) {
    throw ArgumentError.value(jwk['kty'], 'jwk.kty', 'unsupported key type');
  }
  final canonical = <String, Object?>{};
  for (final m in members) {
    final v = jwk[m];
    if (v is! String) {
      throw ArgumentError('jwk lacks the required member "$m" (RFC 7638 3.2)');
    }
    canonical[m] = v;
  }
  return base64UrlNoPad(
    sha256.convert(utf8.encode(jsonEncode(canonical))).bytes,
  );
}

/// The `ath` value of RFC 9449 4.2: base64url of the SHA-256 of the access token (its bytes as sent).
///
/// Example: `accessTokenHash(token)` is compared by the server with the `ath` of the proof.
String accessTokenHash(String accessToken) =>
    base64UrlNoPad(sha256.convert(utf8.encode(accessToken)).bytes);
