// SPDX-License-Identifier: Apache-2.0

// The page "Sign-in" of the console (V1-14, docs/design/sign-in-methods.md): an entry of the
// navigation; a passkey through navigator.credentials.create with the server's creation options
// and the CSRF header; a cancelled browser prompt sends no completion; "Link with" sends the
// tab to the login service with the console's callback and the verifier in sessionStorage; the
// callback completes the link after the reload and the page says so; a cancel at the login
// service completes nothing.

import 'package:auth/auth.dart' show pkceS256;
import 'package:console/console.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../login_test.dart' show pumpConsole;
import '../support/fakes.dart';

const _attempt = '0192f4a0-7b1c-74de-8f28-b1bfbb9fa235';
const _challenge = '0192f4a0-7b1c-747c-835d-13eca158d3c0';
const _at = '2026-10-10T09:30:00.000Z';

Map<String, Object?> _credentials({List<Object?> passkeys = const []}) => {
  'identities': [
    {
      'id': '0192f4a0-7b1c-7dd2-83b9-528d363b749b',
      'provider_id': localProviderId,
      'provider_kind': 'local',
      'subject': 'alex',
      'linked_at': _at,
    },
  ],
  'passkeys': passkeys,
  'local_password_set': true,
  'totp_enrolled': true,
};

TestBrowser _signedIn({TestBrowser? reuse}) {
  final b = reuse ?? TestBrowser();
  b.server.routes['GET /api/v1/instance/discovery'] = (_) =>
      Answer(200, discovery(local: true, password: true));
  b.server.routes['GET /api/v1/console/session'] = (_) =>
      Answer(200, consoleSession(stepUp: true));
  b.server.routes['GET /api/v1/me/credentials'] = (_) =>
      Answer(200, _credentials());
  return b;
}

