// SPDX-License-Identifier: Apache-2.0

// The run tile and its comparison against a server that aligns ranges (red team M6-06 round 1):
// the server widens `from` down and `to` up to whole buckets (stats/domain.Align, day buckets
// from local midnight of `stats.time_zone`, also across daylight saving time), so a preset of N
// days must show N day buckets, and the previous range of the comparison must count the N day
// buckets before them, never a bucket of the current range. This source aligns calendar days of
// a real IANA zone (rounds 2 and 3: midnight and clock changes) and answers one bucket per day
// with the day of the month as its run count, so each total names exactly the days it counted.

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart';
import 'package:flutter_test/flutter_test.dart';

final class _AligningSource implements AdminStatsSource {
  /// Creates the source of the server zone [zone].
  _AligningSource(this.zone) : days = StatsDays(zone);

  /// The server's `stats.time_zone`.
  final String zone;

  /// Its calendar days, computed independently of the client's ranges: the start of the day
  /// that holds an instant, as `Floor` of the server does it.
  final StatsDays days;

  DateTime floor(DateTime t) => days.start(days.dayOf(t));

  DateTime _ceil(DateTime t) {
    final f = floor(t);
    return f == t.toUtc() ? f : days.start(StatsDays.add(days.dayOf(t), 1));
  }

  StatsSeries _days(StatsQuery q) {
    final buckets = <Object>[];
    if (q.bucket == StatsBucketSize.day) {
      for (
        var d = floor(q.from);
        d.isBefore(_ceil(q.to));
        d = days.start(StatsDays.add(days.dayOf(d), 1))
      ) {
        buckets.add({
          'start': d.toIso8601String(),
          'group_key': 'agent',
          'metrics': {'runs': days.dayOf(d).day.toDouble()},
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
  Future<StatsInstance> statsInstance() async => StatsInstance(timeZone: zone);
}

void main() {
  // Times around the boundaries of the server's days, and the cases of the red team: an hour
  // after midnight and before it shortly after a clock change in Europe and in America, and a
  // quarter whose range holds one.
  final nows = [
    DateTime.utc(2026, 10, 17, 9, 30),
    DateTime.utc(2026, 10, 17, 23, 30),
    DateTime.utc(2026, 10, 18),
    DateTime.utc(2026, 10, 17, 23, 59, 59, 999),
    DateTime.utc(2026, 10, 17, 22),
    DateTime.utc(2026, 3, 30, 22, 30),
    DateTime.utc(2026, 10, 26, 22, 40),
    DateTime.utc(2026, 11, 10, 22, 20),
    DateTime.utc(2026, 6, 1, 22, 10),
    DateTime.utc(2026, 11, 3, 4, 30),
  ];
  const zones = ['UTC', 'Europe/Berlin', 'America/New_York', 'Asia/Kolkata'];

  for (final (preset, count) in [
    (StatsPreset.week, 7),
    (StatsPreset.month, 30),
    (StatsPreset.quarter, 90),
  ]) {
    for (final zone in zones) {
      for (final now in nows) {
        test(
          '${preset.name} at $now in $zone: $count day buckets, compared with the $count '
          'before',
          () async {
            final source = _AligningSource(zone);
            final c = StatsController(source: source, clock: () => now);
            addTearDown(c.dispose);
            await c.start();
            await c.setRange(StatsRange.of(preset, now: now, days: c.days));
            final data = c.data!;
            final starts = data.runs.value!.buckets
                .map((b) => b.start)
                .toList();
            expect(
              starts,
              hasLength(count),
              reason: 'the current range shows $count days',
            );
            // The last bucket is the server's day that holds now.
            expect(starts.last, source.floor(now));
            var total = 0;
            for (final s in starts) {
              total += source.days.dayOf(s).day;
            }
            var previous = 0;
            var day = source.days.dayOf(starts.first);
            for (var i = 0; i < count; i++) {
              day = StatsDays.add(day, -1);
              previous += day.day;
            }
            expect(data.runsTotal, total);
            expect(
              data.previousRunsTotal,
              previous,
              reason:
                  'the $count days before the current range, none of them shared',
            );
            expect(
              data.runsPrevious.value!.buckets,
              hasLength(count),
              reason: 'the server widens nothing',
            );
          },
        );
      }
    }
  }

  test('the 24-hour preset stays on whole UTC hours', () {
    final now = DateTime.utc(2026, 10, 17, 9, 30);
    final day = StatsRange.of(
      StatsPreset.day,
      now: now,
      days: StatsDays('Europe/Berlin'),
    );
    expect(day.bucket, StatsBucketSize.hour);
    expect(day.to, DateTime.utc(2026, 10, 17, 10));
    expect(day.to.difference(day.from), const Duration(hours: 24));
    expect(day.previous.to, day.from);
    expect(day.previous.from, day.from.subtract(const Duration(hours: 24)));
  });

  test('a custom range counts calendar days of the server zone', () {
    final days = StatsDays('Europe/Berlin');
    final r = StatsRange.days(
      first: DateTime(2026, 10, 20),
      last: DateTime(2026, 10, 26),
      days: days,
    );
    // 25 October 2026 is the end of summer time in Berlin: the seven days are 169 hours.
    expect(r.from, DateTime.utc(2026, 10, 19, 22));
    expect(r.to, DateTime.utc(2026, 10, 26, 23));
    expect(r.previous.from, DateTime.utc(2026, 10, 12, 22));
    expect(r.bucket, StatsBucketSize.day);
    expect(StatsDays('No/Such_Zone').zone, 'UTC');
  });

  // Review round 4: hour buckets are UTC on the server, so a short custom range in a zone with
  // a half or quarter hour offset must not use them, or the server widens it into the
  // neighbouring days.
  for (final (zone, count) in [
    ('Asia/Kolkata', 1),
    ('Asia/Kathmandu', 2),
    ('Europe/Berlin', 1),
    ('UTC', 2),
  ]) {
    test('a custom range of $count day(s) in $zone is never widened', () async {
      final source = _AligningSource(zone);
      final c = StatsController(
        source: source,
        clock: () => DateTime.utc(2026, 10, 20, 12),
      );
      addTearDown(c.dispose);
      await c.start();
      final range = StatsRange.days(
        first: DateTime(2026, 10, 10),
        last: DateTime(2026, 10, 9 + count),
        days: c.days,
      );
      // Whole UTC hours for the zones on whole hours (hours), else days.
      final hours = zone == 'Europe/Berlin' || zone == 'UTC';
      expect(range.bucket, hours ? StatsBucketSize.hour : StatsBucketSize.day);
      if (hours) return;
      await c.setRange(range);
      final data = c.data!;
      expect(data.runs.value!.buckets, hasLength(count));
      expect(data.runs.value!.buckets.first.start, range.from);
      expect(data.runsPrevious.value!.buckets, hasLength(count));
      expect(
        data.runsPrevious.value!.buckets.last.start.isBefore(range.from),
        isTrue,
      );
    });
  }
}
