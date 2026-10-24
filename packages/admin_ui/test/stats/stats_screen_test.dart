// SPDX-License-Identifier: Apache-2.0

// A7 "Statistics" (brief M6-06 Tests): the tiles with the comparison and the costs per billing
// mode (personal "information only", never added to organization), withheld counts of people as
// "fewer than", the table views with the values of the charts, the telemetry link only with the
// member of getInstance, the scope selector of a space manager (a refused entry leaves with a
// note) and the currency choice of currency_required. The fake answers like the server of M6-05.

import 'package:admin_ui/admin_ui.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../admin/support.dart';

FakeAdminRepository _repository() =>
    FakeAdminRepository(prompt: StepUpDialogPrompt(() => null));

Future<void> _pump(
  WidgetTester tester,
  FakeAdminRepository r, {
  ValueChanged<Uri>? openExternal,
}) async {
  await pumpInApp(
    tester,
    StatsScreen(
      key: ObjectKey(r),
      source: r,
      timeZone: 'Europe/Berlin',
      clock: testClock,
      openExternal: openExternal,
    ),
  );
  await tester.pumpAndSettle();
}

Finder _in(String key, Finder f) =>
    find.descendant(of: find.byKey(ValueKey(key)), matching: f);

Future<void> _table(WidgetTester tester, String section) async {
  final button = find.byKey(ValueKey('stats.$section.asTable'));
  await tester.ensureVisible(button);
  await tester.tap(button);
  await tester.pumpAndSettle();
}

/// Every cell text of the table of [section].
List<String> _cells(WidgetTester tester, String section) => [
  for (final t in tester.widgetList<Text>(
    _in('stats.$section.table', find.byType(Text)),
  ))
    t.data ?? '',
];

