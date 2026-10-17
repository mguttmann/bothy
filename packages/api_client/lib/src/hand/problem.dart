// SPDX-License-Identifier: Apache-2.0

import 'package:dio/dio.dart';
import 'package:http_parser/http_parser.dart';

/// One entry of `errors` of a `validation_failed` problem (docs/spec/06-api.md 5 item 3).
class ProblemFieldError {
  /// Creates an entry from its members.
  const ProblemFieldError({
    required this.pointer,
    required this.code,
    this.detail,
  });

  /// JSON pointer into the body (`#path/<name>`, `#query/<name>` or `#header/<name>` for
  /// parameters); empty for a rule of the whole request.
  final String pointer;

  /// Machine-readable reason, for example `required` or `too_long`.
  final String code;

  /// English text for logs; never shown untranslated.
  final String? detail;
}

/// An RFC 9457 problem of the server, or a non-2xx response without one, as a typed error.
///
/// [code] is the stable key (06 5); an unknown code stays a string, and the apps translate
/// known codes and show unknown ones with the code (01 9). The message never contains the
/// request body.
///
/// Example:
/// ```dart
/// try {
///   await client.getMessagesApi().getMessage(messageId: id);
/// } on DioException catch (e) {
///   final problem = ApiProblem.of(e);
///   if (problem?.code == 'not_found') showGone();
/// }
/// ```
class ApiProblem implements Exception {
  /// Creates a problem from its members.
  const ApiProblem({
    required this.status,
    required this.code,
    this.title,
    this.detail,
    this.correlationId,
    this.reason,
    this.hint,
    this.errors = const [],
    this.current,
    this.retryAfter,
  });

  /// Builds the problem of a failed [response].
  ///
  /// A JSON object body with a string `code` is read as a problem; any other body gives the
  /// code `http_<status>`. `correlationId` comes from the body, else from the header
  /// `Correlation-Id`; `retryAfter` from the header `Retry-After` (seconds or an HTTP date,
  /// relative to [now]). Members of an unexpected type are left out instead of failing.
  factory ApiProblem.fromResponse(Response<Object?> response, {DateTime? now}) {
    final status = response.statusCode ?? 0;
    final body = response.data;
    final json = body is Map && body['code'] is String
        ? body.cast<String, Object?>()
        : const <String, Object?>{};
    return ApiProblem(
      status: status,
      code: json['code'] as String? ?? 'http_$status',
      title: _string(json['title']),
      detail: _string(json['detail']),
      correlationId:
          _string(json['correlation_id']) ??
          response.headers.value('correlation-id'),
      reason: _string(json['reason']),
      hint: _string(json['hint']),
      errors: _errors(json['errors']),
      current: json['current'] is Map
          ? (json['current']! as Map).cast<String, Object?>()
          : null,
      retryAfter: parseRetryAfter(
        response.headers.value('retry-after'),
        now: now,
      ),
    );
  }

  /// The [ApiProblem] that [error] carries, or `null` for any other error.
  ///
  /// The client maps every response error to a [DioException] whose `error` is the problem.
  static ApiProblem? of(Object error) =>
      error is DioException && error.error is ApiProblem
      ? error.error! as ApiProblem
      : null;

  /// HTTP status code.
  final int status;

  /// The `code` member (06 5), or `http_<status>` without a problem body.
  final String code;

  /// English short text of the problem type; for logs.
  final String? title;

  /// English explanation of this occurrence; never content, secrets or internal paths.
  final String? detail;

  /// The trace ID of the request as the server logged it (06 2 "Correlation").
  final String? correlationId;

  /// Named cause of a refusal or conflict, for example `channel_archived`.
  final String? reason;

  /// The way out the server suggests, for example the configuration key to change.
  final String? hint;

  /// Field errors of `validation_failed`; empty otherwise.
  final List<ProblemFieldError> errors;

  /// The current resource of `version_conflict` and `base_version_conflict`, as raw JSON.
  final Map<String, Object?>? current;

  /// How long to wait before the next attempt, from the header `Retry-After`.
  final Duration? retryAfter;

  @override
  String toString() => [
    'ApiProblem $status $code',
    if (title != null) ': $title',
    if (correlationId != null) ' (correlation $correlationId)',
  ].join();

  static String? _string(Object? value) => value is String ? value : null;

  static List<ProblemFieldError> _errors(Object? value) => [
    if (value is List)
      for (final e in value)
        if (e is Map && e['pointer'] is String && e['code'] is String)
          ProblemFieldError(
            pointer: e['pointer']! as String,
            code: e['code']! as String,
            detail: _string(e['detail']),
          ),
  ];
}

/// Reads a `Retry-After` header [value]: delay seconds or an HTTP date (RFC 9110 10.2.3).
///
/// A date in the past gives [Duration.zero]; a missing or unreadable value gives `null`.
/// [now] defaults to the current time.
Duration? parseRetryAfter(String? value, {DateTime? now}) {
  if (value == null) return null;
  final trimmed = value.trim();
  final seconds = int.tryParse(trimmed);
  if (seconds != null) return seconds < 0 ? null : Duration(seconds: seconds);
  try {
    final wait = parseHttpDate(trimmed).difference(now ?? DateTime.now());
    return wait.isNegative ? Duration.zero : wait;
  } on FormatException {
    return null;
  }
}

/// Turns every response error into a [DioException] that carries an [ApiProblem].
///
/// Errors without a response (network, timeout, cancel) pass unchanged. The message of the
/// mapped exception is the problem's text, so no request body reaches logs through it.
class ProblemInterceptor extends Interceptor {
  @override
  void onError(DioException err, ErrorInterceptorHandler handler) {
    final response = err.response;
    if (response == null ||
        err.type != DioExceptionType.badResponse ||
        err.error is ApiProblem) {
      handler.next(err);
      return;
    }
    final problem = ApiProblem.fromResponse(response);
    handler.next(err.copyWith(error: problem, message: problem.toString()));
  }
}
