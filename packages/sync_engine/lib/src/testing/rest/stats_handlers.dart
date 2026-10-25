// SPDX-License-Identifier: Apache-2.0

/// Handlers of the statistics operations of M6-05 (brief M6-06 Tests: A7 against the fake
/// server): `getUsageStats`, `getRunStats`, `getToolCallStats`, `getErrorStats` and
/// `getCostStats`.
///
/// The fake does not count anything: each operation answers the buckets a test stored in
/// [FakeWorld.stats], with the requested granularity (default `day`). It keeps the server's
/// authorization (admin-statistics.md 4): without `space_id` only an owner (`403`), a space that
/// does not exist or that the person does not see is `404`, a seen space without `manage` is
/// `403`. Simplification against the server: `manage` on a space is `manage` in every channel
/// of it, as in the hint handlers.
library;

import '../action_catalog.dart';
import '../rest_fake.dart';
import '../world.dart';
import 'admin_handlers.dart' show requireCatalogStepUp;

/// Registers the five statistics operations on [rest]; [catalog] gives the step-up of
/// `stats.read` and `cost.read.aggregate`, which ask nothing today.
void registerStatsHandlers(RestFake rest, ActionCatalog catalog) {
  final world = rest.world;

  bool manages(String personId, String space) {
    final own = [
      for (final c in world.channels.values)
        if (c.spaceId == space) c,
    ];
    return own.isNotEmpty &&
        own.every((c) => c.capabilities[personId]?.contains('manage') ?? false);
  }

  void authorize(FakeRequest r) {
    final p = r.person;
    final space = r.query('space_id');
    if (space == null) {
      if (!p.isOwner) throw const FakeProblem(403, 'forbidden');
    } else {
      if (!world.spaces.containsKey(space) ||
          (!p.isOwner && !world.seesSpace(p.id, space))) {
        throw const FakeProblem(404, 'not_found');
      }
      if (!p.isOwner && !manages(p.id, space)) {
        throw const FakeProblem(403, 'forbidden');
      }
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
}
