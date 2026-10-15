// SPDX-License-Identifier: Apache-2.0

// Idempotency-Key from the caller, traceparent on every request, Correlation-Id exposed
// (docs/spec/06-api.md 2 "Correlation" and 7, 15 17).

import 'package:api_client/api_client.dart';
import 'package:dio/dio.dart';
import 'package:test/test.dart';

import 'support/fake_adapter.dart';

const _threadId = '0192f4a0-7b1c-78f9-8074-fa94e84bab7d';
const _key = '0192f4a0-7b1c-7000-8000-00000000abcd';
final _traceparent = RegExp(r'^00-([0-9a-f]{32})-([0-9a-f]{16})-0[01]$');

Map<String, Object?> message() => {
  'id': '0192f4a0-7b1c-7353-8d8c-dea659467151',
  'thread_id': _threadId,
  'channel_id': '0192f4a0-7b1c-72d4-8bae-7661d2b2f203',
  'thread_seq': 1,
  'author': {
    'id': '0192f4a0-7b1c-7f89-8d6b-6960453241bc',
    'kind': 'user',
    'handle': 'alex',
    'display_name': 'Alex',
  },
  'client_msg_id': _key,
  'origin': 'human',
  'kind': 'text',
  'content_md': 'Hello Robin',
  'content_format': 1,
  'silent': false,
  'attachments': <Object>[],
  'mention_ids': <Object>[],
  'reactions': <Object>[],
  'pinned': false,
  'components': <Object>[],
  'version': 1,
  'created_at': '2026-10-10T09:30:00.000Z',
};

Future<void> send(ApiClient client, {String? key}) =>
    client.getMessagesApi().createMessage(
      threadId: _threadId,
      messageCreate: MessageCreate((b) => b..clientMsgId = _key),
      extra: {idempotencyKeyExtra: ?key},
    );

void main() {
  test('POST with idempotency_key carries Idempotency-Key', () async {
    final adapter = FakeAdapter()..json(201, message());

    await send(fakeClient(adapter), key: _key);

    final request = adapter.requests.single.options;
    expect(request.method, 'POST');
    expect(request.headers['Idempotency-Key'], _key);
  });

  test('GET never carries Idempotency-Key', () async {
    final adapter = FakeAdapter()..json(200, message());

    await fakeClient(adapter).getMessagesApi().getMessage(
      messageId: '0192f4a0-7b1c-7353-8d8c-dea659467151',
      extra: {idempotencyKeyExtra: _key},
    );

    expect(
      adapter.requests.single.options.headers.containsKey('Idempotency-Key'),
      isFalse,
    );
  });

  test('a POST without the option carries none', () async {
    final adapter = FakeAdapter()..json(201, message());

    await send(fakeClient(adapter));

    expect(
      adapter.requests.single.options.headers.containsKey('Idempotency-Key'),
      isFalse,
    );
  });

  test(
    'every request carries a valid traceparent with a new span ID',
    () async {
      final adapter = FakeAdapter()
        ..json(201, message())
        ..json(200, message())
        ..json(201, message());
      final client = fakeClient(adapter);

      await send(client, key: _key);
      await client.getMessagesApi().getMessage(
        messageId: '0192f4a0-7b1c-7353-8d8c-dea659467151',
      );
      await send(client, key: _key);

      final spans = <String>{};
      for (final r in adapter.requests) {
        final value = r.options.headers['traceparent'] as String;
        final match = _traceparent.firstMatch(value);
        expect(match, isNotNull, reason: value);
        expect(match!.group(1), fixedTrace().traceId);
        expect(match.group(2), isNot('0000000000000000'));
        expect(value.endsWith('-01'), isTrue);
        spans.add(match.group(2)!);
      }
      expect(spans, hasLength(3));
    },
  );

  test('the sampled flag comes from the callback', () async {
    final adapter = FakeAdapter()..json(200, message());
    final client = fakeClient(
      adapter,
      trace: () => TraceContext(
        traceId: '0af7651916cd43dd8448eb211c80319c',
        sampled: false,
      ),
    );

    await client.getMessagesApi().getMessage(
      messageId: '0192f4a0-7b1c-7353-8d8c-dea659467151',
    );

    expect(
      adapter.requests.single.options.headers['traceparent'],
      matches(r'^00-0af7651916cd43dd8448eb211c80319c-[0-9a-f]{16}-00$'),
    );
  });

  test('an invalid trace ID is refused', () {
    expect(
      () => TraceContext(traceId: '0' * 32, sampled: true),
      throwsArgumentError,
    );
    expect(
      () => TraceContext(traceId: 'ABC', sampled: true),
      throwsArgumentError,
    );
  });

  test('Correlation-Id of the response is exposed', () async {
    final adapter = FakeAdapter()
      ..json(
        200,
        message(),
        headers: {'Correlation-Id': '4bf92f35-77b3-4da6-a3ce-929d0e0e4736'},
      );

    final response = await fakeClient(adapter)
        .getMessagesApi()
        .getMessage(messageId: '0192f4a0-7b1c-7353-8d8c-dea659467151');

    final meta = ResponseMeta.of(response);
    expect(meta.correlationId, '4bf92f35-77b3-4da6-a3ce-929d0e0e4736');
    expect(meta.statusCode, 200);
    expect(
      ResponseMeta.of(Response<void>(requestOptions: RequestOptions()))
          .correlationId,
      isNull,
    );
  });

  test('the base URL gets /api/v1 once', () async {
    final adapter = FakeAdapter()
      ..json(200, message())
      ..json(200, message());
    for (final base in [
      'https://chat.example.com/',
      'https://example.com/chat',
    ]) {
      final client = createApiClient(
        baseUrl: Uri.parse(base),
        traceContext: fixedTrace,
        readRetry: ReadRetryPolicy.fromBackoff(
          base: const Duration(milliseconds: 100),
          cap: const Duration(seconds: 1),
        ),
        adapter: adapter,
      );
      await client.getMessagesApi().getMessage(messageId: 'm1');
    }
    expect(adapter.requests.map((r) => r.options.uri.toString()), [
      'https://chat.example.com/api/v1/messages/m1',
      'https://example.com/chat/api/v1/messages/m1',
    ]);
  });

  test(
    'auth interceptors run after the headers and before the problem mapping',
    () async {
      final seen = <String?>[];
      final adapter = FakeAdapter()
        ..problem(401, {
          'type': '/api/v1/problems/use_dpop_nonce',
          'title': 'Nonce',
          'status': 401,
          'code': 'use_dpop_nonce',
          'correlation_id': '4bf92f35-77b3-4da6-a3ce-929d0e0e4736',
        });
      final client = fakeClient(
        adapter,
        authInterceptors: [
          InterceptorsWrapper(
            onRequest: (options, handler) {
              seen.add(options.headers['traceparent'] as String?);
              options.headers['Authorization'] = 'DPoP example';
              handler.next(options);
            },
            onError: (error, handler) {
              // The slot of M4-09 sees the raw error before the mapping.
              seen.add(error.error is ApiProblem ? 'mapped' : 'raw');
              handler.next(error);
            },
          ),
        ],
      );

      await expectLater(
        client.getMessagesApi().getMessage(messageId: 'm1'),
        throwsA(isA<DioException>()),
      );

      expect(seen.first, matches(_traceparent));
      expect(seen.last, 'raw');
      expect(
        adapter.requests.single.options.headers['Authorization'],
        'DPoP example',
      );
    },
  );
}
