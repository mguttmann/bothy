// SPDX-License-Identifier: Apache-2.0

import 'dart:typed_data';

import 'package:bothy_noise/bothy_noise.dart';
import 'package:test/test.dart';

Matcher throwsFrame(FrameError error) =>
    throwsA(isA<FrameException>().having((e) => e.error, 'error', error));

void main() {
  const device = FrameSender.device;
  const host = FrameSender.host;
  final allowed = <TerminalFrame, List<FrameSender>>{
    const StartFrame(120, 40): [device],
    DataFrame(Uint8List.fromList('ls\n'.codeUnits)): [device, host],
    const ResizeFrame(1, 65535): [device],
    const CloseFrame(): [device],
    const ExitFrame(-1): [host],
    const ReplayBeginFrame(): [host],
    const ReplayEndFrame(): [host],
  };

  test('round trip for every type and allowed direction', () {
    allowed.forEach((frame, senders) {
      final bytes = encodeFrame(frame);
      for (final from in senders) {
        expect(encodeFrame(decodeFrame(bytes, from)), bytes);
        expect(decodeFrame(bytes, from), frame);
      }
    });
  });

  test('big-endian sizes', () {
    expect(encodeFrame(const StartFrame(0x0102, 0x0304)), [
      0x01,
      0x01,
      0x02,
      0x03,
      0x04,
    ]);
    expect(encodeFrame(const ResizeFrame(0x0102, 0x0304)), [
      0x03,
      0x01,
      0x02,
      0x03,
      0x04,
    ]);
  });

  test('signed exit code', () {
    final cases = <int, List<int>>{
      -1: [0x05, 0xff, 0xff, 0xff, 0xff],
      0: [0x05, 0, 0, 0, 0],
      -2147483648: [0x05, 0x80, 0, 0, 0],
      2147483647: [0x05, 0x7f, 0xff, 0xff, 0xff],
    };
    cases.forEach((code, bytes) {
      expect(encodeFrame(ExitFrame(code)), bytes);
      expect((decodeFrame(bytes, host) as ExitFrame).code, code);
    });
    expect(
      () => encodeFrame(const ExitFrame(2147483648)),
      throwsFrame(FrameError.malformed),
    );
    expect(
      () => encodeFrame(const ExitFrame(-2147483649)),
      throwsFrame(FrameError.malformed),
    );
  });

  test('unknown type', () {
    for (final from in FrameSender.values) {
      for (var b = 0; b <= 0xff; b++) {
        if (b >= 1 && b <= 7) continue;
        expect(
          () => decodeFrame([b, 0, 0, 0, 0], from),
          throwsFrame(FrameError.unknownType),
        );
      }
    }
  });

  test('wrong direction', () {
    expect(
      () => decodeFrame([0x01, 0, 80, 0, 24], host),
      throwsFrame(FrameError.wrongDirection),
    );
    expect(
      () => decodeFrame([0x03, 0, 80, 0, 24], host),
      throwsFrame(FrameError.wrongDirection),
    );
    expect(
      () => decodeFrame([0x04], host),
      throwsFrame(FrameError.wrongDirection),
    );
    expect(
      () => decodeFrame([0x05, 0, 0, 0, 0], device),
      throwsFrame(FrameError.wrongDirection),
    );
    expect(
      () => decodeFrame([0x06], device),
      throwsFrame(FrameError.wrongDirection),
    );
    expect(
      () => decodeFrame([0x07], device),
      throwsFrame(FrameError.wrongDirection),
    );
  });

  test('malformed', () {
    for (final from in FrameSender.values) {
      expect(() => decodeFrame(const [], from), throwsFrame(FrameError.empty));
    }
    final cases = <List<int>, FrameSender>{
      [0x01, 0, 80, 0]: device,
      [0x01, 0, 80, 0, 24, 0]: device,
      [0x01, 0, 0, 0, 24]: device,
      [0x01, 0, 80, 0, 0]: device,
      [0x03, 0, 80, 0]: device,
      [0x03, 0, 80, 0, 24, 0]: device,
      [0x03, 0, 0, 0, 24]: device,
      [0x03, 0, 80, 0, 0]: device,
      [0x04, 0]: device,
      [0x06, 0]: host,
      [0x07, 0]: host,
      [0x05, 0, 0, 0]: host,
      [0x05, 0, 0, 0, 0, 0]: host,
      [0x02]: device,
    };
    cases.forEach((bytes, from) {
      expect(
        () => decodeFrame(bytes, from),
        throwsFrame(FrameError.malformed),
        reason: '$bytes',
      );
    });
    expect(() => decodeFrame([0x02], host), throwsFrame(FrameError.malformed));
    expect(
      () => encodeFrame(const StartFrame(0, 1)),
      throwsFrame(FrameError.malformed),
    );
    expect(
      () => encodeFrame(const ResizeFrame(1, 65536)),
      throwsFrame(FrameError.malformed),
    );
    expect(
      () => encodeFrame(DataFrame(Uint8List(0))),
      throwsFrame(FrameError.malformed),
    );
  });

  test('size limit', () {
    final big = Uint8List(maxFrameLength - 1);
    final bytes = encodeFrame(DataFrame(big));
    expect(bytes, hasLength(maxFrameLength));
    expect(
      (decodeFrame(bytes, host) as DataFrame).bytes,
      hasLength(maxFrameLength - 1),
    );
    expect(
      () => encodeFrame(DataFrame(Uint8List(maxFrameLength))),
      throwsFrame(FrameError.tooLong),
    );
    expect(
      () => decodeFrame(Uint8List(maxFrameLength + 1)..[0] = 2, device),
      throwsFrame(FrameError.tooLong),
    );
    expect(maxFrameLength, maxTransportPlaintextLength);
    expect(maxFrameLength, 65519);
  });

  test('isInput is true only for data and resize', () {
    for (final frame in allowed.keys) {
      expect(
        frame.isInput,
        frame is DataFrame || frame is ResizeFrame,
        reason: '$frame',
      );
    }
  });

  test('errors carry no content', () {
    const canary = 'CANARY-7f3a91';
    final inputs = <List<int>, FrameSender>{
      [0x04, ...canary.codeUnits]: device,
      [0x05, ...canary.codeUnits]: host,
      [0x09, ...canary.codeUnits]: device,
      [0x01, ...canary.codeUnits]: device,
    };
    inputs.forEach((bytes, from) {
      try {
        decodeFrame(bytes, from);
        fail('no error');
      } on FrameException catch (e) {
        expect(e.toString().contains('7f3a91'), isFalse);
        expect(e.toString().contains('CANARY'), isFalse);
      }
    });
  });

  test('equality and description', () {
    expect(
      DataFrame(Uint8List.fromList([1, 2])),
      DataFrame(Uint8List.fromList([1, 2])),
    );
    expect(
      DataFrame(Uint8List.fromList([1, 2])),
      isNot(DataFrame(Uint8List.fromList([1, 3]))),
    );
    expect(
      DataFrame(Uint8List.fromList([1, 2])).hashCode,
      DataFrame(Uint8List.fromList([1, 2])).hashCode,
    );
    expect(const StartFrame(1, 2).hashCode, const StartFrame(1, 2).hashCode);
    expect(const ExitFrame(3).hashCode, const ExitFrame(3).hashCode);
    expect(
      DataFrame(Uint8List.fromList([1, 2])).toString().contains('2 bytes'),
      isTrue,
    );
    for (final frame in allowed.keys) {
      expect(frame.toString(), isNotEmpty);
      expect(frame.hashCode, isA<int>());
    }
  });

  test('values outside a byte are rejected', () {
    expect(
      () => decodeFrame([0x102, 0x41], device),
      throwsFrame(FrameError.malformed),
    );
    expect(
      () => decodeFrame([0x01, -1, 0, 0, 0x18], device),
      throwsFrame(FrameError.malformed),
    );
    expect(
      decodeFrame([0x02, 0xff], host),
      DataFrame(Uint8List.fromList([0xff])),
    );
  });
}
