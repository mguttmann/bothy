// SPDX-License-Identifier: Apache-2.0

// The statistics calls of the fake repository (brief M6-06 Tests), behaving like the server of
// M6-05: without a space only an owner gets an answer (403 otherwise), a space the caller does
// not manage is 404, the costs need their own permission (403 when withheld), several
// currencies without a choice are 400 validation_failed with currency_required naming them, and
// `active_people` below the minimum group is withheld and named in `below_min_group`. The
// numbers are fixed per space, so a test can read them back from the screen.

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart';

/// The space IDs of the fake.
const String fakeMarketing = '0192f4a0-7b1c-7000-8a00-000000000001';

/// The second space; its active people stay below the minimum group.
const String fakeSupport = '0192f4a0-7b1c-7000-8a00-000000000002';

/// The agent IDs of the fake.
const String fakeTexter = '0192f4a0-7b1c-7000-8a00-000000000201';

/// The second agent.
const String fakeHelper = '0192f4a0-7b1c-7000-8a00-000000000202';

/// The telemetry link of the fake.
final Uri fakeTelemetryUrl = Uri.parse('https://telemetry.example.com/d/bothy');

StatsSeries _series(Object json) =>
    standardSerializers.deserializeWith(StatsSeries.serializer, json)!;

CostSeries _costs(Object json) =>
    standardSerializers.deserializeWith(CostSeries.serializer, json)!;

const SelfActionFailed _forbidden = SelfActionFailed(
  ApiProblem(status: 403, code: 'forbidden'),
);

const SelfActionFailed _notFound = SelfActionFailed(
  ApiProblem(status: 404, code: 'not_found'),
);

