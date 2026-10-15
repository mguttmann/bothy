// SPDX-License-Identifier: Apache-2.0

// A software P-256 key for the tests: the platform signers of M4-10, M4-11 and M5-01 keep the
// key in hardware; this one keeps it in memory, so the tests can sign and verify without a
// device (brief M4-09 Design note 2).

import 'dart:convert';
import 'dart:typed_data';

import 'package:auth/auth.dart';
import 'package:crypto/crypto.dart';
import 'package:pointycastle/export.dart';

final ECDomainParameters _p256 = ECCurve_secp256r1();

/// A [DeviceKeySigner] over a P-256 key held in memory, deterministic per RFC 6979.
class SoftwareP256Signer implements DeviceKeySigner {
  /// Creates a signer from the private scalar [d] (1 < d < n).
  SoftwareP256Signer(BigInt d)
    : _private = ECPrivateKey(d, _p256),
      _public = ECPublicKey(_p256.G * d, _p256);

  /// A signer with a fixed scalar derived from [seed], so every run uses the same key.
  factory SoftwareP256Signer.seeded(int seed) {
    final digest = sha256.convert(utf8.encode('auth-test-key-$seed')).bytes;
    final d = _toBigInt(digest) % (_p256.n - BigInt.two) + BigInt.two;
    return SoftwareP256Signer(d);
  }

  final ECPrivateKey _private;
  final ECPublicKey _public;

  /// How often [sign] ran.
  int signCount = 0;

  /// When set, [sign] throws this instead of signing (a cancelled OS prompt).
  Object? failWith;

  @override
  String get alg => 'ES256';

  @override
  Map<String, String> get publicJwk => {
    'kty': 'EC',
    'crv': 'P-256',
    'x': _b64(_fixed(_public.Q!.x!.toBigInteger()!)),
    'y': _b64(_fixed(_public.Q!.y!.toBigInteger()!)),
  };

  @override
  String get thumbprint => jwkThumbprint(publicJwk);

  @override
  Future<Uint8List> sign(Uint8List signingInput) async {
    signCount++;
    final failure = failWith;
    if (failure != null) throw failure;
    final signer = ECDSASigner(SHA256Digest(), HMac(SHA256Digest(), 64))
      ..init(true, PrivateKeyParameter<ECPrivateKey>(_private));
    final sig = signer.generateSignature(signingInput) as ECSignature;
    final normalized = sig.normalize(_p256);
    return Uint8List.fromList([
      ..._fixed(normalized.r),
      ..._fixed(normalized.s),
    ]);
  }

  /// Verifies a JOSE `R || S` [signature] over [signingInput] with the public key alone.
  bool verify(Uint8List signingInput, Uint8List signature) =>
      verifyWithJwk(publicJwk, signingInput, signature);
}

/// Verifies an ES256 [signature] (`R || S`) over [signingInput] with the public [jwk].
bool verifyWithJwk(
  Map<String, Object?> jwk,
  Uint8List signingInput,
  Uint8List signature,
) {
  if (signature.length != 64) return false;
  final x = _toBigInt(base64Url.decode(_pad(jwk['x']! as String)));
  final y = _toBigInt(base64Url.decode(_pad(jwk['y']! as String)));
  final q = _p256.curve.createPoint(x, y);
  final verifier = ECDSASigner(SHA256Digest())
    ..init(false, PublicKeyParameter<ECPublicKey>(ECPublicKey(q, _p256)));
  return verifier.verifySignature(
    signingInput,
    ECSignature(
      _toBigInt(signature.sublist(0, 32)),
      _toBigInt(signature.sublist(32)),
    ),
  );
}

/// Decodes unpadded base64url.
Uint8List b64Decode(String text) => base64Url.decode(_pad(text));

String _pad(String s) => s.padRight((s.length + 3) ~/ 4 * 4, '=');

String _b64(List<int> bytes) => base64Url.encode(bytes).replaceAll('=', '');

BigInt _toBigInt(List<int> bytes) {
  var result = BigInt.zero;
  for (final b in bytes) {
    result = (result << 8) | BigInt.from(b);
  }
  return result;
}

List<int> _fixed(BigInt value) {
  final out = List<int>.filled(32, 0);
  var v = value;
  for (var i = 31; i >= 0; i--) {
    out[i] = (v & BigInt.from(0xff)).toInt();
    v = v >> 8;
  }
  return out;
}
