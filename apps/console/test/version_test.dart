// SPDX-License-Identifier: Apache-2.0

import 'package:console/console.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support/fakes.dart';

void main() {
  test('equal versions pass, different or missing ones do not', () {
    expect(versionsMatch(built: '0.2.0', server: '0.2.0'), isTrue);
    expect(versionsMatch(built: '0.2.0', server: '0.3.0'), isFalse);
    expect(versionsMatch(built: null, server: '0.2.0'), isFalse);
    expect(versionsMatch(built: '', server: ''), isFalse);
  });

  testWidgets('equal versions open the login', (tester) async {
    final b = TestBrowser();
    await tester.pumpWidget(
      ConsoleApp(
        environment: b.environment,
        initialLocation: Uri.parse('/'),
        locale: const Locale('en'),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text('Log in with Company SSO'), findsOneWidget);
  });

  testWidgets('a mismatch blocks actions with the reload notice', (
    tester,
  ) async {
    final b = TestBrowser(server: FakeServer(builtVersion: '0.1.0'));
    b.server.routes['GET /api/v1/console/session'] = (_) =>
        Answer(200, consoleSession(stepUp: true));
    await tester.pumpWidget(
      ConsoleApp(
        environment: b.environment,
        initialLocation: Uri.parse('/stats'),
        locale: const Locale('en'),
      ),
    );
    await tester.pumpAndSettle();
    expect(
      find.text('The console was updated. Reload the page to continue.'),
      findsOneWidget,
    );
    expect(find.text('Log in with Company SSO'), findsNothing);
    expect(find.text('Statistics'), findsNothing);
    // No call beyond discovery and version.json: actions are blocked.
    expect(b.server.to('GET', '/api/v1/console/session'), isEmpty);
    await tester.tap(find.text('Reload'));
    expect(b.navigation.reloads, 1);
  });
}
