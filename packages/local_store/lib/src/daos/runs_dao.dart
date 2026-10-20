// SPDX-License-Identifier: Apache-2.0

/// Access to run summaries (15 6.1 `runs`); the run document stays in memory (M4-16).
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
}
