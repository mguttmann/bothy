// SPDX-License-Identifier: Apache-2.0

// The evaluation of the terminal targets of the performance level (brief M4-38 Design note 2;
// docs/spec/15-client-architecture.md 19 and 23.5 row "Performance"). Pure Dart over what the
// timing hooks of terminal_ui (TERMINAL_PERF_HOOKS) and the engine's frame timings recorded, so
// the unit test test/perf/terminal_perf_test.dart runs it on recorded samples.
//
// - Frame to paint (`client.perf.terminal_frame_to_paint_p95`): the generator's rate mode writes
//   one numbered line per interval; per sequence number, from the decrypt of the data frame that
//   completes the line to the end of the rasterization of the first drawn frame that shows it;
//   p95 over at least [terminalFrameSamples] lines.
// - Throughput (`client.perf.terminal_throughput_min`): the burst mode writes chunks at full
//   speed, one per acknowledgement; per chunk, from its first decrypted byte to the end of the
//   rasterization of the frame that shows its end line; the throughput is all bytes over all
//   these times, so the acknowledgement round trips do not count. The backlog of the renderer
//   is the output decrypted since the last drawn frame; its largest value is reported.

import 'package:domain/domain.dart';

import 'perf_summary.dart';

/// The fewest lines of frame to paint (brief M4-38 Design note 2: "p95 over at least 1000
/// frames").
const int terminalFrameSamples = 1000;

/// The longest time from the end of a frame's build to the post-frame callback of the same
/// frame that still pairs them; a larger gap means the clocks are not comparable, and the
/// sample is dropped instead of guessed.
const int maxPaintGapMicros = 50000;

/// One rasterized frame of the engine (`FrameTiming`), in microseconds of `Timeline.now`.
typedef RasterFrame = ({int buildFinish, int rasterFinish});

/// The terminal targets of 15 19 for the desktop, limits from [config] (no literal target).
List<PerfTarget> terminalTargets(AppConfig config) => [
  PerfTarget(
    'client.perf.terminal_frame_to_paint_p95',
    config.perfTerminalFrameToPaintP95.inMicroseconds /
        Duration.microsecondsPerMillisecond,
    'ms',
    PerfComparison.atMost,
    PerfAggregate.p95,
    minimumSamples: terminalFrameSamples,
  ),
  PerfTarget(
    'client.perf.terminal_throughput_min',
    config.perfTerminalThroughputMin.toDouble(),
    'bytes/s',
    PerfComparison.atLeast,
    PerfAggregate.single,
  ),
];

/// The end of the rasterization of the frame a post-frame callback at [paintMicros] belongs to:
/// the frame with the last build end at or before it, within [maxPaintGapMicros]; null when no
/// frame qualifies.
int? rasterFinishOf(int paintMicros, List<RasterFrame> frames) {
  RasterFrame? best;
  for (final f in frames) {
    if (f.buildFinish <= paintMicros &&
        (best == null || f.buildFinish > best.buildFinish)) {
      best = f;
    }
  }
  if (best == null || paintMicros - best.buildFinish > maxPaintGapMicros) {
    return null;
  }
  return best.rasterFinish;
}

/// The characters of a data frame kept for the next one, so a marker split across two frames
/// is found when the second arrives.
const int _carry = 32;

final RegExp _rateLine = RegExp(r'TERMGEN (\d{6})');

/// The recording of the rate mode of one session.
final class RatePerf {
  final Map<int, int> _decryptedAt = {};
  final Map<int, int> _drawnAt = {};
  String _tail = '';
  bool _done = false;

  /// The number of lines decrypted so far.
  int get decryptedLines => _decryptedAt.length;

  /// Whether a drawn frame showed the generator's done line.
  bool get done => _done;

  /// A data frame with [text] was decrypted at [micros].
  void decrypted(String text, int micros) {
    final window = _tail + text;
    for (final m in _rateLine.allMatches(window)) {
      _decryptedAt.putIfAbsent(int.parse(m.group(1)!), () => micros);
    }
    _tail = window.length > _carry
        ? window.substring(window.length - _carry)
        : window;
  }

  /// A frame showing [visible] was drawn at [micros].
  void drawn(String visible, int micros) {
    for (final m in _rateLine.allMatches(visible)) {
      final seq = int.parse(m.group(1)!);
      final at = _decryptedAt[seq];
      if (at != null && at <= micros) _drawnAt.putIfAbsent(seq, () => micros);
    }
    if (visible.contains('TERMGEN-DONE')) _done = true;
  }

