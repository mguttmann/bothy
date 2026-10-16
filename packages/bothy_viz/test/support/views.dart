// SPDX-License-Identifier: Apache-2.0

import 'dart:io';

import 'package:api_client/api_client.dart' as api;
import 'package:bothy_viz/bothy_viz.dart';
import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

import 'fixtures.dart';

/// The fixture files, read once; widget tests run in a fake async zone, so the views read
/// them from memory and their futures complete in microtasks.
final Map<String, String> fixtureFiles = {
  for (final f in Directory('fixtures').listSync().whereType<File>())
    if (f.path.endsWith('.json')) f.uri.pathSegments.last: f.readAsStringSync(),
};

/// The fixtures from [fixtureFiles].
final memoryFixtures = VizFixtures((name) async {
  final text = fixtureFiles[name];
  if (text == null) throw FileSystemException('no fixture', name);
  return text;
});

/// The time of the live fixtures; the fake clock starts here.
final DateTime fixtureNow = DateTime.utc(2026, 10, 15, 10);

/// The period of the queries.
final VizRange fixtureRange = VizRange(
  from: DateTime.utc(2026, 10, 15),
  to: DateTime.utc(2026, 10, 16),
);

/// The views, their live and query sources and the fake clock of one test.
final class ViewWorld {
  /// Creates the world for the fixture [state]; [replay] frames follow the first one.
  ViewWorld({
    this.state = 'normal',
    this.scope = const VizScope.instance(),
    this.formFactor = VizFormFactor.desktop,
    this.allowTvMode = true,
    this.showCostsInTv,
    this.withLive = true,
    this.replay = const [],
  }) : clock = FakeVizClock(fixtureNow) {
    queries = FixtureStatsQuerySource(
      memoryFixtures,
      state: state,
      available: fixtureFiles.keys.toSet(),
    );
  }

  /// The fixture state.
  final String state;

  /// Instance or space.
  final VizScope scope;

  /// Desktop, mobile or web.
  final VizFormFactor formFactor;

  /// The host offers TV mode.
  final bool allowTvMode;

  /// `viz.tv_show_costs` override.
  final bool? showCostsInTv;

  /// Whether the views get a live source.
  final bool withLive;

  /// Further frames after the first, each one `stats.live_interval` later.
  final List<api.StatsLiveFrame> replay;

  /// The clock of everything.
  final FakeVizClock clock;

  /// The REST source.
  late final FixtureStatsQuerySource queries;

  /// The live source, created by [load].
  FixtureLiveFrameSource? live;

  /// The opened threads.
  final List<String> opened = [];

  /// The live fixture of [state]: its own file, else `normal`.
  String get frameState =>
      fixtureFiles.containsKey('StatsLiveFrame.$state.json') ? state : 'normal';

  /// Decodes the frame of [state] (or `normal`) and creates the live source.
  Future<void> load() async {
    if (!withLive) return;
    useFrame(await memoryFixtures.liveFrame(frameState));
  }

  /// Creates the live source that replays [frame] and then [replay].
  void useFrame(api.StatsLiveFrame frame) {
    if (!withLive) return;
    final interval = Duration(milliseconds: frame.intervalMs);
    live = FixtureLiveFrameSource(
      clock: clock,
      staleAfter: AppConfig.defaults().staleAfter,
      names: names,
      steps: [
        (after: Duration.zero, frame: frame),
        for (final next in replay) (after: interval, frame: next),
      ],
    );
  }

  /// The host options of this world.
  VizHostOptions get options => VizHostOptions(
    formFactor: formFactor,
    scope: scope,
    range: fixtureRange,
    allowTvMode: allowTvMode,
    showCostsInTv: showCostsInTv,
    names: names,
    clock: clock,
  );

  /// The view of [kind].
  Widget view(VizViewKind kind) => buildVizView(
    kind,
    live: live,
    queries: queries,
    options: options,
    onOpenThread: opened.add,
  );
}

/// Pumps [child] in a themed app and delivers the first frame and the query answers.
Future<void> pumpWorld(
  WidgetTester tester,
  ViewWorld world,
  Widget child, {
  DsThemeMode theme = DsThemeMode.light,
  Locale locale = const Locale('en'),
  DsMotionMode motion = DsMotionMode.reduced,
  bool settleIcons = true,
}) async {
  await tester.pumpWidget(
    MaterialApp(
      locale: locale,
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      theme: buildDsTheme(
        DsContext(
          theme: theme,
          density: DsDensityMode.comfortable,
          motion: motion,
          accent: dsDefaultAccent,
        ),
      ),
      home: Scaffold(body: SingleChildScrollView(child: child)),
    ),
  );
  await settleWorld(tester, world, settleIcons: settleIcons);
}

/// Delivers what is due on the fake clock and lets the query futures complete.
///
/// [settleIcons] waits for the icon assets and settles; a full motion context with a spinning
/// run icon never settles, so its tests pass `false`.
Future<void> settleWorld(
  WidgetTester tester,
  ViewWorld world, {
  bool settleIcons = true,
}) async {
  for (var i = 0; i < 3; i++) {
    world.clock.advance(Duration.zero);
    await tester.pump();
  }
  if (settleIcons) await dsPumpIcons(tester);
}

/// Every text in the rendered tree below [finder] (default: everything), including the
/// semantics labels of the marks.
List<String> renderedTexts(WidgetTester tester) {
  final texts = <String>[
    for (final t in tester.widgetList<Text>(find.byType(Text)))
      t.data ?? t.textSpan?.toPlainText() ?? '',
    for (final t in tester.widgetList<RichText>(find.byType(RichText)))
      t.text.toPlainText(),
    for (final s in tester.widgetList<Semantics>(find.byType(Semantics)))
      ?s.properties.label,
  ];
  return texts;
}

/// The fixture states each view shows (the `load` frame is for the performance test).
const Map<VizViewKind, List<String>> viewStates = {
  VizViewKind.liveTopology: [
    'normal',
    'empty',
    'truncated',
    'no_cost_permission',
    'offline',
  ],
  VizViewKind.backend: [
    'normal',
    'empty',
    'no_cost_permission',
    'rate_limits',
    'offline',
  ],
  VizViewKind.delivery: ['normal', 'empty'],
  VizViewKind.executive: ['normal', 'empty', 'no_cost_permission'],
  VizViewKind.timeline: ['normal', 'empty', 'truncated', 'day_boundary'],
};
