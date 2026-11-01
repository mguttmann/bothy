// SPDX-License-Identifier: Apache-2.0

/// Playback of the daily timeline (docs/spec/25-visualization.md 6.5; brief M6-12, design
/// note 5).
library;

import 'package:flutter/foundation.dart';

import '../../model/bar_model.dart';
import '../../model/series_model.dart';
import '../../model/timeline_model.dart';
import '../../sources/viz_clock.dart';

/// Plays a timeline from [from] to [to] minute by minute on an injected [clock].
///
/// The timeline has minute precision (25 6.5), so the playhead moves in whole minutes: at
/// speed `s` (seconds of time of day per second, `viz.timeline_speeds`) one minute of the
/// timeline passes every `60 / s` seconds of the clock. The playhead starts at [to], so the
/// whole period is visible first; [play] at the end starts again at [from], and the playhead
/// stops at [to]. With a [FakeVizClock], tests advance it deterministically.
///
/// ```dart
/// final playback = TimelinePlayback(from: from, to: to, clock: clock, speeds: [60, 300]);
/// playback.play();
/// clock.advance(const Duration(seconds: 1)); // one minute later at speed 60
/// ```
class TimelinePlayback extends ChangeNotifier {
  /// Creates the playback at [to], paused, at the first of [speeds].
  ///
  /// Throws an [ArgumentError] when [speeds] is empty or holds a value below 1.
  TimelinePlayback({
    required this.from,
    required this.to,
    required this.clock,
    required List<int> speeds,
  }) : speeds = List.unmodifiable(speeds),
       _position = _floor(to),
       _speed = speeds.isEmpty ? 0 : speeds.first {
    if (speeds.isEmpty || speeds.any((s) => s < 1)) {
      throw ArgumentError.value(speeds, 'speeds', 'needs values of 1 or more');
    }
  }

  /// The speeds of `viz.timeline_speeds` as numbers; an entry that is not a whole number of
  /// 1 or more is left out.
  static List<int> parseSpeeds(List<String> values) => [
    for (final v in values)
      if (int.tryParse(v.trim()) case final s? when s >= 1) s,
  ];

  /// Start of the timeline.
  final DateTime from;

  /// End of the timeline.
  final DateTime to;

  /// The clock of the ticks.
  final VizClock clock;

  /// The offered speeds in seconds of time of day per second.
  final List<int> speeds;

  DateTime _position;
  int _speed;
  VizTimer? _tick;

  /// The playhead, a whole minute between [from] and [to].
  DateTime get position => _position;

  /// The current speed, one of [speeds].
  int get speed => _speed;

  /// Whether the playback runs.
  bool get playing => _tick != null;

  /// The first stop of the playhead, [from] floored to its minute.
  DateTime get start => _floor(from);

  /// Every whole minute from [from] to [to], the stops of the playhead.
  int get minutes => _floor(to).difference(_floor(from)).inMinutes;

  /// Starts the playback; at the end it starts again at [from].
  void play() {
    if (playing) return;
    if (!_position.isBefore(_floor(to))) _position = _floor(from);
    _schedule();
    notifyListeners();
  }

  /// Stops the playback at the current minute.
  void pause() {
    if (!playing) return;
    _tick?.cancel();
    _tick = null;
    notifyListeners();
  }

  /// Moves the playhead to the minute of [at], clamped to the timeline.
  void jumpTo(DateTime at) {
    final minute = _floor(at);
    final first = _floor(from);
    final last = _floor(to);
    _position = minute.isBefore(first)
        ? first
        : minute.isAfter(last)
        ? last
        : minute;
    notifyListeners();
  }

  /// Sets the speed to [value]; a running playback continues at the new speed.
  ///
  /// Throws an [ArgumentError] when [value] is not one of [speeds].
  void setSpeed(int value) {
    if (!speeds.contains(value)) {
      throw ArgumentError.value(value, 'value', 'not one of $speeds');
    }
    _speed = value;
    if (playing) {
      _tick?.cancel();
      _schedule();
    }
    notifyListeners();
  }

  void _schedule() {
    final interval = Duration(
      microseconds: Duration.microsecondsPerMinute ~/ _speed,
    );
    _tick = clock.schedule(interval, () {
      _position = _position.add(const Duration(minutes: 1));
      if (_position.isBefore(_floor(to))) {
        _schedule();
      } else {
        _position = _floor(to);
        _tick = null;
      }
      notifyListeners();
    });
  }

  @override
  void dispose() {
    _tick?.cancel();
    _tick = null;
    super.dispose();
  }

  static DateTime _floor(DateTime t) {
    final utc = t.toUtc();
    return DateTime.utc(utc.year, utc.month, utc.day, utc.hour, utc.minute);
  }
}

/// [model] as seen at [at]: the series up to [at] and the runs started up to [at], each
/// bar ending at [at] at the latest. Presentation only; nothing is computed beyond cutting.
TimelineModel timelineAt(TimelineModel model, DateTime at) => TimelineModel(
  from: model.from,
  to: model.to,
  runsTruncated: model.runsTruncated,
  bars: BarModel(
    from: model.bars.from,
    to: model.bars.to,
    rows: [
      for (final row in model.bars.rows)
        if (row.segments.any((s) => !s.start.isAfter(at)))
          BarRow(
            label: row.label,
            run: row.run,
            segments: [
              for (final s in row.segments)
                if (!s.start.isAfter(at))
                  BarSegment(
                    start: s.start,
                    end: s.end.isAfter(at) ? at : s.end,
                    runState: s.runState,
                    colorIndex: s.runState == null ? s.colorIndex : null,
                  ),
            ],
          ),
    ],
  ),
  series: [
    for (final s in model.series)
      SeriesModel(
        metric: s.metric,
        queryMetric: s.queryMetric,
        dimension: s.dimension,
        groupKey: s.groupKey,
        groupLabel: s.groupLabel,
        colorIndex: s.colorIndex,
        points: [
          for (final p in s.points)
            if (!p.at.isAfter(at)) p,
        ],
      ),
  ],
);
