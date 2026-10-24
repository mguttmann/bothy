// SPDX-License-Identifier: Apache-2.0

// The statistics page of the console (brief M6-06 Tests): the route `stats` hosts A7 of
// admin_ui at the full width of the browser, its queries go through the console's cookie client,
// and the telemetry link of an owner opens in a new tab (noopener), never in an iframe.

import 'package:admin_ui/admin_ui.dart';
import 'package:console/console.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../login_test.dart' show pumpConsole;
import '../support/fakes.dart';
import 'admin_pages_test.dart' show browserWindow, signedIn;

const String _url = 'https://telemetry.example.com/d/bothy';
const String _agent = '0192f4a0-7b1c-7000-8a00-000000000201';

Map<String, Object?> _instance({String? url}) => {
  'instance_id': '0192f4a0-7b1c-7db7-89b9-2fe4c48dcb3e',
  'branding': {'product_name': 'Team Chat', 'version': 1},
  'locale_default': 'en',
  'spec_version': '0.2.0',
  'features': <Object>[],
  'limits': {
    'message_max_content_bytes': 40000,
    'message_max_attachments': 10,
    'file_max_upload_bytes': 104857600,
    'thread_title_max_chars': 120,
    'voice_max_duration_ms': 300000,
  },
  'telemetry_dashboard_url': ?url,
};

TestBrowser _browser({String? url}) {
  final b = signedIn();
  final series = <String, Object?>{
    'granularity': 'day',
    'buckets': [
      {
        'start': '2026-10-16T00:00:00.000Z',
        'group_key': _agent,
        'metrics': {'runs': 5.0, 'completed': 5.0},
      },
    ],
  };
  for (final path in ['usage', 'runs', 'tool-calls', 'errors']) {
    b.server.routes['GET /api/v1/stats/$path'] = (_) => Answer(200, series);
  }
  b.server.routes['GET /api/v1/stats/costs'] = (_) => const Answer(200, {
    'granularity': 'day',
    'currency': 'USD',
    'buckets': <Object>[],
  });
  b.server.routes['GET /api/v1/spaces'] = (_) =>
      const Answer(200, {'items': <Object>[]});
  b.server.routes['GET /api/v1/agents'] = (_) =>
      const Answer(200, {'items': <Object>[]});
  b.server.routes['GET /api/v1/instance'] = (_) =>
      Answer(200, _instance(url: url));
  return b;
}

void main() {
  testWidgets('the route stats renders A7 at the full width', (tester) async {
    browserWindow(tester);
    final b = _browser(url: _url);
    final c = await pumpConsole(tester, b, location: '/stats');
    expect(c.page, ConsolePage.stats);
    expect(find.byType(StatsScreen), findsOneWidget);
    final body = tester.widget<AdminBody>(find.byType(AdminBody));
    expect(body.maxWidth, isNull, reason: 'the full browser width');
    expect(find.text('Runs finished'), findsOneWidget);
    expect(b.server.to('GET', '/api/v1/stats/runs'), isNotEmpty);

    await tester.tap(find.byKey(const ValueKey('stats.telemetry')));
    expect(b.links.opened, [Uri.parse(_url)]);
  });

  testWidgets('no link without the member', (tester) async {
    browserWindow(tester);
    final b = _browser();
    await pumpConsole(tester, b, location: '/stats');
    expect(find.byType(StatsScreen), findsOneWidget);
    expect(find.byKey(const ValueKey('stats.telemetry')), findsNothing);
  });
}
