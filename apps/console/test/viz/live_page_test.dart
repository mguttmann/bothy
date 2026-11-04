// SPDX-License-Identifier: Apache-2.0

// The page "Live" of the console (brief M6-16 Design note 4; 01 5.5): the route live hosts the
// views with TV mode, the live frame comes from getStatsLive through the cookie client, "Open in
// Grafana" opens a new tab only with the telemetry link, and a run card shows "Open in the app"
// with the thread ID instead of a chat route (ADR 0028).

import 'dart:convert';
import 'dart:io';

import 'package:admin_ui/admin_ui.dart';
import 'package:console/console.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../admin/admin_pages_test.dart' show browserWindow, signedIn;
import '../login_test.dart' show pumpConsole;
import '../support/fakes.dart';

const String _url = 'https://grafana.example.com/d/bothy-live';

Object _fixture(String name) => jsonDecode(
  File('../../packages/bothy_viz/fixtures/$name').readAsStringSync(),
) as Object;

Map<String, Object?> _instance({String? url}) =>
    instanceInfo(extra: {'telemetry_dashboard_url': ?url});

TestBrowser _browser({String? url}) {
  final b = signedIn();
  // An empty frame: no running run, whose icon would never let the test settle.
  b.server.routes['GET /api/v1/stats/live'] = (_) =>
      Answer(200, _fixture('StatsLiveFrame.empty.json'));
  b.server.routes['GET /api/v1/spaces'] = (_) =>
      const Answer(200, {'items': <Object>[]});
  b.server.routes['GET /api/v1/agents'] = (_) =>
      const Answer(200, {'items': <Object>[]});
  b.server.routes['GET /api/v1/instance'] = (_) =>
      Answer(200, _instance(url: url));
  return b;
}

void main() {
  testWidgets('the route live hosts the views with TV mode and polls '
      'getStatsLive', (tester) async {
    browserWindow(tester);
    final b = _browser(url: _url);
    final c = await pumpConsole(tester, b, location: '/live');
    expect(c.page, ConsolePage.live);
    expect(find.byType(LiveScreen), findsOneWidget);
    expect(find.byKey(const ValueKey('live.tv')), findsOneWidget);
    expect(find.text('No active runs'), findsWidgets);
    expect(b.server.to('GET', '/api/v1/stats/live'), isNotEmpty);

    await tester.tap(find.byKey(const ValueKey('viz-open-grafana')));
    expect(b.links.opened, [Uri.parse(_url)]);
  });

  testWidgets('no Grafana link without the telemetry link', (tester) async {
    browserWindow(tester);
    await pumpConsole(tester, _browser(), location: '/live');
    expect(find.byType(LiveScreen), findsOneWidget);
    expect(find.byKey(const ValueKey('viz-open-grafana')), findsNothing);
  });

  testWidgets('a run card says "Open in the app" with the thread ID', (
    tester,
  ) async {
    browserWindow(tester);
    await pumpConsole(tester, _browser(), location: '/live');
    const thread = '0192f4a0-7b1c-7000-8a00-000000000301';
    tester.widget<LiveScreen>(find.byType(LiveScreen)).onOpenThread(thread);
    await tester.pump();
    expect(
      find.byKey(const ValueKey('console.live.threadInApp')),
      findsOneWidget,
    );
    expect(find.textContaining(thread), findsOneWidget);
  });

  testWidgets('the run history reads listRunHistory and its message jump '
      'says "Open in the app"', (tester) async {
    browserWindow(tester);
    final b = _browser();
    b.server.routes['GET /api/v1/run-history'] = (_) =>
        Answer(200, _fixture('RunHistoryPage.normal.json'));
    await pumpConsole(tester, b, location: '/live');
    final tab = find.byKey(const ValueKey('live.tab.runHistory'));
    await tester.ensureVisible(tab);
    await tester.tap(tab);
    for (var i = 0; i < 5; i++) {
      await tester.pump(const Duration(milliseconds: 50));
    }
    expect(b.server.to('GET', '/api/v1/run-history'), isNotEmpty);
    expect(find.text('backend / Payment integration'), findsOneWidget);
    const thread = '0192f4a0-7b1c-7000-8a00-000000000301';
    tester.widget<LiveScreen>(find.byType(LiveScreen)).onOpenMessage!(
      thread,
      '0192f4a0-7b1c-7000-8a00-000000000501',
    );
    await tester.pump();
    expect(find.textContaining(thread), findsOneWidget);
  });
}
