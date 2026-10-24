// SPDX-License-Identifier: Apache-2.0

/// The time range of the statistics screen A7: the presets, a custom range, the bucket size that
/// follows the length of the range and the previous range of the same length for the comparison
/// (brief M6-06 Design note 1).
library;

import 'stats_source.dart';

/// The choices of the time range selector.
enum StatsPreset {
  /// The last 24 hours.
  day(Duration(days: 1)),

  /// The last 7 days.
  week(Duration(days: 7)),

  /// The last 30 days.
  month(Duration(days: 30)),

  /// The last 90 days.
  quarter(Duration(days: 90)),

  /// A range the person picked; its length is its own.
  custom(Duration.zero);

  const StatsPreset(this.length);

  /// The length of the range ending now; zero for [custom].
  final Duration length;
}

/// The longest range shown in hours. Presentation, not a limit: a day in hours gives 24 bars,
/// and two days still fit a chart, while the server keeps its own bound
/// (`api.stats_max_buckets`).
const Duration statsHourlyUpTo = Duration(days: 2);

/// The longest range shown in days; longer ranges are shown in weeks. Presentation, not a limit:
/// about a quarter in days stays readable as bars, and `stats.max_range` stays the server's.
const Duration statsDailyUpTo = Duration(days: 92);

/// The bucket size for a range of [span]: hours up to [statsHourlyUpTo], days up to
/// [statsDailyUpTo], weeks above.
StatsBucketSize statsBucketFor(Duration span) => span <= statsHourlyUpTo
    ? StatsBucketSize.hour
    : span <= statsDailyUpTo
    ? StatsBucketSize.day
    : StatsBucketSize.week;

/// One range of the statistics from [from] (inclusive) to [to] (exclusive), with the [preset]
/// that made it and its [span], the length the comparison steps back.
///
/// The server widens a range to whole buckets (`from` down, `to` up; days and weeks from local
/// midnight of `stats.time_zone`). A preset with day buckets therefore starts one day after
/// `to - span`: the bucket that holds `from` is the first of the range, so "Last 7 days" are
/// today and the six days before it, seven buckets. The previous range `[from - span, from)`
/// is widened to the bucket of `from` as well; that shared bucket is the first of this range
/// and is left out of the previous total ([StatsData.previousRunsTotal]).
///
/// Example:
/// ```dart
/// final range = StatsRange.of(StatsPreset.week, now: DateTime.now());
/// range.bucket; // StatsBucketSize.day
/// range.previous.to == range.from; // true
/// ```
final class StatsRange {
  /// Creates the range; [span] defaults to its length (a custom range).
  StatsRange({
    required this.from,
    required this.to,
    required this.preset,
    Duration? span,
  }) : span = span ?? to.difference(from);

  /// The range of [preset] ending at [now] (UTC); [preset] must not be [StatsPreset.custom].
  ///
  /// With hour buckets (UTC) the range ends at the full hour after [now] and starts [span]
  /// before, so nothing is widened. With day buckets the range must not end on a day boundary
  /// of `stats.time_zone`, which the client does not know: every zone's midnight falls on a
  /// quarter hour of UTC, so an end exactly on a quarter hour moves one millisecond later. The
  /// server then widens the end to the end of today and the start, one day after `to - span`, to
  /// the start of its day, which gives exactly the preset's number of day buckets (see the
  /// class).
  factory StatsRange.of(StatsPreset preset, {required DateTime now}) {
    assert(preset != StatsPreset.custom, 'a custom range has its own ends');
    final utc = now.toUtc();
    final span = preset.length;
    if (statsBucketFor(span) == StatsBucketSize.hour) {
      final hour = DateTime.utc(utc.year, utc.month, utc.day, utc.hour);
      final to = hour == utc ? hour : hour.add(const Duration(hours: 1));
      return StatsRange(
        from: to.subtract(span),
        to: to,
        preset: preset,
        span: span,
      );
    }
    final onQuarter =
        utc.minute % 15 == 0 &&
        utc.second == 0 &&
        utc.millisecond == 0 &&
        utc.microsecond == 0;
    final to = onQuarter ? utc.add(const Duration(milliseconds: 1)) : utc;
    return StatsRange(
      from: to.subtract(span).add(const Duration(days: 1)),
      to: to,
      preset: preset,
      span: span,
    );
  }

  /// Start of the range.
  final DateTime from;

  /// End of the range.
  final DateTime to;

  /// The selector entry.
  final StatsPreset preset;

  /// The length the range stands for: the preset's length, or the length of a custom range.
  final Duration span;

  /// The bucket size for this range ([statsBucketFor] of [span]).
  StatsBucketSize get bucket => statsBucketFor(span);

  /// The range of [span] right before [from], for the comparison of the run tile.
  StatsRange get previous => StatsRange(
    from: from.subtract(span),
    to: from,
    preset: preset,
    span: span,
  );

  @override
  bool operator ==(Object other) =>
      other is StatsRange &&
      other.from == from &&
      other.to == to &&
      other.preset == preset &&
      other.span == span;

  @override
  int get hashCode => Object.hash(from, to, preset, span);
}
