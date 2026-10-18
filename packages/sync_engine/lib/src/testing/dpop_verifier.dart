// SPDX-License-Identifier: Apache-2.0

/// The DPoP check of the fake server (brief M4-17 Design note 13, 08 7.3, RFC 9449): it
/// rejects bad proofs as the server would.
library;

import 'dart:convert';
import 'dart:typed_data';

import 'package:crypto/crypto.dart';
import 'package:pointycastle/export.dart';

import 'world.dart' show JsonMap;

/// The outcome of [DpopVerifier.verify].
sealed class DpopResult {
  const DpopResult();
}

/// The proof is valid; [jkt] is the RFC 7638 thumbprint of its key.
final class DpopAccepted extends DpopResult {
  /// Creates the result.
  const DpopAccepted(this.jkt, {required this.previousNonce});

  /// The thumbprint of the proof's `jwk`.
  final String jkt;

  /// Whether the proof used the previous nonce; the response then carries the current one.
  final bool previousNonce;
}

/// The proof is rejected with `401` and [code] (`use_dpop_nonce` or `invalid_dpop_proof`,
/// 06 5); [detail] names the failed check, never the proof.
final class DpopRejected extends DpopResult {
  /// Creates the result.
  const DpopRejected(this.code, this.detail);

  /// The problem code.
  final String code;

  /// The failed check.
  final String detail;

  @override
  String toString() => 'DpopRejected($code: $detail)';
}

/// Checks DPoP proofs: signature with the header `jwk` (ES256), `typ`, `htm`, `htu`, `ath`,
/// `iat`, `nonce` and `jti` reuse; the key must be the one the token is bound to.
///
/// Nonces are opaque counters of this fake instead of the HMAC windows of 08 7.3; the current
/// and the previous nonce are accepted, and [rotateNonce] makes a new one, so a test can make
/// a nonce stale. `EdDSA` proofs are rejected (the fake checks ES256 only, README).
///
/// Example:
/// ```dart
/// final verifier = DpopVerifier(now: () => DateTime.now().toUtc());
/// final result = verifier.verify(
///   proof,
///   method: 'GET',
///   htu: 'http://127.0.0.1:4711/api/v1/me',
///   accessToken: token,
///   expectedJkt: session.jkt,
/// );
/// ```
final class DpopVerifier {
  /// Creates the verifier on [now]; [maxAge] is `auth.dpop_proof_max_age`, [leeway]
  /// `auth.clock_leeway` (08 18).
  DpopVerifier({
    required this.now,
    this.maxAge = const Duration(seconds: 60),
    this.leeway = const Duration(seconds: 30),
  });

  /// The time source.
  final DateTime Function() now;

  /// The largest age of `iat`.
  final Duration maxAge;

  /// The allowed clock skew.
  final Duration leeway;

  int _nonces = 1;
  String _current = 'nonce-1';
  String? _previous;
  final Set<String> _seen = {};

  /// The nonce the server hands out now (`DPoP-Nonce`, `hello`).
  String get currentNonce => _current;

  /// Makes a new nonce; the current one becomes the previous one, an older one is stale.
  void rotateNonce() {
    _previous = _current;
    _current = 'nonce-${++_nonces}';
  }

