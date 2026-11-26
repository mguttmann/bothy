// SPDX-License-Identifier: Apache-2.0

// Linking a further identity from a signed-in session (08 3.3; V1-14): beginIdentityLink with
// the redirect of the login ports and PKCE, the browser, the redirect, completeIdentityLink with
// the verifier; the step-up purpose on both calls; a cancel, an error, another attempt and no
// redirect in time complete nothing; an answer of the server passes as its DioException.

import 'dart:convert';

import 'package:api_client/api_client.dart';
import 'package:crypto/crypto.dart';
import 'package:dio/dio.dart';
import 'package:test/test.dart';

import 'support/fakes.dart';

const String _provider = '0192f4a0-7b1c-7b3c-87cd-dea30414c03e';
const String _attempt = '0192f4a0-7b1c-74de-8f28-b1bfbb9fa235';

void main() {
  late Harness h;

  setUp(() async {
    h = Harness();
    await h.signIn();
    h.server.routes['POST /me/identities/begin'] = (_) => const FakeReply(200, {
      'login_attempt_id': _attempt,
      'authorization_url': 'https://idp.example.com/authorize?client_id=c',
      'expires_at': '2026-10-15T10:10:00.000Z',
    });
    h.server.routes['POST /me/identities/complete'] = (_) =>
        const FakeReply(201, {
          'id': '0192f4a0-7b1c-7dd2-83b9-528d363b749b',
          'provider_id': _provider,
          'provider_kind': 'oidc',
          'subject': 'alex',
          'linked_at': '2026-10-15T10:01:00.000Z',
        });
  });

  Future<String?> link() => h.session.login.linkIdentity(
    _provider,
    purposeText: 'Link your account with Company login',
  );

  test('begin, browser, redirect and complete with the verifier', () async {
    h.redirects.next = Uri.parse(
      'http://127.0.0.1:49152/callback?login_code=LC-1&login_attempt_id=$_attempt',
    );
    expect(await link(), isNull);
    expect(h.browser.opened.single.host, 'idp.example.com');
    final begin = h.server.to('/me/identities/begin').single;
    final complete = h.server.to('/me/identities/complete').single;
    final beginBody = begin.body! as Map;
    final completeBody = complete.body! as Map;
    expect(beginBody['provider_id'], _provider);
    expect(beginBody['app_redirect_uri'], 'http://127.0.0.1:49152/callback');
    expect(completeBody['login_code'], 'LC-1');
    final verifier = completeBody['client_verifier']! as String;
    expect(
      beginBody['client_challenge'],
      base64Url
          .encode(sha256.convert(ascii.encode(verifier)).bytes)
          .replaceAll('=', ''),
    );
    expect(begin.headers['authorization'], startsWith('DPoP '));
  });

  test('a cancel at the login service or in the app completes nothing', () async {
    h.redirects.next = Uri.parse(
      'http://127.0.0.1:49152/callback?error=access_denied&login_attempt_id=$_attempt',
    );
    expect(await link(), 'cancelled');
    h.redirects.next = Uri.parse(
      'http://127.0.0.1:49152/callback?login_code=LC-1&login_attempt_id=$_attempt',
    );
    expect(
      await h.session.login.linkIdentity(
        _provider,
        purposeText: 'p',
        isCancelled: () => true,
      ),
      'cancelled',
    );
    expect(h.server.to('/me/identities/complete'), isEmpty);
  });

  test('an error, another attempt or no redirect in time completes nothing', () async {
    h.redirects.next = Uri.parse(
      'http://127.0.0.1:49152/callback?error=login_failed&login_attempt_id=$_attempt',
    );
    expect(await link(), 'login_failed');
    h.redirects.next = Uri.parse(
      'http://127.0.0.1:49152/callback?login_code=LC-1&login_attempt_id=other',
    );
    expect(await link(), 'login_failed');
    h.redirects.next = null;
    expect(await link(), 'flow_expired');
    expect(h.server.to('/me/identities/complete'), isEmpty);
  });

  test('an identity of another person is the 409 of the server', () async {
    h.redirects.next = Uri.parse(
      'http://127.0.0.1:49152/callback?login_code=LC-1&login_attempt_id=$_attempt',
    );
    h.server.routes['POST /me/identities/complete'] = (_) =>
        FakeReply.problem(409, 'conflict');
    await expectLater(
      link(),
      throwsA(
        isA<DioException>().having(
          (e) => ApiProblem.of(e)?.code,
          'code',
          'conflict',
        ),
      ),
    );
  });
}
