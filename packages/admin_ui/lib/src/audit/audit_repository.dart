// SPDX-License-Identifier: Apache-2.0

/// The audit log over REST: one page of entries with the filters of `listAuditEvents`, the
/// hash chain check of `verifyAuditChain` and the NDJSON stream of `exportAuditEvents`
/// (docs/spec/01-product.md 5.2 A6; 03 `audit_events`; ADR 0021; brief M4-29).
///
/// Reading, checking and exporting are owner actions with step-up (05 `audit.read`,
/// `audit.verify`); the step-up runs in the `StepUpInterceptor` of the client when the server
/// answers `401 step_up_required`, never on the client's own reading of grants (15 1
/// principle 1). Entries are never changed or deleted from the client (brief M4-29 fixed
/// requirement 15): this interface has no such call.
library;

import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart';
import 'package:dio/dio.dart';

import '../data/self_repository.dart';

/// The decision of an audit entry (`AuditDecision`).
enum AuditResult {
  /// The action was allowed.
  allow,

  /// The action was denied; the entry names a deny reason.
  deny,

  /// The action failed.
  error,
}

/// One audit entry (`AuditEvent`) as the screens show it.
final class AuditEntry {
  /// Creates an entry.
  const AuditEntry({
    required this.seq,
    required this.id,
    required this.occurredAt,
    required this.action,
    required this.result,
    required this.details,
    required this.prevHash,
    required this.hash,
    this.actorId,
    this.denyReason,
    this.resourceKind,
    this.resourceId,
    this.correlationId,
  });

  /// The number of the entry in the hash chain.
  final int seq;

  /// The entry ID.
  final String id;

  /// When it happened (UTC).
  final DateTime occurredAt;

  /// The acting principal (a person or an agent), or null for the system.
  final String? actorId;

  /// The action ID, for example `grant.create`.
  final String action;

  /// Allowed, denied or failed.
  final AuditResult result;

  /// The reason of a denial, for example `missing_grant`.
  final String? denyReason;

  /// The kind of the resource, for example `space`.
  final String? resourceKind;

  /// The resource ID.
  final String? resourceId;

  /// The details the server returns, as JSON.
  final Map<String, Object?> details;

  /// The trace ID of the request.
  final String? correlationId;

  /// The hash of the entry before (hex).
  final String prevHash;

  /// The hash of this entry (hex).
  final String hash;

  /// The reason to show: the deny reason, else a `reason` text of [details] (an administration
  /// change records the reason the owner typed there), else null.
  String? get reason {
    if (denyReason case final r?) return r;
    final stated = details['reason'];
    return stated is String && stated.isNotEmpty ? stated : null;
  }
}

/// The filters of the audit list; each maps one to one to a query parameter of
/// `listAuditEvents` (brief M4-29 Design note 1). Null means no filter.
final class AuditFilter {
  /// Creates the filter.
  const AuditFilter({
    this.action,
    this.actorId,
    this.result,
    this.resourceKind,
    this.resourceId,
    this.from,
    this.to,
  });

  /// `action`.
  final String? action;

  /// `actor_id`.
  final String? actorId;

  /// `decision`.
  final AuditResult? result;

  /// `resource_kind`.
  final String? resourceKind;

  /// `resource_id`.
  final String? resourceId;

  /// `from`: occurred at or after (UTC).
  final DateTime? from;

  /// `to`: occurred before (UTC).
  final DateTime? to;

  /// Whether no filter is set.
  bool get isEmpty =>
      action == null &&
      actorId == null &&
      result == null &&
      resourceKind == null &&
      resourceId == null &&
      from == null &&
      to == null;
}

/// One page of the audit list, newest first, and the cursor of the next page (null on the
/// last).
final class AuditPage {
  /// Creates a page.
  const AuditPage(this.items, {this.nextCursor});

  /// The entries.
  final List<AuditEntry> items;

  /// The opaque cursor of the next page, or null.
  final String? nextCursor;
}

