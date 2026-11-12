// SPDX-License-Identifier: Apache-2.0

/// A database for the query plan tests: it records every statement the package runs, so a
/// test reads the plan of the statements the code really sends, not of a copy of their text.
library;

import 'package:drift/drift.dart';
import 'package:drift/native.dart';
import 'package:local_store/local_store.dart';

/// One statement with its arguments.
typedef Statement = ({String sql, List<Object?> args});

/// Records the statements of a database while [recording] is on.
final class StatementRecorder extends QueryInterceptor {
  /// The statements in the order they ran.
  final List<Statement> statements = [];

  /// Whether statements are recorded.
  bool recording = false;

  void _add(String sql, List<Object?> args) {
    if (recording) statements.add((sql: sql, args: args));
  }

  @override
  Future<List<Map<String, Object?>>> runSelect(
    QueryExecutor executor,
    String statement,
    List<Object?> args,
  ) {
    _add(statement, args);
    return executor.runSelect(statement, args);
  }

  @override
  Future<int> runInsert(
    QueryExecutor executor,
    String statement,
    List<Object?> args,
  ) {
    _add(statement, args);
    return executor.runInsert(statement, args);
  }

  @override
  Future<int> runUpdate(
    QueryExecutor executor,
    String statement,
    List<Object?> args,
  ) {
    _add(statement, args);
    return executor.runUpdate(statement, args);
  }

  @override
  Future<int> runDelete(
    QueryExecutor executor,
    String statement,
    List<Object?> args,
  ) {
    _add(statement, args);
    return executor.runDelete(statement, args);
  }

  @override
  Future<void> runCustom(
    QueryExecutor executor,
    String statement,
    List<Object?> args,
  ) {
    _add(statement, args);
    return executor.runCustom(statement, args);
  }
}

/// An in-memory database whose statements [recorder] sees; [clock] is its time source.
LocalDatabase openRecorded(
  StatementRecorder recorder, {
  DateTime Function()? clock,
}) => LocalDatabase(
  NativeDatabase.memory().interceptWith(recorder),
  clock: clock,
);

/// The statements [action] runs on [db], recorded by [recorder].
Future<List<Statement>> statementsOf(
  StatementRecorder recorder,
  Future<void> Function() action,
) async {
  recorder
    ..statements.clear()
    ..recording = true;
  try {
    await action();
  } finally {
    recorder.recording = false;
  }
  return List.of(recorder.statements);
}

/// The steps of the query plan of [statement] (`EXPLAIN QUERY PLAN`, the column `detail`).
Future<List<String>> planOf(LocalDatabase db, Statement statement) async {
  final rows = await db
      .customSelect(
        'EXPLAIN QUERY PLAN ${statement.sql}',
        variables: [for (final arg in statement.args) Variable<Object>(arg)],
      )
      .get();
  return [for (final r in rows) r.read<String>('detail')];
}
