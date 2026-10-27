// SPDX-License-Identifier: Apache-2.0

// The terminal targets of 15 19 on a phone (brief M5-13 Tests; docs/decisions.d/M5-13.md
// D-M5-13-0): the measurement of M4-38 (`perf_test/terminal_perf_test.dart`, the generator of the
// host image and the timing hooks) evaluated with the reference list of M5-11. 15 19 lists the two
// targets without a platform limit, so a phone run is evidence only on a reference device that
// meets both, fails on one that misses one, and proves nothing anywhere else. Recorded summaries
// only, no device.

import 'package:domain/domain.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../perf_test/phone/mobile_perf_summary.dart';
import '../../perf_test/terminal_perf.dart';

void main() {
  final config = AppConfig.defaults();
  final targets = terminalTargets(config);
  final frame = targets.singleWhere(
    (t) => t.name == 'client.perf.terminal_frame_to_paint_p95',
  );
  final throughput = targets.singleWhere(
    (t) => t.name == 'client.perf.terminal_throughput_min',
  );

  /// A recorded run: [frames] frame-to-paint samples at [frameFactor] of the limit and the
  /// throughput (one value, the slowest chunk of the run) at [throughputFactor] of its minimum.
  Map<String, List<double>> recorded({
    int? frames,
    double frameFactor = 0.5,
    double throughputFactor = 2,
  }) => {
    frame.name: [
      for (var i = 0; i < (frames ?? terminalFrameSamples); i++)
        frame.limit * frameFactor,
    ],
    throughput.name: [throughput.limit * throughputFactor],
  };

  const pixel = MobileDevice(
    platform: 'android',
    manufacturer: 'Google',
    model: 'Pixel 6a',
    os: 'Android 16 (API 36)',
  );
  const iphone = MobileDevice(
    platform: 'ios',
    manufacturer: 'Apple',
    model: 'iPhone17,3',
    os: 'iOS Version 26.0 (Build 23A341)',
  );
  const emulator = MobileDevice(
    platform: 'android',
    manufacturer: 'Google',
    model: 'sdk_gphone64_arm64',
    os: 'Android 16 (API 36)',
  );

  MobilePerfSummary evaluate(
    MobileDevice device,
    Map<String, List<double>> samples,
  ) => MobilePerfSummary.evaluate(
    targets: targets,
    device: device,
    samples: samples,
  );

  test('a reference phone that meets both terminal targets passes', () {
    for (final device in [pixel, iphone]) {
      final s = evaluate(device, recorded());
      expect(s.result, MobilePerfResult.pass, reason: device.label);
      expect(s.toJson()['pass'], isTrue);
    }
  });

  test('a reference phone that misses frame to paint or throughput fails', () {
    final slow = evaluate(pixel, recorded(frameFactor: 1.5));
    expect(slow.result, MobilePerfResult.fail);
    final missed = slow.perf.results.singleWhere(
      (r) => r.target.name == frame.name,
    );
    expect(missed.pass, isFalse);
    expect(
      evaluate(iphone, recorded(throughputFactor: 0.5)).result,
      MobilePerfResult.fail,
    );
  });

  test(
    'fewer frames than the target asks for, or none, fail on a reference phone',
    () {
      final s = evaluate(pixel, recorded(frames: terminalFrameSamples - 1));
      expect(s.result, MobilePerfResult.fail);
      expect(
        evaluate(pixel, {
          throughput.name: [throughput.limit * 2],
        }).result,
        MobilePerfResult.fail,
      );
    },
  );

  test('an emulator proves nothing, whatever its values', () {
    for (final samples in [recorded(), recorded(frameFactor: 1.5)]) {
      final s = evaluate(emulator, samples);
      expect(s.result, MobilePerfResult.notProven);
      expect(s.toJson()['pass'], isFalse);
    }
  });
}
