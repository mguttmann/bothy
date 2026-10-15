// SPDX-License-Identifier: Apache-2.0

// A fake network for the tests: records every request and answers from a queue, so no test
// opens a socket (brief M4-02, Tests).

import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:api_client/api_client.dart';
import 'package:dio/dio.dart';

/// One request the client sent, with its body as text.
class RecordedRequest {
  /// Records [options] and the [body] the client wrote.
  RecordedRequest(this.options, this.body);

  /// The options as the adapter saw them (method, path, headers).
  final RequestOptions options;

  /// The request body as UTF-8 text; empty without a body.
  final String body;

  /// The JSON body, decoded.
  Object? get json => body.isEmpty ? null : jsonDecode(body);
}

/// A canned answer: a response or an error the adapter throws.
typedef Answer = Future<ResponseBody> Function(RequestOptions options);

/// An [HttpClientAdapter] that records requests and answers them in order.
class FakeAdapter implements HttpClientAdapter {
  /// The requests seen so far, in order.
  final List<RecordedRequest> requests = [];

  final List<Answer> _answers = [];

  /// Queues a JSON answer.
  void json(
    int status,
    Object body, {
    Map<String, String> headers = const {},
    String contentType = 'application/json',
  }) {
    _answers.add(
      (_) async => ResponseBody.fromString(
        jsonEncode(body),
        status,
        headers: {
          Headers.contentTypeHeader: [contentType],
          for (final e in headers.entries) e.key.toLowerCase(): [e.value],
        },
      ),
    );
  }

  /// Queues an RFC 9457 problem answer.
  void problem(
    int status,
    Map<String, Object?> body, {
    Map<String, String> headers = const {},
  }) => json(
    status,
    body,
    headers: headers,
    contentType: 'application/problem+json',
  );

  /// Queues an answer without a body.
  void empty(int status, {Map<String, String> headers = const {}}) {
    _answers.add(
      (_) async => ResponseBody.fromString(
        '',
        status,
        headers: {
          for (final e in headers.entries) e.key.toLowerCase(): [e.value],
        },
      ),
    );
  }

  /// Queues a socket error, as the IO adapter of dio reports it.
  void socketError() {
    _answers.add(
      (options) async => throw DioException.connectionError(
        requestOptions: options,
        reason: 'connection refused',
        error: const SocketException('connection refused'),
      ),
    );
  }

  /// Whether every queued answer was used.
  bool get drained => _answers.isEmpty;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    final bytes = <int>[];
    if (requestStream != null) {
      await for (final chunk in requestStream) {
        bytes.addAll(chunk);
      }
    }
    // A retry sends the same options again; the copy keeps the headers of this attempt.
    requests.add(
      RecordedRequest(
        options.copyWith(headers: Map.of(options.headers)),
        utf8.decode(bytes),
      ),
    );
    if (_answers.isEmpty) {
      throw StateError(
        'no answer queued for ${options.method} ${options.path}',
      );
    }
    return _answers.removeAt(0)(options);
  }

  @override
  void close({bool force = false}) {}
}

/// A trace context with a fixed trace ID, sampled.
TraceContext fixedTrace() =>
    TraceContext(traceId: '4bf92f3577b34da6a3ce929d0e0e4736', sampled: true);

/// The sleeps a [ReadRetryPolicy] asked for; nothing waits.
class FakeClock {
  /// Every requested wait, in order.
  final List<Duration> sleeps = [];

  /// Records [d] and returns at once.
  Future<void> sleep(Duration d) async => sleeps.add(d);
}

/// A client over [adapter] with a retry policy of base 100 ms and cap 1 s on [clock].
ApiClient fakeClient(
  FakeAdapter adapter, {
  FakeClock? clock,
  List<Interceptor> authInterceptors = const [],
  TraceContext Function() trace = fixedTrace,
}) => createApiClient(
  baseUrl: Uri.parse('https://chat.example.com'),
  traceContext: trace,
  readRetry: ReadRetryPolicy.fromBackoff(
    base: const Duration(milliseconds: 100),
    cap: const Duration(seconds: 1),
    sleep: (clock ?? FakeClock()).sleep,
  ),
  authInterceptors: authInterceptors,
  adapter: adapter,
);
