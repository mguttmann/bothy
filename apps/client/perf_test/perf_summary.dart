// SPDX-License-Identifier: Apache-2.0

// The evaluation of the performance level of the desktop app (brief M4-34 Design note 3;
// docs/spec/15-client-architecture.md 19 and 22): the targets `client.perf.*` of the desktop from
// AppConfig, p95 over the samples by nearest rank, the jank ratio of frame timings, and the
// summary that tool/e2e_suite.sh writes to build/perf/summary.json through the driver. Pure Dart:
// the unit test test/perf/perf_summary_test.dart runs it on recorded samples.

import 'package:domain/domain.dart';

/// How a target compares: a value at most the limit, or at least it.
enum PerfComparison {
  /// The value must not be above the limit (durations, ratio, memory).
  atMost,

  /// The value must not be below the limit (the catch-up rate).
  atLeast,
}

/// How the samples of a target become its value.
enum PerfAggregate {
  /// p95 by nearest rank over at least [minSamples] samples (15 19 "p95").
  p95,

  /// The single measured value (a ratio, a rate, a peak).
  single,
}

/// The fewest samples of a p95 target (brief M4-34 Design note 3: "at least twenty runs").
const int minSamples = 20;

/// One target of 15 19 for the desktop.
final class PerfTarget {
  /// A target [name] with its [limit] in [unit]; a p95 target needs [minimumSamples] samples.
  const PerfTarget(
    this.name,
    this.limit,
    this.unit,
    this.comparison,
    this.aggregate, {
    this.minimumSamples = minSamples,
  });

  /// The configuration key, for example `client.perf.cold_start_p95`.
  final String name;

  /// The limit from AppConfig, in [unit].
  final double limit;

  /// `ms`, `ratio`, `events/s` or `bytes`.
  final String unit;

  /// Whether the value must stay at most or at least the limit.
  final PerfComparison comparison;

  /// How the samples become the value.
  final PerfAggregate aggregate;

  /// The fewest samples of a p95 target ([minSamples] unless the target names more, for
  /// example the terminal's frame to paint of brief M4-38 with at least 1000 frames).
  final int minimumSamples;
}

/// The desktop targets of 15 19 with their limits from [config] (brief M4-34 rule 9: no literal
/// target), in the order of the table.
List<PerfTarget> desktopTargets(AppConfig config) {
  double ms(Duration d) =>
      d.inMicroseconds / Duration.microsecondsPerMillisecond;
  return [
    PerfTarget(
      'client.perf.cold_start_p95',
      ms(config.perfColdStartP95),
      'ms',
      PerfComparison.atMost,
      PerfAggregate.p95,
    ),
    PerfTarget(
      'client.perf.thread_open_p95',
      ms(config.perfThreadOpenP95),
      'ms',
      PerfComparison.atMost,
      PerfAggregate.p95,
    ),
    PerfTarget(
      'client.perf.send_local_echo_p95',
      ms(config.perfSendLocalEchoP95),
      'ms',
      PerfComparison.atMost,
      PerfAggregate.p95,
    ),
    PerfTarget(
      'client.perf.patch_to_frame_p95',
      ms(config.perfPatchToFrameP95),
      'ms',
      PerfComparison.atMost,
      PerfAggregate.p95,
    ),
    PerfTarget(
      'client.perf.jank_ratio_max',
      config.perfJankRatioMax,
      'ratio',
      PerfComparison.atMost,
      PerfAggregate.single,
    ),
    PerfTarget(
      'client.perf.catchup_events_per_s_min',
      config.perfCatchupEventsPerSMin.toDouble(),
      'events/s',
      PerfComparison.atLeast,
      PerfAggregate.single,
    ),
    PerfTarget(
      'client.perf.memory_desktop_max',
      config.perfMemoryDesktopMax.toDouble(),
      'bytes',
      PerfComparison.atMost,
      PerfAggregate.single,
    ),
  ];
}

/// The 95th percentile of [samples] by nearest rank: the value at rank ceil(0.95 n) of the
/// sorted samples. Throws an [ArgumentError] for no samples.
double percentile95(List<double> samples) {
  if (samples.isEmpty) throw ArgumentError.value(samples, 'samples', 'empty');
  final sorted = [...samples]..sort();
  final rank = (0.95 * sorted.length).ceil();
  return sorted[rank - 1];
}

