// SPDX-License-Identifier: Apache-2.0

// The duration of a recording as the server reads it (brief M5-07 Design note 1; 12 8.1): the
// `mvhd` duration in whole milliseconds rounded down, else the `mehd` duration of a fragmented
// movie; and the stop before `voice_max_duration_ms`.

import 'dart:io';
import 'dart:typed_data';

import 'package:client/voice/mp4_duration.dart';
import 'package:client/voice/recorder.dart';
import 'package:flutter_test/flutter_test.dart';

/// A box of [type] around [payload]; [large] writes the 64-bit size form.
List<int> box(String type, List<int> payload, {bool large = false}) {
  final header = ByteData(large ? 16 : 8);
  if (large) {
    header
      ..setUint32(0, 1)
      ..setUint64(8, 16 + payload.length);
  } else {
    header.setUint32(0, 8 + payload.length);
  }
  final bytes = header.buffer.asUint8List()..setAll(4, type.codeUnits);
  return [...bytes, ...payload];
}

/// An `mvhd` payload of [version] with [timescale] and [duration].
List<int> mvhd(int version, int timescale, int duration) {
  final b = ByteData(version == 1 ? 112 : 100)..setUint8(0, version);
  if (version == 1) {
    b
      ..setUint32(20, timescale)
      ..setUint64(24, duration);
  } else {
    b
      ..setUint32(12, timescale)
      ..setUint32(16, duration);
  }
  return b.buffer.asUint8List();
}

List<int> ftyp() => box('ftyp', 'M4A isomM4A '.codeUnits);

void main() {
  late Directory dir;
  setUp(() => dir = Directory.systemTemp.createTempSync('mp4'));
  tearDown(() => dir.deleteSync(recursive: true));

  Future<Duration?> durationOf(List<int> bytes) {
    final f = File('${dir.path}/r.m4a')..writeAsBytesSync(bytes);
    return mp4Duration(f.path);
  }

  test('version 0, moov after the media data, rounded down to ms', () async {
    final d = await durationOf([
      ...ftyp(),
      ...box('mdat', List.filled(64, 1)),
      ...box('moov', box('mvhd', mvhd(0, 44100, 44100 * 3 + 44))),
    ]);
    // 3 s and 44 of 44100: 0.99 ms, rounded down to 0.
    expect(d, const Duration(seconds: 3));
  });

  test('version 1 and a 64-bit box size', () async {
    final d = await durationOf([
      ...ftyp(),
      ...box('moov', box('mvhd', mvhd(1, 1000, 15 * 60 * 1000)), large: true),
    ]);
    expect(d, const Duration(minutes: 15));
  });

  test('a fragmented movie takes the duration of mehd', () async {
    final mehd = ByteData(8)..setUint32(4, 90000 * 7);
    final d = await durationOf([
      ...ftyp(),
      ...box('moov', [
        ...box('mvhd', mvhd(0, 90000, 0)),
        ...box('mvex', box('mehd', mehd.buffer.asUint8List())),
      ]),
    ]);
    expect(d, const Duration(seconds: 7));
  });

  test(
    'no duration: not an MP4, no moov, unknown or zero time scale',
    () async {
      expect(await durationOf(List.filled(40, 7)), isNull);
      expect(await durationOf(ftyp()), isNull);
      expect(
        await durationOf([
          ...ftyp(),
          ...box('moov', box('mvhd', mvhd(0, 0, 9))),
        ]),
        isNull,
      );
      expect(
        await durationOf([
          ...ftyp(),
          ...box('moov', box('mvhd', mvhd(0, 1000, 0xFFFFFFFF))),
        ]),
        isNull,
      );
      // A box larger than the file is a cut-off file.
      final cut = box('moov', box('mvhd', mvhd(0, 1000, 5000)));
      expect(
        await durationOf([...ftyp(), ...cut.sublist(0, cut.length - 10)]),
        isNull,
      );
    },
  );

  test(
    'the stop comes three AAC frames and the allowance before the limit',
    () {
      const limit = Duration(minutes: 15);
      final margin = voiceStopMargin(sampleRate: 44100);
      // 3 * 1024 samples at 44.1 kHz (69.66 ms) and the allowance for a late timer.
      expect(margin, const Duration(microseconds: 69659) + voiceStopAllowance);
      expect(voiceStopAt(limit, sampleRate: 44100), limit - margin);
      expect(voiceStopAt(voiceStopAllowance), Duration.zero);
    },
  );
}
