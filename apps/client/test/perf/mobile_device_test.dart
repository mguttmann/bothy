// SPDX-License-Identifier: Apache-2.0

// How the mobile performance level reads the phone it runs on (brief M5-11 Design note 5): the
// model identifier and system of an Android phone from the output of `getprop`, the start of an
// Android process from `/proc/self/stat` (a recorded file) and its age with the runner's time
// since the boot, and on Darwin the model identifier of `uname` and the start time of the
// process from the kernel, which this host (macOS) shares with iOS.

import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

import '../../perf_test/phone/mobile_device.dart';

void main() {
  test('getprop output becomes the properties by name', () {
    const output = '''
[dalvik.vm.heapsize]: [512m]
[ro.build.version.release]: [16]
[ro.build.version.sdk]: [36]
[ro.product.manufacturer]: [Google]
[ro.product.model]: [Pixel 6a]
[ro.product.name]: [bluejay]
[persist.sys.example]: []
not a property line
[ro.product.example.multi]: [a]: [b]
''';
    final props = parseGetprop(output);
    expect(props['ro.product.model'], 'Pixel 6a');
    expect(props['ro.product.manufacturer'], 'Google');
    expect(props['ro.build.version.sdk'], '36');
    expect(props['persist.sys.example'], '');
    expect(props.containsKey('not a property line'), isFalse);
    // A value may hold brackets; the line ends with the value's closing bracket.
    expect(props['ro.product.example.multi'], 'a]: [b');
    expect(props, hasLength(8));
  });

  test('an Android phone is named by maker, model and system', () {
    final device = androidDevice({
      'ro.product.manufacturer': 'Google',
      'ro.product.model': 'Pixel 6a',
      'ro.build.version.release': '16',
      'ro.build.version.sdk': '36',
    });
    expect(device.platform, 'android');
    expect(device.manufacturer, 'Google');
    expect(device.model, 'Pixel 6a');
    expect(device.os, 'Android 16 (API 36)');
    final unknown = androidDevice(const {});
    expect(unknown.model, unknownModel);
    expect(unknown.manufacturer, unknownModel);
  });

  test('an Android process starts at field 22 of its stat, in clock ticks '
      'since the boot', () {
    // /proc/self/stat of an app process: the name may hold spaces and parentheses; field 22
    // (starttime) is 512345 ticks of USER_HZ after the boot.
    const stat =
        '4321 (com.example.chat (main)) S 612 612 0 0 -1 1077952832 '
        '41324 0 3 0 812 190 0 0 10 -10 52 0 512345 15797428224 30218 '
        '0 1 1 0 0 0 0 4612 1 0 0 0 0 17 5 0 0 '
        '0 0 0 0 0 0 0 0 0 0 0';
    expect(processStartTicks(stat), 512345);
    expect(processStartTicks('garbage'), isNull);
    expect(processStartTicks('4321 (short) S 1 2 3'), isNull);
  });

  test('the age at the entry of main is the time since the boot, less the '
      'start of the process and the time since main', () {
    // Started 5123.45 s after the boot; asked 5131.73 s after the boot, 0.5 s after main.
    expect(
      ageAtMain(
        startTicks: 512345,
        sinceBoot: const Duration(milliseconds: 5131730),
        sinceMain: const Duration(milliseconds: 500),
      ),
      const Duration(milliseconds: 7780),
    );
    // A start after the moment of main (clocks that cannot be): no age rather than a negative.
    expect(
      ageAtMain(
        startTicks: 512345,
        sinceBoot: const Duration(milliseconds: 5123000),
        sinceMain: Duration.zero,
      ),
      isNull,
    );
  });

  test('on Android the time since the boot comes from the runner', () async {
    final entry = MainEntry.android(512345);
    var asked = 0;
    final age = await entry.processToMain(
      sinceBootMillis: () async {
        asked++;
        return 5131730;
      },
    );
    expect(asked, 1);
    // The entry was recorded just now, so the age is the boot time less the start, less the
    // few milliseconds since the entry.
    expect(age, isNotNull);
    expect(age!, lessThanOrEqualTo(const Duration(milliseconds: 8280)));
    expect(age, greaterThan(Duration.zero));
    expect(
      await MainEntry.android(null)
          .processToMain(sinceBootMillis: () async => 1),
      isNull,
      reason: 'no start, no age',
    );
  });

  group('Darwin (iOS shares the kernel interfaces of this macOS host)', () {
    test(
      'the model identifier of uname is the machine of `uname -m`',
      () async {
        final expected = (await Process.run('uname', ['-m'])).stdout as String;
        expect(darwinMachine(), expected.trim());
      },
    );

    test('the process started before now and its age grows', () async {
      final start = darwinProcessStart();
      expect(start, isNotNull);
      expect(start!.isBefore(DateTime.now()), isTrue);
      final first = (await MainEntry.record().processToMain())!;
      expect(first, greaterThan(Duration.zero));
      // The kernel's start has microseconds; a later entry is older as soon as the clock moved.
      var later = (await MainEntry.record().processToMain())!;
      while (later == first) {
        await Future<void>.delayed(const Duration(milliseconds: 1));
        later = (await MainEntry.record().processToMain())!;
      }
      expect(later, greaterThan(first));
    });
  }, skip: Platform.isMacOS ? false : 'Darwin only');

  test('off a phone the device is named but never a known model', () async {
    final device = await readMobileDevice();
    expect(device.platform, Platform.operatingSystem);
    expect(device.os, isNotEmpty);
    if (Platform.isMacOS) expect(device.model, darwinMachine());
  });
}
