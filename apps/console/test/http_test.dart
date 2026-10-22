// SPDX-License-Identifier: Apache-2.0

import 'dart:async';

import 'package:api_client/api_client.dart';
import 'package:console/console.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support/fakes.dart';

void main() {
  late FakeServer server;
  late List<ApiProblem> unauthorized;
  late ApiClient client;

  setUp(() {
    server = FakeServer();
    server.routes['POST /api/v1/auth/logout'] = (_) => const Answer(204);
    server.routes['DELETE /api/v1/console/session/view-as'] = (_) =>
        const Answer(204);
    unauthorized = [];
    client = createConsoleApiClient(
      origin: Uri.parse('https://chat.example.com'),
      adapter: server,
      onUnauthorized: unauthorized.add,
    );
  });

  test(
    'state-changing requests carry X-Console-Request: 1, reads do not',
    () async {
      await client.getDiscoveryApi().getDiscovery();
      await client.getAuthApi().logout();
      await client.getAuthApi().endViewAs();
      expect(
        server.seen.first.headers.containsKey('x-console-request'),
        isFalse,
      );
      expect(
        server
            .to('POST', '/api/v1/auth/logout')
            .single
            .headers['x-console-request'],
        '1',
      );
      expect(
        server
            .to('DELETE', '/api/v1/console/session/view-as')
            .single
            .headers['x-console-request'],
        '1',
      );
    },
  );

  test('no request carries Authorization or DPoP, not even when a caller sets them', () async {
    await client.getDiscoveryApi().getDiscovery(
      headers: {'Authorization': 'DPoP abc', 'DPoP': 'proof'},
    );
    await client.getAuthApi().logout(
      headers: {'authorization': 'Bearer x', 'dpop': 'y'},
    );
    for (final r in server.seen) {
      expect(r.headers.containsKey('authorization'), isFalse, reason: r.path);
      expect(r.headers.containsKey('dpop'), isFalse, reason: r.path);
    }
  });

  test('the client talks to the origin it is served from', () async {
    await client.getDiscoveryApi().getDiscovery();
    expect(server.seen.single.path, '/api/v1/instance/discovery');
  });

  test('a 401 reports its problem', () async {
    server.routes['GET /api/v1/console/session'] = (_) =>
        const Answer(401, {'code': 'session_revoked', 'status': 401});
    await expectLater(
      client.getAuthApi().getConsoleSession(),
      throwsA(isA<DioException>()),
    );
    expect(unauthorized.single.code, 'session_revoked');
  });

  testWidgets('a 401 during a session leads to E7', (tester) async {
    final b = TestBrowser();
    var revoked = false;
    b.server.routes['GET /api/v1/console/session'] = (_) => revoked
        ? const Answer(401, {'code': 'unauthenticated', 'status': 401})
        : Answer(200, consoleSession(stepUp: true));
    await tester.pumpWidget(
      ConsoleApp(
        environment: b.environment,
        initialLocation: Uri.parse('/live'),
        locale: const Locale('en'),
      ),
    );
    await tester.pumpAndSettle();
    revoked = true;
    unawaited(
      ConsoleApp.controllerOf(tester.element(find.byType(Navigator).first))
          .refreshSession(),
    );
    await tester.pumpAndSettle();
    expect(find.text('You are logged out.'), findsOneWidget);
    expect(find.text('Reason: Please sign in'), findsOneWidget);
  });
}
