// SPDX-License-Identifier: Apache-2.0

/// Access to principals and agent cards (15 6.1 `principals`, `agents`).
library;

import 'package:domain/domain.dart';
import 'package:drift/drift.dart';

import '../database.dart';
import '../mappers/runs_agents.dart';
import 'upsert.dart';

/// Data access object of principals and agents; hands out domain types only.
final class PrincipalsDao {
  /// Creates the DAO on a database; use `LocalDatabase.principalsDao`.
  PrincipalsDao(this._db);

  final LocalDatabase _db;

  /// Stores [principal]; a `PrincipalRef` carries no version, so the caller orders the writes.
  Future<void> put(Principal principal) async {
    await _db
        .into(_db.principals)
        .insertOnConflictUpdate(principalToCompanion(principal));
  }

  /// The principal [id], or null.
  Future<Principal?> byId(PrincipalId id) async {
    final row = await (_db.select(
      _db.principals,
    )..where((t) => t.id.equals(id.value))).getSingleOrNull();
    return row == null ? null : principalFromRow(row);
  }

  /// All principals by handle.
  Stream<List<Principal>> watchAll() =>
      (_db.select(_db.principals)
            ..orderBy([(t) => OrderingTerm(expression: t.handle)]))
          .map(principalFromRow)
          .watch();

  /// Stores the agent [card] if its version is newer; returns whether it applied.
  Future<bool> upsertAgentIfNewer(AgentCard card) {
    final row = agentToCompanion(card);
    return upsertIfNewerVersion(
      _db,
      _db.agents,
      row,
      update: row,
      version: (t) => t.version,
    );
  }

  /// The agent card [id], or null.
  Future<AgentCard?> agentById(PrincipalId id) async {
    final row = await (_db.select(
      _db.agents,
    )..where((t) => t.principalId.equals(id.value))).getSingleOrNull();
    return row == null ? null : agentFromRow(row);
  }

  /// All agent cards by handle.
  Stream<List<AgentCard>> watchAgents() =>
      (_db.select(_db.agents)
            ..orderBy([(t) => OrderingTerm(expression: t.handle)]))
          .map(agentFromRow)
          .watch();
}
