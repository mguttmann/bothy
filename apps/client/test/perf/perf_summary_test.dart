// SPDX-License-Identifier: Apache-2.0

// The evaluation of the performance level (brief M4-34 Design note 3; 15 19): p95 over the
// samples by nearest rank, the jank ratio of recorded frame timings, the comparison with the
// targets of AppConfig (never a literal target here) and the JSON of build/perf/summary.json.

import 'package:domain/domain.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../perf_test/perf_summary.dart';

void main() {
  final config = AppConfig.defaults();
  final targets = desktopTargets(config);
  PerfTarget target(String name) => targets.singleWhere((t) => t.name == name);

  test('p95 is the nearest-rank value of the samples', () {
    final samples = [for (var i = 1; i <= 20; i++) i.toDouble()];
    expect(percentile95(samples), 19);
    expect(percentile95([...samples.reversed]), 19, reason: 'order free');
    expect(percentile95([7]), 7);
    expect(() => percentile95([]), throwsArgumentError);
  });

  test('the jank ratio counts frames over the budget of one refresh', () {
    const budget = Duration(microseconds: 16667);
    // Recorded totals of a scroll: three of twelve frames over budget.
    final frames = [
      for (final us in [
        8100,
        9200,
        12500,
        16000,
        16600,
        17100,
        33900,
        11000,
        10200,
        9900,
        15100,
        21000,
      ])
        Duration(microseconds: us),
    ];
    expect(jankRatio(frames, budget), 3 / 12);
    expect(() => jankRatio([], budget), throwsArgumentError);
  });

  test('the desktop targets come from AppConfig, one per key of 15 19', () {
    expect(
      [for (final t in targets) t.name],
      [
        'client.perf.cold_start_p95',
        'client.perf.thread_open_p95',
        'client.perf.send_local_echo_p95',
        'client.perf.patch_to_frame_p95',
        'client.perf.jank_ratio_max',
        'client.perf.catchup_events_per_s_min',
        'client.perf.memory_desktop_max',
      ],
    );
    expect(
      target('client.perf.cold_start_p95').limit,
      config.perfColdStartP95.inMicroseconds / 1000,
    );
    expect(
      target('client.perf.catchup_events_per_s_min').limit,
      config.perfCatchupEventsPerSMin.toDouble(),
    );
    expect(
      target('client.perf.memory_desktop_max').limit,
      config.perfMemoryDesktopMax.toDouble(),
    );
  });

  test('a summary passes when every target is met and fails on one miss', () {
    double half(String name) => target(name).limit / 2;
    PerfSummary build({required double coldFactor}) => PerfSummary.evaluate(
      targets: targets,
      device: 'Test device',
      reference: false,
      samples: {
        'client.perf.cold_start_p95': [
          for (var i = 0; i < 20; i++)
            target('client.perf.cold_start_p95').limit * coldFactor,
        ],
        'client.perf.thread_open_p95': [
          for (var i = 0; i < 20; i++) half('client.perf.thread_open_p95'),
        ],
        'client.perf.send_local_echo_p95': [
          for (var i = 0; i < 20; i++) half('client.perf.send_local_echo_p95'),
        ],
        'client.perf.patch_to_frame_p95': [
          for (var i = 0; i < 20; i++) half('client.perf.patch_to_frame_p95'),
        ],
        'client.perf.jank_ratio_max': [half('client.perf.jank_ratio_max')],
        'client.perf.catchup_events_per_s_min': [
          target('client.perf.catchup_events_per_s_min').limit * 2,
        ],
        'client.perf.memory_desktop_max': [
          half('client.perf.memory_desktop_max'),
        ],
      },
    );

    final pass = build(coldFactor: 0.5);
    expect(pass.pass, isTrue);
    final json = pass.toJson();
    expect(json['device'], 'Test device');
    expect(json['reference_device'], isFalse);
    final results = json['results']! as Map<String, Object?>;
    expect(results.keys, hasLength(7));
    final cold = results['client.perf.cold_start_p95']! as Map<String, Object?>;
    expect(cold['samples'], 20);
    expect(cold['unit'], 'ms');
    expect(cold['pass'], isTrue);

    final miss = build(coldFactor: 1.5);
    expect(miss.pass, isFalse);
    expect(
      [
        for (final r in miss.results)
          if (!r.pass) r.target.name,
      ],
      ['client.perf.cold_start_p95'],
    );
  });

  test('a p95 target with fewer than twenty samples fails, and so does a '
      'missing measurement', () {
    final few = PerfSummary.evaluate(
      targets: [target('client.perf.thread_open_p95')],
      device: 'Test device',
      reference: true,
      samples: {
        'client.perf.thread_open_p95': [1, 2, 3],
      },
    );
    expect(few.pass, isFalse);
    expect(few.results.single.note, contains('20'));
    final none = PerfSummary.evaluate(
      targets: [target('client.perf.jank_ratio_max')],
      device: 'Test device',
      reference: true,
      samples: const {},
    );
    expect(none.pass, isFalse);
  });
}
