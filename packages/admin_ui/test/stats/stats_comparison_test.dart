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
  static DateTime _floor(DateTime t) => DateTime.utc(t.year, t.month, t.day);

  static DateTime _ceil(DateTime t) {
    final f = _floor(t);
    return f == t ? f : f.add(const Duration(days: 1));
  }

  StatsSeries _days(StatsQuery q) {
    final buckets = <Object>[];
    if (q.bucket == StatsBucketSize.day) {
      for (
        var d = _floor(q.from);
        d.isBefore(_ceil(q.to));
        d = d.add(const Duration(days: 1))
      ) {
        buckets.add({
          'start': d.toIso8601String(),
          'group_key': 'agent',
          'metrics': {'runs': d.day.toDouble()},
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
  // Saturday, 17 October 2026, 09:30 UTC.
  final now = DateTime.utc(2026, 10, 17, 9, 30);

  for (final (preset, days) in [
    (StatsPreset.week, 7),
    (StatsPreset.month, 30),
  ]) {
    test(
      '${preset.name}: $days day buckets, compared with the $days before',
      () async {
        final c = StatsController(source: _AligningSource(), clock: () => now);
        addTearDown(c.dispose);
        await c.start();
        if (preset != StatsPreset.week) {
          await c.setRange(StatsRange.of(preset, now: now));
        }
        final data = c.data!;
        final current = data.runs.value!.buckets.map((b) => b.start).toSet();
        expect(
          current,
          hasLength(days),
          reason: 'the current range shows $days days',
        );
        // Today and the days before it; for the month the days run back into September.
        final today = DateTime.utc(2026, 10, 17);
        final first = today.subtract(Duration(days: days - 1));
        final prevFirst = first.subtract(Duration(days: days));
        int total(DateTime from, DateTime toInclusive) {
          var sum = 0;
          for (
            var d = from;
            !d.isAfter(toInclusive);
            d = d.add(const Duration(days: 1))
          ) {
            sum += d.day;
          }
          return sum;
        }

        expect(data.runsTotal, total(first, today));
        expect(
          data.previousRunsTotal,
          total(prevFirst, first.subtract(const Duration(days: 1))),
          reason:
              'the previous range ends the day before the current one starts',
        );
      },
    );
  }

  test('the 24-hour preset stays on whole hours', () {
    final day = StatsRange.of(StatsPreset.day, now: now);
    expect(day.bucket, StatsBucketSize.hour);
    expect(day.to.difference(day.from), const Duration(hours: 24));
    expect(day.previous.to, day.from);
  });
}