/// The result of a hash chain check (`AuditVerifyResult`, ADR 0021).
final class AuditChainCheck {
  /// Creates the result.
  const AuditChainCheck({
    required this.ok,
    required this.checkedFromSeq,
    required this.checkedToSeq,
    required this.checkpointsCompared,
    required this.warnings,
    this.firstBrokenSeq,
    this.failureReason,
    this.lastCheckpointSeq,
  });

  /// Whether the checked range is unchanged.
  final bool ok;

  /// The first number checked.
  final int checkedFromSeq;

  /// The last number checked.
  final int checkedToSeq;

  /// The first broken entry, or null when [ok].
  final int? firstBrokenSeq;

  /// Why it broke (`hash_mismatch`, `link_mismatch`, `seq_gap`, `checkpoint_mismatch`,
  /// `checkpoint_invalid`), or null.
  final String? failureReason;

  /// How many checkpoints were compared.
  final int checkpointsCompared;

  /// The number of the last checkpoint, or null.
  final int? lastCheckpointSeq;

  /// The warnings, for example `checkpoint_sink_not_configured`.
  final List<String> warnings;
}

/// The audit log's calls. Each carries the [purpose] text the step-up dialog shows when the
/// server asks for a step-up.
///
/// Errors: [SelfActionFailed] with the server's problem (null without an answer),
/// [StepUpCancelled] when the person cancels the step-up.
abstract interface class AuditRepository {
  /// One page of entries for [filter] (`listAuditEvents`); the page size is the server's
  /// default (brief M4-29 fixed requirement 9).
  Future<AuditPage> auditEvents(
    AuditFilter filter, {
    String? cursor,
    required String purpose,
  });

  /// Checks the hash chain (`verifyAuditChain`): without a range from the last checkpoint,
  /// else [fromSeq] to [toSeq].
  Future<AuditChainCheck> verifyAuditChain({
    int? fromSeq,
    int? toSeq,
    required String purpose,
  });

  /// The server's NDJSON export of [from] (inclusive) to [to] (exclusive), byte for byte
  /// (`exportAuditEvents`). The stream ends with an error when the transfer breaks off, so a
  /// cut file is never reported as complete.
  Stream<List<int>> exportAudit({
    required DateTime from,
    required DateTime to,
    required String purpose,
  });
}

/// [AuditRepository] over the generated client of `api_client`; `api` is the client of the
/// signed-in session, so DPoP, refresh and the step-up interceptor apply.
///
/// Example:
/// ```dart
/// final audit = ApiAuditRepository(session.api);
/// final page = await audit.auditEvents(
///   const AuditFilter(result: AuditResult.deny),
///   purpose: l10n.stepUpPurposeAuditRead,
/// );
/// ```
final class ApiAuditRepository implements AuditRepository {
  /// Creates the repository.
  ApiAuditRepository(this._api);

  final ApiClient _api;

  AuditApi get _audit => _api.getAuditApi();

  Map<String, Object?> _purpose(String text) => {stepUpPurposeExtra: text};

  @override
  Future<AuditPage> auditEvents(
    AuditFilter filter, {
    String? cursor,
    required String purpose,
  }) => _call(() async {
    final page = (await _audit.listAuditEvents(
      cursor: cursor,
      actorId: filter.actorId,
      action: filter.action,
      decision: switch (filter.result) {
        null => null,
        AuditResult.allow => AuditDecision.allow,
        AuditResult.deny => AuditDecision.deny,
        AuditResult.error => AuditDecision.error,
      },
      resourceKind: filter.resourceKind,
      resourceId: filter.resourceId,
      from: filter.from?.toUtc(),
      to: filter.to?.toUtc(),
      extra: _purpose(purpose),
    )).data!;
    return AuditPage([
      for (final e in page.items) _entry(e),
    ], nextCursor: page.nextCursor);
  });

