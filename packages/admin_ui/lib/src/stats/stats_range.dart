// SPDX-License-Identifier: Apache-2.0

/// The time range of the statistics screen A7: the presets, a custom range, the bucket size that
/// follows the length of the range and the previous range of the same length for the comparison
/// (brief M6-06 Design note 1).
library;

import 'dart:ui' show Locale;

import 'package:l10n/l10n.dart' show L10nFormat;

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

/// Calendar days of `stats.time_zone`, the zone in which the server starts its day buckets
/// (`InstanceInfo.stats_time_zone`); an unknown zone counts in UTC.
///
/// Example: `StatsDays('Europe/Berlin').start(DateTime.utc(2026, 10, 17))` is the instant
/// 2026-10-16 22:00 UTC.
final class StatsDays {
  /// Creates the days of [zone].
  StatsDays(String? zone) : zone = _known(zone);

  /// The IANA zone, `UTC` when none or an unknown one was given.
  final String zone;

  static const L10nFormat _format = L10nFormat(Locale('en'));

  static String _known(String? zone) {
    if (zone == null || zone.isEmpty) return 'UTC';
    try {
      _format.dayStart(2000, 1, 1, timeZone: zone);
      return zone;
    } on ArgumentError {
      return 'UTC';
    }
  }

  /// The instant at which the calendar day [date] (only its year, month and day count) starts.
  DateTime start(DateTime date) =>
      _format.dayStart(date.year, date.month, date.day, timeZone: zone);

  /// The calendar day that holds [instant], as a UTC date.
  DateTime dayOf(DateTime instant) {
    final t = instant.toUtc();
    // A zone's day starts at most 14 hours from UTC midnight; of the three candidates the latest
    // start not after the instant is its day.
    var day = DateTime.utc(t.year, t.month, t.day + 1);
    while (start(day).isAfter(t)) {
      day = DateTime.utc(day.year, day.month, day.day - 1);
    }
    return day;
  }

  /// The calendar day [days] after [date].
  static DateTime add(DateTime date, int days) =>
      DateTime.utc(date.year, date.month, date.day + days);
}

/// One range of the statistics from [from] (inclusive) to [to] (exclusive), with the [preset]
/// that made it and the start of the [previous] range of the comparison, which ends at [from].
///
/// The server widens a range to whole buckets (`from` down, `to` up; days and weeks from local
/// midnight of `stats.time_zone`, hours in UTC). The ranges here start and end on such buckets
/// already, so nothing is widened: the 24-hour preset on whole UTC hours, every other range on
/// calendar days of [StatsDays]. "Last 7 days" are today and the six days before it, and the
/// previous range the seven days before those, also across a change of daylight saving time.
///
/// Example:
/// ```dart
/// final range = StatsRange.of(StatsPreset.week, now: DateTime.now(), days: StatsDays('UTC'));
/// range.bucket; // StatsBucketSize.day
/// range.previous.to == range.from; // true
/// ```
final class StatsRange {
  /// Creates the range; [previousFrom] defaults to [from] minus the length of the range.
  StatsRange({
    required this.from,
    required this.to,
    required this.preset,
    DateTime? previousFrom,
  }) : previousFrom = previousFrom ?? from.subtract(to.difference(from));

  /// The range of [preset] ending now; [preset] must not be [StatsPreset.custom].
  ///
  /// The 24-hour preset ends at the full UTC hour after [now] (hour buckets are UTC). The day
  /// presets are the calendar days of [days] up to today: N days end at the start of tomorrow,
  /// and the previous range holds the N days before them.
  factory StatsRange.of(
    StatsPreset preset, {
    required DateTime now,
    required StatsDays days,
  }) {
    assert(preset != StatsPreset.custom, 'a custom range has its own ends');
    final utc = now.toUtc();
    final length = preset.length;
    if (statsBucketFor(length) == StatsBucketSize.hour) {
      final hour = DateTime.utc(utc.year, utc.month, utc.day, utc.hour);
      final to = hour == utc ? hour : hour.add(const Duration(hours: 1));
      return StatsRange(from: to.subtract(length), to: to, preset: preset);
    }
    return StatsRange.days(
      first: StatsDays.add(days.dayOf(utc), 1 - length.inDays),
      last: days.dayOf(utc),
      days: days,
      preset: preset,
    );
  }

  /// The calendar days [first] to [last] (both included) of [days], with the same number of
  /// days before them as the previous range; [preset] is [StatsPreset.custom] for a picked
  /// range.
  factory StatsRange.days({
    required DateTime first,
    required DateTime last,
    required StatsDays days,
    StatsPreset preset = StatsPreset.custom,
  }) {
    final count =
        DateTime.utc(
          last.year,
          last.month,
          last.day,
        ).difference(DateTime.utc(first.year, first.month, first.day)).inDays +
        1;
    return StatsRange(
      from: days.start(first),
      to: days.start(StatsDays.add(last, 1)),
      preset: preset,
      previousFrom: days.start(StatsDays.add(first, -count)),
    );
  }

  /// Start of the range.
  final DateTime from;

  /// End of the range.
  final DateTime to;

  /// The selector entry.
  final StatsPreset preset;

  /// Start of the previous range of the comparison.
  final DateTime previousFrom;

  /// The bucket size: hours for the 24-hour preset, else by the length of the range
  /// ([statsBucketFor]); a range of a few days around a clock change stays in days.
  StatsBucketSize get bucket => preset == StatsPreset.custom
      ? statsBucketFor(to.difference(from))
      : statsBucketFor(preset.length);

  /// The range right before this one with as many buckets, for the comparison of the run tile.
  StatsRange get previous => StatsRange(
    from: previousFrom,
    to: from,
    preset: preset,
    previousFrom: previousFrom,
  );

  @override
  bool operator ==(Object other) =>
      other is StatsRange &&
      other.from == from &&
      other.to == to &&
      other.preset == preset &&
      other.previousFrom == previousFrom;

  @override
  int get hashCode => Object.hash(from, to, preset, previousFrom);
}
