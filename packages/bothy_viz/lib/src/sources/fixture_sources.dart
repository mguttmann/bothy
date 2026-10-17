// SPDX-License-Identifier: Apache-2.0

/// Fixture implementations of the two ports, for tests and the harness.
library;

import 'package:api_client/api_client.dart' as api;

import '../fixtures/viz_fixtures.dart';
import 'live_frame_source.dart';
import 'stats_query_source.dart';
import 'viz_clock.dart';

/// One frame of a replay: `frame` arrives `after` the previous one (the first after start).
typedef ReplayStep = ({Duration after, api.StatsLiveFrame frame});

/// A [LiveFrameSource] that replays frames on its clock, without a server.
///
/// [start] schedules the [steps] one after the other on [clock]; with [loop] the replay starts
/// again after the last frame. [simulateError] emits an error event as a server would. With a
/// [FakeVizClock] the harness drives the replay through its time-lapse and the tests through
/// `advance`.
final class FixtureLiveFrameSource extends LiveFrameSourceBase {
  /// Creates the source with the replay [steps].
  FixtureLiveFrameSource({
    required super.clock,
    required super.staleAfter,
    required this.steps,
    this.loop = false,
    super.names,
  });

  /// The frames of the replay.
  final List<ReplayStep> steps;

  /// Starts the replay again after the last frame.
  final bool loop;

  VizTimer? _next;

  /// The space of the last [start]; `null` for the instance.
  String? spaceId;

  @override
  void onStart(String? spaceId) {
    this.spaceId = spaceId;
    _schedule(0);
  }

  @override
  void onStop() {
    _next?.cancel();
    _next = null;
  }

  void _schedule(int index) {
    _next?.cancel();
    if (steps.isEmpty) return;
    if (index >= steps.length) {
      if (!loop) return;
      index = 0;
    }
    final step = steps[index];
    _next = clock.schedule(step.after, () {
      emitFrame(step.frame);
      _schedule(index + 1);
    });
  }

  /// Emits the error [code] (for example `forbidden`), as a failed request of the stream would.
  void simulateError(String code) => emitError(code);
}

/// A [StatsQuerySource] that answers from the fixtures of `fixtures/`.
///
/// [state] picks the fixture state of every answer (`normal`, `empty`, `no_cost_permission`
/// and so on) where a file for it exists, else `normal`; [failWith] makes every call throw
/// the problem instead, as a server would answer with it. The filter parameters are accepted
/// and ignored: a fixture is one answer.
final class FixtureStatsQuerySource implements StatsQuerySource {
  /// Creates the source on [fixtures].
  FixtureStatsQuerySource(
    this.fixtures, {
    this.state = 'normal',
    this.available = const {},
  });

  /// The fixture loader.
  final VizFixtures fixtures;

  /// The state to answer with.
  String state;

  /// File names that exist, to fall back to `normal` for a state without a file; empty means
  /// "use [state] for every schema".
  final Set<String> available;

  /// When set, every call throws this problem.
  api.ApiProblem? failWith;

  String _pick(String prefix) {
    final exists =
        available.isEmpty || available.contains('$prefix.$state.json');
    return exists ? state : 'normal';
  }

  Future<T> _answer<T>(Future<T> Function() load) async {
    final problem = failWith;
    if (problem != null) throw problem;
    return load();
  }

  @override
  Future<api.StatsLiveFrame> live({String? spaceId}) =>
      _answer(() => fixtures.liveFrame(_pick('StatsLiveFrame')));

  @override
  Future<api.StatsTimeline> timeline({
    required DateTime from,
    required DateTime to,
    String? spaceId,
    List<api.StatsMetric>? metrics,
  }) => _answer(() => fixtures.timeline(_pick('StatsTimeline')));

  @override
  Future<api.StatsSloReport> slo() =>
      _answer(() => fixtures.sloReport(_pick('StatsSloReport')));

  @override
  Future<api.StatsBackendReport> backends({
    required DateTime from,
    required DateTime to,
    String? spaceId,
  }) => _answer(() => fixtures.backendReport(_pick('StatsBackendReport')));

  @override
  Future<api.StatsSeries> usage({
    required DateTime from,
    required DateTime to,
    api.StatsGranularity? granularity,
    String? spaceId,
  }) => _series('usage');

  @override
  Future<api.StatsSeries> runs({
    required DateTime from,
    required DateTime to,
    api.StatsGranularity? granularity,
    String? spaceId,
    String? agentId,
  }) => _series('runs');

  @override
  Future<api.StatsSeries> errors({
    required DateTime from,
    required DateTime to,
    api.StatsGranularity? granularity,
    String? spaceId,
  }) => _series('errors');

  @override
  Future<api.CostSeries> costs({
    required DateTime from,
    required DateTime to,
    api.StatsGranularity? granularity,
    String? groupBy,
    String? spaceId,
    String? agentId,
  }) => _answer(() => fixtures.costSeries(_pick('CostSeries')));

  /// `StatsSeries.<query>.<state>.json`; the state `empty` is shared by every query.
  Future<api.StatsSeries> _series(String query) => _answer(() {
    if (state == 'empty') return fixtures.series('empty');
    final pick = _pick('StatsSeries.$query');
    return fixtures.series(pick, query: query);
  });
}
