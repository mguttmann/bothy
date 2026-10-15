// SPDX-License-Identifier: Apache-2.0

/// Access to the instance information (15 6.1 `instance`): branding, evaluated switches and
/// limits of `getInstance`.
library;

import 'package:domain/domain.dart';

import '../database.dart';
import '../mappers/instance.dart';

/// Data access object of the instance row; hands out domain types only.
final class InstanceDao {
  /// Creates the DAO on [db]; use `LocalDatabase.instanceDao`.
  InstanceDao(this._db);

  final LocalDatabase _db;

  /// Replaces the stored instance information with [info] in one transaction (the table holds
  /// one row; `getInstance` always delivers the whole object).
  Future<void> put(InstanceInfo info) => _db.transaction(() async {
    await _db.delete(_db.instanceTable).go();
    await _db.into(_db.instanceTable).insert(instanceToCompanion(info));
  });

  /// The stored instance information, or null before the first `getInstance`.
  Future<InstanceInfo?> get() async {
    final row = await _db.select(_db.instanceTable).getSingleOrNull();
    return row == null ? null : instanceFromRow(row);
  }

  /// The stored instance information on every change.
  Stream<InstanceInfo?> watch() => _db
      .select(_db.instanceTable)
      .map(instanceFromRow)
      .watchSingleOrNull();
}
