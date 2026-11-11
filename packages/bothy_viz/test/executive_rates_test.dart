// SPDX-License-Identifier: Apache-2.0

import 'dart:convert';

import 'package:api_client/api_client.dart' as api;
import 'package:bothy_viz/bothy_viz.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support/fixtures.dart';
import 'support/views.dart';

/// The body of `getErrorStats` for the instance as the server answers it: a rate is a share
/// from 0 to 1 (failed per finished), a push rate carries its provider. The values are the
/// ones the test of the started server process pins (`TestStats`, subtest `ErrorRates`, in
/// `services/server/internal/wire/serve_stats_test.go`).
const _instanceBody = '''
{
  "granularity": "day",
  "buckets": [
    {
      "start": "2026-10-01T22:00:00.000Z",
      "metrics": {
        "run_failure_rate": 0.2,
        "invocation_denials": 2,
        "webhook_failure_rate": 0.5,
        "push_failure_rate_apns": 0.5,
        "push_failure_rate_fcm": 0,
        "scan_errors": 1,
        "scan_infected": 1
      }
    }
  ]
}
''';

/// The same answer for one space (`space_id`): the group key is the space.
const _spaceBody = '''
{
  "granularity": "day",
  "buckets": [
    {
      "start": "2026-10-01T22:00:00.000Z",
      "group_key": "0192f4a0-7b1c-7000-8a00-000000000001",
      "metrics": {"run_failure_rate": 0.25, "invocation_denials": 2}
    }
  ]
}
''';

/// Decodes [body] as the generated client decodes a response.
api.StatsSeries _decode(String body) => api.standardSerializers.deserializeWith(
  api.StatsSeries.serializer,
  jsonDecode(body),
)!;

/// The value text of every tile of the group "Runs and error rate" by its label.
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
  testWidgets(
    'a failure rate of the server (a share from 0 to 1) shows as percent, '
    'a push rate per provider too',
    (tester) async {
      final tiles = ExecutiveTiles.of(
        ExecutiveData(errors: _decode(_instanceBody)),
        costsVisible: true,
      );
      await tester.pumpWidget(app(ExecutiveTileGroups(tiles: tiles)));
      final shown = _shown(tester);
      expect(shown['Failed runs'], '20%');
      expect(shown['Failed webhook deliveries'], '50%');
      expect(shown['Disabled push registrations (APNs)'], '50%');
      expect(shown['Disabled push registrations (FCM)'], '0%');
      expect(
        shown['Refused invocations'],
        '2',
        reason: 'a count stays a count',
      );
      // The texts are on the screen, not only in the helper.
      expect(find.text('20%'), findsOneWidget);
      expect(find.text('50%'), findsNWidgets(2));
    },
  );

  testWidgets('a failure rate of 0.25 of one space shows as 25 %', (
    tester,
  ) async {
    final tiles = ExecutiveTiles.of(
      ExecutiveData(errors: _decode(_spaceBody)),
      costsVisible: true,
    );
    await tester.pumpWidget(app(ExecutiveTileGroups(tiles: tiles)));
    expect(_shown(tester)['Failed runs'], '25%');
    expect(find.text('25%'), findsOneWidget);
    await tester.pumpWidget(
      app(ExecutiveTileGroups(tiles: tiles), locale: const Locale('de')),
    );
    expect(find.text('25 %'), findsOneWidget);
  });

  testWidgets(
    'the executive page shows the rates of the fixture answer as percent',
    (tester) async {
      final world = ViewWorld(withLive: false);
      await pumpWorld(tester, world, world.view(VizViewKind.executive));
      final errors = await tester.runAsync(
        () => memoryFixtures.series('normal', query: 'errors'),
      );
      final latest = errors!.buckets.last.metrics;
      expect(
        latest['run_failure_rate'],
        0.14,
        reason: 'the fixture is a share',
      );
      final shown = _shown(tester);
      expect(shown['Failed runs'], '14%');
      expect(shown['Failed webhook deliveries'], '3%');
      expect(shown['Disabled push registrations (APNs)'], '6%');
    },
  );

  test('the tile of a rate carries percent, the tile of a count its count', () {
    final model = ExecutiveModel.from(errors: _decode(_spaceBody));
    final byName = {for (final t in model.tiles) t.queryMetric: t};
    expect(byName['run_failure_rate']!.unit, TileUnit.percent);
    expect(byName['run_failure_rate']!.value, closeTo(25, 1e-9));
    expect(byName['invocation_denials']!.unit, TileUnit.count);
    expect(byName['invocation_denials']!.value, 2);
  });

  test('a rate is a metric named <what>_rate or the push rate of a provider, '
      'never a count', () {
    for (final name in [
      'run_failure_rate',
      'webhook_failure_rate',
      'push_failure_rate_apns',
      'push_failure_rate_unifiedpush',
    ]) {
      expect(vizIsRate(name), isTrue, reason: name);
    }
    for (final name in [
      'runs',
      'invocation_denials',
      'duration_ms_p95',
      // The runs that failed for a reason whose code reads like a rate are a count.
      'failed_reason_rate_limited',
      'failed_reason_exceeded_rate',
    ]) {
      expect(vizIsRate(name), isFalse, reason: name);
    }
    final model = ExecutiveModel.from(
      runs: _decode(
        '{"granularity": "hour", "buckets": ['
        '{"start": "2026-10-15T08:00:00.000Z", "metrics": {"failed_reason_rate_limited": 2}},'
        '{"start": "2026-10-15T09:00:00.000Z", "metrics": {"failed_reason_rate_limited": 3}}'
        ']}',
      ),
    );
    expect(model.tiles.single.unit, TileUnit.count);
    expect(model.tiles.single.value, 5);
  });
}
