// SPDX-License-Identifier: Apache-2.0

// The run tile and its comparison against a server that aligns ranges (red team M6-06 round 1):
// the server widens `from` down and `to` up to whole buckets (stats/domain.Align), so a preset
// of N days must show N day buckets, and the previous range of the comparison must count the N
// day buckets before them, never a bucket of the current range. This source aligns days in UTC
// and answers one bucket per day with the day of the month as its run count, so each total
// names exactly the days it counted.

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart';
import 'package:flutter_test/flutter_test.dart';

final class _AligningSource implements AdminStatsSource {
  /// Creates the source for a `stats.time_zone` [offset] from UTC.
  _AligningSource(this.offset);

  /// The offset of the server's zone; day buckets start at its local midnight.
  final Duration offset;

  /// The start of the local day that holds [t], as a UTC instant.
  DateTime _floor(DateTime t) {
    final local = t.toUtc().add(offset);
    return DateTime.utc(local.year, local.month, local.day).subtract(offset);
  }

  DateTime _ceil(DateTime t) {
    final f = _floor(t);
    return f == t.toUtc() ? f : _floor(f.add(const Duration(hours: 36)));
  }

  /// The local day of the month of the bucket that starts at [start].
  int _day(DateTime start) => start.add(offset).day;

  StatsSeries _days(StatsQuery q) {
    final buckets = <Object>[];
    if (q.bucket == StatsBucketSize.day) {
      for (
        var d = _floor(q.from);
        d.isBefore(_ceil(q.to));
        d = _floor(d.add(const Duration(hours: 36)))
      ) {
        buckets.add({
          'start': d.toIso8601String(),
          'group_key': 'agent',
          'metrics': {'runs': _day(d).toDouble()},
        });
      }
    }
    return standardSerializers.deserializeWith(StatsSeries.serializer, {
      'granularity': q.bucket.name,
      'buckets': buckets,
    })!;
  }

  StatsSeries _none(StatsQuery q) => standardSerializers.deserializeWith(
    StatsSeries.serializer,
    {'granularity': q.bucket.name, 'buckets': <Object>[]},
  )!;

  @override
  Future<StatsSeries> runStats(StatsQuery q) async => _days(q);

  @override
  Future<StatsSeries> usageStats(StatsQuery q) async => _none(q);

  @override
  Future<StatsSeries> toolCallStats(StatsQuery q) async => _none(q);

  @override
  Future<StatsSeries> errorStats(StatsQuery q) async => _none(q);

  @override
  Future<CostSeries> costStats(
    StatsQuery q, {
    required StatsCostGrouping groupBy,
    String? currency,
  }) async => standardSerializers.deserializeWith(CostSeries.serializer, {
    'granularity': q.bucket.name,
    'currency': 'USD',
    'buckets': <Object>[],
  })!;

  @override
  Future<List<StatsSpace>> statsSpaces() async => const [];

  @override
  Future<Map<String, String>> statsAgentNames() async => const {};

  @override
  Future<Uri?> telemetryDashboardUrl() async => null;
}

void main() {
  // Times of day around the boundaries of the server's days: the middle of a day, the hour
  // before midnight, midnight itself and the last millisecond before it.
  final nows = [
    DateTime.utc(2026, 10, 17, 9, 30),
    DateTime.utc(2026, 10, 17, 23, 30),
    DateTime.utc(2026, 10, 18),
    DateTime.utc(2026, 10, 17, 23, 59, 59, 999),
    DateTime.utc(2026, 10, 17, 22),
    DateTime.utc(2026, 10, 17, 22, 15),
  ];
  // The server's zone: UTC, east of it (midnight at 22:00 UTC) and west of it.
  const offsets = [Duration.zero, Duration(hours: 2), Duration(hours: -5)];

  for (final (preset, days) in [
    (StatsPreset.week, 7),
    (StatsPreset.month, 30),
    (StatsPreset.quarter, 90),
  ]) {
    for (final offset in offsets) {
      for (final now in nows) {
        test(
          '${preset.name} at $now, zone $offset: $days day buckets, compared with the $days '
          'before',
          () async {
            final source = _AligningSource(offset);
            final c = StatsController(source: source, clock: () => now);
            addTearDown(c.dispose);
            await c.start();
            await c.setRange(StatsRange.of(preset, now: now));
            final data = c.data!;
            final starts = data.runs.value!.buckets
                .map((b) => b.start)
                .toList();
            expect(
              starts,
              hasLength(days),
              reason: 'the current range shows $days days',
            );
            // The last bucket is the server's day that holds now.
            expect(starts.last, source._floor(now));
            final current = [for (final s in starts) source._day(s)];
            final first = starts.first;
            final previous = <int>[];
            var d = first;
            for (var i = 0; i < days; i++) {
              d = source._floor(d.subtract(const Duration(hours: 12)));
              previous.add(source._day(d));
            }
            expect(data.runsTotal, current.fold<int>(0, (a, b) => a + b));
            expect(
              data.previousRunsTotal,
              previous.fold<int>(0, (a, b) => a + b),
              reason:
                  'the $days days before the current range, none of them shared',
            );
          },
        );
      }
    }
  }

  test('the 24-hour preset stays on whole hours', () {
    final now = DateTime.utc(2026, 10, 17, 9, 30);
    final day = StatsRange.of(StatsPreset.day, now: now);
    expect(day.bucket, StatsBucketSize.hour);
    expect(day.to, DateTime.utc(2026, 10, 17, 10));
    expect(day.to.difference(day.from), const Duration(hours: 24));
    expect(day.previous.to, day.from);
    expect(day.previous.from, day.from.subtract(const Duration(hours: 24)));
  });
}