/// The share of [frames] (total frame times) above [budget], one refresh interval of the
/// display. Throws an [ArgumentError] for no frames.
double jankRatio(Iterable<Duration> frames, Duration budget) {
  var total = 0;
  var over = 0;
  for (final f in frames) {
    total++;
    if (f > budget) over++;
  }
  if (total == 0) throw ArgumentError.value(frames, 'frames', 'no frames');
  return over / total;
}

/// The result of one target.
final class PerfResult {
  /// The result of [target] with its [value] (null when it was not measured).
  const PerfResult(this.target, this.value, this.samples, this.pass, this.note);

  /// The target.
  final PerfTarget target;

  /// The measured value in the unit of the target, or null.
  final double? value;

  /// The number of samples behind [value].
  final int samples;

  /// Whether the target is met.
  final bool pass;

  /// Why it failed beyond the comparison (too few samples, not measured), or empty.
  final String note;

  /// The JSON of one result in summary.json.
  Map<String, Object?> toJson() => {
    'value': value,
    'limit': target.limit,
    'unit': target.unit,
    'comparison': target.comparison.name,
    'samples': samples,
    'pass': pass,
    if (note.isNotEmpty) 'note': note,
  };
}

/// The summary of one run of the performance level.
final class PerfSummary {
  PerfSummary._(this.device, this.reference, this.results, this.details);

  /// Evaluates [samples] (by target name, in the unit of the target) against [targets]. A p95
  /// target needs its [PerfTarget.minimumSamples] samples; a target without samples fails.
  /// [details] go to the summary as they are (settings and side values of a measurement).
  factory PerfSummary.evaluate({
    required List<PerfTarget> targets,
    required String device,
    required bool reference,
    required Map<String, List<double>> samples,
    Map<String, Object?> details = const {},
  }) {
    final results = <PerfResult>[];
    for (final t in targets) {
      final s = samples[t.name] ?? const <double>[];
      if (s.isEmpty) {
        results.add(PerfResult(t, null, 0, false, 'not measured'));
        continue;
      }
      if (t.aggregate == PerfAggregate.p95 && s.length < t.minimumSamples) {
        results.add(
          PerfResult(
            t,
            percentile95(s),
            s.length,
            false,
            'fewer than ${t.minimumSamples} samples',
          ),
        );
        continue;
      }
      final value = switch (t.aggregate) {
        PerfAggregate.p95 => percentile95(s),
        PerfAggregate.single => s.single,
      };
      final pass = switch (t.comparison) {
        PerfComparison.atMost => value <= t.limit,
        PerfComparison.atLeast => value >= t.limit,
      };
      results.add(PerfResult(t, value, s.length, pass, ''));
    }
    return PerfSummary._(device, reference, results, details);
  }

  /// The device the numbers come from (15 19 names the reference devices).
  final String device;

  /// Whether [device] is a reference device of 15 19 (brief M4-34 Design note 4).
  final bool reference;

  /// One result per target, in the order of the targets.
  final List<PerfResult> results;

  /// Settings and side values of the measurement, by name.
  final Map<String, Object?> details;

  /// Whether every target is met.
  bool get pass => results.every((r) => r.pass);

  /// The JSON of build/perf/summary.json.
  Map<String, Object?> toJson() => {
    'device': device,
    'reference_device': reference,
    'pass': pass,
    'results': {for (final r in results) r.target.name: r.toJson()},
    if (details.isNotEmpty) 'details': details,
  };
}

/// Merges the summary [next] of a later part of the performance level into [previous] (`make
/// perf-desktop` writes the chat targets first and the terminal targets of brief M4-38 after
/// them into the same build/perf/summary.json): the results and details of both, the device of
/// [next], and a pass only when both passed.
Map<String, Object?> mergeSummaries(
  Map<String, Object?> previous,
  Map<String, Object?> next,
) => {
  ...previous,
  ...next,
  'pass': previous['pass'] == true && next['pass'] == true,
  'results': {
    ...?(previous['results'] as Map<String, Object?>?),
    ...?(next['results'] as Map<String, Object?>?),
  },
  if (previous['details'] != null || next['details'] != null)
    'details': {
      ...?(previous['details'] as Map<String, Object?>?),
      ...?(next['details'] as Map<String, Object?>?),
    },
};
