// SPDX-License-Identifier: Apache-2.0

// DPoP proof structure after RFC 9449 4.2 and 4.3 (brief M4-09, Tests).

import 'dart:convert';
import 'dart:typed_data';

import 'package:auth/auth.dart';
import 'package:crypto/crypto.dart';
import 'package:domain/domain.dart';
import 'package:test/test.dart';

import 'support/fakes.dart';
import 'support/software_p256_signer.dart';

({
  Map<String, Object?> header,
  Map<String, Object?> payload,
  Uint8List sig,
  String input,
})
_decode(String jws) {
  final parts = jws.split('.');
  expect(parts, hasLength(3));
  return (
    header:
        jsonDecode(utf8.decode(b64Decode(parts[0]))) as Map<String, Object?>,
    payload:
        jsonDecode(utf8.decode(b64Decode(parts[1]))) as Map<String, Object?>,
    sig: b64Decode(parts[2]),
    input: '${parts[0]}.${parts[1]}',
  );
}

void main() {
  late SoftwareP256Signer key;
  late NonceStore nonces;
  late FakeClock clock;
  late DpopProofFactory proofs;

  setUp(() {
    key = SoftwareP256Signer.seeded(1);
    nonces = NonceStore();
    clock = FakeClock(DateTime.utc(2026, 10, 15, 10, 0, 0, 750));
    var n = 0;
    proofs = DpopProofFactory(
      key,
      nonces,
      clock,
      Uuid7Generator(
        nowMillis: () => clock.now().millisecondsSinceEpoch,
        random: (bits) => (n++ * 7919) & ((1 << bits) - 1),
      ),
    );
  });

  test(
    'header has exactly typ, alg and jwk in this order, jwk without d',
    () async {
      final p = _decode(
        await proofs.forRequest(
          method: 'GET',
          url: Uri.parse('https://chat.example.com/api/v1/me'),
        ),
      );
      expect(p.header.keys.toList(), ['typ', 'alg', 'jwk']);
      expect(p.header['typ'], 'dpop+jwt');
      expect(p.header['alg'], 'ES256');
      final jwk = p.header['jwk']! as Map<String, Object?>;
      expect(jwk.keys.toSet(), {'kty', 'crv', 'x', 'y'});
      expect(jwk.containsKey('d'), isFalse);
      expect(jwk, key.publicJwk);
    },
  );

  test('JSON segments carry no whitespace', () async {
    final jws = await proofs.forRequest(
      method: 'GET',
      url: Uri.parse('https://chat.example.com/api/v1/me'),
      accessToken: 'tok',
    );
    final parts = jws.split('.');
    for (final segment in parts.take(2)) {
      expect(utf8.decode(b64Decode(segment)), isNot(contains(' ')));
    }
    expect(jws, isNot(contains('=')));
  });

  test('payload without a token has jti, htm, htu, iat and no ath', () async {
    final p = _decode(
      await proofs.forRequest(
        method: 'post',
        url: Uri.parse('https://chat.example.com/api/v1/auth/pairing/begin'),
      ),
    );
    expect(p.payload.keys.toSet(), {'jti', 'htm', 'htu', 'iat'});
    expect(p.payload['htm'], 'POST');
    expect(
      p.payload['iat'],
      DateTime.utc(2026, 10, 15, 10).millisecondsSinceEpoch ~/ 1000,
    );
    expect(p.payload['jti'], matches(RegExp(r'^[0-9a-f]{8}-[0-9a-f]{4}-7')));
  });

  test('ath is base64url(SHA-256(token)) only with a token', () async {
    const token = 'access-for-alex';
    final p = _decode(
      await proofs.forRequest(
        method: 'GET',
        url: Uri.parse('https://chat.example.com/api/v1/me'),
        accessToken: token,
      ),
    );
    final expected = base64Url
        .encode(sha256.convert(utf8.encode(token)).bytes)
        .replaceAll('=', '');
    expect(p.payload['ath'], expected);
    expect(accessTokenHash(token), expected);
  });

  test('nonce appears when the server issued one; the latest wins', () async {
    nonces.update('n-1');
    nonces.update('n-2');
    final p = _decode(
      await proofs.forRequest(
        method: 'GET',
        url: Uri.parse('https://chat.example.com/api/v1/me'),
      ),
    );
    expect(p.payload['nonce'], 'n-2');
  });

  test(
    'htu is normalized: lower case, no default port, no query and fragment',
    () async {
      final p = _decode(
        await proofs.forRequest(
          method: 'GET',
          url: Uri.parse('HTTPS://Chat.Example.com:443/api/v1/threads?x=1#y'),
        ),
      );
      expect(p.payload['htu'], 'https://chat.example.com/api/v1/threads');
      expect(
        DpopProofFactory.normalizeHtu(
          Uri.parse('http://chat.example.com:80/a'),
        ),
        'http://chat.example.com/a',
      );
      expect(
        DpopProofFactory.normalizeHtu(
          Uri.parse('https://chat.example.com:8443/a'),
        ),
        'https://chat.example.com:8443/a',
      );
      expect(
        DpopProofFactory.normalizeHtu(Uri.parse('https://[::1]:443/a')),
        'https://[::1]/a',
      );
    },
  );

  test('the signature is 64 bytes and verifies with the public JWK', () async {
    final p = _decode(
      await proofs.forRequest(
        method: 'GET',
        url: Uri.parse('https://chat.example.com/api/v1/me'),
        accessToken: 'tok',
      ),
    );
    expect(p.sig, hasLength(64));
    final jwk = p.header['jwk']! as Map<String, Object?>;
    expect(verifyWithJwk(jwk, utf8.encode(p.input), p.sig), isTrue);
    final tampered = Uint8List.fromList(p.sig)..[5] ^= 1;
    expect(verifyWithJwk(jwk, utf8.encode(p.input), tampered), isFalse);
  });

  test('two proofs have different jti', () async {
    final url = Uri.parse('https://chat.example.com/api/v1/me');
    final a = _decode(await proofs.forRequest(method: 'GET', url: url));
    final b = _decode(await proofs.forRequest(method: 'GET', url: url));
    expect(a.payload['jti'], isNot(b.payload['jti']));
  });

  test(
    'gateway form uses GET, https for wss, the given nonce and ath',
    () async {
      nonces.update('rest-nonce');
      final p = _decode(
        await proofs.forGateway(
          gatewayUrl: Uri.parse(
            'WSS://Chat.Example.com:443/api/v1/gateway?v=1',
          ),
          accessToken: 'tok',
          nonce: 'hello-nonce',
        ),
      );
      expect(p.payload['htm'], 'GET');
      expect(p.payload['htu'], 'https://chat.example.com/api/v1/gateway');
      expect(p.payload['nonce'], 'hello-nonce');
      expect(p.payload['ath'], accessTokenHash('tok'));
      expect(
        DpopProofFactory.normalizeHtu(
          Uri.parse('ws://chat.example.com:80/g'),
          gateway: true,
        ),
        'http://chat.example.com/g',
      );
    },
  );

  test('a signer that returns no 64-byte signature is rejected', () async {
    final bad = DpopProofFactory(
      _ShortSigner(key),
      nonces,
      clock,
      Uuid7Generator(nowMillis: () => 1, random: (_) => 1),
    );
    await expectLater(
      bad.forRequest(
        method: 'GET',
        url: Uri.parse('https://chat.example.com/x'),
      ),
      throwsStateError,
    );
  });

  test('JWK thumbprint follows RFC 7638 (example of section 3.1)', () {
    // RFC 7638 3.1 uses an RSA key; the members are e, kty, n in lexicographic order.
    const jwk = {
      'kty': 'RSA',
      'n': '0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw',
      'e': 'AQAB',
      'alg': 'RS256',
      'kid': '2011-04-29',
    };
    expect(jwkThumbprint(jwk), 'NzbLsXh8uDCcd-6MNwXF4W_7noWXFZAfHkxZsRGC9Xs');
  });
}

class _ShortSigner implements DeviceKeySigner {
  _ShortSigner(this._inner);
  final DeviceKeySigner _inner;
  @override
  String get alg => _inner.alg;
  @override
  Map<String, String> get publicJwk => _inner.publicJwk;
  @override
  String get thumbprint => _inner.thumbprint;
  @override
  Future<Uint8List> sign(Uint8List signingInput) async => Uint8List(10);
}
