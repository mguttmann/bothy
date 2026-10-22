// SPDX-License-Identifier: Apache-2.0

import 'dart:async';

import 'package:auth/auth.dart';
import 'package:console/console.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support/fakes.dart';

/// Pumps the console of [b] at [location] (a path below /console/) in English and settles.
Future<ConsoleController> pumpConsole(
  WidgetTester tester,
  TestBrowser b, {
  String location = '/',
}) async {
  final app = ConsoleApp(
    environment: b.environment,
    initialLocation: Uri.parse(location),
    locale: const Locale('en'),
  );
  await tester.pumpWidget(app);
  await tester.pumpAndSettle();
  return ConsoleApp.controllerOf(tester.element(find.byType(Navigator).first));
}

const _attempt = '0192f4a0-7b1c-74de-8f28-b1bfbb9fa235';
const _challenge = '0192f4a0-7b1c-747c-835d-13eca158d3c0';

void main() {
  group('E2 shows only the methods of getDiscovery', () {
    testWidgets('one OIDC provider and nothing local', (tester) async {
      final b = TestBrowser();
      await pumpConsole(tester, b);
      expect(find.text('Log in to Team Chat'), findsOneWidget);
      expect(find.text('Log in with Company SSO'), findsOneWidget);
      expect(find.text('Log in with passkey'), findsNothing);
      expect(find.text('Password and one-time code'), findsNothing);
      expect(find.text('Login name'), findsNothing);
      // No device screens in the console: no enrollment code and no pairing (E3, E5).
      expect(find.textContaining('enrollment code'), findsNothing);
      expect(find.textContaining('Pair'), findsNothing);
    });

    testWidgets('local passkeys and passwords, no OIDC', (tester) async {
      final b = TestBrowser();
      b.server.routes['GET /api/v1/instance/discovery'] = (_) =>
          Answer(200, discovery(oidc: false, local: true, password: true));
      await pumpConsole(tester, b);
      expect(find.text('Log in with Company SSO'), findsNothing);
      expect(find.text('Login name'), findsOneWidget);
      expect(find.text('Log in with passkey'), findsOneWidget);
      expect(find.text('Password and one-time code'), findsOneWidget);
    });

    testWidgets('local without passwords offers no password login', (
      tester,
    ) async {
      final b = TestBrowser();
      b.server.routes['GET /api/v1/instance/discovery'] = (_) =>
          Answer(200, discovery(oidc: false, local: true));
      await pumpConsole(tester, b);
      expect(find.text('Log in with passkey'), findsOneWidget);
      expect(find.text('Password and one-time code'), findsNothing);
    });

    testWidgets('no method at all', (tester) async {
      final b = TestBrowser();
      b.server.routes['GET /api/v1/instance/discovery'] = (_) =>
          Answer(200, discovery(oidc: false));
      await pumpConsole(tester, b);
      expect(
        find.text(
          'No login is set up for this instance. Ask the owner to set one up.',
        ),
        findsOneWidget,
      );
    });
  });

  group('OIDC login', () {
    testWidgets(
      'navigates, survives the reload and completes on the callback',
      (tester) async {
        final b = TestBrowser();
        b.server.routes['POST /api/v1/auth/oidc/begin'] = (_) => const Answer(
          200,
          {
            'login_attempt_id': _attempt,
            'authorization_url': 'https://login.example.com/authorize?state=s1',
            'expires_at': '2026-10-10T09:30:00.000Z',
          },
        );
        await pumpConsole(tester, b);
        await tester.tap(find.text('Log in with Company SSO'));
        await tester.pumpAndSettle();

        final begin = b.server.to('POST', '/api/v1/auth/oidc/begin').single;
        final body = begin.body! as Map<String, Object?>;
        expect(body['client'], 'console');
        expect(body['provider_id'], oidcProviderId);
        expect(
          body['app_redirect_uri'],
          'https://chat.example.com/console/login/callback',
        );
        expect(b.navigation.assigned, [
          Uri.parse('https://login.example.com/authorize?state=s1'),
        ]);
        final key = oidcFlowKey(OidcFlowKind.login, _attempt);
        final verifier = b.flows.read(key);
        expect(verifier, isNotNull);
        expect(body['client_challenge'], pkceS256(verifier!));

        // The redirect unloads the app: a new app in the same tab (same sessionStorage).
        final reloaded = TestBrowser(flows: b.flows);
        reloaded.server.routes['POST /api/v1/auth/oidc/complete'] = (_) =>
            Answer(200, consoleSession());
        await tester.pumpWidget(const SizedBox());
        await pumpConsole(
          tester,
          reloaded,
          location:
              '/login/callback?login_code=lc_1&login_attempt_id=$_attempt',
        );
        final complete = reloaded.server
            .to('POST', '/api/v1/auth/oidc/complete')
            .single;
        expect(complete.body, {
          'client': 'console',
          'login_code': 'lc_1',
          'client_verifier': verifier,
        });
        expect(reloaded.flows.read(key), isNull);
        // After the login the step-up screen comes first.
        expect(find.text('Confirm that it is you'), findsOneWidget);
        expect(find.text('Statistics'), findsNothing);
      },
    );

    testWidgets('an error on the callback deletes the entry', (tester) async {
      final b = TestBrowser();
      final key = oidcFlowKey(OidcFlowKind.login, _attempt);
      b.flows.write(key, 'verifier-1');
      await pumpConsole(
        tester,
        b,
        location:
            '/login/callback?error=access_denied&login_attempt_id=$_attempt',
      );
      expect(b.flows.read(key), isNull);
      expect(b.server.to('POST', '/api/v1/auth/oidc/complete'), isEmpty);
      expect(
        find.text(
          'The login was cancelled at the login service. Start it again or choose another way.',
        ),
        findsOneWidget,
      );
    });

    testWidgets('a refused completion deletes the entry', (tester) async {
      final b = TestBrowser();
      final key = oidcFlowKey(OidcFlowKind.login, _attempt);
      b.flows.write(key, 'verifier-1');
      b.server.routes['POST /api/v1/auth/oidc/complete'] = (_) =>
          const Answer(401, {'code': 'unauthenticated', 'status': 401});
      await pumpConsole(
        tester,
        b,
        location: '/login/callback?login_code=lc_1&login_attempt_id=$_attempt',
      );
      expect(b.flows.read(key), isNull);
      expect(
        find.text(
          'The login service did not confirm the login. Start the login again.',
        ),
        findsOneWidget,
      );
    });

    testWidgets('a callback without its stored flow does not complete', (
      tester,
    ) async {
      final b = TestBrowser();
      await pumpConsole(
        tester,
        b,
        location: '/login/callback?login_code=lc_1&login_attempt_id=$_attempt',
      );
      expect(b.server.to('POST', '/api/v1/auth/oidc/complete'), isEmpty);
      expect(
        find.text('The login took too long. Start it again.'),
        findsOneWidget,
      );
    });

    testWidgets('a proxy provider logs in without input', (tester) async {
      final b = TestBrowser();
      b.server.routes['GET /api/v1/instance/discovery'] = (_) =>
          Answer(200, discovery(oidc: false, trustedHeader: true));
      b.server.routes['POST /api/v1/auth/trusted-header/login'] = (_) =>
          Answer(200, consoleSession());
      await pumpConsole(tester, b);
      await tester.tap(find.text('Log in with Proxy'));
      await tester.pumpAndSettle();
      final login = b.server
          .to('POST', '/api/v1/auth/trusted-header/login')
          .single;
      expect(login.body, {'client': 'console'});
      expect(find.text('Confirm that it is you'), findsOneWidget);
    });
  });

  group('OIDC re-login of the step-up', () {
    testWidgets('navigates and completes on /step-up/callback after a reload', (
      tester,
    ) async {
      final b = TestBrowser();
      b.server.routes['GET /api/v1/console/session'] = (_) =>
          Answer(200, consoleSession(methods: ['oidc_reauth']));
      b.server.routes['POST /api/v1/auth/step-up/challenge'] = (_) =>
          const Answer(200, {
            'challenge_id': _challenge,
            'challenge': 'Zm9vYmFy',
            'expires_at': '2026-10-10T09:30:00.000Z',
          });
      b.server.routes['POST /api/v1/auth/step-up/oidc'] = (_) =>
          const Answer(200, {
            'login_attempt_id': _attempt,
            'authorization_url': 'https://login.example.com/authorize?state=s2',
            'expires_at': '2026-10-10T09:30:00.000Z',
          });
      await pumpConsole(tester, b);
      expect(find.text('Confirm that it is you'), findsOneWidget);
      await tester.tap(find.text('Log in again with Company SSO'));
      await tester.pumpAndSettle();
      final begin =
          b.server.to('POST', '/api/v1/auth/step-up/oidc').single.body!
              as Map<String, Object?>;
      expect(begin['challenge_id'], _challenge);
      expect(begin['provider_id'], oidcProviderId);
      expect(
        begin['app_redirect_uri'],
        'https://chat.example.com/console/step-up/callback',
      );
      final key = oidcFlowKey(OidcFlowKind.stepUp, _attempt);
      final verifier = b.flows.read(key)!;
      expect(begin['client_challenge'], pkceS256(verifier));
      expect(b.navigation.assigned.single.host, 'login.example.com');

      final reloaded = TestBrowser(flows: b.flows);
      var stepped = false;
      reloaded.server.routes['GET /api/v1/console/session'] = (_) =>
          Answer(200, consoleSession(stepUp: stepped));
      reloaded.server.routes['POST /api/v1/auth/step-up/oidc/complete'] = (_) {
        stepped = true;
        return const Answer(200, stepUpResult);
      };
      await tester.pumpWidget(const SizedBox());
      await pumpConsole(
        tester,
        reloaded,
        location:
            '/step-up/callback?challenge_id=$_challenge&login_code=lc_2&login_attempt_id=$_attempt',
      );
      final complete = reloaded.server
          .to('POST', '/api/v1/auth/step-up/oidc/complete')
          .single;
      expect(complete.body, {
        'login_code': 'lc_2',
        'client_verifier': verifier,
      });
      expect(complete.headers['x-console-request'], '1');
      expect(reloaded.flows.read(key), isNull);
      expect(find.text('Statistics'), findsOneWidget);
    });

    // Brief M6-04 Design note 2 (red team): the OIDC re-login of an action step-up returns to a
    // console that already had its first step-up; the callback must still complete it.
    testWidgets('/step-up/callback completes the re-login of an action step-up', (
      tester,
    ) async {
      final b = TestBrowser();
      final key = oidcFlowKey(OidcFlowKind.stepUp, _attempt);
      b.flows.write(key, 'verifier-3');
      b.server.routes['GET /api/v1/console/session'] = (_) =>
          Answer(200, consoleSession(stepUp: true, methods: ['oidc_reauth']));
      b.server.routes['POST /api/v1/auth/step-up/oidc/complete'] = (_) =>
          const Answer(200, stepUpResult);
      final c = await pumpConsole(
        tester,
        b,
        location:
            '/step-up/callback?challenge_id=$_challenge&login_code=lc_3&login_attempt_id=$_attempt',
      );
      final complete = b.server
          .to('POST', '/api/v1/auth/step-up/oidc/complete')
          .single;
      expect(complete.body, {
        'login_code': 'lc_3',
        'client_verifier': 'verifier-3',
      });
      expect(b.flows.read(key), isNull);
      expect(c.state, isA<ConsoleSignedIn>());
    });

    testWidgets('an error on /step-up/callback deletes the entry', (
      tester,
    ) async {
      final b = TestBrowser();
      final key = oidcFlowKey(OidcFlowKind.stepUp, _attempt);
      b.flows.write(key, 'verifier-2');
      b.server.routes['GET /api/v1/console/session'] = (_) =>
          Answer(200, consoleSession(methods: ['oidc_reauth']));
      await pumpConsole(
        tester,
        b,
        location:
            '/step-up/callback?challenge_id=$_challenge&error=login_failed&login_attempt_id=$_attempt',
      );
      expect(b.flows.read(key), isNull);
      expect(
        b.server.to('POST', '/api/v1/auth/step-up/oidc/complete'),
        isEmpty,
      );
      expect(
        find.text('The confirmation failed. Try again or choose another way.'),
        findsOneWidget,
      );
    });
  });

  group('passkey', () {
    testWidgets('login and step-up with a fake navigator.credentials', (
      tester,
    ) async {
      final b = TestBrowser();
      b.server.routes['GET /api/v1/instance/discovery'] = (_) =>
          Answer(200, discovery(oidc: false, local: true));
      b.server.routes['POST /api/v1/auth/passkey/begin'] = (_) =>
          const Answer(200, {
            'challenge_id': _challenge,
            'public_key': {'challenge': 'Zm9v', 'rpId': 'chat.example.com'},
            'expires_at': '2026-10-10T09:30:00.000Z',
          });
      var stepped = false;
      b.server.routes['POST /api/v1/auth/passkey/complete'] = (_) =>
          Answer(200, consoleSession());
      b.server.routes['GET /api/v1/console/session'] = (_) => stepped
          ? Answer(200, consoleSession(stepUp: true))
          : const Answer(401, {'code': 'unauthenticated', 'status': 401});
      b.server.routes['POST /api/v1/auth/step-up/challenge'] = (_) =>
          const Answer(200, {
            'challenge_id': _challenge,
            'challenge': 'YmFy',
            'passkey_options': {'challenge': 'YmFy'},
            'expires_at': '2026-10-10T09:30:00.000Z',
          });
      b.server.routes['POST /api/v1/auth/step-up/passkey'] = (_) {
        stepped = true;
        return const Answer(200, stepUpResult);
      };
      await pumpConsole(tester, b);
      await tester.enterText(find.byType(TextField).first, 'alex');
      await tester.tap(find.text('Log in with passkey'));
      await tester.pumpAndSettle();
      expect(b.server.to('POST', '/api/v1/auth/passkey/begin').single.body, {
        'login_name': 'alex',
      });
      expect(b.passkeys.calls.first, {
        'challenge': 'Zm9v',
        'rpId': 'chat.example.com',
      });
      final complete =
          b.server.to('POST', '/api/v1/auth/passkey/complete').single.body!
              as Map<String, Object?>;
      expect(complete['client'], 'console');
      expect(complete['challenge_id'], _challenge);
      expect((complete['credential']! as Map)['id'], 'Y3JlZA');
      expect(complete.containsKey('device'), isFalse);

      expect(find.text('Confirm that it is you'), findsOneWidget);
      await tester.tap(find.text('Confirm with passkey'));
      await tester.pumpAndSettle();
      expect(b.passkeys.calls.last, {'challenge': 'YmFy'});
      final stepUp = b.server.to('POST', '/api/v1/auth/step-up/passkey').single;
      expect((stepUp.body! as Map)['challenge_id'], _challenge);
      expect(stepUp.headers['x-console-request'], '1');
      expect(find.text('Statistics'), findsOneWidget);
    });

    testWidgets('a cancelled passkey shows the cause and the next step', (
      tester,
    ) async {
      final b = TestBrowser();
      b.server.routes['GET /api/v1/instance/discovery'] = (_) =>
          Answer(200, discovery(oidc: false, local: true));
      b.server.routes['POST /api/v1/auth/passkey/begin'] = (_) =>
          const Answer(200, {
            'challenge_id': _challenge,
            'public_key': {'challenge': 'Zm9v'},
            'expires_at': '2026-10-10T09:30:00.000Z',
          });
      b.passkeys.failWith = const PasskeyCancelled();
      await pumpConsole(tester, b);
      await tester.tap(find.text('Log in with passkey'));
      await tester.pumpAndSettle();
      // Without a login name the server offers discoverable credentials.
      expect(
        b.server.to('POST', '/api/v1/auth/passkey/begin').single.body,
        <String, Object?>{},
      );
      expect(b.server.to('POST', '/api/v1/auth/passkey/complete'), isEmpty);
      expect(
        find.text(
          'The passkey was not accepted. Try again or choose another way.',
        ),
        findsOneWidget,
      );
    });
  });

  group('password with the one-time code', () {
    testWidgets('login, then the step-up with password and code', (
      tester,
    ) async {
      final b = TestBrowser();
      b.server.routes['GET /api/v1/instance/discovery'] = (_) =>
          Answer(200, discovery(oidc: false, local: true, password: true));
      var stepped = false;
      b.server.routes['POST /api/v1/auth/password/login'] = (r) {
        final body = r.body! as Map;
        return body['totp_code'] == '492039'
            ? Answer(200, consoleSession(methods: ['password_totp']))
            : const Answer(401, {'code': 'unauthenticated', 'status': 401});
      };
      b.server.routes['GET /api/v1/console/session'] = (_) => stepped
          ? Answer(200, consoleSession(stepUp: true))
          : const Answer(401, {'code': 'unauthenticated', 'status': 401});
      b.server.routes['POST /api/v1/auth/step-up/challenge'] = (_) =>
          const Answer(200, {
            'challenge_id': _challenge,
            'challenge': 'YmFy',
            'expires_at': '2026-10-10T09:30:00.000Z',
          });
      b.server.routes['POST /api/v1/auth/step-up/password'] = (_) {
        stepped = true;
        return const Answer(200, stepUpResult);
      };
      await pumpConsole(tester, b);
      await tester.tap(find.text('Password and one-time code'));
      await tester.pumpAndSettle();
      await tester.enterText(
        find.widgetWithText(TextField, 'Login name'),
        'alex',
      );
      await tester.enterText(
        find.widgetWithText(TextField, 'Password'),
        'secret-1',
      );
      await tester.enterText(
        find.widgetWithText(TextField, 'One-time code'),
        '000000',
      );
      await tester.tap(find.text('Log in'));
      await tester.pumpAndSettle();
      expect(
        find.text(
          'The login failed. Check the login name, password and one-time code and try again.',
        ),
        findsOneWidget,
      );
      // A refused login is no expiry: the console stays on E2.
      expect(find.text('You are logged out.'), findsNothing);
      await tester.enterText(
        find.widgetWithText(TextField, 'One-time code'),
        '492039',
      );
      await tester.tap(find.text('Log in'));
      await tester.pumpAndSettle();
      expect(b.server.to('POST', '/api/v1/auth/password/login').last.body, {
        'client': 'console',
        'login_name': 'alex',
        'password': 'secret-1',
        'totp_code': '492039',
      });

      expect(find.text('Confirm that it is you'), findsOneWidget);
      await tester.tap(find.text('Confirm with password and one-time code'));
      await tester.pumpAndSettle();
      await tester.enterText(
        find.widgetWithText(TextField, 'Password'),
        'secret-1',
      );
      await tester.enterText(
        find.widgetWithText(TextField, 'One-time code'),
        '123456',
      );
      await tester.tap(find.text('Confirm'));
      await tester.pumpAndSettle();
      expect(b.server.to('POST', '/api/v1/auth/step-up/password').single.body, {
        'challenge_id': _challenge,
        'password': 'secret-1',
        'totp_code': '123456',
      });
      expect(find.text('Statistics'), findsOneWidget);
    });
  });

  group('session and E7', () {
    testWidgets('a reload with a stepped-up session opens the console', (
      tester,
    ) async {
      final b = TestBrowser();
      b.server.routes['GET /api/v1/console/session'] = (_) =>
          Answer(200, consoleSession(stepUp: true));
      await pumpConsole(tester, b, location: '/stats');
      expect(find.text('Signed in: Alex'), findsOneWidget);
      expect(
        find.text('This page comes with a later version of the console.'),
        findsOneWidget,
      );
    });

    testWidgets('E7 shows the reason of the expiry', (tester) async {
      final b = TestBrowser();
      var revoked = false;
      b.server.routes['GET /api/v1/console/session'] = (_) => revoked
          ? const Answer(401, {'code': 'session_revoked', 'status': 401})
          : Answer(200, consoleSession(stepUp: true));
      final controller = await pumpConsole(tester, b, location: '/people');
      revoked = true;
      unawaited(controller.refreshSession());
      await tester.pumpAndSettle();
      expect(find.text('You are logged out.'), findsOneWidget);
      expect(find.text('Reason: This session was ended'), findsOneWidget);
      await tester.tap(find.text('Log in again'));
      await tester.pumpAndSettle();
      expect(find.text('Log in to Team Chat'), findsOneWidget);
    });

    testWidgets('logout sends the CSRF header and shows E7', (tester) async {
      final b = TestBrowser();
      b.server.routes['GET /api/v1/console/session'] = (_) =>
          Answer(200, consoleSession(stepUp: true));
      b.server.routes['POST /api/v1/auth/logout'] = (_) => const Answer(204);
      await pumpConsole(tester, b, location: '/live');
      await tester.tap(find.text('Log out'));
      await tester.pumpAndSettle();
      expect(
        b.server
            .to('POST', '/api/v1/auth/logout')
            .single
            .headers['x-console-request'],
        '1',
      );
      expect(find.text('You are logged out.'), findsOneWidget);
      expect(find.text('You logged out of the console.'), findsOneWidget);
    });

    // Brief M6-04 Design note 2 (D-M6-04-2): a signed-in console answers step_up_required with
    // the step-up of the action in a dialog over the page, not with the step-up screen of the
    // login; a cancel keeps the page.
    testWidgets(
      'step_up_required while signed in asks in a dialog and keeps the page',
      (tester) async {
        final b = TestBrowser();
        var lapsed = false;
        b.server.routes['GET /api/v1/console/session'] = (_) => lapsed
            ? const Answer(401, {'code': 'step_up_required', 'status': 401})
            : Answer(200, consoleSession(stepUp: true));
        b.server.routes['POST /api/v1/auth/step-up/challenge'] = (_) =>
            const Answer(200, {
              'challenge_id': _challenge,
              'challenge': 'Zm9vYmFy',
              'passkey_options': {'challenge': 'Zm9vYmFy'},
              'expires_at': '2026-10-10T09:35:00.000Z',
            });
        final controller = await pumpConsole(tester, b, location: '/audit');
        lapsed = true;
        unawaited(controller.refreshSession());
        await tester.pumpAndSettle();
        expect(
          find.byKey(const ValueKey('consoleStepUp.dialog')),
          findsOneWidget,
        );
        expect(
          find.text('Next, your browser asks for your passkey.'),
          findsOneWidget,
        );
        await tester.tap(find.byKey(const ValueKey('consoleStepUp.cancel')));
        await tester.pumpAndSettle();
        expect(
          find.byKey(const ValueKey('consoleStepUp.dialog')),
          findsNothing,
        );
        expect(find.text('Confirm that it is you'), findsNothing);
        expect(controller.state, isA<ConsoleSignedIn>());
        expect(controller.page, ConsolePage.audit);
        expect(b.server.to('POST', '/api/v1/auth/step-up/passkey'), isEmpty);
      },
    );

    testWidgets('a session without a method says so', (tester) async {
      final b = TestBrowser();
      b.server.routes['GET /api/v1/console/session'] = (_) =>
          Answer(200, consoleSession(methods: []));
      await pumpConsole(tester, b);
      expect(
        find.text(
          'No way to confirm is set up for you. Ask the owner to set up a passkey for you.',
        ),
        findsOneWidget,
      );
    });
  });
}
