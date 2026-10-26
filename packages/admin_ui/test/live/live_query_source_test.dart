// SPDX-License-Identifier: Apache-2.0

// The REST source of the views (brief M6-16 Scope): each query operation is called with the
// range, the space and its filters, the answer decodes into the generated type, and a refusal
// reaches the views as the ApiProblem with its code (25 11). A routing adapter answers by method
// and path; no socket is opened.

import 'dart:convert';
import 'dart:io' as io;

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart';
import 'package:flutter_test/flutter_test.dart';

import '../access/api_access_repository_test.dart' show RoutingAdapter;

const String _space = '0192f4a0-7b1c-7000-8a00-000000000001';

Object _fixture(String name) =>
    jsonDecode(io.File('../bothy_viz/fixtures/$name').readAsStringSync())
        as Object;

Map<String, Object?> _problem(int status, String code) => {
  'type': 'https://bothy.example.com/problems/$code',
  'title': code,
  'status': status,
  'code': code,
};

void main() {
  final from = DateTime.utc(2026, 10, 15, 9);
  final to = DateTime.utc(2026, 10, 15, 10);

  (ApiStatsQuerySource, RoutingAdapter) build(
    Map<String, (int, Object)> routes,
  ) {
    final adapter = RoutingAdapter(routes);
    final client = createApiClient(
      baseUrl: Uri.parse('https://chat.example.com'),
      traceContext: () => TraceContext(
        traceId: '4bf92f3577b34da6a3ce929d0e0e4736',
        sampled: true,
      ),
      readRetry: ReadRetryPolicy.fromBackoff(
        base: const Duration(milliseconds: 1),
        cap: const Duration(milliseconds: 1),
        sleep: (_) async {},
      ),
      adapter: adapter,
    );
    return (ApiStatsQuerySource(client), adapter);
  }

  test('the views read live, timeline, SLO and backends with their '
      'parameters', () async {
    final (source, adapter) = build({
      'GET /api/v1/stats/live': (200, _fixture('StatsLiveFrame.normal.json')),
      'GET /api/v1/stats/timeline': (
        200,
        _fixture('StatsTimeline.normal.json'),
      ),
      'GET /api/v1/stats/slo': (200, _fixture('StatsSloReport.normal.json')),
      'GET /api/v1/stats/backends': (
        200,
        _fixture('StatsBackendReport.normal.json'),
      ),
    });
    expect((await source.live(spaceId: _space)).runs, isNotEmpty);
    expect(
      (await source.timeline(
        from: from,
        to: to,
        spaceId: _space,
        metrics: [StatsMetric.runsStarted],
      )).runs,
      isNotEmpty,
    );
    expect((await source.slo()).items, isNotEmpty);
    expect((await source.backends(from: from, to: to)).items, isNotEmpty);
    final queries = [
      for (final s in adapter.seen) s.options.uri.queryParameters,
    ];
    expect(queries[0], {'space_id': _space});
    expect(queries[1]['from'], '2026-10-15T09:00:00.000Z');
    expect(queries[1]['space_id'], _space);
    expect(queries[1]['metric'], 'runs_started');
    expect(queries[3].containsKey('space_id'), isFalse);
  });

  for (final (status, code) in const [
    (403, 'forbidden'),
    (404, 'not_found'),
    (403, 'feature_disabled'),
    (429, 'rate_limited'),
  ]) {
    test('a $status $code reaches the views as its ApiProblem', () async {
      final (source, _) = build({
        'GET /api/v1/stats/live': (status, _problem(status, code)),
      });
      await expectLater(
        source.live(),
        throwsA(isA<ApiProblem>().having((p) => p.code, 'code', code)),
      );
    });
  }
}
