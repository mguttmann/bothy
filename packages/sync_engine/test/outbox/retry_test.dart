// SPDX-License-Identifier: Apache-2.0

// Retries with full jitter and the same ID; permanent errors end in `failed` (04 13.2 items 4
// and 5, brief M4-15 Design note 2).

import 'dart:async';
import 'dart:io';
import 'dart:math';

import 'package:api_client/api_client.dart';
import 'package:dio/dio.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late Harness h;
  setUp(() => h = Harness.open());
  tearDown(() => h.close());

  final base = const Duration(seconds: 2); // client.outbox_retry_base default
  final cap = const Duration(minutes: 5); // client.outbox_retry_cap default

  /// [wait] is the server's Retry-After, if the error names one.
  Future<void> expectRetried(Object error, {Duration? wait}) async {
    final exec = h.fake('createMessage')..then([error]);
    final e = await h.add('hello', thread: threadA);
    await h.outbox.start();
    await settle();
    final waiting = (await h.entry(e.id))!;
    expect(waiting.state, OutboxState.waiting);
    expect(waiting.attempts, 1);
    final delay = waiting.nextAttemptAt!.difference(h.time.now);
    if (wait != null) {
      // Retry-After is the floor; the full jitter of the first retry comes on top
      // (addendum of 2026-10-30).
      expect(delay >= wait, isTrue, reason: '$delay');
      expect(delay <= wait + base, isTrue, reason: '$delay');
    } else {
      // Full jitter for the first retry: uniform in [0, min(cap, base)].
      expect(delay.isNegative, isFalse);
      expect(delay <= base, isTrue);
    }
    expect(exec.calls, hasLength(1));
    await h.time.advance(delay);
    expect(exec.calls, hasLength(2));
    expect(exec.calls[1].id, e.id, reason: 'the same ID on every retry');
    expect(exec.calls[1].idempotencyKey, e.id);
    expect(await h.entry(e.id), isNull, reason: 'the retry was confirmed');
  }

  test('5xx is retried', () => expectRetried(problem(500, 'internal')));
  test(
    '503 without Retry-After uses the backoff',
    () => expectRetried(problem(503, 'unavailable')),
  );
  test(
    '503 waits Retry-After',
    () => expectRetried(
      problem(503, 'unavailable', retryAfter: const Duration(seconds: 30)),
      wait: const Duration(seconds: 30),
    ),
  );
  test(
    '429 without Retry-After uses the backoff',
    () => expectRetried(problem(429, 'rate_limited')),
  );
  test(
    '429 waits Retry-After',
    () => expectRetried(
      problem(429, 'rate_limited', retryAfter: const Duration(seconds: 12)),
      wait: const Duration(seconds: 12),
    ),
  );
  test('a timeout is retried', () => expectRetried(TimeoutException('slow')));
  test(
    'a transport timeout of the client is retried',
    () => expectRetried(
      DioException(
        requestOptions: RequestOptions(path: '/x'),
        type: DioExceptionType.receiveTimeout,
      ),
    ),
  );
  test(
    'a network error is retried',
    () => expectRetried(const SocketException('unreachable')),
  );
  test(
    '409 request_in_progress is retried',
    () => expectRetried(problem(409, 'request_in_progress')),
  );
  // A voice message sent while its file is still being checked (12 8.1 rule 1): the outbox
  // keeps it until the file is ready instead of failing it (FIX-voice-send, 15 13).
  test(
    '409 file_not_ready is retried',
    () => expectRetried(problem(409, 'file_not_ready')),
  );
  test(
    '409 file_not_ready waits Retry-After',
    () => expectRetried(
      problem(409, 'file_not_ready', retryAfter: const Duration(seconds: 5)),
      wait: const Duration(seconds: 5),
    ),
  );
  test('classify keeps file_not_ready as a retry with its Retry-After', () {
    final c = classify(
      problem(409, 'file_not_ready', retryAfter: const Duration(seconds: 5)),
    );
    expect(c.outcome, OutcomeClass.retry);
    expect(c.code, 'file_not_ready');
    expect(c.retryAfter, const Duration(seconds: 5));
  });

  test('the backoff grows with the attempts and stays below the cap', () async {
    final exec = h.fake('createMessage');
    exec.then(List.filled(12, problem(500, 'internal')));
    final e = await h.add('hello', thread: threadA);
    await h.outbox.start();
    await settle();
    for (var retry = 0; retry < 11; retry++) {
      final waiting = (await h.entry(e.id))!;
      final delay = waiting.nextAttemptAt!.difference(h.time.now);
      final bound = Duration(
        microseconds: min(
          cap.inMicroseconds,
          base.inMicroseconds * pow(2, retry).toInt(),
        ),
      );
      expect(delay <= bound, isTrue, reason: 'retry $retry: $delay > $bound');
      await h.time.advance(delay);
    }
    expect(exec.calls, hasLength(12));
    expect({for (final c in exec.calls) c.id}, {e.id});
  });

  test('the outbox jitter is the full-jitter function of the read retries', () {
    final ours = Random(3);
    final reads = ReadRetryPolicy.fromBackoff(
      base: base,
      cap: cap,
      random: Random(3),
    );
    for (var retry = 0; retry < 12; retry++) {
      expect(
        fullJitterBackoff(base: base, cap: cap, retry: retry, random: ours),
        reads.backoff(retry),
      );
    }
  });

  for (final (status, code) in [
    (400, 'validation_failed'),
    (403, 'forbidden'),
    (404, 'not_found'),
    (409, 'conflict'),
    (409, 'file_blocked'),
    (410, 'thread_deleted'),
    (413, 'payload_too_large'),
    (415, 'unsupported_media_type'),
    (422, 'idempotency_key_reuse'),
    (428, 'precondition_required'),
    (401, 'unauthorized'),
    (503, 'setup_required'),
  ]) {
    test('$status $code is final with the reason', () async {
      final exec = h.fake('createMessage')
        ..then([problem(status, code, reason: 'because')]);
      final e = await h.add('hello', thread: threadA);
      await h.outbox.start();
      await settle();
      final failed = (await h.entry(e.id))!;
      expect(failed.state, OutboxState.failed);
      expect(failed.lastErrorCode, code);
      expect(failed.lastErrorReason, 'because');
      expect(failed.payload, contains('hello'), reason: 'the text stays');
      await h.time.advance(const Duration(hours: 1));
      expect(exec.calls, hasLength(1), reason: 'no further call');
    });
  }

  test('a problem inside a DioException is classified by its status', () {
    DioException wrap(ApiProblem p) => DioException(
      requestOptions: RequestOptions(path: '/x'),
      type: DioExceptionType.badResponse,
      error: p,
    );
    expect(
      classify(wrap(problem(500, 'internal'))).outcome,
      OutcomeClass.retry,
    );
    expect(
      classify(wrap(problem(404, 'not_found'))).outcome,
      OutcomeClass.failed,
    );
    expect(
      classify(wrap(problem(412, 'version_conflict', current: {'version': 3})))
          .outcome,
      OutcomeClass.conflict,
    );
  });

  test(
    'a response error without a mapped problem is read from the response',
    () {
      final options = RequestOptions(path: '/x');
      final failure = classify(
        DioException(
          requestOptions: options,
          type: DioExceptionType.badResponse,
          response: Response<Object?>(
            requestOptions: options,
            statusCode: 413,
            data: {'code': 'payload_too_large'},
          ),
        ),
      );
      expect(failure.outcome, OutcomeClass.failed);
      expect(failure.code, 'payload_too_large');
    },
  );

  test('a programming error of an executor is final', () {
    final failure = classify(StateError('bug'));
    expect(failure.outcome, OutcomeClass.failed);
    expect(failure.code, 'executor_error');
  });

  test('a kind without an executor fails with no_executor', () async {
    final e = await h.add('hello', thread: threadA, kind: 'unknownOperation');
    await h.outbox.start();
    await settle();
    final failed = (await h.entry(e.id))!;
    expect(failed.state, OutboxState.failed);
    expect(failed.lastErrorCode, 'no_executor');
  });

  test('a lower base from AppConfig shortens the backoff', () async {
    await h.close();
    h = Harness.open(
      config: {
        'client.outbox_retry_base': '100ms',
        'client.outbox_retry_cap': '200ms',
      },
    );
    h.fake('createMessage').then(List.filled(5, problem(502, 'bad_gateway')));
    final e = await h.add('hello', thread: threadA);
    await h.outbox.start();
    await settle();
    for (var i = 0; i < 5; i++) {
      final w = (await h.entry(e.id))!;
      expect(
        w.nextAttemptAt!.difference(h.time.now) <=
            const Duration(milliseconds: 200),
        isTrue,
      );
      await h.time.advance(const Duration(milliseconds: 200));
    }
    expect(await h.entry(e.id), isNull);
  });
}
