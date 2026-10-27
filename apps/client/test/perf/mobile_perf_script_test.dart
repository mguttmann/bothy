// SPDX-License-Identifier: Apache-2.0

// tool/mobile_perf.sh (brief M5-11 Design note 5; the contract of `make perf`): without an
// attached physical phone it prints exactly its SKIP line and ends with 0; with one it runs the
// measurement through `flutter drive --profile` on that phone and ends with the result of the
// summary the driver wrote (0 pass, 1 failure, 3 not a reference device). A fake `flutter` in
// front of PATH stands in for the tool; `dart` is the real one.

import 'dart:convert';
import 'dart:io';

import 'package:domain/domain.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../perf_test/perf_summary.dart';
import '../../perf_test/phone/mobile_perf_summary.dart';

/// The line of a host without a physical phone (the contract of `make perf`).
const String skipLine =
    'SKIP perf-mobile: no physical reference device attached (targets not proven)';

void main() {
  late Directory dir;
  late File devices;
  late File summary;
  late File driveLog;

  setUp(() async {
    dir = await Directory.systemTemp.createTemp('mobile-perf-script');
    final bin = Directory('${dir.path}/bin')..createSync();
    devices = File('${dir.path}/devices.json');
    summary = File('${dir.path}/out/mobile-summary.json');
    driveLog = File('${dir.path}/drive.log');
    // The fake tool: `devices --machine` prints the recorded list; `drive` records its
    // arguments, copies the recorded summary to the place the driver writes it and ends with
    // the code of FAKE_DRIVE_EXIT.
    final flutter = File('${bin.path}/flutter')
      ..writeAsStringSync(r'''#!/bin/sh
case "$1" in
devices) cat "$FAKE_DEVICES" ;;
drive)
  printf '%s\n' "$*" >"$FAKE_LOG"
  if [ -n "${FAKE_SUMMARY:-}" ]; then cp "$FAKE_SUMMARY" "$MOBILE_PERF_SUMMARY"; fi
  exit "${FAKE_DRIVE_EXIT:-0}"
  ;;
*) exit 64 ;;
esac
''');
    await Process.run('chmod', ['755', flutter.path]);
  });

  tearDown(() => dir.delete(recursive: true));

  Map<String, Object?> phone(String id, String platform) => {
    'name': id,
    'id': id,
    'isSupported': true,
    'targetPlatform': platform,
    'emulator': false,
    'sdk': 'EXAMPLE',
  };

  const emulator = {
    'name': 'Android SDK built for arm64',
    'id': 'emulator-5554',
    'isSupported': true,
    'targetPlatform': 'android-arm64',
    'emulator': true,
    'sdk': 'EXAMPLE',
  };
  const desktop = {
    'name': 'macOS',
    'id': 'macos',
    'isSupported': true,
    'targetPlatform': 'darwin',
    'emulator': false,
    'sdk': 'EXAMPLE',
  };

  /// A summary file as the driver writes it for [device], every target at [factor] of its
  /// limit.
  File recordedSummary(MobileDevice device, double factor) {
    final targets = mobileTargets(AppConfig.defaults());
    final json = MobilePerfSummary.evaluate(
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
    return File('${dir.path}/recorded-${device.model}-$factor.json')
      ..writeAsStringSync(const JsonEncoder.withIndent('  ').convert(json));
  }

  const pixel = MobileDevice(
    platform: 'android',
    manufacturer: 'Google',
    model: 'Pixel 6a',
    os: 'Android 16 (API 36)',
  );
  const other = MobileDevice(
    platform: 'android',
    manufacturer: 'Example',
    model: 'EXAMPLE-1',
    os: 'Android 16 (API 36)',
  );

  Future<ProcessResult> run({
    List<Object?> attached = const [],
    File? recorded,
    int driveExit = 0,
    String? path,
    Map<String, String> extra = const {},
  }) {
    devices.writeAsStringSync(jsonEncode(attached));
    return Process.run(
      'bash',
      ['tool/mobile_perf.sh'],
      environment: {
        'PATH': path ?? '${dir.path}/bin:${Platform.environment['PATH']}',
        'FAKE_DEVICES': devices.path,
        'FAKE_LOG': driveLog.path,
        'FAKE_SUMMARY': recorded?.path ?? '',
        'FAKE_DRIVE_EXIT': '$driveExit',
        'MOBILE_PERF_SUMMARY': summary.path,
        // A choice of the developer's shell never reaches the script under test.
        'MOBILE_PERF_DEVICE': '',
        ...extra,
      },
    );
  }

  test('without a physical phone it prints exactly the SKIP line and ends '
      'with 0', () async {
    for (final attached in [
      const <Object?>[],
      [emulator, desktop],
    ]) {
      final r = await run(attached: attached);
      expect(r.exitCode, 0, reason: '${r.stderr}');
      expect(r.stdout, '$skipLine\n');
      expect(driveLog.existsSync(), isFalse, reason: 'nothing measured');
    }
  });

  test('without flutter on PATH it prints the same SKIP line, names the reason '
      'on stderr and measures nothing', () async {
    final r = await run(path: '/usr/bin:/bin');
    expect(r.exitCode, 0, reason: '${r.stderr}');
    expect(r.stdout, '$skipLine\n');
    expect(r.stderr, contains('not on PATH'));
    expect(driveLog.existsSync(), isFalse);
  });

  test('a physical phone gets the profile build of the measurement, and a '
      'pass on a reference device ends with 0', () async {
    final r = await run(
      attached: [emulator, phone('EXAMPLE0SERIAL', 'android-arm64')],
      recorded: recordedSummary(pixel, 0.5),
    );
    expect(r.exitCode, 0, reason: '${r.stdout}${r.stderr}');
    final args = driveLog.readAsStringSync();
    expect(args, startsWith('drive '));
    expect(args, contains('--profile'));
    // The app runs `main` at once: a start paused for the driver would count the wait into
    // the cold start.
    expect(args, contains('--no-start-paused'));
    expect(args, contains('-d EXAMPLE0SERIAL'));
    expect(args, contains('--target=perf_test/phone/mobile_perf_test.dart'));
    expect(args, contains('--driver=test_driver/mobile_perf_driver.dart'));
    expect(r.stdout, contains('Pixel 6a'));
    expect(r.stdout, isNot(contains('SKIP')));
  });

  test('a missed target on a reference device ends with 1', () async {
    final r = await run(
      attached: [phone('EXAMPLE0SERIAL', 'android-arm64')],
      recorded: recordedSummary(pixel, 1.5),
      driveExit: 1,
    );
    expect(r.exitCode, 1, reason: '${r.stdout}${r.stderr}');
    expect(r.stdout, contains('MISSED'));
  });

  test('another device reports its values and ends with 3', () async {
    final r = await run(
      attached: [phone('EXAMPLE0SERIAL', 'android-arm64')],
      recorded: recordedSummary(other, 0.5),
    );
    expect(r.exitCode, 3, reason: '${r.stdout}${r.stderr}');
    expect(r.stdout, contains('not proven'));
  });

  test('a failed run fails, also with a summary that says pass', () async {
    var r = await run(
      attached: [phone('EXAMPLE0SERIAL', 'android-arm64')],
      driveExit: 1,
    );
    expect(r.exitCode, 1, reason: '${r.stdout}${r.stderr}');
    r = await run(
      attached: [phone('EXAMPLE0SERIAL', 'android-arm64')],
      recorded: recordedSummary(pixel, 0.5),
      driveExit: 1,
    );
    expect(r.exitCode, 1, reason: '${r.stdout}${r.stderr}');
  });

  test('a summary of an earlier run never counts', () async {
    summary
      ..createSync(recursive: true)
      ..writeAsStringSync(recordedSummary(pixel, 0.5).readAsStringSync());
    final r = await run(attached: [phone('EXAMPLE0SERIAL', 'android-arm64')]);
    expect(r.exitCode, 1, reason: '${r.stdout}${r.stderr}');
    expect(summary.existsSync(), isFalse);
  });

  test('MOBILE_PERF_DEVICE picks one of the attached phones', () async {
    final attached = [
      phone('00008140-EXAMPLE0IPHONE', 'ios'),
      phone('EXAMPLE0SERIAL', 'android-arm64'),
    ];
    var r = await run(
      attached: attached,
      recorded: recordedSummary(pixel, 0.5),
      extra: {'MOBILE_PERF_DEVICE': 'EXAMPLE0SERIAL'},
    );
    expect(r.exitCode, 0, reason: '${r.stdout}${r.stderr}');
    expect(driveLog.readAsStringSync(), contains('-d EXAMPLE0SERIAL'));
    r = await run(attached: attached, recorded: recordedSummary(pixel, 0.5));
    expect(
      driveLog.readAsStringSync(),
      contains('-d 00008140-EXAMPLE0IPHONE'),
      reason: 'the first phone by default',
    );
    r = await run(
      attached: attached,
      extra: {'MOBILE_PERF_DEVICE': 'EXAMPLE0MISSING'},
    );
    expect(r.exitCode, 2, reason: '${r.stdout}${r.stderr}');
  });
}
