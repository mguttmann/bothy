// SPDX-License-Identifier: Apache-2.0

/// Access to slash commands for suggestions offline (15 6.1 `commands`).
library;

import 'package:domain/domain.dart';
import 'package:drift/drift.dart';

import '../database.dart';
import '../mappers/instance.dart';
import 'upsert.dart';

/// Data access object of commands; hands out domain types only.
final class CommandsDao {
  /// Creates the DAO on [db]; use `LocalDatabase.commandsDao`.
  CommandsDao(this._db);

  final LocalDatabase _db;

  /// Stores [command] if its version is newer; returns whether it applied.
  Future<bool> upsertIfNewer(Command command) {
    final row = commandToCompanion(command);
    return upsertIfNewerVersion(
      _db,
      _db.commands,
      row,
      update: row,
      version: (t) => t.version,
    );
  }

  /// Deletes command [id] (removed on the server).
  Future<void> delete(CommandId id) async {
    await (_db.delete(_db.commands)..where((t) => t.id.equals(id.value))).go();
  }

  /// The commands whose name starts with [prefix] (empty: all), by name; the suggestion list
  /// while typing.
  Future<List<Command>> withPrefix(String prefix) async {
    final rows =
        await (_db.select(_db.commands)
              ..where(
                (t) => t.name
                    .substr(1, prefix.length)
                    .equals(prefix.toLowerCase()),
              )
              ..orderBy([(t) => OrderingTerm(expression: t.name)]))
            .get();
    return [for (final row in rows) commandFromRow(row)];
  }

  /// All commands by name.
  Stream<List<Command>> watchAll() =>
      (_db.select(_db.commands)
            ..orderBy([(t) => OrderingTerm(expression: t.name)]))
          .map(commandFromRow)
          .watch();
}
