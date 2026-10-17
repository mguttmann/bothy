// SPDX-License-Identifier: Apache-2.0

// The fake checks DPoP proofs as the server does (brief M4-17 Design note 13; 08 7.3): a wrong
// signature, a reused jti, a wrong htu, a stale or missing nonce, a key that is not the bound
// one and a Bearer token are rejected with 401 and the code of 06 5; the gateway closes with
// 4004.

import 'package:test/test.dart';

import 'support.dart';

void main() {
  late FakeWorldHarness h;
  late RawClient raw;
  late String token;
  late String me;

  setUp(() async {
    h = await FakeWorldHarness.start();
    final device = h.device();
    final s = await signedIn(h, device);
    token = s.token;
    raw = RawClient(h.server.baseUri, device.key);
    me = '${h.server.baseUri.origin}/api/v1/me';
  });
  tearDown(() async {
    raw.close();
    await h.stop();
  });

  Future<RawResponse> getMe(String proof, {String scheme = 'DPoP'}) => raw.send(
    'GET',
    '/api/v1/me',
    headers: {'Authorization': '$scheme $token', 'DPoP': proof},
  );

  Future<String> proof({
    String? htu,
    String? nonce,
    String? jti,
    SoftwareP256Signer? signWith,
    String method = 'GET',
  }) => raw.proof(
    method: method,
    htu: htu ?? me,
    accessToken: token,
    nonce: nonce ?? h.server.verifier.currentNonce,
    jti: jti,
    signWith: signWith,
  );

  test('a valid proof is accepted', () async {
    final r = await getMe(await proof());
    expect(r.status, 200);
    expect(h.server.violations, isEmpty);
  });

  test('a wrong signature is rejected with invalid_dpop_proof', () async {
    final r = await getMe(await proof(signWith: SoftwareP256Signer.seeded(77)));
    expect(r.status, 401);
    expect(r.code, 'invalid_dpop_proof');
    expect(
      r.headers.value('www-authenticate'),
      'DPoP error="invalid_dpop_proof"',
    );
  });

  test('a reused jti is rejected', () async {
    expect((await getMe(await proof(jti: 'once'))).status, 200);
    final second = await getMe(await proof(jti: 'once'));
    expect(second.status, 401);
    expect(second.code, 'invalid_dpop_proof');
  });

  test('a wrong htu or htm is rejected', () async {
    final htu = await getMe(
      await proof(htu: '${h.server.baseUri.origin}/api/v1/spaces'),
    );
    expect(htu.code, 'invalid_dpop_proof');
    final htm = await getMe(await proof(method: 'POST'));
    expect(htm.code, 'invalid_dpop_proof');
  });

  test('a missing or stale nonce gets use_dpop_nonce with the current nonce; the previous one '
      'is still accepted', () async {
    final missing = await raw.send(
      'GET',
      '/api/v1/me',
      headers: {
        'Authorization': 'DPoP $token',
        'DPoP': await raw.proof(method: 'GET', htu: me, accessToken: token),
      },
    );
    expect(missing.status, 401);
    expect(missing.code, 'use_dpop_nonce');
    expect(missing.headers.value('dpop-nonce'), h.server.verifier.currentNonce);

    final old = h.server.verifier.currentNonce;
    h.server.verifier.rotateNonce();
    expect(
      (await getMe(await proof(nonce: old))).status,
      200,
      reason: 'previous',
    );
    h.server.verifier.rotateNonce();
    final stale = await getMe(await proof(nonce: old));
    expect(stale.status, 401);
    expect(stale.code, 'use_dpop_nonce');
    expect(stale.headers.value('dpop-nonce'), h.server.verifier.currentNonce);
  });

  test('a proof with another key than the bound one is rejected', () async {
    final other = RawClient(h.server.baseUri, SoftwareP256Signer.seeded(42));
    final r = await raw.send(
      'GET',
      '/api/v1/me',
      headers: {
        'Authorization': 'DPoP $token',
        'DPoP': await other.proof(
          method: 'GET',
          htu: me,
          accessToken: token,
          nonce: h.server.verifier.currentNonce,
        ),
      },
    );
    other.close();
    expect(r.code, 'invalid_dpop_proof');
  });

  test(
    'a proof for another token (ath) and a Bearer token are rejected',
    () async {
      final wrongAth = await raw.send(
        'GET',
        '/api/v1/me',
        headers: {
          'Authorization': 'DPoP $token',
          'DPoP': await raw.proof(
            method: 'GET',
            htu: me,
            accessToken: 'another-token',
            nonce: h.server.verifier.currentNonce,
          ),
        },
      );
      expect(wrongAth.code, 'invalid_dpop_proof');
      final bearer = await getMe(await proof(), scheme: 'Bearer');
      expect(bearer.code, 'invalid_dpop_proof');
    },
  );

  test('the gateway closes with 4004 for a bad proof in identify', () async {
    final device = h.server.world.sessions.values.single.deviceId;
    final stale = await RawGateway.connect(h.server.gatewayUri, raw);
    await stale.identify(token, device, nonce: 'not-a-nonce');
    await stale.waitClosed();
    expect(stale.closeCode, 4004);

    final wrongHtu = await RawGateway.connect(h.server.gatewayUri, raw);
    await wrongHtu.identify(
      token,
      device,
      htu: '${h.server.baseUri.origin}/api/v1/me',
    );
    await wrongHtu.waitClosed();
    expect(wrongHtu.closeCode, 4004);

    final good = await RawGateway.connect(h.server.gatewayUri, raw);
    await good.identify(token, device);
    await good.waitFor('ready');
    await good.close();
    expect(h.server.violations, isEmpty);
  });
}
