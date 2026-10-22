// SPDX-License-Identifier: Apache-2.0

/// The `api_client` factory of the console (brief M6-02 Design note 2, D-M6-02-2): the browser's
/// HTTP stack with the cookie of the same origin, the CSRF header of M6-01 on every
/// state-changing request, never `Authorization` or `DPoP`, and no token anywhere.
library;

import 'dart:math';

import 'package:api_client/api_client.dart';
import 'package:dio/dio.dart';

/// The CSRF header of the console session (parameter `ConsoleRequest` of openapi.yaml,
/// docs/DECISIONS.md D-M6-01-5).
const String consoleRequestHeader = 'X-Console-Request';

/// The only value of [consoleRequestHeader].
const String consoleRequestValue = '1';

/// Methods that change nothing (RFC 9110 9.2.1); every other method carries the CSRF header.
const Set<String> _safeMethods = {'GET', 'HEAD', 'OPTIONS'};

/// Headers the console never sends: it holds no token and no DPoP key (08 7.5).
const Set<String> _forbiddenHeaders = {'authorization', 'dpop'};

/// Adds the CSRF header to state-changing requests, removes every credential header and
/// reports each `401` with its problem.
class ConsoleRequestInterceptor extends Interceptor {
  /// Creates the interceptor; [onUnauthorized] gets the problem of every `401`.
  ConsoleRequestInterceptor({required this.onUnauthorized});

  /// Called with the problem of every `401` answer, before the caller sees the error.
  final void Function(ApiProblem problem) onUnauthorized;

  @override
  void onRequest(RequestOptions options, RequestInterceptorHandler handler) {
    options.headers.removeWhere(
      (name, _) => _forbiddenHeaders.contains(name.toLowerCase()),
    );
    if (!_safeMethods.contains(options.method.toUpperCase())) {
      options.headers[consoleRequestHeader] = consoleRequestValue;
    }
    handler.next(options);
  }

  @override
  void onError(DioException err, ErrorInterceptorHandler handler) {
    final response = err.response;
    if (response != null && response.statusCode == 401) {
      onUnauthorized(ApiProblem.fromResponse(response));
    }
    handler.next(err);
  }
}

/// Builds the generated API root for the console at [origin], the origin the console is served
/// from (same origin as the API).
///
/// [onUnauthorized] receives the problem of every `401`; [adapter] replaces the browser's HTTP
/// stack in tests (null keeps Dio's browser adapter, which sends the cookies of the same origin).
/// Reads are not retried: the console fetches on opening (15 10, row Web). Throws an
/// [ArgumentError] when [origin] is not an absolute http or https address.
///
/// Example:
/// ```dart
/// final api = createConsoleApiClient(
///   origin: Uri.parse('https://chat.example.com'),
///   onUnauthorized: controller.onUnauthorized,
/// );
/// final session = await api.getAuthApi().getConsoleSession();
/// ```
ApiClient createConsoleApiClient({
  required Uri origin,
  required void Function(ApiProblem problem) onUnauthorized,
  HttpClientAdapter? adapter,
  Random? random,
}) {
  final rnd = random ?? Random.secure();
  return createApiClient(
    baseUrl: origin,
    traceContext: () => _traceContext(rnd),
    readRetry: ReadRetryPolicy(
      base: Duration.zero,
      cap: Duration.zero,
      maxAttempts: 1,
    ),
    authInterceptors: [
      ConsoleRequestInterceptor(onUnauthorized: onUnauthorized),
    ],
    adapter: adapter,
  );
}

/// A fresh sampled-out trace context per request; the console continues no trace of its own.
TraceContext _traceContext(Random random) {
  while (true) {
    final id = List.generate(
      16,
      (_) => random.nextInt(256).toRadixString(16).padLeft(2, '0'),
    ).join();
    if (id != '0' * 32) return TraceContext(traceId: id, sampled: false);
  }
}