  /// Checks [proof] for a request with [method] to [htu] (normalized: scheme and host in
  /// lower case, default port removed, no query).
  ///
  /// [accessToken] requires a matching `ath`; [expectedJkt] requires the proof's key to be the
  /// bound one; [nonces] replaces the accepted nonces (the gateway accepts only the nonce of
  /// its `hello` or `reauth_required`).
  DpopResult verify(
    String? proof, {
    required String method,
    required String htu,
    String? accessToken,
    String? expectedJkt,
    Set<String>? nonces,
  }) {
    if (proof == null || proof.isEmpty) {
      return const DpopRejected('invalid_dpop_proof', 'no proof');
    }
    final parts = proof.split('.');
    if (parts.length != 3) {
      return const DpopRejected('invalid_dpop_proof', 'not a compact JWS');
    }
    final JsonMap header;
    final JsonMap payload;
    final Uint8List signature;
    try {
      header = _json(parts[0]);
      payload = _json(parts[1]);
      signature = _b64(parts[2]);
    } on FormatException {
      return const DpopRejected('invalid_dpop_proof', 'not decodable');
    }
    if (header['typ'] != 'dpop+jwt') {
      return const DpopRejected('invalid_dpop_proof', 'typ');
    }
    final jwk = header['jwk'];
    if (header['alg'] != 'ES256' || jwk is! JsonMap) {
      return const DpopRejected('invalid_dpop_proof', 'alg or jwk');
    }
    if (jwk.containsKey('d')) {
      return const DpopRejected('invalid_dpop_proof', 'private key in jwk');
    }
    final signingInput = Uint8List.fromList(
      ascii.encode('${parts[0]}.${parts[1]}'),
    );
    if (!_verifyEs256(jwk, signingInput, signature)) {
      return const DpopRejected('invalid_dpop_proof', 'signature');
    }
    final jkt = thumbprint(jwk);
    if (expectedJkt != null && jkt != expectedJkt) {
      return const DpopRejected(
        'invalid_dpop_proof',
        'key is not the bound key',
      );
    }
    if (payload['htm'] != method.toUpperCase()) {
      return const DpopRejected('invalid_dpop_proof', 'htm');
    }
    if (payload['htu'] != htu) {
      return const DpopRejected('invalid_dpop_proof', 'htu');
    }
    if (accessToken != null && payload['ath'] != athOf(accessToken)) {
      return const DpopRejected('invalid_dpop_proof', 'ath');
    }
    final iat = payload['iat'];
    final t = now().millisecondsSinceEpoch ~/ 1000;
    if (iat is! int ||
        iat < t - maxAge.inSeconds - leeway.inSeconds ||
        iat > t + leeway.inSeconds) {
      return const DpopRejected('invalid_dpop_proof', 'iat');
    }
    final nonce = payload['nonce'];
    final accepted = nonces ?? {_current, ?_previous};
    if (nonce is! String || !accepted.contains(nonce)) {
      return const DpopRejected('use_dpop_nonce', 'nonce missing or stale');
    }
    final jti = payload['jti'];
    if (jti is! String || jti.isEmpty) {
      return const DpopRejected('invalid_dpop_proof', 'jti');
    }
    if (!_seen.add('$htu|$jti')) {
      return const DpopRejected('invalid_dpop_proof', 'jti reused');
    }
    return DpopAccepted(
      jkt,
      previousNonce: nonces == null && nonce != _current,
    );
  }

  /// The `ath` of [accessToken]: base64url of its SHA-256 (RFC 9449 4.2).
  static String athOf(String accessToken) =>
      _encode(sha256.convert(ascii.encode(accessToken)).bytes);

  /// The payload of the compact JWS [jws] when its header is only `{"alg":"ES256"}` and the
  /// signature verifies with the EC [jwk]; null otherwise (a step-up confirmation, 08 6.2).
  static JsonMap? verifyCompactJws(JsonMap jwk, String jws) {
    final parts = jws.split('.');
    if (parts.length != 3) return null;
    try {
      final header = _json(parts[0]);
      if (header.length != 1 || header['alg'] != 'ES256') return null;
      final valid = _verifyEs256(
        jwk,
        Uint8List.fromList(ascii.encode('${parts[0]}.${parts[1]}')),
        _b64(parts[2]),
      );
      return valid ? _json(parts[1]) : null;
    } on FormatException {
      return null;
    }
  }

  /// The RFC 7638 thumbprint of an EC [jwk].
  static String thumbprint(Map<String, Object?> jwk) => _encode(
    sha256
        .convert(
          utf8.encode(
            '{"crv":"${jwk['crv']}","kty":"${jwk['kty']}",'
            '"x":"${jwk['x']}","y":"${jwk['y']}"}',
          ),
        )
        .bytes,
  );

  static final ECDomainParameters _p256 = ECCurve_secp256r1();

  static bool _verifyEs256(JsonMap jwk, Uint8List input, Uint8List signature) {
    if (jwk['kty'] != 'EC' || jwk['crv'] != 'P-256' || signature.length != 64) {
      return false;
    }
    try {
      final x = _int(_b64(jwk['x']! as String));
      final y = _int(_b64(jwk['y']! as String));
      final q = _p256.curve.createPoint(x, y);
      final verifier = ECDSASigner(SHA256Digest())
        ..init(false, PublicKeyParameter<ECPublicKey>(ECPublicKey(q, _p256)));
      return verifier.verifySignature(
        input,
        ECSignature(
          _int(signature.sublist(0, 32)),
          _int(signature.sublist(32)),
        ),
      );
    } on Object {
      return false;
    }
  }

  static JsonMap _json(String part) {
    final decoded = jsonDecode(utf8.decode(_b64(part)));
    if (decoded is! JsonMap) throw const FormatException('not an object');
    return decoded;
  }

  static Uint8List _b64(String text) =>
      base64Url.decode(text.padRight((text.length + 3) ~/ 4 * 4, '='));

  static String _encode(List<int> bytes) =>
      base64Url.encode(bytes).replaceAll('=', '');

  static BigInt _int(List<int> bytes) {
    var r = BigInt.zero;
    for (final b in bytes) {
      r = (r << 8) | BigInt.from(b);
    }
    return r;
  }
}
