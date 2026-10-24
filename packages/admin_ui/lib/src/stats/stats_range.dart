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

/// One range of the statistics from [from] (inclusive) to [to] (exclusive), with the [preset]
/// that made it.
///
/// Example:
/// ```dart
/// final range = StatsRange.of(StatsPreset.week, now: DateTime.now());
/// range.bucket; // StatsBucketSize.day
/// range.previous.to == range.from; // true
/// ```
final class StatsRange {
  /// Creates the range.
  const StatsRange({
    required this.from,
    required this.to,
    required this.preset,
  });

  /// The range of [preset] ending at the full hour after [now] (UTC); [preset] must not be
  /// [StatsPreset.custom]. Both ends on full hours keep the previous range of the comparison
  /// from sharing an hourly bucket with this one when the server aligns them.
  factory StatsRange.of(StatsPreset preset, {required DateTime now}) {
    assert(preset != StatsPreset.custom, 'a custom range has its own ends');
    final utc = now.toUtc();
    final hour = DateTime.utc(utc.year, utc.month, utc.day, utc.hour);
    final to = hour == utc ? hour : hour.add(const Duration(hours: 1));
    return StatsRange(from: to.subtract(preset.length), to: to, preset: preset);
  }

  /// Start of the range.
  final DateTime from;

  /// End of the range.
  final DateTime to;

  /// The selector entry.
  final StatsPreset preset;

  /// The length of the range.
  Duration get length => to.difference(from);

  /// The bucket size for this range: hours up to [statsHourlyUpTo], days up to
  /// [statsDailyUpTo], weeks above.
  StatsBucketSize get bucket => length <= statsHourlyUpTo
      ? StatsBucketSize.hour
      : length <= statsDailyUpTo
      ? StatsBucketSize.day
      : StatsBucketSize.week;

  /// The range of the same length right before this one, for the comparison of the run tile.
  StatsRange get previous =>
      StatsRange(from: from.subtract(length), to: from, preset: preset);

  @override
  bool operator ==(Object other) =>
      other is StatsRange &&
      other.from == from &&
      other.to == to &&
      other.preset == preset;

  @override
  int get hashCode => Object.hash(from, to, preset);
}
