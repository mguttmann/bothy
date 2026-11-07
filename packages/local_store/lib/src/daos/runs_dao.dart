// SPDX-License-Identifier: Apache-2.0

/// Access to run summaries (15 6.1 `runs`) and the wake-ups of threads (`run_wakes`, AR-08); the
/// run document stays in memory (M4-16).
library;

import 'package:domain/domain.dart';
import 'package:drift/drift.dart';

import '../database.dart';
import '../mappers/runs_agents.dart';
import 'upsert.dart';

/// Data access object of runs; hands out domain types only.
final class RunsDao {
  /// Creates the DAO on a database; use `LocalDatabase.runsDao`.
  RunsDao(this._db);

  final LocalDatabase _db;

  /// Stores the summary [run] of `run.state_changed` if its version is newer; returns whether
  /// it applied.
  Future<bool> upsertIfNewer(RunSummary run) {
    final row = runToCompanion(run);
    return upsertIfNewerVersion(
      _db,
      _db.runs,
      row,
      update: row,
      version: (t) => t.version,
    );
  }

  /// The run [id], or null.
  Future<RunSummary?> byId(RunId id) async {
    final row = await (_db.select(
      _db.runs,
    )..where((t) => t.id.equals(id.value))).getSingleOrNull();
    return row == null ? null : runFromRow(row);
  }

  /// The runs of [threadId] in queue order.
  Stream<List<RunSummary>> watchForThread(ThreadId threadId) =>
      (_db.select(_db.runs)
            ..where((t) => t.threadId.equals(threadId.value))
            ..orderBy([(t) => OrderingTerm(expression: t.queuedAt)]))
          .map(runFromRow)
          .watch();

  /// The runs that have not ended (`queued`, `running`, `waiting_input`; 04 11.3 item 6), in
  /// queue order. The sidebar shows a run dot per channel and the post list a state icon per
  /// thread from them (components.md 6 and 7).
  Stream<List<RunSummary>> watchActive() =>
      (_db.select(_db.runs)
            ..where(
              (t) => t.state.isIn([
                RunState.queued.code,
                RunState.running.code,
                RunState.waitingInput.code,
              ]),
            )
            ..orderBy([(t) => OrderingTerm(expression: t.queuedAt)]))
          .map(runFromRow)
          .watch();

  /// The stored runs of [channelId] in queue order: the post list shows the state of the
  /// active run and the failure of the newest run per thread (components.md 6).
  Stream<List<RunSummary>> watchForChannel(ChannelId channelId) =>
      (_db.select(_db.runs)
            ..where((t) => t.channelId.equals(channelId.value))
            ..orderBy([(t) => OrderingTerm(expression: t.queuedAt)]))
          .map(runFromRow)
          .watch();

  /// Stores the wake-up [wake] if its version is newer (04 9 item 3); returns whether it
  /// applied.
  Future<bool> upsertWakeIfNewer(RunWake wake) {
    final row = wakeToCompanion(wake);
    return upsertIfNewerVersion(
      _db,
      _db.runWakes,
      row,
      update: row,
      version: (t) => t.version,
    );
  }

  /// Replaces the wake-ups of [threadId] with [wakes], the whole list `listThreadWakes` returned
  /// (09 5.8.2): rows the list no longer has go, the others are stored by version, in one
  /// transaction.
  Future<void> replaceThreadWakes(ThreadId threadId, List<RunWake> wakes) =>
      _db.transaction(() async {
        final keep = [for (final w in wakes) w.id.value];
        await (_db.delete(_db.runWakes)..where(
              (t) => t.threadId.equals(threadId.value) & t.id.isNotIn(keep),
            ))
            .go();
        for (final wake in wakes) {
          await upsertWakeIfNewer(wake);
        }
      });

  /// Sets [state] and [stateReason] of the stored wake-up [id] from `run.wake_changed`, whose
  /// payload carries no version, so the row keeps its version and the reload that follows
  /// replaces it; returns whether a row was stored.
  Future<bool> applyWakeState(
    WakeId id,
    RunWakeState state, {
    String? stateReason,
  }) async {
    final changed =
        await (_db.update(
          _db.runWakes,
        )..where((t) => t.id.equals(id.value))).write(
          RunWakesCompanion(
            state: Value(state.code),
            stateReason: Value(stateReason),
          ),
        );
    return changed > 0;
  }

  /// The wake-ups of [threadId] in the order they fire (`not_before`, then the ID).
  Stream<List<RunWake>> watchWakesForThread(ThreadId threadId) =>
      (_db.select(_db.runWakes)
            ..where((t) => t.threadId.equals(threadId.value))
            ..orderBy([
              (t) => OrderingTerm(expression: t.notBefore),
              (t) => OrderingTerm(expression: t.id),
            ]))
          .map(wakeFromRow)
          .watch();
}
