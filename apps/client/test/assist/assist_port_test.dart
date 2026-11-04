// SPDX-License-Identifier: Apache-2.0

// The port of the search assistant (docs/spec/14-search.md 4.10): the request carries the
// question and the earlier turns, the answer maps picks, full-text hits, the fallback and the
// cost, the deep link names the message, and the problems map to their kinds.

import 'dart:convert';
import 'dart:typed_data';

import 'package:api_client/api_client.dart' as rest;
import 'package:client/assist/assist_conversation.dart';
import 'package:client/assist/assist_port.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/structure.dart';

/// Records the request and answers with [body].
class CaptureAdapter implements HttpClientAdapter {
  CaptureAdapter(this.body);

  final Map<String, Object?> body;
  RequestOptions? request;
  Object? sent;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    request = options;
    if (requestStream != null) {
      final bytes = await requestStream.expand((c) => c).toList();
      sent = jsonDecode(utf8.decode(bytes));
    }
    return ResponseBody.fromString(
      jsonEncode(body),
      200,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

final Map<String, Object?> answerJson = {
  'mode': 'assisted',
  'terms': ['takeover', 'larkspur', 'migration'],
  'hits': [
    {
      'thread_id': threadId(1).value,
      'message_id': uuid(0x7a1),
      'channel_id': channelId(1).value,
      'thread_title': 'Takeover of Larkspur',
      'channel_name': 'larkspur',
      'space_name': 'Projects',
      'at': '2026-11-02T10:00:00.000Z',
      'source': 'assistant',
      'reason': 'The move.',
      'segments': [
        {'text': 'moved', 'match': true},
      ],
    },
    {
      'thread_id': threadId(2).value,
      'channel_id': channelId(2).value,
      'channel_name': 'general',
      'at': '2026-11-01T10:00:00.000Z',
      'source': 'fulltext',
      'reason': 'ignored for a full-text hit',
      'segments': <Object>[],
    },
  ],
  'cost_estimate': {'amount_micros': 1200, 'currency': 'USD'},
};

void main() {
  test('the request carries the question and the earlier turns', () async {
    final adapter = CaptureAdapter(answerJson);
    final port = ApiAssistPort(
      rest.ApiClient(
        dio: Dio(BaseOptions(baseUrl: 'https://chat.example.com'))
          ..httpClientAdapter = adapter,
      ),
    );
    final answer = await port.ask('no, the older one', [
      AssistTurn(
        q: 'takeover',
        terms: const ['takeover'],
        threadIds: [threadId(1)],
      ),
      const AssistTurn(q: 'larkspur'),
    ]);
    expect(adapter.request!.method, 'POST');
    expect(adapter.request!.uri.path, '/search/assist');
    expect(adapter.sent, {
      'q': 'no, the older one',
      'history': [
        {
          'q': 'takeover',
          'terms': ['takeover'],
          'thread_ids': [threadId(1).value],
        },
        {'q': 'larkspur'},
      ],
    });
    expect(answer.hits, hasLength(2));
  });

  test('the answer maps picks, full-text hits and the cost', () {
    final result = rest.standardSerializers.deserializeWith(
      rest.SearchAssistResult.serializer,
      answerJson,
    )!;
    final a = assistAnswerFromRest(result);
    expect(a.fallback, isNull);
    expect(a.terms, ['takeover', 'larkspur', 'migration']);
    expect(a.costMicros, 1200);
    expect(a.currency, 'USD');
    final [pick, full] = a.hits;
    expect(pick.picked, isTrue);
    expect(pick.reason, 'The move.');
    expect(pick.threadTitle, 'Takeover of Larkspur');
    expect(pick.spaceName, 'Projects');
    expect(assistTargetOf(pick), '/t/${threadId(1).value}?m=${uuid(0x7a1)}');
    expect(full.picked, isFalse);
    expect(full.reason, isNull);
    expect(full.threadTitle, isNull);
    expect(assistTargetOf(full), '/t/${threadId(2).value}');
  });

  test('a full-text answer names its reason', () {
    for (final (wire, want) in [
      ('assistant_not_configured', AssistFallback.notConfigured),
      ('assistant_unavailable', AssistFallback.unavailable),
      ('assistant_timeout', AssistFallback.timeout),
    ]) {
      final result = rest.standardSerializers.deserializeWith(
        rest.SearchAssistResult.serializer,
        {
          'mode': 'fulltext',
          'fallback_reason': wire,
          'terms': <String>[],
          'hits': <Object>[],
        },
      )!;
      expect(assistAnswerFromRest(result).fallback, want);
    }
  });

  test('the problems of a question map to their kinds', () {
    DioException problem(int status, String code, [String? field]) =>
        DioException(
          requestOptions: RequestOptions(path: '/search/assist'),
          type: DioExceptionType.badResponse,
          error: rest.ApiProblem(
            status: status,
            code: code,
            errors: [
              if (field != null)
                rest.ProblemFieldError(pointer: '/q', code: field),
            ],
          ),
        );
    expect(
      assistFailureOf(problem(400, 'validation_failed', 'empty')).kind,
      AssistFailureKind.queryEmpty,
    );
    expect(
      assistFailureOf(problem(400, 'validation_failed', 'too_long')).kind,
      AssistFailureKind.queryTooLong,
    );
    expect(
      assistFailureOf(problem(503, 'unavailable')).kind,
      AssistFailureKind.failed,
    );
    expect(
      assistFailureOf(
        DioException(
          requestOptions: RequestOptions(path: '/search/assist'),
          type: DioExceptionType.connectionError,
        ),
      ).kind,
      AssistFailureKind.offline,
    );
  });

  test(
    'the conversation sends at most the last turns and ignores failed ones',
    () async {
      final port = _Recording();
      final conversation = AssistConversation(port);
      for (var i = 0; i < assistMaxTurns + 2; i++) {
        await conversation.ask('q$i');
      }
      port.fail = true;
      await conversation.ask('failing');
      port.fail = false;
      await conversation.ask('last');
      final history = port.calls.last;
      expect(history, hasLength(assistMaxTurns));
      expect(history.last.q, 'q${assistMaxTurns + 1}');
      expect(history.any((t) => t.q == 'failing'), isFalse);
      await conversation.ask('   ');
      expect(port.calls, hasLength(assistMaxTurns + 4));
      conversation.dispose();
    },
  );

  test('signed out, a question fails', () async {
    final conversation = AssistConversation(null);
    await conversation.ask('takeover');
    expect(conversation.exchanges.single.failure, AssistFailureKind.failed);
    conversation.dispose();
  });
}

class _Recording implements AssistPort {
  final List<List<AssistTurn>> calls = [];
  bool fail = false;

  @override
  Future<AssistAnswer> ask(String q, List<AssistTurn> history) async {
    calls.add(history);
    if (fail) throw const AssistFailure(AssistFailureKind.failed);
    return AssistAnswer(hits: const [], terms: [q]);
  }
}