/// The statistics calls of `FakeAdminRepository`.
mixin FakeStats {
  /// Whether the caller is an owner (statistics of the whole instance and every space).
  bool statsOwner = true;

  /// The spaces the caller manages when not an owner.
  Set<String> statsManaged = {};

  /// Whether `getCostStats` answers (`cost.read.aggregate`).
  bool statsCostsAllowed = true;

  /// The currencies of the costs; more than one needs a choice.
  List<String> statsCurrencyList = ['USD'];

  /// The telemetry link `getInstance` delivers, or null.
  Uri? statsTelemetry;

  /// The spaces of `listSpaces`.
  List<StatsSpace> statsSpaceList = const [
    StatsSpace(id: fakeMarketing, name: 'Marketing'),
    StatsSpace(id: fakeSupport, name: 'Support'),
  ];

  /// Every query in the order of the calls: the operation and the query.
  final List<(String, StatsQuery)> statsCalls = [];

  /// Completes the calls; a test holds them to see the loading state.
  Future<void> statsGate = Future.value();

  Future<void> _check(String op, StatsQuery q) async {
    statsCalls.add((op, q));
    await statsGate;
    final space = q.spaceId;
    if (space == null) {
      if (!statsOwner) throw _forbidden;
      return;
    }
    if (!statsOwner && !statsManaged.contains(space)) throw _notFound;
  }

  List<String> _spaces(StatsQuery q) =>
      q.spaceId == null ? [fakeMarketing, fakeSupport] : [q.spaceId!];

  /// Whether [q] is the previous range of the comparison (it ends before the test's week).
  bool _previous(StatsQuery q) => q.to.isBefore(DateTime.utc(2026, 10, 12));

  String _day(int d) => DateTime.utc(2026, 10, d).toIso8601String();

  Future<StatsSeries> runStats(StatsQuery q) async {
    await _check('runs', q);
    final factor = _spaces(q).length;
    if (_previous(q)) {
      return _series({
        'granularity': 'day',
        'buckets': [
          {
            'start': _day(5),
            'group_key': fakeTexter,
            'metrics': {'runs': 4.0 * factor, 'completed': 4.0 * factor},
          },
        ],
      });
    }
    return _series({
      'granularity': 'day',
      'buckets': [
        {
          'start': _day(15),
          'group_key': fakeTexter,
          'metrics': {
            'runs': 6.0 * factor,
            'completed': 4.0 * factor,
            'failed': 1.0 * factor,
            'cancelled': 1.0 * factor,
            'duration_ms_p50': 1200.0,
            'duration_ms_p95': 4800.0,
            'failed_reason_adapter_error': 1.0 * factor,
          },
        },
        {
          'start': _day(16),
          'group_key': fakeTexter,
          'metrics': {
            'runs': 3.0 * factor,
            'completed': 3.0 * factor,
            'duration_ms_p50': 900.0,
            'duration_ms_p95': 2500.0,
          },
        },
        {
          'start': _day(16),
          'group_key': fakeHelper,
          'metrics': {
            'runs': 2.0 * factor,
            'completed': 1.0 * factor,
            'failed': 1.0 * factor,
            'duration_ms_p50': 300.0,
            'duration_ms_p95': 700.0,
            'failed_reason_cancel_timeout': 1.0 * factor,
          },
        },
      ],
    });
  }

  Future<StatsSeries> toolCallStats(StatsQuery q) async {
    await _check('tools', q);
    return _series({
      'granularity': 'day',
      'buckets': [
        {
          'start': _day(15),
          'group_key': 'web_search',
          'metrics': {'calls': 12.0, 'failed': 2.0, 'duration_ms_p50': 400.0},
        },
        {
          'start': _day(16),
          'group_key': 'read_file',
          'metrics': {'calls': 5.0, 'failed': 0.0},
        },
      ],
    });
  }

  Future<StatsSeries> errorStats(StatsQuery q) async {
    await _check('errors', q);
    return _series({
      'granularity': 'day',
      'buckets': [
        {
          'start': _day(16),
          if (q.spaceId != null) 'group_key': q.spaceId,
          'metrics': {
            'run_failure_rate': 0.25,
            'invocation_denials': 1.0,
            if (q.spaceId == null) 'webhook_failure_rate': 0.1,
            if (q.spaceId == null) 'push_failure_rate_apns': 0.05,
            if (q.spaceId == null) 'scan_infected': 1.0,
          },
        },
      ],
    });
  }

  Future<StatsSeries> usageStats(StatsQuery q) async {
    await _check('usage', q);
    return _series({
      'granularity': q.bucket.name,
      'buckets': [
        for (final space in _spaces(q))
          {
            'start': DateTime.utc(2026, 10, 12).toIso8601String(),
            'group_key': space,
            'metrics': {
              'messages': space == fakeMarketing ? 40.0 : 7.0,
              'threads_created': space == fakeMarketing ? 6.0 : 2.0,
              'threads_done': space == fakeMarketing ? 3.0 : 1.0,
              'runs': space == fakeMarketing ? 9.0 : 2.0,
              if (space == fakeMarketing) 'active_people': 8.0,
              'storage_bytes': space == fakeMarketing ? 5242880.0 : 2048.0,
            },
            if (space == fakeSupport) 'below_min_group': ['active_people'],
          },
      ],
    });
  }

  Future<CostSeries> costStats(
    StatsQuery q, {
    required StatsCostGrouping groupBy,
    String? currency,
  }) async {
    await _check('costs.${groupBy.name}', q);
    if (!statsCostsAllowed) throw _forbidden;
    if (currency == null && statsCurrencyList.length > 1) {
      final detail =
          'The range has costs in several currencies: ${statsCurrencyList.join(', ')}.';
      throw SelfActionFailed(
        ApiProblem(
          status: 400,
          code: 'validation_failed',
          detail: detail,
          errors: [
            ProblemFieldError(
              pointer: '/currency',
              code: 'currency_required',
              detail: detail,
            ),
          ],
        ),
      );
    }
    final code = currency ?? statsCurrencyList.first;
    // EUR costs are a tenth of the USD ones, so a test sees which currency answered.
    final scale = code == 'EUR' ? 0.1 : 1.0;
    final groups = groupBy == StatsCostGrouping.agent
        ? [fakeTexter, fakeHelper]
        : _spaces(q);
    return _costs({
      'granularity': 'day',
      'currency': code,
      'buckets': [
        for (final (i, g) in groups.indexed) ...[
          {
            'start': _day(16),
            'group_key': g,
            'billing_mode': 'organization',
            'amount_micros': ((i == 0 ? 2500000 : 1500000) * scale).round(),
            'is_estimate': i == 0,
          },
          {
            'start': _day(16),
            'group_key': g,
            'billing_mode': 'personal',
            'amount_micros': ((i == 0 ? 600000 : 400000) * scale).round(),
            'is_estimate': true,
          },
        ],
      ],
    });
  }

  Future<List<StatsSpace>> statsSpaces() async => statsSpaceList;

  Future<Map<String, String>> statsAgentNames() async => {
    fakeTexter: 'Texter',
    fakeHelper: 'Helper',
  };

  Future<Uri?> telemetryDashboardUrl() async => statsTelemetry;
}
