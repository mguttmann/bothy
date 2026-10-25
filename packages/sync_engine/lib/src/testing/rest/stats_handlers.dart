// SPDX-License-Identifier: Apache-2.0

/// Handlers of the statistics operations of M6-05 (brief M6-06 Tests: A7 against the fake
/// server): `getUsageStats`, `getRunStats`, `getToolCallStats`, `getErrorStats` and
/// `getCostStats`; and of the views of M6-16: `getStatsLive`, `getStatsTimeline`,
/// `getStatsSlo` and `getStatsBackends`.
///
/// The fake does not count anything: each operation answers the buckets a test stored in
/// [FakeWorld.stats], with the requested granularity (default `day`); the views answer the
/// body a test stored there (a fixture of `bothy_viz`, 25 7.2 item 3) or an empty one. It keeps the server's
/// authorization (admin-statistics.md 4): without `space_id` only an owner (`403`), a space that
/// does not exist or that the person does not see is `404`, a seen space without `manage` is
/// `403`. Simplification against the server: `manage` on a space is `manage` in every channel
/// of it, as in the hint handlers.
library;

import 'dart:convert';

import '../action_catalog.dart';
import '../rest_fake.dart';
import '../world.dart';
import 'admin_handlers.dart' show requireCatalogStepUp;

/// Registers the five statistics operations on [rest]; [catalog] gives the step-up of
/// `stats.read` and `cost.read.aggregate`, which ask nothing today.
void registerStatsHandlers(RestFake rest, ActionCatalog catalog) {
  final world = rest.world;

  void authorize(FakeRequest r) {
    final code = statsDenial(world, r.person, r.query('space_id'));
    if (code != null) {
      throw FakeProblem(code == 'not_found' ? 404 : 403, code);
    }
    requireCatalogStepUp(r, rest.contract, catalog);
  }

  for (final op in const [
    'getUsageStats',
    'getRunStats',
    'getToolCallStats',
    'getErrorStats',
  ]) {
    rest.handle(op, (r) {
      authorize(r);
      return FakeResponse(200, {
        'granularity': r.query('granularity') ?? 'day',
        'buckets': world.stats[op]?['buckets'] ?? <Object?>[],
      });
    });
  }
  rest.handle('getCostStats', (r) {
    authorize(r);
    final stored = world.stats['getCostStats'];
    return FakeResponse(200, {
      'granularity': r.query('granularity') ?? 'day',
      'currency': r.query('currency') ?? stored?['currency'] ?? 'USD',
      'buckets': stored?['buckets'] ?? <Object?>[],
    });
  });

  // The views of M6-16 (25 5, 6): the stored body, else an empty one.
  JsonMap stored(String op, JsonMap empty) =>
      jsonDecode(jsonEncode(world.stats[op] ?? empty)) as JsonMap;

  rest.handle('getStatsLive', (r) {
    if (!world.statsLiveEnabled) {
      throw const FakeProblem(403, 'feature_disabled');
    }
    authorize(r);
    final space = r.query('space_id');
    final body = stored('getStatsLive', {
      'at': world.stamp(),
      'interval_ms': 1000,
      'runs': <Object?>[],
      'runs_truncated': false,
      'agents': <Object?>[],
      'rates': <Object?>[],
    });
    body['scope'] = statsScopeOf(space);
    if (space != null) body.remove('sli');
    return FakeResponse(200, body);
  });
  rest.handle('getStatsTimeline', (r) {
    authorize(r);
    // The server's range rule (25 11, ValidateTimeline): both ends full minutes, the range not
    // longer than stats.timeline_max_range; retention is not modelled.
    final from = DateTime.tryParse(r.query('from') ?? '');
    final to = DateTime.tryParse(r.query('to') ?? '');
    bool fullMinute(DateTime t) => t.millisecondsSinceEpoch % 60000 == 0;
    final errors = <JsonMap>[
      if (from == null || to == null || !from.isBefore(to))
        {'pointer': '/from', 'code': 'invalid_range'}
      else ...[
        if (!fullMinute(from)) {'pointer': '/from', 'code': 'not_full_minute'},
        if (!fullMinute(to)) {'pointer': '/to', 'code': 'not_full_minute'},
        if (to.difference(from) > world.statsTimelineMaxRange)
          {'pointer': '/to', 'code': 'range_too_long'},
      ],
    ];
    if (errors.isNotEmpty) {
      throw FakeProblem(400, 'validation_failed', errors: errors);
    }
    return FakeResponse(
      200,
      stored('getStatsTimeline', {
        'from': r.query('from') ?? world.stamp(),
        'to': r.query('to') ?? world.stamp(),
        'buckets': <Object?>[],
        'runs': <Object?>[],
        'runs_truncated': false,
      }),
    );
  });
  rest.handle('getStatsSlo', (r) {
    // The traffic light is the instance's own measurement: owners only (25 11).
    if (!r.person.isOwner) throw const FakeProblem(403, 'forbidden');
    requireCatalogStepUp(r, rest.contract, catalog);
    return FakeResponse(
      200,
      stored('getStatsSlo', {
        'at': world.stamp(),
        'measured_by': 'own',
        'items': <Object?>[],
      }),
    );
  });
  rest.handle('getStatsBackends', (r) {
    authorize(r);
    return FakeResponse(
      200,
      stored('getStatsBackends', {'items': <Object?>[]}),
    );
  });
}

/// The problem code that refuses `stats.read` of [person] on [spaceId] (null: the instance),
/// or null when allowed; the REST operations and `stats.subscribe` share it (admin-statistics.md
/// 4; 25 5.1 item 1).
String? statsDenial(FakeWorld world, FakePerson person, String? spaceId) {
  if (spaceId == null) return person.isOwner ? null : 'forbidden';
  if (!world.spaces.containsKey(spaceId) ||
      (!person.isOwner && !world.seesSpace(person.id, spaceId))) {
    return 'not_found';
  }
  if (person.isOwner) return null;
  final own = [
    for (final c in world.channels.values)
      if (c.spaceId == spaceId) c,
  ];
  final manages =
      own.isNotEmpty &&
      own.every((c) => c.capabilities[person.id]?.contains('manage') ?? false);
  return manages ? null : 'forbidden';
}

/// The `scope` member of a live frame for [spaceId] (null: the instance).
JsonMap statsScopeOf(String? spaceId) => spaceId == null
    ? {'kind': 'instance'}
    : {'kind': 'space', 'space_id': spaceId};