void main() {
  setUp(() {
    final view =
        TestWidgetsFlutterBinding.instance.platformDispatcher.views.first;
    view
      ..physicalSize = const Size(1600, 1400)
      ..devicePixelRatio = 1;
  });
  tearDown(
    () => TestWidgetsFlutterBinding.instance.platformDispatcher.views.first
        .reset(),
  );

  testWidgets('a passkey through the browser, and a cancelled prompt', (
    tester,
  ) async {
    final b = _signedIn();
    b.server.routes['POST /api/v1/me/passkeys/begin'] = (_) =>
        const Answer(200, {
          'challenge_id': _challenge,
          'public_key': {
            'challenge': 'Zm9v',
            'rp': {'id': 'chat.example.com', 'name': 'Team Chat'},
            'user': {'id': 'dXNlcg', 'name': 'alex', 'displayName': 'Alex'},
          },
          'expires_at': _at,
        });
    b.server.routes['POST /api/v1/me/passkeys/complete'] = (_) =>
        const Answer(201, {
          'id': '0192f4a0-7b1c-7ad4-8a99-f31c00005918',
          'name': 'Work laptop',
          'backup_eligible': false,
          'created_at': _at,
        });
    final c = await pumpConsole(tester, b);
    expect(consoleNavigation, contains(ConsolePage.signIn));
    c.open(ConsolePage.signIn);
    await tester.pumpAndSettle();
    expect(
      find.byKey(const ValueKey('signIn.password.change')),
      findsOneWidget,
    );

    await tester.tap(find.byKey(const ValueKey('signIn.passkey.add')));
    await tester.pumpAndSettle();
    await tester.enterText(
      find.byKey(const ValueKey('signIn.passkey.name')),
      'Work laptop',
    );
    await tester.pump();
    await tester.tap(find.byKey(const ValueKey('signIn.passkey.submit')));
    await tester.pumpAndSettle();
    expect(b.passkeys.created.single['rp'], {
      'id': 'chat.example.com',
      'name': 'Team Chat',
    });
    final complete = b.server.to('POST', '/api/v1/me/passkeys/complete').single;
    expect(complete.headers['x-console-request'], '1');
    expect(complete.body, {
      'challenge_id': _challenge,
      'credential': {
        'id': 'bmV3',
        'rawId': 'bmV3',
        'type': 'public-key',
        'response': {'clientDataJSON': 'e30', 'attestationObject': 'YXR0'},
        'clientExtensionResults': <String, Object?>{},
      },
      'name': 'Work laptop',
    });
    expect(find.text('The passkey Work laptop was added.'), findsOneWidget);

    b.passkeys.failWith = const PasskeyCancelled();
    await tester.tap(find.byKey(const ValueKey('signIn.passkey.add')));
    await tester.pumpAndSettle();
    await tester.enterText(
      find.byKey(const ValueKey('signIn.passkey.name')),
      'Phone',
    );
    await tester.pump();
    await tester.tap(find.byKey(const ValueKey('signIn.passkey.submit')));
    await tester.pumpAndSettle();
    expect(b.server.to('POST', '/api/v1/me/passkeys/complete'), hasLength(1));
    expect(find.text('No passkey was created.'), findsOneWidget);
  });

  testWidgets('link with a login service through the console callback', (
    tester,
  ) async {
    final b = _signedIn();
    b.server.routes['POST /api/v1/me/identities/begin'] = (_) =>
        const Answer(200, {
          'login_attempt_id': _attempt,
          'authorization_url': 'https://login.example.com/authorize?state=s9',
          'expires_at': _at,
        });
    final c = await pumpConsole(tester, b, location: '/sign-in');
    expect(c.page, ConsolePage.signIn);
    await tester.tap(find.byKey(const ValueKey('signIn.link.$oidcProviderId')));
    await tester.pumpAndSettle();
    final begin =
        b.server.to('POST', '/api/v1/me/identities/begin').single.body!
            as Map<String, Object?>;
    expect(begin['provider_id'], oidcProviderId);
    expect(
      begin['app_redirect_uri'],
      'https://chat.example.com/console/sign-in/callback',
    );
    final key = oidcFlowKey(OidcFlowKind.link, _attempt);
    final verifier = b.flows.read(key)!;
    expect(begin['client_challenge'], pkceS256(verifier));
    expect(b.navigation.assigned.single.host, 'login.example.com');

    final reloaded = _signedIn(reuse: TestBrowser(flows: b.flows));
    reloaded.server.routes['POST /api/v1/me/identities/complete'] = (_) =>
        const Answer(201, {
          'id': '0192f4a0-7b1c-7dd2-83b9-528d363b749c',
          'provider_id': oidcProviderId,
          'provider_kind': 'oidc',
          'subject': 'alex@example.com',
          'linked_at': _at,
        });
    await tester.pumpWidget(const SizedBox());
    final after = await pumpConsole(
      tester,
      reloaded,
      location: '/sign-in/callback?login_code=lc_9&login_attempt_id=$_attempt',
    );
    final complete = reloaded.server
        .to('POST', '/api/v1/me/identities/complete')
        .single;
    expect(complete.body, {'login_code': 'lc_9', 'client_verifier': verifier});
    expect(complete.headers['x-console-request'], '1');
    expect(reloaded.flows.read(key), isNull);
    expect(after.page, ConsolePage.signIn);
    expect(after.path, '/sign-in');
    expect(
      find.text('Your account is now linked with Company SSO.'),
      findsOneWidget,
    );
  });

  testWidgets('a cancel at the login service completes nothing', (
    tester,
  ) async {
    final b = _signedIn();
    b.flows
      ..write(oidcFlowKey(OidcFlowKind.link, _attempt), 'verifier-9')
      ..write(
        '${oidcFlowKey(OidcFlowKind.link, _attempt)}.provider',
        'Company SSO',
      );
    await pumpConsole(
      tester,
      b,
      location:
          '/sign-in/callback?error=access_denied&login_attempt_id=$_attempt',
    );
    expect(b.server.to('POST', '/api/v1/me/identities/complete'), isEmpty);
    expect(b.flows.read(oidcFlowKey(OidcFlowKind.link, _attempt)), isNull);
    expect(
      find.text('The link was cancelled at the login service.'),
      findsOneWidget,
    );
  });
}
