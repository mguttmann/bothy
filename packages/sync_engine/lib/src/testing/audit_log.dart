// SPDX-License-Identifier: Apache-2.0

/// The audit log of the fake server: a hash chain of `AuditEvent` rows that a test can break at
/// a given entry (brief M4-29 Scope "Fake server handlers"; 03 `audit_events`; ADR 0021).
///
/// The hash of a row covers its fields and the hash of the row before, so changing a stored row
/// without its hash breaks the chain at that row, as on the server. The exact hash input of the
/// server (M1-24) is not reproduced: the fake only needs a chain that its own check verifies.
library;

import 'dart:convert';

import 'package:crypto/crypto.dart';

import 'world.dart';

/// The `prev_hash` of the first row: 32 zero bytes in hex.
const String auditGenesisHash =
    '0000000000000000000000000000000000000000000000000000000000000000';

/// The result of [FakeAuditLog.verify], shaped as `AuditVerifyResult`.
typedef FakeAuditCheck = JsonMap;

/// The audit rows of the fake in ascending `seq`, each an `AuditEvent` as JSON.
///
/// Example:
/// ```dart
/// final log = server.audit;
/// log.add(world, action: 'grant.create', actor: alex.id);
/// log.tamper(1); // the check now reports seq 1 as broken
/// ```
final class FakeAuditLog {
  /// The rows in ascending `seq`.
  final List<JsonMap> rows = [];

  /// The `warnings` of every check, for example `checkpoint_sink_not_configured`.
  final List<String> warnings = [];

  /// The `seq` of the last checkpoint, or null without one; a check without a range starts
  /// after it (M1-50).
  int? lastCheckpointSeq;

  /// Appends a row and returns it; the hash links it to the row before.
  ///
  /// [at] defaults to the world's clock; [details] defaults to an empty object.
  JsonMap add(
    FakeWorld world, {
    required String action,
    String decision = 'allow',
    String? actor,
    String? denyReason,
    String? resourceKind,
    String? resourceId,
    JsonMap details = const {},
    DateTime? at,
  }) {
    final prev = rows.isEmpty ? auditGenesisHash : rows.last['hash']! as String;
    final row = <String, Object?>{
      'seq': rows.length + 1,
      'id': world.nextId(),
      'occurred_at': world.stamp(at),
      'actor_principal_id': ?actor,
      'action': action,
      'decision': decision,
      'deny_reason': ?denyReason,
      'resource_kind': ?resourceKind,
      'resource_id': ?resourceId,
      'details': details,
      'correlation_id':
          '4bf92f35-77b3-4da6-a3ce-${(rows.length + 1).toRadixString(16).padLeft(12, '0')}',
      'prev_hash': prev,
    };
    row['hash'] = hashOf(row);
    rows.add(row);
    return row;
  }

  /// Changes the `details` of the row [seq] without its hash, so the chain breaks there.
  void tamper(int seq) {
    final row = rows[seq - 1];
    row['details'] = {...(row['details']! as JsonMap), 'tampered': true};
  }

  /// The hash of [row]: SHA-256 over its fields without `hash`, in a fixed key order, after
  /// its `prev_hash`.
  static String hashOf(JsonMap row) {
    final fields = Map.fromEntries(
      row.entries.where((e) => e.key != 'hash').toList()
        ..sort((a, b) => a.key.compareTo(b.key)),
    );
    return sha256.convert(utf8.encode(jsonEncode(fields))).toString();
  }

  /// Checks the rows [fromSeq] to [toSeq] (both inclusive); without [fromSeq] it starts after
  /// [lastCheckpointSeq] (or at 1), without [toSeq] it ends at the last row.
  FakeAuditCheck verify({int? fromSeq, int? toSeq}) {
    final from = fromSeq ?? (lastCheckpointSeq ?? 0) + 1;
    final to = toSeq ?? rows.length;
    int? broken;
    String? reason;
    for (var seq = from; seq <= to && seq <= rows.length; seq++) {
      final row = rows[seq - 1];
      final prev = seq == 1 ? auditGenesisHash : rows[seq - 2]['hash'];
      if (row['prev_hash'] != prev) {
        broken = seq;
        reason = 'link_mismatch';
        break;
      }
      if (row['hash'] != hashOf(row)) {
        broken = seq;
        reason = 'hash_mismatch';
        break;
      }
    }
    return {
      'ok': broken == null,
      'checked_from_seq': from,
      'checked_to_seq': broken ?? to,
      'first_broken_seq': ?broken,
      'failure_reason': ?reason,
      'checkpoints_compared': lastCheckpointSeq == null ? 0 : 1,
      'last_checkpoint_seq': ?lastCheckpointSeq,
      'warnings': List.of(warnings),
    };
  }
}