  @override
  Future<AuditChainCheck> verifyAuditChain({
    int? fromSeq,
    int? toSeq,
    required String purpose,
  }) => _call(() async {
    final r = (await _audit.verifyAuditChain(
      auditVerifyRequest: AuditVerifyRequest(
        (b) => b
          ..fromSeq = fromSeq
          ..toSeq = toSeq,
      ),
      extra: _purpose(purpose),
    )).data!;
    return AuditChainCheck(
      ok: r.ok,
      checkedFromSeq: r.checkedFromSeq,
      checkedToSeq: r.checkedToSeq,
      firstBrokenSeq: r.firstBrokenSeq,
      failureReason: switch (r.failureReason) {
        null => null,
        AuditVerifyResultFailureReasonEnum.hashMismatch => 'hash_mismatch',
        AuditVerifyResultFailureReasonEnum.linkMismatch => 'link_mismatch',
        AuditVerifyResultFailureReasonEnum.seqGap => 'seq_gap',
        AuditVerifyResultFailureReasonEnum.checkpointMismatch =>
          'checkpoint_mismatch',
        AuditVerifyResultFailureReasonEnum.checkpointInvalid =>
          'checkpoint_invalid',
        _ => 'unknown',
      },
      checkpointsCompared: r.checkpointsCompared,
      lastCheckpointSeq: r.lastCheckpointSeq,
      warnings: r.warnings.toList(),
    );
  });

  @override
  Stream<List<int>> exportAudit({
    required DateTime from,
    required DateTime to,
    required String purpose,
  }) async* {
    // The step-up (and a 403) comes from one entry of the same range first; the export follows
    // within the step-up's validity (08 6.2; docs/DECISIONS.md D-M4-29-4). When the step-up
    // ends in between, the client decodes the refused stream's problem before the step-up
    // interceptors see it (`BinaryProblemInterceptor`), so the export's own step-up runs.
    await _call(
      () => _audit.listAuditEvents(
        from: from.toUtc(),
        to: to.toUtc(),
        limit: 1,
        extra: _purpose(purpose),
      ),
    );
    final Response<ResponseBody> response;
    try {
      response = await _api.dio.get<ResponseBody>(
        '/audit/export',
        queryParameters: {
          'from': from.toUtc().toIso8601String(),
          'to': to.toUtc().toIso8601String(),
        },
        options: Options(
          responseType: ResponseType.stream,
          extra: {..._secure, ..._purpose(purpose)},
        ),
      );
    } on DioException catch (e) {
      final cause = e.error;
      if (cause is AuthException) throw cause;
      throw SelfActionFailed(ApiProblem.of(e));
    }
    try {
      yield* response.data!.stream;
    } on Object {
      // A transfer that broke off (07 9.5: the server aborts instead of ending early).
      throw const SelfActionFailed(null);
    }
  }

  /// The security requirement of `exportAuditEvents` as the generated client sends it, so
  /// the auth interceptors sign this request too.
  static const Map<String, Object?> _secure = {
    'secure': <Map<String, String>>[
      {
        'type': 'apiKey',
        'name': 'consoleSession',
        'keyName': '__Host-console_session',
        'where': '',
      },
      {'type': 'http', 'scheme': 'DPoP', 'name': 'dpopAuth'},
      {'type': 'http', 'scheme': 'bearer', 'name': 'bearerAuth'},
    ],
  };

  static AuditEntry _entry(AuditEvent e) => AuditEntry(
    seq: e.seq,
    id: e.id,
    occurredAt: e.occurredAt,
    actorId: e.actorPrincipalId,
    action: e.action,
    result: switch (e.decision) {
      AuditDecision.deny => AuditResult.deny,
      AuditDecision.error => AuditResult.error,
      _ => AuditResult.allow,
    },
    denyReason: e.denyReason,
    resourceKind: e.resourceKind,
    resourceId: e.resourceId,
    details: {
      for (final MapEntry(:key, :value) in e.details.entries) key: value?.value,
    },
    correlationId: e.correlationId,
    prevHash: e.prevHash,
    hash: e.hash,
  );

  /// Runs [call] and turns its errors into [SelfActionFailed]; an error of the login layer
  /// ([StepUpCancelled], a lost session) passes as it is.
  static Future<T> _call<T>(Future<T> Function() call) async {
    try {
      return await call();
    } on DioException catch (e) {
      final cause = e.error;
      if (cause is AuthException) throw cause;
      final response = e.response;
      throw SelfActionFailed(
        ApiProblem.of(e) ??
            (response == null ? null : ApiProblem.fromResponse(response)),
      );
    }
  }
}