  /// Frame to paint in milliseconds per drawn line, in the order of the sequence numbers; a line
  /// whose frame has no rasterization in [frames] gives no sample.
  List<double> frameToPaintMs(List<RasterFrame> frames) {
    final out = <double>[];
    for (final seq in _drawnAt.keys.toList()..sort()) {
      final raster = rasterFinishOf(_drawnAt[seq]!, frames);
      if (raster == null) continue;
      out.add(
        (raster - _decryptedAt[seq]!) / Duration.microsecondsPerMillisecond,
      );
    }
    return out;
  }

  /// The two parts of each sample of [frameToPaintMs], in milliseconds: from the decrypt to the
  /// end of the frame on the UI thread (the post-frame callback), and from there to the end of
  /// its rasterization; a side value of the summary that shows where the time goes.
  ({List<double> toFrame, List<double> toRaster}) partsMs(
    List<RasterFrame> frames,
  ) {
    final toFrame = <double>[];
    final toRaster = <double>[];
    for (final seq in _drawnAt.keys.toList()..sort()) {
      final drawn = _drawnAt[seq]!;
      final raster = rasterFinishOf(drawn, frames);
      if (raster == null) continue;
      toFrame.add(
        (drawn - _decryptedAt[seq]!) / Duration.microsecondsPerMillisecond,
      );
      toRaster.add((raster - drawn) / Duration.microsecondsPerMillisecond);
    }
    return (toFrame: toFrame, toRaster: toRaster);
  }

  /// The latest drawn frame of a line, for the wait for the frame timings.
  int? get lastDrawn =>
      _drawnAt.isEmpty ? null : _drawnAt.values.reduce((a, b) => a > b ? a : b);
}

/// The throughput of the burst mode.
typedef Throughput = ({
  int bytes,
  double seconds,
  double bytesPerSecond,
  int chunks,
  int backlogMaxBytes,
});

/// The recording of the burst mode of one session.
final class BurstPerf {
  int? _chunk;
  final Map<int, int> _start = {};
  final Map<int, int> _bytes = {};
  final Set<int> _ended = {};
  final Map<int, int> _drawnAt = {};
  String _tail = '';
  int _pending = 0;
  int _backlogMax = 0;

  /// The test typed the acknowledgement of chunk [i]; its output starts with the next decrypt.
  void beginChunk(int i) {
    _chunk = i;
    _tail = '';
    _pending = 0;
  }

  /// A data frame with [text] of [bytes] bytes was decrypted at [micros].
  void decrypted(String text, int bytes, int micros) {
    final i = _chunk;
    if (i == null || _ended.contains(i)) return;
    _start.putIfAbsent(i, () => micros);
    _bytes[i] = (_bytes[i] ?? 0) + bytes;
    _pending += bytes;
    final window = _tail + text;
    if (window.contains('TERMGEN-END $i ')) _ended.add(i);
    _tail = window.length > _carry
        ? window.substring(window.length - _carry)
        : window;
  }

  /// A frame showing [visible] was drawn at [micros].
  void drawn(String visible, int micros) {
    final i = _chunk;
    if (i == null || _drawnAt.containsKey(i) || !_start.containsKey(i)) return;
    if (_pending > _backlogMax) _backlogMax = _pending;
    _pending = 0;
    if (_ended.contains(i) && visible.contains('TERMGEN-END $i ')) {
      _drawnAt[i] = micros;
    }
  }

  /// Whether a drawn frame showed the end line of chunk [i].
  bool chunkDrawn(int i) => _drawnAt.containsKey(i);

  /// The latest drawn end line, for the wait for the frame timings.
  int? get lastDrawn =>
      _drawnAt.isEmpty ? null : _drawnAt.values.reduce((a, b) => a > b ? a : b);

  /// All bytes over all chunk times; null when no chunk was drawn or a drawn chunk has no
  /// rasterization in [frames].
  Throughput? throughput(List<RasterFrame> frames) {
    if (_drawnAt.isEmpty) return null;
    var bytes = 0;
    var micros = 0;
    for (final MapEntry(key: i, value: drawn) in _drawnAt.entries) {
      final raster = rasterFinishOf(drawn, frames);
      if (raster == null) return null;
      bytes += _bytes[i]!;
      micros += raster - _start[i]!;
    }
    final seconds = micros / Duration.microsecondsPerSecond;
    return (
      bytes: bytes,
      seconds: seconds,
      bytesPerSecond: bytes / seconds,
      chunks: _drawnAt.length,
      backlogMaxBytes: _backlogMax,
    );
  }
}
