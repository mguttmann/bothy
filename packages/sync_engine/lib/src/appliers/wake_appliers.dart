// SPDX-License-Identifier: Apache-2.0

/// Applier of `run.wake_changed` (09 5.8.2, 04 15.9; brief AR-08): the payload names the
/// wake-up, its thread and its state but never its label or condition (D-AR-04-5), so the state
/// of a stored wake-up changes at once and the thread's wake-ups are loaded again
/// (`listThreadWakes`) after the commit. A thread this device does not hold is not loaded; the
/// view loads its wake-ups when it opens it ([loadThreadWakes]).
library;

import 'package:domain/domain.dart';

import '../dispatcher.dart';
import '../wire.dart';

/// The appliers of this group by event type.
final Map<String, Applier> wakeAppliers = {
  'run.wake_changed': (c) async {
    final p = c.payload;
    final threadId = ThreadId.parse(str(p, 'thread_id'));
    await c.db.runsDao.applyWakeState(
      WakeId.parse(str(p, 'wake_id')),
      RunWakeState.fromCode(str(p, 'state')),
      stateReason: optStr(p, 'state_reason'),
    );
    if (await c.db.threadsDao.byId(threadId) == null) return;
    final sync = c.sync;
    c.after.reads.add(() => loadThreadWakes(sync, threadId));
  },
};

/// Loads every wake-up of [threadId] (`listThreadWakes`) and replaces the thread's rows of
/// `run_wakes` in one transaction, but only while the thread is stored here and was not
/// removed meanwhile. A server before stage AR (`404`) leaves the rows as they are. Errors are
/// those of the read; the caller decides whether to log or show them.
Future<void> loadThreadWakes(SyncContext sync, ThreadId threadId) async {
  final wakes = await sync.fetcher.threadWakes(threadId);
  if (wakes == null) return;
  final db = sync.db;
  await db.transaction(() async {
    final thread = await db.threadsDao.byId(threadId);
    if (thread == null ||
        !await sync.holdsContent(db, thread.channelId, threadId)) {
      return;
    }
    await db.runsDao.replaceThreadWakes(threadId, wakes);
  });
}
