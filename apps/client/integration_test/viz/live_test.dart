// SPDX-License-Identifier: Apache-2.0

// D12 "Live" and TV mode D13 on the desktop against the fake server with the fixtures of
// bothy_viz (brief M6-16 Tests; 25 5.1, 6, 9.1): the sidebar group opens the page, the engine
// subscribes with stats.subscribe, the frames the fake sends appear and "stale" follows when it
// stops sending; TV mode starts, rotates, hides costs by default and shows them with
// viz.tv_show_costs, and ends on a key and on a click; "Open in Grafana" appears only with the
// telemetry link and opens the system browser.

import 'dart:convert';

import 'package:bothy_viz/bothy_viz.dart';
import 'package:flutter/services.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';
import 'package:sync_engine/testing.dart';

import '../../test/support/fakes.dart' show RecordingBrowser, fastConfig;
import '../support/test_app.dart';

/// Short intervals, so stale and the rotation show within the bound of [TestApp.waitFor].
const Map<String, String> vizConfig = {
  ...fastConfig,
  'viz.stale_after': '1s',
  'viz.tv_rotation_interval': '1s',
};

Future<Map<String, Object?>> fixture(String name) async =>
    jsonDecode(await rootBundle.loadString('packages/bothy_viz/fixtures/$name'))
        as Map<String, Object?>;

/// The answers of the query operations, from the fixtures (25 7.2 item 3).
Future<void> storeFixtures(FakeWorld world) async {
  world.stats['getStatsLive'] = await fixture('StatsLiveFrame.normal.json');
  world.stats['getStatsTimeline'] = await fixture('StatsTimeline.normal.json');
  world.stats['getStatsSlo'] = await fixture('StatsSloReport.normal.json');
  world.stats['getStatsBackends'] = await fixture(
    'StatsBackendReport.normal.json',
  );
}

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  Future<TestApp> openLive(
    WidgetTester tester, {
    Map<String, String> config = vizConfig,
    String? dashboard,
  }) async {
    final app = await TestApp.start(tester, config: config);
    final world = app.fake.server.world;
    world.telemetryDashboardUrl = dashboard;
    await storeFixtures(world);
    await app.connect();
    await app.loginWithPassword();
    await app.waitFor(find.byKey(const ValueKey('column.sidebar')));
    await app.waitLive();
    await app.tap('sidebar.live');
    await app.waitFor(find.byKey(const ValueKey('live.tabs')));
    return app;
  }

  Future<void> until(
    WidgetTester tester,
    String what,
    bool Function() condition,
  ) async {
    final end = DateTime.now().add(const Duration(seconds: 20));
    while (!condition()) {
      if (DateTime.now().isAfter(end)) fail('timed out: $what');
      await tester.pump(const Duration(milliseconds: 50));
    }
  }

  testWidgets('frames from the fake, stale without them, TV mode ends on a '
      'key and a click, costs hidden', (tester) async {
    final app = await openLive(tester);
    try {
      final gateway = app.fake.server.gateway;
      await until(
        tester,
        'the subscription of the instance',
        () => gateway.connections.any(
          (c) =>
              c.open &&
              c.statsSubscription?.spaceId == null &&
              c.statsSubscription != null,
        ),
      );
      final frame = await fixture('StatsLiveFrame.normal.json');
      expect(gateway.sendStatsLive(frame), 1);
      await app.waitFor(find.byType(RunCardMark));
      expect(find.text('No new data'), findsNothing);
      // The fake sends nothing more: after viz.stale_after the view says so.
      await app.waitFor(find.text('No new data'));
      expect(find.byKey(const ValueKey('viz-open-grafana')), findsNothing);

      // TV mode: over the window, rotating, without costs; a key ends it.
      await app.tap('live.tv');
      await app.waitFor(find.byType(TvMode));
      expect(find.byKey(const ValueKey('live.tabs')), findsNothing);
      gateway.sendStatsLive(frame);
      // The backend view comes second; with viz.tv_show_costs off neither view shows a cost.
      await app.waitFor(find.textContaining('(2 of 5)'));
      expect(find.textContaining('USD'), findsNothing);
      await tester.sendKeyEvent(LogicalKeyboardKey.space);
      await until(
        tester,
        'TV mode ended by a key',
        () => find.byType(TvMode).evaluate().isEmpty,
      );
      await app.waitFor(find.byKey(const ValueKey('live.tabs')));

      // A click ends it too.
      await app.tap('live.tv');
      await app.waitFor(find.byType(TvMode));
      await tester.tapAt(const Offset(700, 500));
      await until(
        tester,
        'TV mode ended by a click',
        () => find.byType(TvMode).evaluate().isEmpty,
      );
      // The tab view subscribes again after TV mode.
      await until(
        tester,
        'the subscription after TV mode',
        () => gateway.connections.any(
          (c) => c.open && c.statsSubscription != null,
        ),
      );
      expect(app.fake.server.violations, isEmpty);
    } finally {
      await app.stop();
    }
  });

  testWidgets('costs in TV mode with viz.tv_show_costs; "Open in Grafana" with '
      'the telemetry link opens the system browser', (tester) async {
    const url = 'https://grafana.example.com/d/bothy-live';
    final app = await openLive(
      tester,
      config: {...vizConfig, 'viz.tv_show_costs': 'true'},
      dashboard: url,
    );
    try {
      final gateway = app.fake.server.gateway;
      await app.waitFor(find.byKey(const ValueKey('viz-open-grafana')));
      await app.tap('viz-open-grafana');
      final browser = app.fake.ports.browser as RecordingBrowser;
      expect(browser.opened, [Uri.parse(url)]);

      await app.tap('live.tv');
      await app.waitFor(find.byType(TvMode));
      await until(
        tester,
        'the subscription in TV mode',
        () => gateway.connections.any(
          (c) => c.open && c.statsSubscription != null,
        ),
      );
      gateway.sendStatsLive(await fixture('StatsLiveFrame.normal.json'));
      await app.waitFor(find.textContaining('USD'));
      await tester.sendKeyEvent(LogicalKeyboardKey.escape);
      await until(
        tester,
        'TV mode ended',
        () => find.byType(TvMode).evaluate().isEmpty,
      );
    } finally {
      await app.stop();
    }
  });
}
