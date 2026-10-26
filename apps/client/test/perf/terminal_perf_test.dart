// SPDX-License-Identifier: Apache-2.0

// The evaluation of the terminal targets of the performance level (brief M4-38 Design note 2;
// 15 19): frame to paint per sequence number of the generator from its decrypted data frame to
// the rasterization of the first drawn frame that shows it, p95 over at least 1000 frames;
// throughput per acknowledged chunk from its first decrypted byte to the rasterization of the
// frame that shows its end line; the backlog of the renderer; the targets from AppConfig.
// Recorded samples only, no device.

import 'package:domain/domain.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../perf_test/perf_summary.dart';
import '../../perf_test/terminal_perf.dart';

/// A line of the rate mode of the generator (clientstack/termgen).
String rate(int seq) => 'TERMGEN ${seq.toString().padLeft(6, '0')} rrrr\r\n';

void main() {
  test('the terminal targets come from AppConfig', () {
    final config = AppConfig.defaults();
    final targets = terminalTargets(config);
    expect(
      [for (final t in targets) t.name],
      [
        'client.perf.terminal_frame_to_paint_p95',
        'client.perf.terminal_throughput_min',
      ],
    );
    expect(
      targets.first.limit,
      config.perfTerminalFrameToPaintP95.inMicroseconds / 1000,
    );
    expect(targets.first.minimumSamples, terminalFrameSamples);
    expect(targets.first.aggregate, PerfAggregate.p95);
    expect(targets.last.limit, config.perfTerminalThroughputMin.toDouble());
    expect(targets.last.comparison, PerfComparison.atLeast);
  });

  test(
    'a drawn frame pairs with the rasterized frame whose build ended before it',
    () {
      const frames = [
        (buildFinish: 1000, rasterFinish: 4000),
        (buildFinish: 17000, rasterFinish: 21000),
        (buildFinish: 34000, rasterFinish: 36500),
      ];
      expect(rasterFinishOf(17200, frames), 21000);
      expect(rasterFinishOf(36000, frames), 36500);
      // Before any timing, or too long after the last build (another clock): no pairing.
      expect(rasterFinishOf(900, frames), isNull);
      expect(rasterFinishOf(34000 + maxPaintGapMicros + 1, frames), isNull);
    },
  );

  test('frame to paint per sequence number, also across a split line', () {
    final r = RatePerf();
    r
      ..decrypted(rate(1), 10000)
      ..drawn(rate(1).trim(), 12000)
      // Line 2 arrives in two data frames: it is complete with the second one.
      ..decrypted('TERMGEN 0000', 110000)
      ..decrypted('02 rrrr\r\n', 111000)
      ..drawn('TERMGEN 000001 rrrr\nTERMGEN 000002 rrrr', 116000)
      // Line 3 is drawn twice; only the first frame counts.
      ..decrypted(rate(3), 210000)
      ..drawn('TERMGEN 000003 rrrr', 214000)
      ..drawn('TERMGEN 000003 rrrr', 230000);
    const frames = [
      (buildFinish: 11500, rasterFinish: 14000),
      (buildFinish: 115500, rasterFinish: 119000),
      (buildFinish: 213000, rasterFinish: 217000),
      (buildFinish: 229000, rasterFinish: 233000),
    ];
    expect(r.decryptedLines, 3);
    expect(r.frameToPaintMs(frames), [4.0, 8.0, 7.0]);
    final parts = r.partsMs(frames);
    expect(parts.toFrame, [2.0, 5.0, 4.0]);
    expect(parts.toRaster, [2.0, 3.0, 3.0]);
    expect(r.pairs.first, (decrypted: 10000, drawn: 12000));
    expect(r.done, isFalse);
    r.drawn('TERMGEN-DONE', 300000);
    expect(r.done, isTrue);
  });

  test('a line drawn without a decrypted frame before it is no sample', () {
    final r = RatePerf()..drawn('TERMGEN 000009 rrrr', 5000);
    expect(
      r.frameToPaintMs(const [(buildFinish: 4000, rasterFinish: 6000)]),
      isEmpty,
    );
  });

  test('throughput per chunk without the acknowledgement round trips', () {
    final b = BurstPerf()
      ..beginChunk(0)
      ..decrypted('BURST 00 000000 bbb\r\n', 400000, 1000000)
      ..drawn('BURST 00 000000 bbb', 1050000)
      ..decrypted('BURST 00 000001 bbb\r\n', 600000, 1100000)
      ..decrypted('TERMGEN-END 0 1048576\r\n', 48576, 1200000)
      ..drawn('TERMGEN-END 0 1048576', 1250000);
    expect(b.chunkDrawn(0), isTrue);
    // Waiting for the acknowledgement takes long; it is not part of the number.
    b
      ..beginChunk(1)
      ..decrypted('BURST 01 000000 bbb\r\n', 524288, 9000000)
      ..decrypted('TERMGEN-END 1 524288\r\n', 0, 9100000)
      ..drawn('TERMGEN-END 1 524288', 9150000);
    const frames = [
      (buildFinish: 1049000, rasterFinish: 1060000),
      (buildFinish: 1249000, rasterFinish: 1300000),
      (buildFinish: 9149000, rasterFinish: 9200000),
    ];
    final t = b.throughput(frames)!;
    expect(t.bytes, 1048576 + 524288);
    // 300 ms for the first chunk, 200 ms for the second.
    expect(t.seconds, closeTo(0.5, 1e-9));
    expect(t.bytesPerSecond, closeTo((1048576 + 524288) / 0.5, 1e-6));
    expect(t.chunks, 2);
    // The most output that waited for one drawn frame: 600000 + 48576 bytes.
    expect(t.backlogMaxBytes, 648576);
  });

  test('a chunk that was never drawn gives no throughput', () {
    final b = BurstPerf()
      ..beginChunk(0)
      ..decrypted('BURST 00 000000 bbb\r\n', 1000, 1000)
      ..decrypted('TERMGEN-END 0 1000\r\n', 0, 2000);
    expect(b.chunkDrawn(0), isFalse);
    expect(
      b.throughput(const [(buildFinish: 1000, rasterFinish: 3000)]),
      isNull,
    );
  });

  test('the summary fails a frame to paint with fewer than 1000 frames', () {
    final targets = terminalTargets(AppConfig.defaults());
    final summary = PerfSummary.evaluate(
      targets: targets,
      device: 'test device',
      reference: false,
      samples: {
        'client.perf.terminal_frame_to_paint_p95': [
          for (var i = 0; i < 999; i++) 1.0,
        ],
        'client.perf.terminal_throughput_min': [targets.last.limit * 2],
      },
      details: {'generator': 'rate'},
    );
    final json = summary.toJson();
    final results = json['results']! as Map<String, Object?>;
    final f2p =
        results['client.perf.terminal_frame_to_paint_p95']!
            as Map<String, Object?>;
    expect(f2p['pass'], isFalse);
    expect(f2p['note'], 'fewer than 1000 samples');
    expect(
      (results['client.perf.terminal_throughput_min']! as Map)['pass'],
      isTrue,
    );
    expect(json['details'], {'generator': 'rate'});
    expect(summary.pass, isFalse);
  });
}
