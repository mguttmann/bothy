// SPDX-License-Identifier: Apache-2.0

/// Handlers of the audit log: `listAuditEvents`, `exportAuditEvents` and `verifyAuditChain`
/// over a [FakeAuditLog] (brief M4-29 Scope; 03 `audit_events`; ADR 0021).
///
/// Reading, exporting and checking are owner actions; the step-up comes from the `step_up` value
/// of `audit.read` and `audit.verify` in `docs/spec/actions.yaml` through
/// [requireCatalogStepUp], so the fake holds no copy of 05.
library;

import 'dart:convert';

import '../action_catalog.dart';
import '../audit_log.dart';
import '../rest_fake.dart';
import '../world.dart';
import 'admin_handlers.dart' show requireCatalogStepUp;

/// The page size of `listAuditEvents` when the request names no `limit` (the server's default
/// comes from its configuration; the fake uses a fixed one).
const int fakeAuditPageSize = 50;

/// Registers the audit handlers on [rest] over [log], with the step-up of [catalog].
void registerAuditHandlers(
  RestFake rest,
  ActionCatalog catalog,
  FakeAuditLog log,
) {
  void owner(FakeRequest r) {
    if (!r.person.isOwner) {
      throw const FakeProblem(403, 'forbidden', reason: 'owner_only');
    }
    requireCatalogStepUp(r, rest.contract, catalog);
  }

  DateTime? time(FakeRequest r, String name) {
    final text = r.query(name);
    if (text == null) return null;
    return DateTime.tryParse(text) ??
        (throw const FakeProblem(400, 'validation_failed'));
  }

  bool inRange(JsonMap row, DateTime? from, DateTime? to) {
    final at = DateTime.parse(row['occurred_at']! as String);
    return (from == null || !at.isBefore(from)) &&
        (to == null || at.isBefore(to));
  }

  rest
    ..handle('listAuditEvents', (r) {
      owner(r);
      final from = time(r, 'from');
      final to = time(r, 'to');
      final filters = {
        'actor_principal_id': r.query('actor_id'),
        'action': r.query('action'),
        'decision': r.query('decision'),
        'resource_kind': r.query('resource_kind'),
        'resource_id': r.query('resource_id'),
      };
      final matching = [
        for (final row in log.rows.reversed)
          if (inRange(row, from, to) &&
              filters.entries.every(
                (f) => f.value == null || row[f.key] == f.value,
              ))
            row,
      ];
      final start = int.tryParse(r.query('cursor') ?? '') ?? 0;
      final limit = int.tryParse(r.query('limit') ?? '') ?? fakeAuditPageSize;
      final end = (start + limit).clamp(0, matching.length);
      return FakeResponse(200, {
        'items': matching.sublist(start.clamp(0, matching.length), end),
        if (end < matching.length) 'next_cursor': '$end',
      });
    })
    ..handle('exportAuditEvents', (r) {
      owner(r);
      final from = time(r, 'from');
      final to = time(r, 'to');
      if (from == null || to == null) {
        throw const FakeProblem(400, 'validation_failed');
      }
      // The contiguous seq span from the smallest to the largest seq in [from, to): every line
      // links the line before (openapi `exportAuditEvents`).
      final seqs = [
        for (final row in log.rows)
          if (inRange(row, from, to)) row['seq']! as int,
      ];
      final out = StringBuffer();
      if (seqs.isNotEmpty) {
        for (var seq = seqs.first; seq <= seqs.last; seq++) {
          final row = log.rows[seq - 1];
          final problems = rest.contract.validate(row, 'AuditEvent');
          if (problems.isNotEmpty) {
            rest.violations.add(
              'response exportAuditEvents line $seq: $problems',
            );
          }
          out.writeln(jsonEncode(row));
        }
      }
      return FakeResponse.bytes(
        utf8.encode(out.toString()),
        contentType: 'application/x-ndjson',
      );
    })
    ..handle('verifyAuditChain', (r) {
      owner(r);
      final body = r.json;
      return FakeResponse(
        200,
        log.verify(
          fromSeq: body['from_seq'] as int?,
          toSeq: body['to_seq'] as int?,
        ),
      );
    });
}
