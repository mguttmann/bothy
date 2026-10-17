// SPDX-License-Identifier: Apache-2.0

/// Handlers of `getSnapshot` and `listEvents` (04 7.3, 7.4).
library;

import '../rest_fake.dart';
import '../world.dart';

/// Registers the sync handlers on [rest].
///
/// `listEvents` applies the rules of the gateway catch-up: an epoch that is not the current
/// one, a cursor before the retained log, a backlog above `catchupMaxEvents` or a cursor
/// ahead of the head answer `410 resync_required` with the reason.
void registerSyncHandlers(RestFake rest) {
  final world = rest.world;
  rest
    ..handle(
      'getSnapshot',
      (r) => FakeResponse(200, world.snapshot(r.person.id)),
    )
    ..handle('listEvents', (r) {
      final after = int.tryParse(r.query('after') ?? '');
      if (after == null) {
        throw const FakeProblem(
          400,
          'validation_failed',
          errors: [
            {'pointer': '#query/after', 'code': 'required'},
          ],
        );
      }
      final reason = resyncReason(world, after, r.query('epoch'));
      if (reason != null)
        throw FakeProblem(410, 'resync_required', reason: reason);
      final requested =
          int.tryParse(r.query('limit') ?? '') ?? world.catchupPageSize;
      final limit = requested.clamp(1, world.catchupPageSize);
      final checked = world.eventsAfter(after).take(limit).toList();
      return FakeResponse(200, {
        'events': [
          for (final e in checked)
            if (world.mayReceive(r.person.id, e)) e.envelope,
        ],
        'next_cursor': checked.isEmpty ? after : checked.last.seq,
        'head_seq': world.headSeq,
        'sync_epoch': world.syncEpoch,
      });
    });
}

/// The reason of `resync_required` for a catch-up from [cursor] with [epoch] (04 7.3), or
/// null when the catch-up is possible.
String? resyncReason(FakeWorld world, int cursor, String? epoch) {
  if (epoch != null && epoch != world.syncEpoch) return 'epoch_changed';
  if (cursor > world.headSeq) return 'cursor_ahead';
  if (cursor < world.minRetainedSeq - 1) return 'retention';
  if (world.headSeq - cursor > world.catchupMaxEvents) return 'too_far_behind';
  return null;
}
