// SPDX-License-Identifier: Apache-2.0

// The REST client ends a request whose server accepts the connection and never answers
// (`client.http_receive_timeout`, 15 22), so it cannot hold an outbox lane forever.

import 'dart:async';
import 'dart:io';

import 'package:api_client/api_client.dart';
import 'package:dio/dio.dart';
import 'package:test/test.dart';

import 'support/fake_adapter.dart';

void main() {
  test('a server that accepts and stays silent ends the request with a '
      'receive timeout', () async {
    final server = await ServerSocket.bind(InternetAddress.loopbackIPv4, 0);
    final accepted = <Socket>[];
    server.listen(accepted.add);
    addTearDown(() async {
      for (final s in accepted) {
        s.destroy();
      }
      await server.close();
    });
    final client = createApiClient(
      baseUrl: Uri.parse('http://127.0.0.1:${server.port}'),
      traceContext: fixedTrace,
      readRetry: ReadRetryPolicy(
        base: Duration.zero,
        cap: Duration.zero,
        maxAttempts: 1,
      ),
      connectTimeout: const Duration(seconds: 5),
      receiveTimeout: const Duration(milliseconds: 200),
    );
    addTearDown(() => client.dio.close(force: true));

    await expectLater(
      client.dio
          .post<Object>('/messages', data: {'body': 'hello'})
          .timeout(const Duration(seconds: 10)),
      throwsA(
        isA<DioException>().having(
          (e) => e.type,
          'type',
          DioExceptionType.receiveTimeout,
        ),
      ),
    );
  });

  test('the timeouts are the base options of every request', () {
    final client = createApiClient(
      baseUrl: Uri.parse('https://chat.example.com'),
      traceContext: fixedTrace,
      readRetry: ReadRetryPolicy(
        base: Duration.zero,
        cap: Duration.zero,
        maxAttempts: 1,
      ),
      connectTimeout: const Duration(seconds: 30),
      receiveTimeout: const Duration(minutes: 2),
    );
    expect(client.dio.options.connectTimeout, const Duration(seconds: 30));
    expect(client.dio.options.receiveTimeout, const Duration(minutes: 2));
    expect(client.dio.options.sendTimeout, isNull);
  });
}
