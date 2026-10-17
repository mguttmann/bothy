// SPDX-License-Identifier: Apache-2.0

import 'package:api_client/src/generated/api.dart';
import 'package:api_client/src/hand/headers.dart';
import 'package:api_client/src/hand/problem.dart';
import 'package:api_client/src/hand/read_retry.dart';
import 'package:dio/dio.dart';

/// Builds the generated API root for one server address (brief M4-02 Design note 3).
///
/// One `Dio` with `baseUrl` = [baseUrl] plus `/api/v1` (06 R-API-01), JSON responses and the
/// interceptors in this fixed order: [HeadersInterceptor] (`traceparent` from [traceContext],
/// `Idempotency-Key` from the request option [idempotencyKeyExtra]), then [authInterceptors]
/// (the slot for DPoP, nonce, refresh and step-up of M4-09), then [ProblemInterceptor], then
/// [ReadRetryInterceptor] with [readRetry]. Errors reach the caller as a `DioException`; a
/// response error carries an [ApiProblem] (`ApiProblem.of`). The generated auth interceptors
/// of openapi-generator are not installed.
///
/// [adapter] replaces the network (tests and the fake server of M4-17); [connectTimeout]
/// comes from the caller's configuration, the default is the one of `Dio`. There is no global
/// state: each call returns an independent client. Throws an [ArgumentError] when [baseUrl]
/// is not an absolute `http` or `https` address.
///
/// Example:
/// ```dart
/// final client = createApiClient(
///   baseUrl: Uri.parse('https://chat.example.com'),
///   traceContext: tracer.next,
///   readRetry: ReadRetryPolicy.fromBackoff(base: base, cap: cap),
///   authInterceptors: auth.interceptors,
/// );
/// final me = await client.getMeApi().getMe();
/// ```
ApiClient createApiClient({
  required Uri baseUrl,
  required TraceContext Function() traceContext,
  required ReadRetryPolicy readRetry,
  List<Interceptor> authInterceptors = const [],
  HttpClientAdapter? adapter,
  Duration? connectTimeout,
}) {
  if (!baseUrl.isAbsolute ||
      !(baseUrl.isScheme('https') || baseUrl.isScheme('http'))) {
    throw ArgumentError.value(
      baseUrl,
      'baseUrl',
      'must be an absolute http or https address',
    );
  }
  final root = baseUrl.toString().replaceFirst(RegExp(r'/+$'), '');
  final dio = Dio(
    BaseOptions(
      baseUrl: '$root${ApiClient.basePath}',
      responseType: ResponseType.json,
      connectTimeout: connectTimeout,
    ),
  );
  if (adapter != null) dio.httpClientAdapter = adapter;
  dio.interceptors.addAll([
    HeadersInterceptor(traceContext: traceContext),
    ...authInterceptors,
    ProblemInterceptor(),
    ReadRetryInterceptor(dio: dio, policy: readRetry),
  ]);
  return ApiClient(dio: dio, interceptors: const []);
}
