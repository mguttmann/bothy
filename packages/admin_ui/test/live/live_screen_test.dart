// SPDX-License-Identifier: Apache-2.0

// The page "Live" (brief M6-16 Tests and Design notes 2 to 4): one tab per view, TV mode only on
// desktop and web, over the whole window, rotating, costs only with viz.tv_show_costs, ended by a
// key or a click with the tab view live again; "Open in Grafana" only with the telemetry link;
// the scope selector restarts the live source for the chosen space; a run card opens its thread.

import 'dart:io';

import 'package:admin_ui/admin_ui.dart';
import 'package:bothy_viz/bothy_viz.dart';
import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

final Map<String, String> _files = {
  for (final f in Directory(
    '../bothy_viz/fixtures',
  ).listSync().whereType<File>())
    if (f.path.endsWith('.json')) f.uri.pathSegments.last: f.readAsStringSync(),
};

final VizFixtures _fixtures = VizFixtures((name) async => _files[name]!);

final DateTime _now = DateTime.utc(2026, 10, 15, 10);

const String _support = '0192f4a0-7b1c-7000-8a00-000000000001';

/// The directory of the page: two spaces, the agent names and, when given, the telemetry link.
final class _Directory implements AdminStatsSource {
  _Directory({this.dashboard});

  final Uri? dashboard;

  @override
  Future<List<StatsSpace>> statsSpaces() async => const [
    StatsSpace(id: _support, name: 'Support'),
    StatsSpace(id: '0192f4a0-7b1c-7000-8a00-000000000002', name: 'Engineering'),
  ];

  @override
  Future<Map<String, String>> statsAgentNames() async => const {
    '0192f4a0-7b1c-7000-8a00-000000000202': 'dev-runner',
  };

  @override
  Future<StatsInstance> statsInstance() async =>
      StatsInstance(telemetryDashboardUrl: dashboard);

  @override
  dynamic noSuchMethod(Invocation invocation) =>
      throw UnimplementedError('${invocation.memberName}');
}

final class _World {
  _World({this.dashboard, this.config});

  final Uri? dashboard;
  final AppConfig? config;
  final FakeVizClock clock = FakeVizClock(_now);
  late final FixtureLiveFrameSource live;
  late final FixtureStatsQuerySource queries = FixtureStatsQuerySource(
    _fixtures,
    available: _files.keys.toSet(),
  );
  final List<String> opened = [];
  final List<Uri> external = [];

  Future<void> load() async {
    final frame = await _fixtures.liveFrame('normal');
    live = FixtureLiveFrameSource(
      clock: clock,
      staleAfter: AppConfig.defaults().staleAfter,
      loop: true,
      steps: [
        (after: Duration.zero, frame: frame),
        (after: Duration(milliseconds: frame.intervalMs), frame: frame),
      ],
    );
  }

  Widget screen(VizFormFactor formFactor) => LiveScreen(
    key: ObjectKey(this),
    formFactor: formFactor,
    live: live,
    queries: queries,
    directory: _Directory(dashboard: dashboard),
    onOpenThread: opened.add,
    openExternal: external.add,
    config: config,
    clock: clock,
  );
}

Future<_World> _pump(
  WidgetTester tester, {
  VizFormFactor formFactor = VizFormFactor.desktop,
  Uri? dashboard,
  AppConfig? config,
}) async {
  final world = _World(dashboard: dashboard, config: config);
  await tester.runAsync(world.load);
  tester.view
    ..physicalSize = const Size(1600, 1200)
    ..devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  const ds = DsContext(
    theme: DsThemeMode.light,
    density: DsDensityMode.comfortable,
    motion: DsMotionMode.reduced,
    accent: dsDefaultAccent,
  );
  await tester.pumpWidget(
    MaterialApp(
      locale: const Locale('en'),
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      theme: buildDsTheme(ds),
      builder: (context, app) => DsThemeScope(dsContext: ds, child: app!),
      home: Scaffold(body: world.screen(formFactor)),
    ),
  );
  await _settle(tester, world);
  return world;
}

Future<void> _settle(WidgetTester tester, _World world) async {
  for (var i = 0; i < 3; i++) {
    world.clock.advance(Duration.zero);
    await tester.pump();
  }
  await dsPumpIcons(tester);
}

Finder _tab(VizViewKind kind) => find.byKey(ValueKey('live.tab.${kind.name}'));

