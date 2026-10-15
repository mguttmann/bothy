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
  /// Creates the DAO on [db]; use `LocalDatabase.runsDao`.
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
}
