// SPDX-License-Identifier: Apache-2.0

/// Access to the outbox (15 7.5, 04 13.2): FIFO per thread by `created_at`, then `id`; a new ID
/// is always a new row, never an update of the key.
library;

import 'package:domain/domain.dart';
import 'package:drift/drift.dart';

import '../database.dart';
import '../mappers/codec.dart';
import '../models.dart';

/// Data access object of the outbox.
final class OutboxDao {
  /// Creates the DAO on [db]; use `LocalDatabase.outboxDao`.
  OutboxDao(this._db);

  final LocalDatabase _db;

  /// Adds a `pending` entry with [id] (the `client_msg_id`, `client_interaction_id` or
  /// `Idempotency-Key`) and returns it. Throws when [id] already exists: sending again after a
  /// failure takes a new ID (04 13.2).
  Future<OutboxEntry> enqueue({
    required String id,
    required String kind,
    required String payload,
    ThreadId? threadId,
    String? dependsOn,
    ResourceVersion? expectedVersion,
  }) async {
    final now = toMillis(_db.clock());
    await _db
        .into(_db.outbox)
        .insert(
          OutboxCompanion.insert(
            id: id,
            kind: kind,
            threadId: Value(threadId?.value),
            dependsOn: Value(dependsOn),
            payload: payload,
            expectedVersion: Value(expectedVersion?.value),
            state: OutboxState.pending.code,
            createdAt: now,
            stateChangedAt: now,
          ),
          mode: InsertMode.insert,
        );
    return (await byId(id))!;
  }

  /// The entry [id], or null.
  Future<OutboxEntry?> byId(String id) async {
    final row = await (_db.select(
      _db.outbox,
    )..where((t) => t.id.equals(id))).getSingleOrNull();
    return row == null ? null : _fromRow(row);
  }

  /// The oldest entry of [threadId] (null: writes outside a thread) that is not final, or null
  /// when there is none or when that entry still waits for its `depends_on` entry (FIFO: a
  /// later entry never overtakes it).
  Future<OutboxEntry?> nextForThread(ThreadId? threadId) async {
    final row =
        await (_db.select(_db.outbox)
              ..where(
                (t) =>
                    (threadId == null
                        ? t.threadId.isNull()
                        : t.threadId.equals(threadId.value)) &
                    t.state.isNotIn([
                      OutboxState.failed.code,
                      OutboxState.confirmed.code,
                    ]),
              )
              ..orderBy(_fifo)
              ..limit(1))
            .getSingleOrNull();
    if (row == null) return null;
    final dependency = row.dependsOn;
    if (dependency != null) {
      final parent = await byId(dependency);
      if (parent != null && parent.state != OutboxState.confirmed) return null;
    }
    return _fromRow(row);
  }

  /// Moves entry [id] to [state]; `sending` counts an attempt, [nextAttemptAt] sets the end of
  /// a backoff in `waiting`.
  Future<void> markState(
    String id,
    OutboxState state, {
    DateTime? nextAttemptAt,
  }) async {
    await _db.customUpdate(
      'UPDATE outbox SET state = ?, state_changed_at = ?, next_attempt_at = ?, '
      'attempts = attempts + ? WHERE id = ?',
      variables: [
        Variable<String>(state.code),
        Variable<int>(toMillis(_db.clock())),
        Variable<int>(nextAttemptAt == null ? null : toMillis(nextAttemptAt)),
        Variable<int>(state == OutboxState.sending ? 1 : 0),
        Variable<String>(id),
      ],
      updates: {_db.outbox},
      updateKind: UpdateKind.update,
    );
  }

  /// Replaces the request body of entry [id] (an edit before sending).
  Future<void> updatePayload(String id, String payload) async {
    await (_db.update(_db.outbox)..where((t) => t.id.equals(id))).write(
      OutboxCompanion(payload: Value(payload)),
    );
  }

  /// Deletes entry [id] (discarded by the person).
  Future<void> delete(String id) async {
    await (_db.delete(_db.outbox)..where((t) => t.id.equals(id))).go();
  }

  /// Marks entry [id] as failed with the problem [code], [reason] and, for a `412`, the JSON of
  /// the `current` revision [conflictCurrent]; the text stays for copying (04 8.3 item 4).
  Future<void> recordFailure(
    String id, {
    required String code,
    String? reason,
    String? conflictCurrent,
  }) async {
    await (_db.update(_db.outbox)..where((t) => t.id.equals(id))).write(
      OutboxCompanion(
        state: Value(OutboxState.failed.code),
        stateChangedAt: Value(toMillis(_db.clock())),
        lastErrorCode: Value(code),
        lastErrorReason: Value(reason),
        conflictCurrent: Value(conflictCurrent),
      ),
    );
  }

  /// The threads with entries that are not final, in the order of their oldest entry; null
  /// stands for writes outside a thread.
  Future<List<ThreadId?>> pendingThreads() async {
    final rows = await _db
        .customSelect(
          'SELECT thread_id, MIN(created_at) AS first FROM outbox '
          'WHERE state NOT IN (?, ?) GROUP BY thread_id ORDER BY first, thread_id',
          variables: [
            Variable<String>(OutboxState.failed.code),
            Variable<String>(OutboxState.confirmed.code),
          ],
          readsFrom: {_db.outbox},
        )
        .get();
    return [
      for (final row in rows)
        optional(row.readNullable<String>('thread_id'), ThreadId.parse),
    ];
  }

  /// All entries of [threadId] in FIFO order, for the display of pending and failed sends.
  Future<List<OutboxEntry>> entriesFor(ThreadId? threadId) =>
      _entries(threadId).get();

  /// [entriesFor] on every change.
  Stream<List<OutboxEntry>> watchEntriesFor(ThreadId? threadId) =>
      _entries(threadId).watch();

  /// Removes entry [id] once the server's message replaced it (the echo or the `2xx`).
  Future<void> replaceWithServer(String id) => delete(id);

  Selectable<OutboxEntry> _entries(ThreadId? threadId) =>
      (_db.select(_db.outbox)
            ..where(
              (t) => threadId == null
                  ? t.threadId.isNull()
                  : t.threadId.equals(threadId.value),
            )
            ..orderBy(_fifo))
          .map(_fromRow);

  static final List<OrderClauseGenerator<$OutboxTable>> _fifo = [
    (t) => OrderingTerm(expression: t.createdAt),
    (t) => OrderingTerm(expression: t.id),
  ];

  static OutboxEntry _fromRow(OutboxRow row) => OutboxEntry(
    id: row.id,
    kind: row.kind,
    threadId: optional(row.threadId, ThreadId.parse),
    dependsOn: row.dependsOn,
    payload: row.payload,
    expectedVersion: row.expectedVersion == null
        ? null
        : ResourceVersion(row.expectedVersion!),
    state: OutboxState.fromCode(row.state),
    attempts: row.attempts,
    nextAttemptAt: fromMillisOrNull(row.nextAttemptAt),
    lastErrorCode: row.lastErrorCode,
    lastErrorReason: row.lastErrorReason,
    conflictCurrent: row.conflictCurrent,
    createdAt: fromMillis(row.createdAt),
    stateChangedAt: fromMillis(row.stateChangedAt),
  );
}