void main() {
  testWidgets('desktop: one tab per view and TV mode; the topology is live', (
    tester,
  ) async {
    final world = await _pump(tester);
    for (final kind in [
      VizViewKind.liveTopology,
      VizViewKind.backend,
      VizViewKind.delivery,
      VizViewKind.executive,
      VizViewKind.timeline,
    ]) {
      expect(_tab(kind), findsOneWidget, reason: kind.name);
    }
    expect(_tab(VizViewKind.tvMode), findsNothing);
    expect(find.byKey(const ValueKey('live.tv')), findsOneWidget);
    expect(find.byType(LiveTopologyView), findsOneWidget);
    expect(world.live.isRunning, isTrue);
    expect(world.live.spaceId, isNull, reason: 'the instance scope first');
    expect(find.byType(RunCardMark), findsWidgets);

    await tester.tap(_tab(VizViewKind.executive));
    await _settle(tester, world);
    expect(find.byType(ExecutiveView), findsOneWidget);
    await tester.tap(_tab(VizViewKind.timeline));
    await _settle(tester, world);
    expect(find.byType(TimelineView), findsOneWidget);
  });

  testWidgets('mobile: the same tabs without TV mode', (tester) async {
    await _pump(tester, formFactor: VizFormFactor.mobile);
    expect(_tab(VizViewKind.timeline), findsOneWidget);
    expect(find.byKey(const ValueKey('live.tv')), findsNothing);
  });

  testWidgets('every view has its table view', (tester) async {
    final world = await _pump(tester);
    for (final kind in [
      VizViewKind.liveTopology,
      VizViewKind.backend,
      VizViewKind.delivery,
      VizViewKind.executive,
      VizViewKind.timeline,
    ]) {
      await tester.tap(_tab(kind));
      await _settle(tester, world);
      final toggle = find.byKey(const ValueKey('viz-as-table')).first;
      await tester.ensureVisible(toggle);
      await tester.tap(toggle);
      await _settle(tester, world);
      expect(find.byType(VizTable), findsWidgets, reason: kind.name);
    }
  });

  testWidgets('a run card opens its thread', (tester) async {
    final world = await _pump(tester);
    await tester.tap(find.byType(RunCardMark).first);
    await tester.pump();
    expect(world.opened, hasLength(1));
  });

  testWidgets('TV mode covers the window, rotates, hides costs and ends on a '
      'key; the tab view is live again', (tester) async {
    final world = await _pump(tester);
    await tester.tap(find.byKey(const ValueKey('live.tv')));
    await tester.pumpAndSettle();
    await _settle(tester, world);
    expect(find.byType(TvMode), findsOneWidget);
    expect(find.byKey(const ValueKey('live.tabs')), findsNothing);
    expect(find.text('Live topology (1 of 5)'), findsOneWidget);
    expect(find.textContaining('USD'), findsNothing);
    world.clock.advance(AppConfig.defaults().tvRotationInterval);
    await tester.pump();
    await _settle(tester, world);
    expect(find.text('Backend view (2 of 5)'), findsOneWidget);
    await tester.sendKeyEvent(LogicalKeyboardKey.space);
    await tester.pumpAndSettle();
    await _settle(tester, world);
    expect(find.byType(TvMode), findsNothing);
    expect(find.byType(LiveTopologyView), findsOneWidget);
    expect(world.live.isRunning, isTrue, reason: 'started again');
  });

  testWidgets(
    'TV mode shows costs with viz.tv_show_costs and ends on a click',
    (tester) async {
      final world = await _pump(
        tester,
        config: AppConfig.defaults().withOverrides({
          'viz.tv_show_costs': 'true',
        }),
      );
      await tester.tap(find.byKey(const ValueKey('live.tv')));
      await tester.pumpAndSettle();
      await _settle(tester, world);
      expect(find.textContaining('USD'), findsWidgets);
      await tester.tapAt(const Offset(800, 600));
      await tester.pumpAndSettle();
      expect(find.byType(TvMode), findsNothing);
    },
  );

  testWidgets(
    '"Open in Grafana" only with the telemetry link, opened outside',
    (tester) async {
      await _pump(tester);
      expect(find.byKey(const ValueKey('viz-open-grafana')), findsNothing);

      final url = Uri.parse('https://grafana.example.com/d/bothy-live');
      final world = await _pump(tester, dashboard: url);
      await tester.tap(find.byKey(const ValueKey('viz-open-grafana')));
      expect(world.external, [url]);
    },
  );

  testWidgets('the scope selector restarts the live source for the space', (
    tester,
  ) async {
    final world = await _pump(tester);
    await tester.tap(find.byKey(const ValueKey('live.scope')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Support').last);
    await tester.pumpAndSettle();
    await _settle(tester, world);
    expect(world.live.spaceId, _support);
    expect(world.live.isRunning, isTrue);
    await tester.tap(_tab(VizViewKind.delivery));
    await _settle(tester, world);
    expect(find.text('Only for the owner'), findsOneWidget);
  });

  test('the queries cover the last day up to the last full minute', () {
    final range = liveQueryRangeAt(DateTime.utc(2026, 10, 15, 10, 30, 45, 120));
    expect(range.to, DateTime.utc(2026, 10, 15, 10, 30));
    expect(range.from, DateTime.utc(2026, 10, 14, 10, 30));
    final exact = liveQueryRangeAt(DateTime.utc(2026, 10, 15, 10, 30));
    expect(exact.to, DateTime.utc(2026, 10, 15, 10, 30));
  });
}
