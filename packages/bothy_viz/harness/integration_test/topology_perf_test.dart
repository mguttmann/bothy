// SPDX-License-Identifier: Apache-2.0

/// The performance test of the live topology (docs/spec/25-visualization.md 7.3; brief
/// M6-12, design note 6): the `load` frame with exactly the default of `stats.live_max_runs`
/// runs, replayed every `stats.live_interval` of the fixture clock, builds a frame within
/// `viz.perf.frame_build_p95` at p95.
///
/// Runs in profile mode on macOS through `platform-tests.json` (`make apps-platform`) and
/// `make perf`: `flutter drive --profile -d macos --driver=test_driver/integration_test.dart
/// --target=integration_test/topology_perf_test.dart`.
library;

import 'package:api_client/api_client.dart' as api;
import 'package:bothy_viz/bothy_viz.dart';
import 'package:bothy_viz_harness/main.dart' show harnessNames;
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter/scheduler.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';
import 'package:l10n/l10n.dart';

/// Frames of the replay: at least 200 (brief M6-12, design note 6).
const int replayFrames = 200;

/// The 95th percentile of [durations] (nearest rank).
Duration p95(List<Duration> durations) {
  final sorted = [...durations]..sort();
  final rank = (sorted.length * 95 / 100).ceil();
  return sorted[rank - 1];
}

/// [frame] with every third run in the next state, so a replayed frame changes some cards
/// and keeps the others, as a live stream does.
api.StatsLiveFrame changedFrame(api.StatsLiveFrame frame) => frame.rebuild(
  (b) => b.runs.replace([
    for (final (i, run) in frame.runs.indexed)
      i % 3 == 0
          ? run.rebuild(
              (r) => r.state = r.state == api.StatsLiveRunStateEnum.running
                  ? api.StatsLiveRunStateEnum.waitingInput
                  : api.StatsLiveRunStateEnum.running,
            )
          : run,
  ]),
);

void main() {
  final binding = IntegrationTestWidgetsFlutterBinding.ensureInitialized();
  binding.framePolicy = LiveTestWidgetsFlutterBindingFramePolicy.fullyLive;

  testWidgets('the live topology of the load frame builds within '
      'viz.perf.frame_build_p95', (tester) async {
    final config = AppConfig.defaults();
    final fixtures = VizFixtures(
      (name) => rootBundle.loadString('packages/bothy_viz/fixtures/$name'),
    );
    final load = await fixtures.liveFrame('load');
    expect(load.runsTruncated, isFalse);
    final changed = changedFrame(load);
    final interval = Duration(milliseconds: load.intervalMs);
    final clock = FakeVizClock(load.at);
    final live = FixtureLiveFrameSource(
      clock: clock,
      staleAfter: config.staleAfter,
      names: harnessNames,
      steps: [
        (after: Duration.zero, frame: load),
        for (var i = 1; i <= replayFrames; i++)
          (after: interval, frame: i.isOdd ? changed : load),
      ],
    );
    final options = VizHostOptions(
      formFactor: VizFormFactor.desktop,
      scope: const VizScope.instance(),
      range: VizRange(
        from: load.at.subtract(const Duration(days: 1)),
        to: load.at,
      ),
      names: harnessNames,
      clock: clock,
    );
    await tester.pumpWidget(
      MaterialApp(
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        theme: buildDsTheme(
          const DsContext(
            theme: DsThemeMode.light,
            density: DsDensityMode.comfortable,
            motion: DsMotionMode.full,
            accent: dsDefaultAccent,
          ),
        ),
        home: Scaffold(
          body: SingleChildScrollView(
            child: LiveTopologyView(
              live: live,
              queries: FixtureStatsQuerySource(fixtures),
              options: options,
              onOpenThread: (_) {},
            ),
          ),
        ),
      ),
    );
    clock.advance(Duration.zero);
    await tester.pump();
    expect(find.byType(RunCardMark), findsWidgets);

    final builds = <Duration>[];
    void collect(List<FrameTiming> timings) =>
        builds.addAll(timings.map((t) => t.buildDuration));
    SchedulerBinding.instance.addTimingsCallback(collect);
    for (var i = 0; i < replayFrames; i++) {
      clock.advance(interval);
      await tester.pump();
    }
    // The engine reports frame timings in batches; let the last batch arrive.
    await Future<void>.delayed(const Duration(seconds: 2));
    SchedulerBinding.instance.removeTimingsCallback(collect);

    final measured = p95(builds);
    binding.reportData = {
      'frames': builds.length,
      'frame_build_p95_us': measured.inMicroseconds,
      'target_us': config.perfFrameBuildP95.inMicroseconds,
    };
    expect(builds.length, greaterThanOrEqualTo(replayFrames));
    expect(measured, lessThanOrEqualTo(config.perfFrameBuildP95));
  });
}
