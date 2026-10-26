// SPDX-License-Identifier: Apache-2.0

// The status page of the console (brief M7-21 Tests): the route `status` hosts the status page
// of admin_ui with the owner forms, reads `getStatus` through the console's cookie client and
// again every `client.status_poll_interval` (the console has no sync engine), and the public
// page `/public-status` reads `getPublicStatus` without login, says "only after login" for the
// server's 404, and is reachable from the login.

import 'package:admin_ui/admin_ui.dart';
import 'package:console/console.dart';
import 'package:domain/domain.dart' show AppConfig, AppConfigValues;
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../login_test.dart' show pumpConsole;
import '../support/fakes.dart';
import 'admin_pages_test.dart' show browserWindow, signedIn;

const String _messages = '0192f4a0-7b1c-7000-8a00-000000000301';
const String _lotse = '0192f4a0-7b1c-7000-8a00-000000000305';

Map<String, Object?> _component(String id, String name, String state) => {
  'id': id,
  'name': name,
  'description': '',
  'sort_key': '000001',
  'state': state,
  'updated_at': '2026-10-17T08:00:00.000Z',
};

Map<String, Object?> _page({bool agent = true}) => {
  'overall_state': 'degraded',
  'components': [
    _component(_messages, 'Messages', 'degraded'),
    if (agent)
      {
        ..._component(_lotse, 'Lotse', 'operational'),
        'agent_id': '0192f4a0-7b1c-7000-8a00-000000000399',
      },
  ],
  'incidents': <Object>[],
};

Finder _key(String key) => find.byKey(ValueKey(key));

void main() {
  testWidgets('the route status hosts the page with the owner forms', (
    tester,
  ) async {
    browserWindow(tester);
    final b = signedIn();
    b.server.routes['GET /api/v1/status'] = (_) => Answer(200, _page());
    final c = await pumpConsole(tester, b, location: '/status');
    expect(c.page, ConsolePage.status);
    expect(find.byType(StatusScreen), findsOneWidget);
    expect(_key('status.component.$_messages'), findsOneWidget);
    expect(_key('status.component.$_lotse'), findsOneWidget);
    expect(_key('status.addComponent'), findsOneWidget);

    // The console reads the page again after the named interval.
    final reads = b.server.to('GET', '/api/v1/status').length;
    await tester.pump(AppConfig.defaults().statusPollInterval);
    await tester.pumpAndSettle();
    expect(b.server.to('GET', '/api/v1/status').length, reads + 1);
  });

  testWidgets('the interval follows the client value of the configuration', (
    tester,
  ) async {
    browserWindow(tester);
    final c = await pumpConsole(tester, signedIn(), location: '/people');
    final admin = ConsoleAdmin(
      c,
      config: AppConfig.defaults().withOverrides({
        'client.status_poll_interval': '5s',
      }),
    );
    expect(
      (admin.statusSource as PollingStatusSource).interval,
      const Duration(seconds: 5),
    );
    expect(
      (admin.publicStatusSource as PollingStatusSource).interval,
      const Duration(seconds: 5),
    );
    admin.dispose();
    await tester.pumpWidget(const SizedBox());
    await tester.pumpAndSettle();
  });

  testWidgets('a read without an answer shows Server unreachable', (
    tester,
  ) async {
    browserWindow(tester);
    final b = signedIn();
    b.server.routes['GET /api/v1/status'] = (_) => Answer(200, _page());
    await pumpConsole(tester, b, location: '/status');
    // No answer at all: the request fails before any response, as on a lost connection.
    b.server.routes['GET /api/v1/status'] = (_) =>
        throw StateError('connection refused');
    await tester.pump(AppConfig.defaults().statusPollInterval);
    await tester.pumpAndSettle();
    expect(_key('status.unreachable'), findsOneWidget);
    expect(_key('status.component.$_messages'), findsNothing);
  });

  testWidgets('the public page without login only when the server serves it', (
    tester,
  ) async {
    browserWindow(tester);
    final b = TestBrowser();
    b.server.routes['GET /api/v1/status/public'] = (_) => const Answer(404, {
      'type': 'about:blank',
      'title': 'Not found',
      'status': 404,
      'code': 'not_found',
    });
    final c = await pumpConsole(tester, b, location: '/public-status');
    expect(c.page, ConsolePage.publicStatus);
    expect(_key('status.notFound'), findsOneWidget);
    expect(_key('status.addComponent'), findsNothing);

    await tester.tap(_key('console.publicStatus.back'));
    await tester.pumpAndSettle();
    expect(c.page, ConsolePage.login);

    b.server.routes['GET /api/v1/status/public'] = (_) =>
        Answer(200, _page(agent: false));
    await tester.tap(_key('console.login.publicStatus'));
    await tester.pumpAndSettle();
    expect(c.page, ConsolePage.publicStatus);
    expect(_key('status.component.$_messages'), findsOneWidget);
    expect(_key('status.addComponent'), findsNothing);
  });
}
