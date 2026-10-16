// SPDX-License-Identifier: Apache-2.0

/// Outcome classes of an outbox request (04 13.2 items 4 and 5, 04 13.3, brief M4-15 Design
/// note 2, docs/DECISIONS.md D-M4-15-1).
library;

import 'dart:async';
import 'dart:io';

import 'package:api_client/api_client.dart';
import 'package:dio/dio.dart';

/// What the outbox does with an entry after a request.
enum OutcomeClass {
  /// `2xx` (also `200` with `Idempotent-Replay: true` and `204` on a delete): the entry is
  /// done and leaves the outbox.
  confirmed,

  /// A network error, a timeout, `5xx`, `429` or `409 request_in_progress`: the same request
  /// with the same ID again after the backoff or `Retry-After`.
  retry,

  /// A permanent refusal: the entry stays visible as failed with the code and reason.
  failed,

  /// `412 version_conflict`: failed, with the server's current revision kept for the person.
  conflict,
}

/// The result of [classify]: the class and what the entry keeps of the error.
final class Classification {
  /// Creates a classification.
  const Classification(
    this.outcome, {
    this.code,
    this.reason,
    this.retryAfter,
    this.current,
  });

  /// The class.
  final OutcomeClass outcome;

  /// The problem code (06 5), or a client code: `network`, `timeout`, `executor_error`.
  final String? code;

  /// The `reason` member of the problem, if any.
  final String? reason;

  /// The server's `Retry-After` of a `429` or `503`, if any.
  final Duration? retryAfter;

  /// The `current` revision of a `412 version_conflict`, as raw JSON.
  final Map<String, Object?>? current;
}

/// Classifies an [error] an executor threw.
///
/// Accepts a `DioException` of the generated client (with or without an [ApiProblem]), an
/// [ApiProblem], a [TimeoutException], an [IOException] (socket, HTTP and TLS errors) or any
/// other object. Rules (Design note 2): transport errors, timeouts, `5xx`, `429` and
/// `409 request_in_progress` retry; `429` and `503` carry `Retry-After`; `503 setup_required`
/// is final; `412` is a conflict; every other status is final with its problem code and
/// reason. `401` is handled by the interceptors of `auth` before the outbox sees it, so one
/// that arrives here is final. Any other error is a programming error of the executor
/// (`executor_error`) and final, so it never loops.
///
/// Example: `classify(problem).outcome == OutcomeClass.retry` for a `503`.
Classification classify(Object error) {
  if (error is ApiProblem) return _problem(error);
  if (error is DioException) {
    final problem = ApiProblem.of(error);
    if (problem != null) return _problem(problem);
    final response = error.response;
    if (error.type == DioExceptionType.badResponse && response != null) {
      return _problem(ApiProblem.fromResponse(response));
    }
    return switch (error.type) {
      DioExceptionType.connectionTimeout ||
      DioExceptionType.sendTimeout ||
      DioExceptionType.receiveTimeout => const Classification(
        OutcomeClass.retry,
        code: 'timeout',
      ),
      DioExceptionType.unknown => _unknown(error),
      // Connection errors, a refused certificate and a cancelled request: the request may
      // not have arrived; the same ID makes the repeat safe (04 5).
      _ => const Classification(OutcomeClass.retry, code: 'network'),
    };
  }
  if (error is TimeoutException) {
    return const Classification(OutcomeClass.retry, code: 'timeout');
  }
  // SocketException, HttpException and the TLS errors (HandshakeException) are IOExceptions.
  if (error is IOException) {
    return const Classification(OutcomeClass.retry, code: 'network');
  }
  return const Classification(OutcomeClass.failed, code: 'executor_error');
}

/// A `DioException` of type `unknown`: the generated client throws it for an error inside
/// the adapter and for a `2xx` body it cannot decode.
Classification _unknown(DioException error) {
  final status = error.response?.statusCode;
  if (status != null && status >= 200 && status < 300) {
    // The server stored the write; repeating it would only replay the same body (04 5).
    return const Classification(
      OutcomeClass.confirmed,
      code: 'undecodable_response',
    );
  }
  final inner = error.error;
  if (inner == null || inner is IOException || inner is TimeoutException) {
    return const Classification(OutcomeClass.retry, code: 'network');
  }
  return const Classification(OutcomeClass.failed, code: 'executor_error');
}

Classification _problem(ApiProblem p) {
  final status = p.status;
  if (status == 412) {
    return Classification(
      OutcomeClass.conflict,
      code: p.code,
      reason: p.reason,
      current: p.current,
    );
  }
  final retry =
      (status >= 500 && !(status == 503 && p.code == 'setup_required')) ||
      status == 429 ||
      (status == 409 && p.code == 'request_in_progress');
  if (retry) {
    return Classification(
      OutcomeClass.retry,
      code: p.code,
      reason: p.reason,
      // Only 429 and 503 name a wait (06 5); other statuses back off.
      retryAfter: status == 429 || status == 503 ? p.retryAfter : null,
    );
  }
  return Classification(OutcomeClass.failed, code: p.code, reason: p.reason);
}
