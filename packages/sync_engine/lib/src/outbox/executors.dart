// SPDX-License-Identifier: Apache-2.0

/// The executor registry of the outbox (brief M4-15 Design note 3, docs/DECISIONS.md
/// D-M4-15-2): one executor per operation kind, the only coupling to `api_client`.
library;

import 'dart:convert';

import 'package:api_client/api_client.dart';
import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';

/// What the outbox hands an executor besides the entry.
final class ApiCallContext {
  /// Creates the context of one attempt; [checkpoint] keeps values in the entry (the outbox
  /// passes its own, the default keeps nothing).
  const ApiCallContext({
    required this.idempotencyKey,
    this._checkpoint = _keepNothing,
  });

  /// The entry's ID: the `client_msg_id` of a message, the `client_interaction_id` of an
  /// interaction and the `Idempotency-Key` of every other write (04 5). It never changes on
  /// retry.
  final String idempotencyKey;

  /// The request option of `api_client` that writes [idempotencyKey] as the header
  /// `Idempotency-Key` (06 7); pass it as `extra` to a generated operation.
  Map<String, dynamic> get extra => {idempotencyKeyExtra: idempotencyKey};

  final OutboxCheckpoint _checkpoint;

  /// Writes [values] into the payload of the entry being sent, member by member, so the next
  /// attempt of the same entry reads them (FIX-upload-retry-storm: an upload keeps its
  /// session, so a retry resumes it instead of announcing it again). A value of null removes
  /// the member. An entry that left the outbox meanwhile (discarded, confirmed by its echo)
  /// changes nothing.
  Future<void> checkpoint(Map<String, Object?> values) => _checkpoint(values);

  static Future<void> _keepNothing(Map<String, Object?> values) async {}
}

/// Keeps [values] in the payload of the entry of an attempt (see [ApiCallContext.checkpoint]).
typedef OutboxCheckpoint = Future<void> Function(Map<String, Object?> values);

/// What an executor returns for a `2xx`.
final class OutboxResponse {
  /// Creates a response.
  const OutboxResponse({this.status, this.body, this.result = const {}});

  /// The HTTP status, if the executor knows it.
  final int? status;

  /// The decoded response body (for example the `Message`), for the confirmation hook of the
  /// outbox that stores it.
  final Object? body;

  /// Values the entries that depend on this one need, for example `message_id` and `version`
  /// of a created message; the outbox fills them into the members a dependent holds as null
  /// (D-M4-15-6).
  final Map<String, Object?> result;
}

/// Sends one entry. Returns on a `2xx` and throws whatever the client throws otherwise; the
/// outbox classifies the error (`classify`).
typedef OutboxExecutor = Future<OutboxResponse> Function(
  OutboxEntry entry,
  ApiCallContext ctx,
);

/// Names the FIFO lane of an entry outside a thread, for example `setting:<key>`; null puts
/// the entry into its thread's lane.
typedef OutboxLaneOf = String? Function(OutboxEntry entry);

/// Builds the follow-up request that applies [edited] to an entry that may already be stored
/// on the server (04 13.3); the outbox makes it depend on the entry.
typedef OutboxFollowUpEdit = OutboxRequest Function(
  OutboxEntry entry,
  Map<String, Object?> edited,
);

/// Builds the follow-up request that deletes what an attempted entry created (04 13.3).
typedef OutboxFollowUpRemove = OutboxRequest Function(OutboxEntry entry);

/// A write the outbox stores and sends (04 13.2 item 1).
final class OutboxRequest {
  /// Creates a request.
  const OutboxRequest({
    required this.kind,
    required this.payload,
    this.threadId,
    this.dependsOn = const [],
    this.expectedVersion,
    this.id,
  });

  /// The operation, a kind registered in [OutboxExecutors].
  final String kind;

  /// The JSON body the executor reads; members it does not know are ignored.
  final Map<String, Object?> payload;

  /// The thread of the write; its entries are sent strictly in order (04 13.2 item 2).
  final ThreadId? threadId;

  /// Entries that must be confirmed before this one is sent, for example the uploads of a
  /// message (15 7.5).
  final List<String> dependsOn;

  /// The `If-Match` version of an edit, if any.
  final ResourceVersion? expectedVersion;

  /// A UUIDv7 the caller created already, for example the share extension's `client_msg_id`;
  /// null lets the outbox create one.
  final String? id;

  /// A copy with [ids] as [dependsOn].
  OutboxRequest withDependsOn(List<String> ids) => OutboxRequest(
    kind: kind,
    payload: payload,
    threadId: threadId,
    dependsOn: ids,
    expectedVersion: expectedVersion,
    id: id,
  );
}

/// One registered operation.
final class OutboxOperation {
  /// Creates the registration of an operation.
  const OutboxOperation({
    required this.send,
    this.lane,
    this.followUpEdit,
    this.followUpRemove,
  });

  /// Sends an entry.
  final OutboxExecutor send;

  /// The lane of an entry; null: the thread's lane.
  final OutboxLaneOf? lane;

  /// The follow-up edit of an attempted entry, if the operation supports one.
  final OutboxFollowUpEdit? followUpEdit;

  /// The follow-up delete of an attempted entry, if the operation supports one.
  final OutboxFollowUpRemove? followUpRemove;
}

/// The registry of executors: features register their writes here, so a new write never
/// changes the outbox (brief M4-15 Goal).
///
/// Example:
/// ```dart
/// final executors = OutboxExecutors();
/// MessageOps.register(executors, api);
/// PersonalOps.register(executors, api);
/// executors.register('setting', settingExecutor,
///     lane: (e) => 'setting:${decodePayload(e.payload)['key']}');
/// ```
final class OutboxExecutors {
  final Map<String, OutboxOperation> _ops = {};

  /// Registers [executor] for [kind], replacing an earlier registration.
  ///
  /// [lane] names the FIFO lane of an entry (null or a null result: its thread, or `global`
  /// without a thread); [followUpEdit] and [followUpRemove] let the person edit or delete an
  /// entry after its first attempt (04 13.3). Without them such an edit is refused.
  void register(
    String kind,
    OutboxExecutor executor, {
    OutboxLaneOf? lane,
    OutboxFollowUpEdit? followUpEdit,
    OutboxFollowUpRemove? followUpRemove,
  }) {
    _ops[kind] = OutboxOperation(
      send: executor,
      lane: lane,
      followUpEdit: followUpEdit,
      followUpRemove: followUpRemove,
    );
  }

  /// The operation of [kind], or null when nothing is registered.
  OutboxOperation? lookup(String kind) => _ops[kind];

  /// The FIFO lane of [entry]: the executor's lane, else `thread:<id>`, else `global`.
  String laneOf(OutboxEntry entry) {
    final own = _ops[entry.kind]?.lane?.call(entry);
    if (own != null) return own;
    final thread = entry.threadId;
    return thread == null ? 'global' : 'thread:${thread.value}';
  }
}

/// Decodes the JSON object [payload] of an entry; throws a [FormatException] for anything
/// else (a programming error the outbox records as `executor_error`).
Map<String, Object?> decodePayload(String payload) {
  final value = jsonDecode(payload);
  if (value is! Map) {
    throw const FormatException('an outbox payload is a JSON object');
  }
  return value.cast<String, Object?>();
}
