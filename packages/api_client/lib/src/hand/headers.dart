// SPDX-License-Identifier: Apache-2.0

import 'dart:math';

import 'package:dio/dio.dart';

/// Key of [RequestOptions.extra] that carries the caller's idempotency key.
///
/// The outbox (M4-15) passes its stored UUIDv7 here; [HeadersInterceptor] writes it as the
/// header `Idempotency-Key` on writes (docs/spec/06-api.md 7).
///
/// Example:
/// ```dart
/// await client.getMessagesApi().createMessage(
///   threadId: threadId,
///   messageCreate: body,
///   extra: {idempotencyKeyExtra: outboxEntry.id},
/// );
/// ```
const String idempotencyKeyExtra = 'idempotency_key';

const _writeMethods = {'POST', 'PATCH', 'PUT', 'DELETE'};
final _traceIdPattern = RegExp(r'^[0-9a-f]{32}$');

/// The W3C trace context the caller continues for one request (docs/spec/15 17).
///
/// The client derives a new span ID per request from it; the server only links the
/// `traceparent` it receives (06 2 "Correlation").
class TraceContext {
  /// Creates a context for [traceId] (32 lowercase hex digits, not all zero) and the
  /// [sampled] flag.
  ///
  /// Throws an [ArgumentError] for a malformed or all-zero [traceId], which W3C trace
  /// context forbids.
  TraceContext({required this.traceId, required this.sampled}) {
    if (!_traceIdPattern.hasMatch(traceId) || traceId == '0' * 32) {
      throw ArgumentError.value(
        traceId,
        'traceId',
        'must be 32 lowercase hex digits, not all zero',
      );
    }
  }

  /// The trace ID: 16 bytes as 32 lowercase hex digits.
  final String traceId;

  /// Whether the caller records this trace; becomes the flag `01` or `00`.
  final bool sampled;
}

/// Writes `traceparent` on every request and `Idempotency-Key` on writes that carry
/// [idempotencyKeyExtra].
///
/// It never invents a key: a write without one is the caller's choice for operations marked
/// `x-idempotency-exempt` (brief M4-02 Design note 4).
class HeadersInterceptor extends Interceptor {
  /// Creates the interceptor; [traceContext] is called once per request, and [random]
  /// draws the span IDs (a secure generator unless a test passes one).
  HeadersInterceptor({required this.traceContext, Random? random})
    : _random = random ?? Random.secure();

  /// Returns the trace context of the next request.
  final TraceContext Function() traceContext;
  final Random _random;

  @override
  void onRequest(RequestOptions options, RequestInterceptorHandler handler) {
    final trace = traceContext();
    options.headers['traceparent'] =
        '00-${trace.traceId}-${_spanId()}-${trace.sampled ? '01' : '00'}';
    final key = options.extra[idempotencyKeyExtra];
    if (_writeMethods.contains(options.method.toUpperCase()) &&
        key is String &&
        key.isNotEmpty) {
      options.headers['Idempotency-Key'] = key;
    }
    handler.next(options);
  }

  /// Eight random bytes as hex; an all-zero span ID is invalid in W3C trace context.
  String _spanId() {
    while (true) {
      final id = List.generate(
        8,
        (_) => _random.nextInt(256).toRadixString(16).padLeft(2, '0'),
      ).join();
      if (id != '0000000000000000') return id;
    }
  }
}

/// What a caller may log about a response: its status and the server's `Correlation-Id`.
///
/// The value is not secret (06 2 "Correlation"); it is the key for the server's log search.
class ResponseMeta {
  /// Creates the metadata from its parts.
  const ResponseMeta({this.statusCode, this.correlationId});

  /// Reads [response]; members the response lacks stay `null`.
  ///
  /// Example:
  /// ```dart
  /// final response = await client.getMessagesApi().getMessage(messageId: id);
  /// log('getMessage ${ResponseMeta.of(response).correlationId}');
  /// ```
  factory ResponseMeta.of(Response<Object?> response) => ResponseMeta(
    statusCode: response.statusCode,
    correlationId: response.headers.value('correlation-id'),
  );

  /// The HTTP status code, if the response has one.
  final int? statusCode;

  /// The header `Correlation-Id` of the response, if present.
  final String? correlationId;
}
