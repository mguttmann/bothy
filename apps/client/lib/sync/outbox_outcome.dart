// SPDX-License-Identifier: Apache-2.0

/// The outcome of a one-time action sent through the outbox (ruling AUD-CA01/4): "Retry" on the
/// line of a run, the cancel of a wake-up, the channel switch of autonomous runs. Such an entry
/// has no thread, so no history shows its failure: the view that sent it waits for the outcome
/// here and shows a refusal itself.
library;

import 'package:sync_engine/sync_engine.dart';

/// Why the server refused a one-time action: the problem code and its reason.
typedef OutboxRefusal = ({String code, String? reason});

/// Waits for the outcome of the entry [entryId] of [outbox], an entry without a thread: null
/// once it left the outbox (the server accepted it), else the refusal of the failed entry.
///
/// The failed entry is discarded before the refusal is returned, so no failed entry without a
/// view stays in the table. The wait belongs to the entry and not to a widget: it ends with
/// the outcome also when the view that sent the entry is gone by then.
///
/// Example:
/// ```dart
/// final entry = await engine.outbox.enqueue(request);
/// final refusal = await outboxOutcome(engine.outbox, entry.id);
/// ```
Future<OutboxRefusal?> outboxOutcome(Outbox outbox, String entryId) async {
  await for (final views in outbox.watchThread(null)) {
    final mine = views.where((v) => v.id == entryId).firstOrNull;
    if (mine == null) return null;
    if (mine.stateLabel == OutboxStateLabel.failed) {
      await outbox.discard(mine.id);
      return (code: mine.failureCode ?? 'unknown', reason: mine.failureReason);
    }
  }
  return null;
}
