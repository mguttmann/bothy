// SPDX-License-Identifier: Apache-2.0

// The evaluation of the mobile performance level (brief M5-11 Design note 5; 15 19): the mobile
// targets of AppConfig (never a literal target here), the fixed list of reference devices and
// the result of a run: a pass only on a reference device that meets every target, a failure on
// a reference device that misses one, and "not proven" on every other device, whatever its
// values.

import 'package:domain/domain.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../perf_test/perf_summary.dart';
import '../../perf_test/phone/mobile_perf_summary.dart';

void main() {
  final config = AppConfig.defaults();
  final targets = mobileTargets(config);
  PerfTarget target(String name) => targets.singleWhere((t) => t.name == name);

  /// Samples as the measurement records them, each target at [factor] of its limit (a rate
  /// target at the inverse, so a factor below one meets every target); [skip] leaves targets
  /// out.
  Map<String, List<double>> recorded(
    double factor, {
    Map<String, double> override = const {},
    Set<String> skip = const {},
  }) => {
    for (final t in targets)
      if (!skip.contains(t.name))
        t.name: [
          for (
            var i = 0;
            i < (t.aggregate == PerfAggregate.p95 ? t.minimumSamples : 1);
            i++
          )
            t.limit *
                (override[t.name] ??
                    (t.comparison == PerfComparison.atLeast
                        ? 1 / factor
                        : factor)),
        ],
  };

  const iphone = MobileDevice(
    platform: 'ios',
    manufacturer: 'Apple',
    model: 'iPhone17,3',
    os: 'iOS Version 26.0 (Build 23A341)',
  );
  const pixel = MobileDevice(
    platform: 'android',
    manufacturer: 'Google',
    model: 'Pixel 6a',
    os: 'Android 16 (API 36)',
  );
  const proMax = MobileDevice(
    platform: 'ios',
    manufacturer: 'Apple',
    model: 'iPhone17,2',
    os: 'iOS Version 26.0 (Build 23A341)',
  );

  test('the mobile targets come from AppConfig, the memory of a phone in '
      'place of the desktop memory', () {
    expect(
      [for (final t in targets) t.name],
      [
        'client.perf.cold_start_p95',
        'client.perf.thread_open_p95',
        'client.perf.send_local_echo_p95',
        'client.perf.patch_to_frame_p95',
        'client.perf.jank_ratio_max',
        'client.perf.catchup_events_per_s_min',
        'client.perf.memory_mobile_max',
      ],
    );
    final memory = target('client.perf.memory_mobile_max');
    expect(memory.limit, config.perfMemoryMobileMax.toDouble());
    expect(memory.unit, 'bytes');
    expect(memory.comparison, PerfComparison.atMost);
    expect(
      target('client.perf.send_local_echo_p95').limit,
      config.perfSendLocalEchoP95.inMicroseconds / 1000,
    );
    // The desktop targets stay as they are (brief M4-34).
    expect(
      [for (final t in desktopTargets(config)) t.name].last,
      'client.perf.memory_desktop_max',
    );
  });

  test('the reference list names an iPhone and an Android phone of 15 19, '
      'each entry with a reason', () {
    expect(
      {for (final r in mobileReferenceDevices) r.platform},
      {'ios', 'android'},
    );
    for (final r in mobileReferenceDevices) {
      expect(r.reason, isNotEmpty, reason: r.model);
      expect(r.name, isNotEmpty, reason: r.model);
    }
    expect(
      {for (final r in mobileReferenceDevices) '${r.platform}/${r.model}'},
      hasLength(mobileReferenceDevices.length),
      reason: 'no entry twice',
    );
  });

  test('a device matches a reference entry only by its exact model '
      'identifier on its platform', () {
    expect(referenceDeviceOf(iphone)?.model, 'iPhone17,3');
    expect(referenceDeviceOf(pixel)?.model, 'Pixel 6a');
    // The manufacturer as some builds report it, in lower case.
    expect(
      referenceDeviceOf(
        const MobileDevice(
          platform: 'android',
          manufacturer: 'google',
          model: 'Pixel 6a',
          os: 'Android 16 (API 36)',
        ),
      )?.model,
      'Pixel 6a',
    );
    for (final other in const [
      // A faster model of the same generation.
      proMax,
      // The neighbour of the Android entry, and its model under another maker.
      MobileDevice(
        platform: 'android',
        manufacturer: 'Google',
        model: 'Pixel 6',
        os: 'Android 16 (API 36)',
      ),
      MobileDevice(
        platform: 'android',
        manufacturer: 'Example',
        model: 'Pixel 6a',
        os: 'Android 16 (API 36)',
      ),
      // The right identifier on the wrong platform.
      MobileDevice(
        platform: 'android',
        manufacturer: 'Apple',
        model: 'iPhone17,3',
        os: 'Android 16 (API 36)',
      ),
      // A simulator and an emulator report no model of a phone.
      MobileDevice(
        platform: 'ios',
        manufacturer: 'Apple',
        model: 'arm64',
        os: 'iOS Version 26.0 (Build 23A341)',
      ),
      MobileDevice(
        platform: 'android',
        manufacturer: 'Google',
        model: 'sdk_gphone64_arm64',
        os: 'Android 16 (API 36)',
      ),
      // Not an exact identifier.
      MobileDevice(
        platform: 'ios',
        manufacturer: 'Apple',
        model: 'iphone17,3',
        os: 'iOS Version 26.0 (Build 23A341)',
      ),
    ]) {
      expect(referenceDeviceOf(other), isNull, reason: other.label);
    }
  });

  test('a reference device that meets every target passes', () {
    final summary = MobilePerfSummary.evaluate(
      targets: targets,
      device: iphone,
      samples: recorded(0.5),
      details: const {'refresh_rate_hz': 60},
    );
    expect(summary.result, MobilePerfResult.pass);
    final json = summary.toJson();
    expect(json['result'], 'pass');
    expect(json['pass'], isTrue);
    expect(json['targets_met'], isTrue);
    expect(json['reference_device'], isTrue);
    expect(json['device'], {
      'platform': 'ios',
      'manufacturer': 'Apple',
      'model': 'iPhone17,3',
      'os': 'iOS Version 26.0 (Build 23A341)',
    });
    final reference = json['reference']! as Map<String, Object?>;
    expect(reference['model'], 'iPhone17,3');
    expect(reference['reason'], isNotEmpty);
    final results = json['results']! as Map<String, Object?>;
    expect(results.keys, [for (final t in targets) t.name]);
    final cold = results['client.perf.cold_start_p95']! as Map<String, Object?>;
    expect(cold['samples'], minSamples);
    expect(cold['pass'], isTrue);
    expect(json['details'], {'refresh_rate_hz': 60});
  });

  test('a missed target on a reference device fails the run', () {
    final summary = MobilePerfSummary.evaluate(
      targets: targets,
      device: pixel,
      samples: recorded(0.5, override: {'client.perf.memory_mobile_max': 1.5}),
    );
    expect(summary.result, MobilePerfResult.fail);
    expect(summary.toJson()['result'], 'fail');
    expect(summary.toJson()['pass'], isFalse);
    expect(
      [
        for (final r in summary.perf.results)
          if (!r.pass) r.target.name,
      ],
      ['client.perf.memory_mobile_max'],
    );
    final slowRate = MobilePerfSummary.evaluate(
      targets: targets,
      device: pixel,
      samples: recorded(
        0.5,
        override: {'client.perf.catchup_events_per_s_min': 0.5},
      ),
    );
    expect(slowRate.result, MobilePerfResult.fail, reason: 'a rate too low');
  });

  test('a target that was not measured fails on a reference device', () {
    final summary = MobilePerfSummary.evaluate(
      targets: targets,
      device: iphone,
      samples: recorded(0.5, skip: {'client.perf.cold_start_p95'}),
    );
    expect(summary.result, MobilePerfResult.fail);
    final cold =
        (summary.toJson()['results']!
            as Map<String, Object?>)['client.perf.cold_start_p95']!;
    expect((cold as Map<String, Object?>)['note'], 'not measured');
  });

  test('a device that is not a reference device never yields a pass', () {
    for (final factor in [0.5, 1.5]) {
      final summary = MobilePerfSummary.evaluate(
        targets: targets,
        device: proMax,
        samples: recorded(factor),
      );
      expect(summary.result, MobilePerfResult.notProven, reason: '$factor');
      final json = summary.toJson();
      expect(json['result'], 'not_proven');
      expect(json['pass'], isFalse);
      expect(json['reference_device'], isFalse);
      expect(json.containsKey('reference'), isFalse);
      // The values are kept and compared, only the evidence is missing.
      expect(json['targets_met'], factor < 1);
      expect((json['results']! as Map<String, Object?>), hasLength(7));
    }
  });
}
