// SPDX-License-Identifier: Apache-2.0

// Software P-256 device keys for the widget and integration tests, the pattern of
// packages/sync_engine/test/support (test folders cannot be shared between packages): the
// platform keys live in hardware; these live in memory, so no test touches the keychain or the
// credential store of the host.

import 'dart:convert';
import 'dart:typed_data';

import 'package:auth/auth.dart';
import 'package:client/platform/device_key_signer.dart';
import 'package:crypto/crypto.dart';
import 'package:pointycastle/export.dart';

final ECDomainParameters _p256 = ECCurve_secp256r1();

/// A [DeviceKeySigner] over a P-256 key held in memory, deterministic per RFC 6979.
class SoftwareP256Signer implements DeviceKeySigner {
  /// A signer with a fixed scalar derived from [seed].
  factory SoftwareP256Signer.seeded(String seed) {
    final digest = sha256.convert(utf8.encode('client-test-key-$seed')).bytes;
    return SoftwareP256Signer._(
      _toBigInt(digest) % (_p256.n - BigInt.two) + BigInt.two,
    );
  }

  SoftwareP256Signer._(BigInt d)
    : _private = ECPrivateKey(d, _p256),
      _public = ECPublicKey(_p256.G * d, _p256);

  final ECPrivateKey _private;
  final ECPublicKey _public;

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
    final signer = ECDSASigner(SHA256Digest(), HMac(SHA256Digest(), 64))
      ..init(true, PrivateKeyParameter<ECPrivateKey>(_private));
    final sig = (signer.generateSignature(signingInput) as ECSignature)
        .normalize(_p256);
    return Uint8List.fromList([..._fixed(sig.r), ..._fixed(sig.s)]);
  }
}

/// A [DeviceKeyStore] with software keys: a new pair per scope and generation, so the keys
/// after a wipe differ from the keys before it.
class SoftwareKeyStore implements DeviceKeyStore {
  final Map<String, DeviceKeys> _keys = {};
  int _generation = 0;

  /// The scopes that currently hold keys.
  Set<String> get scopes => _keys.keys.toSet();

  /// How often [delete] ran.
  int deletes = 0;

  @override
  Future<DeviceKeys> ensure(String scope) async => _keys[scope] ??= () {
    final g = _generation++;
    return DeviceKeys(
      dpop: SoftwareP256Signer.seeded('$scope/$g/dpop'),
      stepUp: SoftwareP256Signer.seeded('$scope/$g/step_up'),
    );
  }();

  @override
  Future<void> delete(String scope) async {
    deletes++;
    _keys.remove(scope);
  }
}

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
