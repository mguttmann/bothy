// SPDX-License-Identifier: Apache-2.0

import 'dart:convert';

import 'package:api_client/api_client.dart' as api;
import 'package:bothy_viz/bothy_viz.dart';
import 'package:bothy_viz/src/views/table/executive_table.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support/fixtures.dart';
import 'support/views.dart';

const _support = '0192f4a0-7b1c-7000-8a00-000000000001';

/// Decodes [json] as the generated client decodes a response.
api.StatsSeries _series(Map<String, Object?> json) =>
    api.standardSerializers.deserializeWith(api.StatsSeries.serializer, json)!;

/// One space in two hour buckets, as `getUsageStats`, `getRunStats` and `getErrorStats`
/// answer a query in hours with `space_id`.
api.StatsSeries _hours(Map<String, Object?> first, Map<String, Object?> last) =>
    _series({
      'granularity': 'hour',
      'buckets': [
        {
          'start': '2026-10-15T08:00:00.000Z',
          'group_key': _support,
          'metrics': first,
        },
        {
          'start': '2026-10-15T09:00:00.000Z',
          'group_key': _support,
          'metrics': last,
        },
      ],
    });

/// What the server does to the bounds of a query in hours (`domain.Align`): `from` down to
/// its UTC hour, `to` up to the end of its UTC hour.
(DateTime, DateTime) _serverHours(DateTime from, DateTime to) {
  DateTime floor(DateTime t) => DateTime.utc(t.year, t.month, t.day, t.hour);
  final end = floor(to);
  return (
    floor(from),
    end.isBefore(to) ? end.add(const Duration(hours: 1)) : end,
  );
}

/// The fixtures with the files of [replaced] answered from memory instead.
VizFixtures _fixturesWith(Map<String, Object?> replaced) =>
    VizFixtures((name) async {
      final other = replaced[name];
      return other == null ? memoryFixtures.read(name) : jsonEncode(other);
    });

/// The executive page on [queries] for the host's range [from] to [to].
Future<void> _pumpPage(
  WidgetTester tester,
  ViewWorld world,
  FixtureStatsQuerySource queries, {
  DateTime? from,
  DateTime? to,
  Locale locale = const Locale('en'),
  Key? key,
}) => pumpWorld(
  tester,
  world,
  ExecutiveView(
    key: key,
    live: null,
    queries: queries,
    options: VizHostOptions(
      formFactor: VizFormFactor.desktop,
      scope: world.scope,
      range: VizRange(
        from: from ?? fixtureRange.from,
        to: to ?? fixtureRange.to,
      ),
      names: names,
      clock: world.clock,
    ),
    onOpenThread: (_) {},
  ),
  locale: locale,
);

/// Label to value of every tile on the screen.
Map<String, String> _shown(WidgetTester tester) {
  final texts = VizTexts.of(tester.element(find.byType(MetricTileMark).first));
  return {
    for (final m in tester.widgetList<MetricTileMark>(
      find.byType(MetricTileMark),
    ))
      MetricTileMark.label(texts, m.model): MetricTileMark.value(
        texts,
        m.model,
      ),
  };
}