void main() {
  testWidgets(
    'the owner sees the tiles with the comparison and costs per mode',
    (tester) async {
      final r = _repository();
      await _pump(tester, r);

      // 22 runs in the range (both spaces), 8 in the previous week of the same length.
      expect(_in('stats.tile.0', find.text('22')), findsOneWidget);
      expect(
        _in('stats.tile.0', find.text('14 more than the previous period')),
        findsOneWidget,
      );
      // Organization 2.50 + 1.50 with an estimate among them; personal 0.60 + 0.40 is information
      // only and a tile of its own.
      expect(_in('stats.tile.1', find.text('Costs, Organization')), findsOne);
      expect(_in('stats.tile.1', find.textContaining('4.00')), findsOne);
      expect(_in('stats.tile.1', find.text('Estimate')), findsOne);
      expect(_in('stats.tile.2', find.text('Costs, Personal')), findsOne);
      expect(_in('stats.tile.2', find.textContaining('1.00')), findsOne);
      expect(_in('stats.tile.2', find.text('information only')), findsOne);
      expect(find.textContaining('5.00'), findsNothing, reason: 'never summed');

      // The queries: one week by day for both runs ranges, usage in weeks.
      final ops = [
        for (final (op, q) in r.statsCalls) (op, q.bucket, q.spaceId),
      ];
      expect(ops, contains(('runs', StatsBucketSize.day, null)));
      expect(ops, contains(('usage', StatsBucketSize.week, null)));
      final runs = [
        for (final (op, q) in r.statsCalls)
          if (op == 'runs') q,
      ];
      expect(runs, hasLength(2));
      expect(
        runs[1].to,
        runs[0].from,
        reason: 'the previous range ends where it starts',
      );
      // The previous range steps back the whole preset; the current one starts a day later,
      // because the server widens it to the day of its start (stats_comparison_test.dart).
      expect(runs[1].to.difference(runs[1].from), const Duration(days: 7));
      expect(runs[0].to.difference(runs[0].from), const Duration(days: 6));
    },
  );

  testWidgets('withheld active people read "fewer than"', (tester) async {
    await _pump(tester, _repository());
    await _table(tester, 'usage');
    final cells = _cells(tester, 'usage');
    expect(cells, contains('fewer than the minimum group'));
    // Marketing's 8 active people are shown; Support's value never appears as a number.
    expect(cells, containsAllInOrder(['Marketing', 'Monday']));
    expect(cells, contains('8'));
    expect(cells.where((c) => c == '0'), isEmpty);
  });

  testWidgets('every chart has a table with the same values', (tester) async {
    await _pump(tester, _repository());
    const sections = {
      'runs': ['8', '2', 'Thursday', 'Yesterday'],
      'failures': [
        'Error in the adapter',
        'Adapter did not confirm the cancellation',
        '2',
      ],
      'tools': ['web_search', '12', 'read_file', '5'],
      'errors': ['Failed runs', '25%', 'Failed webhook deliveries', '10%'],
      'costs-agent': ['Texter', 'Helper', 'Organization', 'Personal'],
      'costs-space': ['Marketing', 'Support'],
      'duration': ['Texter', 'Helper', '900\u00a0ms', '700\u00a0ms'],
    };
    for (final MapEntry(key: section, value: values) in sections.entries) {
      final chart = tester.getSemantics(
        find.byKey(ValueKey('stats.$section.chart')),
      );
      for (final v in values) {
        expect(chart.label, contains(v), reason: '$section chart: $v');
      }
      await _table(tester, section);
      final cells = _cells(tester, section);
      for (final v in values) {
        expect(
          cells.any((c) => c.contains(v)),
          isTrue,
          reason: '$section table: $v in $cells',
        );
      }
    }
  });

  testWidgets('personal costs are labelled and never added to organization', (
    tester,
  ) async {
    await _pump(tester, _repository());
    await _table(tester, 'costs-agent');
    final cells = _cells(tester, 'costs-agent');
    expect(
      cells.where((c) => c.contains('information only')),
      hasLength(2),
      reason: 'each personal amount of each agent',
    );
    expect(
      cells.any((c) => c.contains('3.10')),
      isFalse,
      reason: '2.50 + 0.60',
    );
  });

  testWidgets('the telemetry link appears only with the member', (
    tester,
  ) async {
    final opened = <Uri>[];
    await _pump(tester, _repository(), openExternal: opened.add);
    expect(find.byKey(const ValueKey('stats.telemetry')), findsNothing);

    final r = _repository()..statsTelemetry = fakeTelemetryUrl;
    await _pump(tester, r, openExternal: opened.add);
    await tester.tap(find.text('Open telemetry dashboard'));
    expect(opened, [fakeTelemetryUrl]);

    // Without a way to open it outside the app the host shows no link.
    await _pump(tester, _repository()..statsTelemetry = fakeTelemetryUrl);
    expect(find.byKey(const ValueKey('stats.telemetry')), findsNothing);
  });

  testWidgets('a space manager: refused entries leave with a note', (
    tester,
  ) async {
    final r = _repository()
      ..statsOwner = false
      ..statsManaged = {fakeSupport}
      ..statsSpaceList = const [
        StatsSpace(id: fakeMarketing, name: 'Marketing'),
        StatsSpace(id: fakeSupport, name: 'Support'),
      ];
    await _pump(tester, r);

    // "All spaces" answered 403, Marketing 404; Support loads.
    expect(
      find.text('All spaces is not offered: You are not allowed to do this'),
      findsOneWidget,
    );
    expect(
      find.text('Marketing is not offered: Not found or no access'),
      findsOneWidget,
    );
    expect(r.statsCalls.last.$2.spaceId, fakeSupport);
    expect(find.byKey(const ValueKey('stats.tiles')), findsOneWidget);
    await tester.tap(find.byKey(const ValueKey('stats.scope')));
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('stats.scope.instance')), findsNothing);
    expect(find.byKey(ValueKey('stats.scope.$fakeMarketing')), findsNothing);
  });

  testWidgets('no entry left shows the server reason', (tester) async {
    final r = _repository()
      ..statsOwner = false
      ..statsSpaceList = const [];
    await _pump(tester, r);
    expect(find.byKey(const ValueKey('admin.denied')), findsOneWidget);
    expect(find.byKey(const ValueKey('stats.tiles')), findsNothing);
  });

  testWidgets('without cost permission the costs are not shown, never zero', (
    tester,
  ) async {
    final r = _repository()..statsCostsAllowed = false;
    await _pump(tester, r);
    expect(_in('stats.tile.1', find.text('not shown')), findsOneWidget);
    expect(
      _in(
        'stats.section.costs-agent',
        find.byKey(const ValueKey('admin.denied')),
      ),
      findsOneWidget,
    );
  });

  testWidgets('several currencies: one at a time, with a choice', (
    tester,
  ) async {
    final r = _repository()..statsCurrencyList = ['EUR', 'USD'];
    await _pump(tester, r);
    expect(
      find.textContaining('The range has costs in several currencies'),
      findsOneWidget,
    );
    // EUR answered first: a tenth of the USD amounts.
    expect(_in('stats.tile.1', find.textContaining('0.40')), findsOne);
    await tester.tap(find.byKey(const ValueKey('stats.currency')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('stats.currency.USD')).last);
    await tester.pumpAndSettle();
    expect(_in('stats.tile.1', find.textContaining('4.00')), findsOne);
  });

  testWidgets('a range change loads again with the bucket of its length', (
    tester,
  ) async {
    final r = _repository();
    await _pump(tester, r);
    await tester.tap(find.byKey(const ValueKey('stats.range')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('stats.range.day')).last);
    await tester.pumpAndSettle();
    final last = r.statsCalls.where((c) => c.$1 == 'runs').toList();
    expect(last[last.length - 2].$2.bucket, StatsBucketSize.hour);
    expect(last[last.length - 2].$2.to, DateTime.utc(2026, 10, 17, 9));
  });

  testWidgets('the custom range picker opens late in the day', (tester) async {
    // At 23:30 the preset ends at the next full hour, tomorrow; the picker's initial days must
    // still end today (its last date), or Flutter refuses the range.
    final late = DateTime.utc(2026, 10, 17, 23, 30);
    final r = _repository();
    await pumpInApp(tester, StatsScreen(source: r, clock: () => late));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('stats.range')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('stats.range.custom')).last);
    await tester.pumpAndSettle();
    expect(tester.takeException(), isNull);
    expect(find.byType(DateRangePickerDialog), findsOneWidget);
  });

  testWidgets('loading shows its state with icon and text', (tester) async {
    final r = _repository()
      ..statsGate = Future.delayed(const Duration(days: 1));
    await pumpInApp(tester, StatsScreen(source: r, clock: testClock));
    await tester.pump();
    expect(find.byKey(const ValueKey('stats.loading')), findsOneWidget);
    expect(find.text('Loading statistics'), findsOneWidget);
    // Let the held queries finish, so no timer outlives the test.
    await tester.pump(const Duration(days: 1));
    await tester.pumpAndSettle();
  });

  test('the bucket follows the length of the range', () {
    final now = DateTime.utc(2026, 10, 17, 9, 30);
    expect(
      StatsRange.of(StatsPreset.day, now: now).bucket,
      StatsBucketSize.hour,
    );
    expect(
      StatsRange.of(StatsPreset.week, now: now).bucket,
      StatsBucketSize.day,
    );
    expect(
      StatsRange.of(StatsPreset.quarter, now: now).bucket,
      StatsBucketSize.day,
    );
    final year = StatsRange(
      from: now.subtract(const Duration(days: 365)),
      to: now,
      preset: StatsPreset.custom,
    );
    expect(year.bucket, StatsBucketSize.week);
    // Presets end at the full hour after now, so both ranges of the comparison are whole hours.
    final day = StatsRange.of(StatsPreset.day, now: now);
    expect(day.to, DateTime.utc(2026, 10, 17, 10));
    expect(day.previous.to, day.from);
  });
}
