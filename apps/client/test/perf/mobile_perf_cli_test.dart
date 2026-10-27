// SPDX-License-Identifier: Apache-2.0

// The helper of tool/mobile_perf.sh (brief M5-11 Design note 5): which attached devices are
// physical phones (`flutter devices --machine`, where only `emulator` tells a simulator from a
// device), and the exit code and report of a recorded mobile summary: 0 for a pass on a
// reference device, 1 for a failure, 3 for values from a device that is not a reference device.

import 'dart:convert';

import 'package:domain/domain.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../perf_test/perf_summary.dart';
import '../../perf_test/phone/mobile_perf_cli.dart';
import '../../perf_test/phone/mobile_perf_summary.dart';

void main() {
  /// A device object as `flutter devices --machine` prints it (flutter_tools `Device.toJson`).
  Map<String, Object?> device(
    String id,
    String platform, {
    required bool emulator,
    String? name,
  }) => {
    'name': name ?? id,
    'id': id,
    'isSupported': true,
    'targetPlatform': platform,
    'emulator': emulator,
    'sdk': 'EXAMPLE',
    'capabilities': {'hotReload': true, 'hotRestart': true},
  };

  test(
    'only physical iOS and Android devices are phones of the measurement',
    () {
      final json = const JsonEncoder.withIndent('  ').convert([
        device('emulator-5554', 'android-arm64', emulator: true),
        device('macos', 'darwin', emulator: false),
        device('chrome', 'web-javascript', emulator: false),
        device(
          '00000000-EXAMPLE0SIM',
          'ios',
          emulator: true,
          name: 'iPhone 16',
        ),
        device(
          '00008140-EXAMPLE0IPHONE',
          'ios',
          emulator: false,
          name: "Alex's iPhone",
        ),
        device('EXAMPLE0SERIAL', 'android-arm64', emulator: false),
        device('EXAMPLE0ARM', 'android-arm', emulator: false),
        device('EXAMPLE0X86', 'unsupported', emulator: false),
      ]);
      final phones = attachedPhones(json);
      expect(
        [for (final p in phones) '${p.id} ${p.platform}'],
        [
          '00008140-EXAMPLE0IPHONE ios',
          'EXAMPLE0SERIAL android',
          'EXAMPLE0ARM android',
        ],
      );
      expect(phones.first.name, "Alex's iPhone");
      expect(
        phones.first.line,
        "00008140-EXAMPLE0IPHONE\tios\tAlex's iPhone",
        reason: 'the line mobile_perf.sh reads',
      );
    },
  );

  test('no phone in an empty list, in a list of emulators and in output '
      'without JSON', () {
    expect(attachedPhones('[]'), isEmpty);
    expect(
      attachedPhones(
        jsonEncode([device('emulator-5554', 'android-x64', emulator: true)]),
      ),
      isEmpty,
    );
    expect(attachedPhones('No devices detected.'), isEmpty);
    // A line before the JSON (a warning of the tool on the same stream) is skipped.
    expect(
      attachedPhones(
        'Waiting for another flutter command to release the startup lock...\n'
        '${jsonEncode([device('EXAMPLE0SERIAL', 'android-arm64', emulator: false)])}',
      ),
      hasLength(1),
    );
  });

  group('the exit code of a recorded summary', () {
    final targets = mobileTargets(AppConfig.defaults());

    Map<String, Object?> summary(MobileDevice device, double factor) =>
        MobilePerfSummary.evaluate(
          targets: targets,
          device: device,
          samples: {
            for (final t in targets)
              t.name: [
                for (
                  var i = 0;
                  i < (t.aggregate == PerfAggregate.p95 ? t.minimumSamples : 1);
                  i++
                )
                  t.comparison == PerfComparison.atLeast
                      ? t.limit / factor
                      : t.limit * factor,
              ],
          },
        ).toJson();

    const iphone = MobileDevice(
      platform: 'ios',
      manufacturer: 'Apple',
      model: 'iPhone17,3',
      os: 'iOS Version 26.0 (Build 23A341)',
    );
    const other = MobileDevice(
      platform: 'android',
      manufacturer: 'Example',
      model: 'EXAMPLE-1',
      os: 'Android 16 (API 36)',
    );

    /// The summary as the driver writes it and the helper reads it back.
    Map<String, Object?> written(Map<String, Object?> s) =>
        (jsonDecode(const JsonEncoder.withIndent('  ').convert(s)) as Map)
            .cast<String, Object?>();

    test('a pass on a reference device ends with 0', () {
      expect(summaryExitCode(written(summary(iphone, 0.5))), exitPass);
      expect(exitPass, 0);
    });

    test('a missed target on a reference device ends with 1', () {
      expect(summaryExitCode(written(summary(iphone, 1.5))), exitFail);
      expect(exitFail, 1);
    });

    test('another device ends with 3, whatever its values', () {
      expect(summaryExitCode(written(summary(other, 0.5))), exitNotProven);
      expect(summaryExitCode(written(summary(other, 1.5))), exitNotProven);
      expect(exitNotProven, 3);
    });

    test('a missing, unreadable or forged summary fails', () {
      expect(summaryExitCode(null), exitFail);
      expect(summaryExitCode(const {}), exitFail);
      expect(summaryExitCode(const {'result': 'maybe'}), exitFail);
      // A pass needs a reference device and every target met; a result field alone is no pass.
      final forged = written(summary(other, 0.5))..['result'] = 'pass';
      expect(summaryExitCode(forged), exitFail);
      final missed = written(summary(iphone, 1.5))..['result'] = 'pass';
      expect(summaryExitCode(missed), exitFail);
    });

    test('the report names the device, whether it is a reference device and '
        'every target with its value and limit', () {
      final lines = describeSummary(written(summary(iphone, 1.5)));
      final text = lines.join('\n');
      expect(text, contains('Apple iPhone17,3'));
      expect(text, contains('iOS Version 26.0 (Build 23A341)'));
      expect(text, contains('reference device'));
      for (final t in targets) {
        expect(text, contains(t.name));
      }
      expect(text, contains('MISSED'));
      final notProven = describeSummary(written(summary(other, 0.5)))
          .join('\n');
      expect(notProven, contains('not a reference device'));
      expect(notProven, contains('not proven'));
    });
  });
}
