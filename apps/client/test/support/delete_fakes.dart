// SPDX-License-Identifier: Apache-2.0

// A scripted delete port (01 6.18): answers from queues and records every call.

import 'dart:async';

import 'package:client/delete/delete_port.dart';
import 'package:client/new_project/new_project_flow.dart';

/// A [DeletePort] whose answers the test sets; [calls] records each call as text.
final class ScriptedDeletePort implements DeletePort {
  /// The answers of [info] in order; the last one repeats. An [Exception] in the list is
  /// thrown instead.
  final List<Object> infos = [const DeletionView()];

  /// The answer of [delete].
  DeletionView deleted = DeletionView(undoUntil: DateTime.utc(2026, 11, 1, 9));

  /// The failures of [delete], one per call while not empty.
  final List<NewProjectFailure> deleteFailures = [];

  /// The failures of [restore], one per call while not empty.
  final List<Object> restoreFailures = [];

  /// When set, [info] waits for it before it answers.
  Completer<void>? gate;

  /// The calls: `info`, `delete trash=<bool> key=<key> purpose=<text>`, `restore ...`.
  final List<String> calls = [];

  var _keys = 0;

  @override
  String newKey() => 'key-${++_keys}';

  @override
  Future<DeletionView> info(DeleteTarget target) async {
    calls.add('info ${target.id}');
    await gate?.future;
    final next = infos.length > 1 ? infos.removeAt(0) : infos.first;
    if (next is Exception) throw next;
    return next as DeletionView;
  }

  @override
  Future<DeletionView> delete(
    DeleteTarget target, {
    required bool trashFolder,
    required String idempotencyKey,
    required String purpose,
  }) async {
    calls.add(
      'delete ${target.id} trash=$trashFolder key=$idempotencyKey '
      'purpose=$purpose',
    );
    if (deleteFailures.isNotEmpty) throw deleteFailures.removeAt(0);
    return deleted;
  }

  @override
  Future<void> restore(
    DeleteTarget target, {
    required String idempotencyKey,
    required String purpose,
  }) async {
    calls.add('restore ${target.id} purpose=$purpose');
    if (restoreFailures.isNotEmpty) throw restoreFailures.removeAt(0);
  }
}
