// SPDX-License-Identifier: Apache-2.0

// The DPoP contract between the Dart client and ClientKit (brief M5-04 Design note 2): the
// Dart `DpopProofFactory` builds the signing input of the shared vector, and the Swift test
// `DPoPProofTests.testProofMatchesDartProfile` builds the same bytes from the same file.

import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:auth/auth.dart';
import 'package:domain/domain.dart';
import 'package:flutter_test/flutter_test.dart';

/// The vector both languages read.
const String vectorPath =
    'ios/ClientKit/Tests/ClientKitTests/Fixtures/dpop_vector.json';

/// A signer with the vector's public key; the signature is not part of the contract (ECDSA is
/// randomized), so it returns 64 zero bytes.
final class _VectorSigner implements DeviceKeySigner {
  _VectorSigner(this.x, this.y);

  final String x;
  final String y;

  @override
  String get alg => 'ES256';

  @override
  Map<String, String> get publicJwk => {
    'crv': 'P-256',
    'kty': 'EC',
    'x': x,
    'y': y,
  };

  @override
  String get thumbprint => jwkThumbprint(publicJwk);

  @override
  Future<Uint8List> sign(Uint8List signingInput) async => Uint8List(64);
}

final class _FixedClock implements Clock {
  _FixedClock(this.time);

  final DateTime time;

  @override
  DateTime now() => time;

  @override
  Future<void> delay(Duration duration) async {}
}

void main() {
  test(
    'the Dart proof of the shared vector has the recorded signing input',
    () async {
      final vector = jsonDecode(
        File(vectorPath).readAsStringSync(),
      ) as Map<String, Object?>;
      final iat = vector['iat']! as int;
      final nonces = NonceStore()..update(vector['nonce']! as String);
      // A fixed clock and a random source of zeros make the `jti` fixed; the vector records it
      // and the Swift test passes it in.
      final ids = Uuid7Generator(nowMillis: () => iat * 1000, random: (_) => 0);
      final proofs = DpopProofFactory(
        _VectorSigner(vector['x']! as String, vector['y']! as String),
        nonces,
        _FixedClock(
          DateTime.fromMillisecondsSinceEpoch(iat * 1000, isUtc: true),
        ),
        ids,
      );
      final proof = await proofs.forRequest(
        method: vector['method']! as String,
        url: Uri.parse(vector['url']! as String),
        accessToken: vector['access_token']! as String,
      );
      final parts = proof.split('.');
      final payload = jsonDecode(
        utf8.decode(base64Url.decode(base64Url.normalize(parts[1]))),
      ) as Map<String, Object?>;
      expect(payload['jti'], vector['jti']);
      expect('${parts[0]}.${parts[1]}', vector['signing_input']);
    },
  );
}
