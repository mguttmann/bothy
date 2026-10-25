// SPDX-License-Identifier: Apache-2.0

// The REST source of A7 (brief M6-06 Scope): each statistics operation of M6-05 is called with
// the range, the bucket size and the space as query parameters, the costs with group_by and
// the chosen currency; getInstance gives the telemetry link only when the member is present;
// a currency_required answer names its currencies; a problem becomes SelfActionFailed. A
// routing adapter answers by method and path; no socket is opened.

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart';
import 'package:flutter_test/flutter_test.dart';

import '../access/api_access_repository_test.dart' show RoutingAdapter;

const String _space = '0192f4a0-7b1c-7fd5-831e-d4370e087874';
const String _agent = '0192f4a0-7b1c-7d07-84b0-358556a08a99';

Map<String, Object?> _series(String metric) => {
  'granularity': 'day',
  'buckets': [
    {
      'start': '2026-10-16T00:00:00.000Z',
      'group_key': _space,
      'metrics': {metric: 3.0},
      'below_min_group': ['active_people'],
    },
  ],
};

Map<String, Object?> _instance({String? url}) => {
  'instance_id': '0192f4a0-7b1c-7000-8000-000000000001',
  'branding': {'product_name': 'Team Chat', 'version': 1},
  'locale_default': 'en',
  'spec_version': '1.0.0',
  'features': <Object?>[],
  'limits': {
    'message_max_content_bytes': 262144,
    'message_max_attachments': 10,
    'file_max_upload_bytes': 104857600,
    'thread_title_max_chars': 200,
    'voice_max_duration_ms': 900000,
  },
  'telemetry_dashboard_url': ?url,
  'stats_time_zone': 'Europe/Berlin',
};

Map<String, (int, Object)> _routes({String? url, bool currencies = false}) => {
  'GET /api/v1/stats/usage': (200, _series('messages')),
  'GET /api/v1/stats/runs': (200, _series('runs')),
  'GET /api/v1/stats/tool-calls': (200, _series('calls')),
  'GET /api/v1/stats/errors': (200, _series('run_failure_rate')),
  'GET /api/v1/stats/costs': currencies
      ? (
          400,
          {
            'type': 'https://bothy.example.com/problems/validation_failed',
            'title': 'Validation failed',
            'status': 400,
            'code': 'validation_failed',
            'detail': 'The range has costs in several currencies: EUR, USD.',
            'errors': [
              {
                'pointer': '/currency',
                'code': 'currency_required',
                'detail':
                    'The range has costs in several currencies: EUR, USD.',
              },
            ],
          },
        )
      : (
          200,
          {
            'granularity': 'day',
            'currency': 'EUR',
            'buckets': [
              {
                'start': '2026-10-16T00:00:00.000Z',
                'group_key': _agent,
                'billing_mode': 'personal',
                'amount_micros': 600000,
                'is_estimate': true,
              },
            ],
          },
        ),
  'GET /api/v1/spaces': (
    200,
    {
      'items': [
        {
          'space': {
            'id': _space,
            'name': 'Marketing',
            'sort_key': 'a0',
            'version': 1,
            'created_at': '2026-10-01T00:00:00.000Z',
          },
          'level': 'manage',
          'capabilities': <Object?>[],
        },
      ],
    },
  ),
  'GET /api/v1/instance': (200, _instance(url: url)),
};

void main() {
  late RoutingAdapter adapter;
  late AdminRepository repository;
  final from = DateTime.utc(2026, 10, 10, 9);
  final to = DateTime.utc(2026, 10, 17, 9);

  void build(Map<String, (int, Object)> routes) {
    adapter = RoutingAdapter(routes);
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
    repository = ApiAdminRepository(
      client,
      newIdempotencyKey: () => '0192f4a0-0000-7000-8000-000000000001',
    );
  }

  test(
    'every operation sends the range, the bucket size and the space',
    () async {
      build(_routes());
      final q = StatsQuery(
        from: from,
        to: to,
        bucket: StatsBucketSize.day,
        spaceId: _space,
      );
      final usage = await repository.usageStats(
        q.withBucket(StatsBucketSize.week),
      );
      await repository.runStats(q);
      await repository.toolCallStats(q);
      await repository.errorStats(q);
      final costs = await repository.costStats(
        q,
        groupBy: StatsCostGrouping.agent,
        currency: 'EUR',
      );

      expect(usage.buckets.single.belowMinGroup, ['active_people']);
      expect(costs.currency, 'EUR');
      final queries = [
        for (final s in adapter.seen) s.options.uri.queryParameters,
      ];
      for (final (i, query) in queries.indexed) {
        expect(DateTime.parse(query['from']!), from, reason: 'call $i');
        expect(DateTime.parse(query['to']!), to, reason: 'call $i');
        expect(query['space_id'], _space, reason: 'call $i');
      }
      expect(
        [for (final q in queries) q['granularity']],
        ['week', 'day', 'day', 'day', 'day'],
      );
      expect(queries.last['group_by'], 'agent');
      expect(queries.last['currency'], 'EUR');
    },
  );

  test('the whole instance sends no space', () async {
    build(_routes());
    await repository.runStats(
      StatsQuery(from: from, to: to, bucket: StatsBucketSize.hour),
    );
    final query = adapter.seen.single.options.uri.queryParameters;
    expect(query.containsKey('space_id'), isFalse);
    expect(query['granularity'], 'hour');
  });

  test('currency_required names the currencies of the detail', () async {
    build(_routes(currencies: true));
    final error = await repository
        .costStats(
          StatsQuery(from: from, to: to, bucket: StatsBucketSize.day),
          groupBy: StatsCostGrouping.space,
        )
        .then<Object?>((_) => null, onError: (Object e) => e);
    expect(error, isA<SelfActionFailed>());
    expect(requiredCurrencies(error!), ['EUR', 'USD']);
    expect(requiredCurrencies(StateError('other')), isEmpty);
  });

  test('the telemetry link only with the member of getInstance', () async {
    build(_routes(url: 'https://telemetry.example.com/d/bothy'));
    final owner = await repository.statsInstance();
    expect(
      owner.telemetryDashboardUrl,
      Uri.parse('https://telemetry.example.com/d/bothy'),
    );
    expect(owner.timeZone, 'Europe/Berlin');
    build(_routes());
    expect((await repository.statsInstance()).telemetryDashboardUrl, isNull);
  });

  test('the spaces of the selector', () async {
    build(_routes());
    final spaces = await repository.statsSpaces();
    expect([for (final s in spaces) (s.id, s.name)], [(_space, 'Marketing')]);
  });
}
