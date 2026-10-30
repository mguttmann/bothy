// SPDX-License-Identifier: Apache-2.0

// Retry only for reads, with Retry-After or the caller's full-jitter backoff (brief M4-02
// Design note 4; 15 8: the outbox retries writes).

import 'dart:math';

import 'package:api_client/api_client.dart';
import 'package:dio/dio.dart';
import 'package:test/test.dart';

import 'support/fake_adapter.dart';

const _id = '0192f4a0-7b1c-7353-8d8c-dea659467151';

Map<String, Object?> problem(int status, String code) => {
  'type': '/api/v1/problems/$code',
  'title': code,
  'status': status,
  'code': code,
  'correlation_id': '4bf92f35-77b3-4da6-a3ce-929d0e0e4736',
};

final _instance = {'name': 'Example'};

void main() {
  test('GET is retried after a socket error', () async {
    final clock = FakeClock();
    final adapter = FakeAdapter()
      ..socketError()
      ..json(200, _instance);

    final response = await fakeClient(
      adapter,
      clock: clock,
    ).dio.get<Object>('/instance');

    expect(response.statusCode, 200);
    expect(adapter.requests, hasLength(2));
    expect(clock.sleeps, hasLength(1));
    expect(
      clock.sleeps.single,
      lessThanOrEqualTo(const Duration(milliseconds: 100)),
    );
  });

  test('GET is retried after 503 with Retry-After: 1', () async {
    final clock = FakeClock();
    final adapter = FakeAdapter()
      ..problem(503, problem(503, 'unavailable'), headers: {'Retry-After': '1'})
      ..json(200, _instance);

    final response = await fakeClient(
      adapter,
      clock: clock,
    ).dio.get<Object>('/instance');

    expect(response.statusCode, 200);
    // Retry-After is the floor; the full jitter of the first retry comes on top
    // (FIX-upload-retry-storm).
    expect(clock.sleeps.single.inMilliseconds, inInclusiveRange(1000, 1100));
    // Each attempt is a new request with its own span.
    expect(
      adapter.requests[0].options.headers['traceparent'],
      isNot(adapter.requests[1].options.headers['traceparent']),
    );
  });

  test('GET is retried after 429 and waits an HTTP-date Retry-After', () async {
    final clock = FakeClock();
    final adapter = FakeAdapter()
      ..problem(
        429,
        problem(429, 'rate_limited'),
        headers: {'Retry-After': 'Wed, 21 Oct 2015 07:28:00 GMT'},
      )
      ..json(200, _instance);

    await fakeClient(adapter, clock: clock).dio.get<Object>('/instance');

    expect(adapter.requests, hasLength(2));
    // A date in the past means: no wait beyond the jitter.
    expect(
      clock.sleeps.single,
      lessThanOrEqualTo(const Duration(milliseconds: 100)),
    );
  });

  test('GET is not retried after 404', () async {
    final clock = FakeClock();
    final adapter = FakeAdapter()..problem(404, problem(404, 'not_found'));

    await expectLater(
      fakeClient(
        adapter,
        clock: clock,
      ).getMessagesApi().getMessage(messageId: _id),
      throwsA(isA<DioException>()),
    );

    expect(adapter.requests, hasLength(1));
    expect(clock.sleeps, isEmpty);
  });

  test('GET is not retried after 503 setup_required', () async {
    final clock = FakeClock();
    final adapter = FakeAdapter()..problem(503, problem(503, 'setup_required'));

    try {
      await fakeClient(adapter, clock: clock).dio.get<Object>('/instance');
      fail('the call did not fail');
    } on DioException catch (e) {
      expect(ApiProblem.of(e)?.code, 'setup_required');
    }

    expect(adapter.requests, hasLength(1));
    expect(clock.sleeps, isEmpty);
  });

  test('POST is never retried', () async {
    final clock = FakeClock();
    final adapter = FakeAdapter()
      ..problem(
        503,
        problem(503, 'unavailable'),
        headers: {'Retry-After': '1'},
      );

    await expectLater(
      fakeClient(adapter, clock: clock).getMessagesApi().createMessage(
        threadId: _id,
        messageCreate: MessageCreate((b) => b..clientMsgId = _id),
        extra: {idempotencyKeyExtra: _id},
      ),
      throwsA(isA<DioException>()),
    );
    final socket = FakeAdapter()..socketError();
    await expectLater(
      fakeClient(
        socket,
        clock: clock,
      ).dio.post<Object>('/threads', data: {'x': 1}),
      throwsA(isA<DioException>()),
    );

    expect(adapter.requests, hasLength(1));
    expect(socket.requests, hasLength(1));
    expect(clock.sleeps, isEmpty);
  });

  test('at most maxAttempts', () async {
    final clock = FakeClock();
    final adapter = FakeAdapter();
    // base 100 ms, cap 1 s: bounds 100, 200, 400, 800, 1000 ms, so five attempts.
    for (var i = 0; i < 6; i++) {
      adapter.socketError();
    }

    await expectLater(
      fakeClient(adapter, clock: clock).dio.get<Object>('/instance'),
      throwsA(
        isA<DioException>().having(
          (e) => e.type,
          'type',
          DioExceptionType.connectionError,
        ),
      ),
    );

    expect(adapter.requests, hasLength(5));
    expect(clock.sleeps, hasLength(4));
    final bounds = [100, 200, 400, 800];
    for (var i = 0; i < 4; i++) {
      expect(clock.sleeps[i].inMilliseconds, inInclusiveRange(0, bounds[i]));
    }
  });

  test('maxAttempts follows from base and cap', () {
    ReadRetryPolicy policy(int base, int cap) => ReadRetryPolicy.fromBackoff(
      base: Duration(milliseconds: base),
      cap: Duration(milliseconds: cap),
    );
    expect(policy(100, 1000).maxAttempts, 5);
    expect(policy(1000, 60000).maxAttempts, 7);
    expect(policy(1000, 1000).maxAttempts, 1);
    expect(policy(2000, 1000).maxAttempts, 1);
    expect(policy(0, 1000).maxAttempts, 1);
  });

  test('the full-jitter delay stays within the bound and the cap', () {
    final policy = ReadRetryPolicy.fromBackoff(
      base: const Duration(milliseconds: 100),
      cap: const Duration(milliseconds: 300),
      random: Random(1),
    );
    for (var n = 0; n < 10; n++) {
      final d = policy.backoff(n);
      expect(
        d.inMicroseconds,
        inInclusiveRange(0, min(100 * 1000 << n, 300 * 1000)),
      );
    }
  });

  test('a read that opts out is not retried, also after 429 '
      '(FIX-upload-retry-storm)', () async {
    final clock = FakeClock();
    final adapter = FakeAdapter()
      ..problem(
        429,
        problem(429, 'rate_limited'),
        headers: {'Retry-After': '1'},
      )
      ..json(200, _instance);

    await expectLater(
      fakeClient(adapter, clock: clock).dio.get<Object>(
        '/instance',
        options: Options(extra: {noReadRetryExtra: true}),
      ),
      throwsA(
        isA<DioException>().having(
          (e) => e.response?.statusCode,
          'status',
          429,
        ),
      ),
    );

    expect(adapter.requests, hasLength(1));
    expect(clock.sleeps, isEmpty);
  });

  test('reads refused together with the same Retry-After wait different '
      'times (FIX-upload-retry-storm)', () async {
    final waits = <Duration>{};
    for (var i = 0; i < 5; i++) {
      final clock = FakeClock();
      final adapter = FakeAdapter()
        ..problem(
          429,
          problem(429, 'rate_limited'),
          headers: {'Retry-After': '1'},
        )
        ..json(200, _instance);
      await fakeClient(adapter, clock: clock).dio.get<Object>('/instance');
      waits.add(clock.sleeps.single);
    }
    expect(waits.length, greaterThan(1));
  });
}
