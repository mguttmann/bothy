// SPDX-License-Identifier: Apache-2.0

// Mapping of RFC 9457 problems to ApiProblem with their `code` (docs/spec/06-api.md 5).

import 'package:api_client/api_client.dart';
import 'package:dio/dio.dart';
import 'package:test/test.dart';

import 'support/fake_adapter.dart';

const _messageId = '0192f4a0-7b1c-7353-8d8c-dea659467151';
const _correlation = '4bf92f35-77b3-4da6-a3ce-929d0e0e4736';

Map<String, Object?> problemBody(
  int status,
  String code, [
  Map<String, Object?> extra = const {},
]) => {
  'type': '/api/v1/problems/$code',
  'title': 'Title of $code',
  'status': status,
  'code': code,
  'correlation_id': _correlation,
  ...extra,
};

Future<ApiProblem> problemOf(Future<Object?> call) async {
  try {
    await call;
  } on DioException catch (e) {
    final problem = ApiProblem.of(e);
    expect(problem, isNotNull, reason: 'no ApiProblem on $e');
    return problem!;
  }
  fail('the call did not fail');
}

/// An interceptor in the slot of `auth` that records the problem it sees on an error.
class _Seen extends Interceptor {
  final List<Object?> bodies = [];

  @override
  void onError(DioException err, ErrorInterceptorHandler handler) {
    bodies.add(err.response?.data);
    handler.next(err);
  }
}

void main() {
  for (final type in [ResponseType.bytes, ResponseType.stream]) {
    test(
      'a problem of a ${type.name} response is a JSON object before the auth '
      'interceptors (M4-22)',
      () async {
        final adapter = FakeAdapter()
          ..problem(401, problemBody(401, 'use_dpop_nonce'));
        final seen = _Seen();
        final client = fakeClient(adapter, authInterceptors: [seen]);
        final p = await problemOf(
          client.dio.get<Object?>(
            '/files/x/content',
            options: Options(responseType: type),
          ),
        );
        expect(p.code, 'use_dpop_nonce');
        expect(seen.bodies.single, isA<Map<String, Object?>>());
      },
    );
  }

  test('403 forbidden carries reason', () async {
    final adapter = FakeAdapter()
      ..problem(
        403,
        problemBody(403, 'forbidden', {
          'reason': 'level_missing',
          'hint': 'ask Robin',
        }),
        headers: {'Correlation-Id': _correlation},
      );

    final p = await problemOf(
      fakeClient(adapter).getMessagesApi().getMessage(messageId: _messageId),
    );

    expect(p.status, 403);
    expect(p.code, 'forbidden');
    expect(p.title, 'Title of forbidden');
    expect(p.reason, 'level_missing');
    expect(p.hint, 'ask Robin');
    expect(p.correlationId, _correlation);
  });

  test('400 validation_failed with two errors', () async {
    final adapter = FakeAdapter()
      ..problem(
        400,
        problemBody(400, 'validation_failed', {
          'detail': 'Two fields are wrong.',
          'errors': [
            {
              'pointer': '/content_md',
              'code': 'too_long',
              'detail': 'At most 4000 bytes.',
            },
            {'pointer': '/client_msg_id', 'code': 'required'},
          ],
        }),
      );

    final p = await problemOf(
      fakeClient(adapter).getMessagesApi().getMessage(messageId: _messageId),
    );

    expect(p.code, 'validation_failed');
    expect(p.detail, 'Two fields are wrong.');
    expect(p.errors.map((e) => (e.pointer, e.code, e.detail)), [
      ('/content_md', 'too_long', 'At most 4000 bytes.'),
      ('/client_msg_id', 'required', null),
    ]);
  });

  test('412 version_conflict carries current', () async {
    final adapter = FakeAdapter()
      ..problem(
        412,
        problemBody(412, 'version_conflict', {
          'current': {'id': _messageId, 'version': 4},
        }),
      );

    final p = await problemOf(
      fakeClient(adapter).getMessagesApi().updateMessage(
        messageId: _messageId,
        ifMatch: '"3"',
        messageUpdate: MessageUpdate((b) => b..contentMd = 'Hello Sam'),
        extra: {idempotencyKeyExtra: '0192f4a0-7b1c-7000-8000-000000000001'},
      ),
    );

    expect(p.status, 412);
    expect(p.code, 'version_conflict');
    expect(p.current, {'id': _messageId, 'version': 4});
  });

  test('429 rate_limited carries Retry-After', () async {
    final adapter = FakeAdapter()
      ..problem(
        429,
        problemBody(429, 'rate_limited'),
        headers: {'Retry-After': '7'},
      );

    final p = await problemOf(
      fakeClient(adapter).getMessagesApi().createMessage(
        threadId: _messageId,
        messageCreate: MessageCreate((b) => b..clientMsgId = _messageId),
      ),
    );

    expect(p.code, 'rate_limited');
    expect(p.retryAfter, const Duration(seconds: 7));
  });

  test('a 502 without a body gives http_502', () async {
    final adapter = FakeAdapter()
      ..empty(502, headers: {'Correlation-Id': _correlation});

    final p = await problemOf(
      fakeClient(adapter).getMessagesApi().createMessage(
        threadId: _messageId,
        messageCreate: MessageCreate((b) => b..clientMsgId = _messageId),
      ),
    );

    expect(p.status, 502);
    expect(p.code, 'http_502');
    expect(p.correlationId, _correlation);
    expect(p.errors, isEmpty);
    expect(p.current, isNull);
  });

  test('an unknown code stays a string', () async {
    final adapter = FakeAdapter()
      ..problem(409, problemBody(409, 'future_conflict'));

    final p = await problemOf(
      fakeClient(adapter).getMessagesApi().getMessage(messageId: _messageId),
    );

    expect(p.code, 'future_conflict');
  });

  test('the exception text contains no request body', () async {
    const canary = 'canary-7c1f-request-body';
    final adapter = FakeAdapter()
      ..problem(400, problemBody(400, 'validation_failed'));
    final client = fakeClient(adapter);

    try {
      await client.getMessagesApi().createMessage(
        threadId: _messageId,
        messageCreate: MessageCreate(
          (b) => b
            ..clientMsgId = _messageId
            ..contentMd = canary,
        ),
      );
      fail('the call did not fail');
    } on DioException catch (e) {
      expect(adapter.requests.single.body, contains(canary));
      expect(e.toString(), isNot(contains(canary)));
      expect(e.message ?? '', isNot(contains(canary)));
      expect(ApiProblem.of(e).toString(), isNot(contains(canary)));
      expect(ApiProblem.of(e).toString(), contains('validation_failed'));
    }
  });

  test('an error without a response is no problem', () {
    expect(ApiProblem.of(StateError('x')), isNull);
  });
}
