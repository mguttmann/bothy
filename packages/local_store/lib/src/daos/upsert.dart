// SPDX-License-Identifier: Apache-2.0

/// The conditional upsert of the version rule (04 9 item 3, 15 5; docs/DECISIONS.md D-M4-12-4).
library;

import 'package:drift/drift.dart';

/// Inserts [insert] into [table], or on a key conflict replaces the row with [update] only when
/// the incoming version is greater than the stored one, in one statement:
/// `INSERT ... ON CONFLICT DO UPDATE SET ... WHERE excluded.version > <table>.version`.
///
/// [version] selects the version column of a table instance. Returns whether a row was
/// written; an older or equal version leaves the row as it is and returns false.
Future<bool> upsertIfNewerVersion<T extends Table, D>(
  DatabaseConnectionUser db,
  TableInfo<T, D> table,
  Insertable<D> insert, {
  required Insertable<D> update,
  required GeneratedColumn<int> Function(T table) version,
}) async {
  final row = await db
      .into(table)
      .insertReturningOrNull(
        insert,
        onConflict: DoUpdate.withExcluded(
          (old, excluded) => update,
          where: (old, excluded) => version(excluded).isBiggerThan(version(old)),
        ),
      );
  return row != null;
}

/// Builds the placeholder list `(?, ?, ...)` for [count] values; [count] must be positive.
String placeholders(int count) =>
    '(${List.filled(count, '?').join(', ')})';

/// Wraps [values] as SQL variables of text.
List<Variable<Object>> textVariables(Iterable<String> values) => [
  for (final value in values) Variable<String>(value),
];