void main() {
  group('the period of the executive page', () {
    test('is the UTC hour that holds the end of the host range and the hours '
        'before it', () {
      final rolling = executiveRange(
        VizRange(
          from: DateTime.utc(2026, 10, 14, 10, 30),
          to: DateTime.utc(2026, 10, 15, 10, 30),
        ),
      );
      expect(rolling.from, DateTime.utc(2026, 10, 14, 11));
      expect(rolling.to, DateTime.utc(2026, 10, 15, 11));
      final day = executiveRange(fixtureRange);
      expect((day.from, day.to), (fixtureRange.from, fixtureRange.to));
      final short = executiveRange(
        VizRange(
          from: DateTime.utc(2026, 10, 15, 10, 10),
          to: DateTime.utc(2026, 10, 15, 10, 20),
        ),
      );
      expect(short.from, DateTime.utc(2026, 10, 15, 10));
      expect(short.to, DateTime.utc(2026, 10, 15, 11));
    });

    testWidgets('the page asks in hours on bounds the server does not widen', (
      tester,
    ) async {
      final world = ViewWorld(withLive: false);
      final from = DateTime.utc(2026, 10, 14, 9, 30);
      final to = DateTime.utc(2026, 10, 15, 9, 30);
      await _pumpPage(tester, world, world.queries, from: from, to: to);
      final asked = world.queries.rangeQueries;
      expect(
        [for (final q in asked) (q.operation, q.granularity)],
        unorderedEquals([
          ('usage', api.StatsGranularity.hour),
          ('usage', api.StatsGranularity.week),
          ('runs', api.StatsGranularity.hour),
          ('errors', api.StatsGranularity.hour),
          ('costs', api.StatsGranularity.hour),
        ]),
      );
      for (final q in asked) {
        expect(
          (q.from, q.to),
          (DateTime.utc(2026, 10, 14, 10), DateTime.utc(2026, 10, 15, 10)),
          reason: q.operation,
        );
        if (q.granularity == api.StatsGranularity.hour) {
          expect(_serverHours(q.from, q.to), (
            q.from,
            q.to,
          ), reason: 'the server answers ${q.operation} as asked');
        }
      }
      // Without a bucket size the server would take days and answer two calendar days.
      expect(asked.where((q) => q.granularity == null), isEmpty);
    });

    testWidgets('the page names the period of its tiles', (tester) async {
      final world = ViewWorld(withLive: false);
      await _pumpPage(
        tester,
        world,
        world.queries,
        from: DateTime.utc(2026, 10, 14, 9, 30),
        to: DateTime.utc(2026, 10, 15, 9, 30),
      );
      expect(
        find.text('Yesterday, 10:00 AM to Today, 10:00 AM'),
        findsOneWidget,
      );
      await _pumpPage(
        tester,
        ViewWorld(withLive: false),
        world.queries,
        from: DateTime.utc(2026, 10, 14, 9, 30),
        to: DateTime.utc(2026, 10, 15, 9, 30),
        locale: const Locale('de'),
      );
      expect(find.text('Gestern, 10:00 bis Heute, 10:00'), findsOneWidget);
    });
  });

  group('the tiles over the period', () {
    final usage = _hours(
      {'messages': 5, 'messages_human': 3, 'threads_done': 1},
      {
        'messages': 7,
        'messages_human': 4,
        'threads_done': 2,
        'storage_bytes': 4096,
      },
    );
    final runs = _hours(
      {
        'runs': 4,
        'failed': 1,
        'failed_reason_stalled': 1,
        'duration_ms_p95': 900,
      },
      {
        'runs': 6,
        'failed': 2,
        'failed_reason_stalled': 2,
        'duration_ms_p95': 700,
      },
    );
    final errors = _hours(
      {'run_failure_rate': 0.25, 'scan_errors': 1},
      {'run_failure_rate': 0.5, 'scan_errors': 2},
    );

    test('every count is its sum over the period, also the counts by origin, '
        'by reason and of the scans', () {
      final model = ExecutiveModel.from(
        usage: usage,
        runs: runs,
        errors: errors,
      );
      final byName = {for (final t in model.tiles) t.queryMetric: t};
      for (final MapEntry(key: name, value: sum) in {
        'messages': 12,
        'messages_human': 7,
        'threads_done': 3,
        'runs': 10,
        'failed': 3,
        'failed_reason_stalled': 3,
        'scan_errors': 3,
      }.entries) {
        expect(byName[name]!.value, sum, reason: name);
        expect(byName[name]!.since, isNull, reason: '$name covers the period');
      }
    });

    test('a level is the value of its latest bucket and carries the start of '
        'that bucket', () {
      final model = ExecutiveModel.from(
        usage: usage,
        runs: runs,
        errors: errors,
      );
      final byName = {for (final t in model.tiles) t.queryMetric: t};
      final latest = DateTime.utc(2026, 10, 15, 9);
      expect(byName['duration_ms_p95']!.value, 700);
      expect(byName['duration_ms_p95']!.since, latest);
      expect(byName['run_failure_rate']!.value, closeTo(50, 1e-9));
      expect(byName['run_failure_rate']!.since, latest);
      // The current storage belongs to no bucket.
      expect(byName['storage_bytes']!.value, 4096);
      expect(byName['storage_bytes']!.since, isNull);
    });

    testWidgets('the tile and the table name the bucket of a level, not of a '
        'count', (tester) async {
      final tiles = ExecutiveTiles.of(
        ExecutiveData(usage: usage, runs: runs, errors: errors),
        costsVisible: true,
      );
      final handle = tester.ensureSemantics();
      await tester.pumpWidget(
        app(ExecutiveTileGroups(tiles: tiles, now: fixtureNow)),
      );
      const since = 'Since: Today, 9:00 AM';
      // The level tiles: storage has no bucket, the p95 and the failure rate have one.
      expect(find.text(since), findsNWidgets(2));
      final semantics = tester.getSemantics(
        find.bySemanticsLabel(RegExp('^Failed runs, ')),
      );
      expect(semantics.label, 'Failed runs, 50%, $since');
      final texts = VizTexts.of(
        tester.element(find.byType(MetricTileMark).first),
      );
      final table = tileTable(texts, 'Runs', tiles.runs, now: fixtureNow);
      final rows = {for (final r in table.rows) r[0].text: r[2].text};
      expect(rows['Failed runs'], since);
      expect(rows['Runs, Failed'], '');
      handle.dispose();
    });
  });

  group('active people', () {
    Map<String, Object?> week({
      Map<String, Object?> metrics = const {'messages': 52, 'active_people': 6},
      List<String>? withheld,
      bool twoSpaces = false,
    }) => {
      'granularity': 'week',
      'buckets': [
        {
          'start': '2026-10-05T00:00:00.000Z',
          'group_key': _support,
          'metrics': {'messages': 61, 'active_people': 7},
        },
        {
          'start': '2026-10-12T00:00:00.000Z',
          'group_key': _support,
          'metrics': metrics,
          'below_min_group': ?withheld,
        },
        if (twoSpaces)
          {
            'start': '2026-10-12T00:00:00.000Z',
            'group_key': '0192f4a0-7b1c-7000-8a00-000000000002',
            'metrics': {'messages': 12, 'active_people': 4},
          },
      ],
    };

    testWidgets('the page asks the usage in weeks and shows the people of the '
        'current week of one space', (tester) async {
      final world = ViewWorld(
        withLive: false,
        scope: const VizScope.space(_support),
      );
      final queries = FixtureStatsQuerySource(
        _fixturesWith({'StatsSeries.usage.week.json': week()}),
      );
      final handle = tester.ensureSemantics();
      await _pumpPage(tester, world, queries);
      expect(
        queries.rangeQueries.where(
          (q) =>
              q.operation == 'usage' &&
              q.granularity == api.StatsGranularity.week &&
              q.spaceId == _support,
        ),
        hasLength(1),
      );
      expect(_shown(tester)['Active people'], '6');
      final tile = tester.getSemantics(
        find.bySemanticsLabel(RegExp('^Active people, ')),
      );
      expect(tile.label, 'Active people, 6, Since: Monday, 12:00 AM');
      handle.dispose();
    });

    testWidgets('a number below the minimum group reads "fewer than the '
        'minimum group", never a number', (tester) async {
      final world = ViewWorld(withLive: false);
      final queries = FixtureStatsQuerySource(
        _fixturesWith({
          'StatsSeries.usage.week.json': week(
            metrics: {'messages': 52},
            withheld: ['active_people'],
          ),
        }),
      );
      await _pumpPage(tester, world, queries);
      expect(_shown(tester)['Active people'], 'fewer than the minimum group');
      expect(find.text('fewer than the minimum group'), findsOneWidget);
    });

    testWidgets('the people of several spaces are no sum: no tile', (
      tester,
    ) async {
      final world = ViewWorld(withLive: false);
      final queries = FixtureStatsQuerySource(
        _fixturesWith({'StatsSeries.usage.week.json': week(twoSpaces: true)}),
      );
      await _pumpPage(tester, world, queries);
      expect(_shown(tester).keys, isNot(contains('Active people')));
      // The fixture of the instance (two spaces, one below the minimum group) has none either.
      await _pumpPage(
        tester,
        ViewWorld(withLive: false),
        world.queries,
        key: const ValueKey('instance fixture'),
      );
      expect(world.queries.calls, contains('usage'));
      expect(_shown(tester).keys, isNot(contains('Active people')));
    });

    test(
      'the answer in days carries no people, as the server sends it',
      () async {
        final days = await fixtures.series('normal', query: 'usage');
        expect(days.granularity, api.StatsGranularity.day);
        expect(
          days.buckets.where((b) => b.metrics.containsKey('active_people')),
          isEmpty,
        );
        final weeks = await fixtures.series('week', query: 'usage');
        expect(weeks.granularity, api.StatsGranularity.week);
        expect(weeks.buckets.last.belowMinGroup, ['active_people']);
      },
    );
  });

  group('costs in several currencies', () {
    testWidgets('show the first currency with a note, and the rest of the page '
        'stays', (tester) async {
      final world = ViewWorld(withLive: false);
      world.queries.costCurrencies = const ['USD', 'EUR'];
      await _pumpPage(tester, world, world.queries);
      final costs = world.queries.rangeQueries.where(
        (q) => q.operation == 'costs',
      );
      expect([for (final q in costs) q.currency], [null, 'USD']);
      expect(
        find.text(
          'The range has costs in several currencies. The costs show one '
          'currency at a time.',
        ),
        findsOneWidget,
      );
      expect(
        tester
            .widgetList<MetricTileMark>(find.byType(MetricTileMark))
            .where(
              (m) => m.model.costMode != null && m.model.currency == 'USD',
            ),
        isNotEmpty,
      );
      expect(find.byType(TrafficLightMark), findsWidgets);
      expect(_shown(tester)['Messages'], isNotNull);
    });

    testWidgets('one currency needs no second query and no note', (
      tester,
    ) async {
      final world = ViewWorld(withLive: false);
      await _pumpPage(tester, world, world.queries);
      expect(world.queries.calls.where((c) => c == 'costs'), hasLength(1));
      expect(find.textContaining('several currencies'), findsNothing);
    });

    testWidgets('another refusal of the costs still shows its text', (
      tester,
    ) async {
      final world = ViewWorld(withLive: false);
      world.queries.failOn['costs'] = const api.ApiProblem(
        status: 400,
        code: 'validation_failed',
        errors: [api.ProblemFieldError(pointer: '/to', code: 'range_too_long')],
      );
      await _pumpPage(tester, world, world.queries);
      expect(find.byType(MetricTileMark), findsNothing);
      expect(world.queries.calls.where((c) => c == 'costs'), hasLength(1));
    });

    test('the currencies are read from the detail of currency_required', () {
      const detail = 'The range has costs in several currencies: EUR, USD.';
      expect(
        vizRequiredCurrencies(
          const api.ApiProblem(
            status: 400,
            code: 'validation_failed',
            detail: detail,
            errors: [
              api.ProblemFieldError(
                pointer: '/currency',
                code: 'currency_required',
                detail: detail,
              ),
            ],
          ),
        ),
        ['EUR', 'USD'],
      );
      expect(
        vizRequiredCurrencies(
          const api.ApiProblem(status: 403, code: 'forbidden'),
        ),
        isEmpty,
      );
      expect(vizRequiredCurrencies(StateError('offline')), isEmpty);
    });
  });
}
