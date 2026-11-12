// SPDX-License-Identifier: Apache-2.0

// The interview at the server's turn limit (26 6.1 item 1; ruling AUD-AA1/1): the client keeps
// every question and answer as a turn of its own, and the server refuses more turns than
// `agents.interview.max_turns` with `400 validation_failed` and `too_many_items` on `/turns`
// before it asks the builder. The interview then ends with the draft from the turns that fit,
// instead of an error the person cannot leave. The requests run through `ApiNewAgentPort`, the
// generated client and its decoder; the adapter below is the server's side of the contract: the
// turn count of the default configuration and the problem document of that refusal.

import 'dart:convert';
import 'dart:typed_data';

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart' hide AgentPage;
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

/// The default of `agents.interview.max_turns` (26 14).
const int _maxTurns = 12;

/// `interviewAgent` as the server answers it: more than [maxTurns] turns are refused before the
/// builder is asked; otherwise the builder asks [questionsPerAnswer] questions, or drafts once
/// the request carries `want_draft`.
class _InterviewServer implements HttpClientAdapter {
  _InterviewServer({this.maxTurns = _maxTurns, this.questionsPerAnswer = 2});

  final int maxTurns;
  final int questionsPerAnswer;

  /// Every request body in order.
  final List<Map<String, Object?>> requests = [];

  /// How many requests reached the builder.
  int asked = 0;

  /// The problem document of the next request, answered once with `400`.
  Map<String, Object?>? failNextWith;

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
    if (options.method != 'POST' ||
        options.uri.path != '/api/v1/agents/interview') {
      throw StateError('no route for ${options.method} ${options.uri.path}');
    }
    final body = (jsonDecode(utf8.decode(bytes)) as Map)
        .cast<String, Object?>();
    requests.add(body);
    if (failNextWith case final problem?) {
      failNextWith = null;
      return _json(400, problem);
    }
    final turns = body['turns']! as List;
    if (turns.length > maxTurns) {
      return _json(400, {
        'type': '/api/v1/problems/validation_failed',
        'title': 'Validation failed',
        'status': 400,
        'code': 'validation_failed',
        'correlation_id': '0192f4a0-7b1c-7d07-84b0-358556a08a90',
        'errors': [
          {
            'pointer': '/turns',
            'code': 'too_many_items',
            'detail': 'more turns than agents.interview.max_turns',
          },
        ],
      });
    }
    asked++;
    if (body['want_draft'] == true) {
      return _json(200, {
        'outcome': 'draft',
        'draft': {
          'instructions': 'Keep the notes of Sam.',
          'skills': <Object?>[],
          'mcp_servers': <Object?>[],
          'settings': {'tool_class_wish': 'chat'},
          'origin': {'kind': 'interview'},
          'name': 'Notes',
          'handle': 'notes',
          'description': 'Keeps notes',
        },
        'errors': <Object?>[],
        'cost_estimate': {'amount_micros': 4000, 'currency': 'USD'},
      });
    }
    return _json(200, {
      'outcome': 'questions',
      'questions': [
        for (var q = 1; q <= questionsPerAnswer; q++) 'Question $asked.$q?',
      ],
    });
  }

  static ResponseBody _json(int status, Object body) => ResponseBody.fromString(
    jsonEncode(body),
    status,
    headers: {
      Headers.contentTypeHeader: [
        status >= 400 ? 'application/problem+json' : 'application/json',
      ],
    },
  );

  @override
  void close({bool force = false}) {}
}

AgentInterview _interview(_InterviewServer server) => AgentInterview(
  ApiNewAgentPort(
    createApiClient(
      baseUrl: Uri.parse('https://chat.example.com'),
      traceContext: () => TraceContext(
        traceId: '4bf92f3577b34da6a3ce929d0e0e4736',
        sampled: true,
      ),
      readRetry: ReadRetryPolicy.fromBackoff(
        base: const Duration(milliseconds: 1),
        cap: const Duration(milliseconds: 1),
        sleep: (_) async {},
      ),
      adapter: server,
    ),
    purpose: (name) => 'Create the agent $name',
    reason: (name) => 'New agent $name',
  ),
);

List<String> _texts(Map<String, Object?> request, String role) => [
  for (final t in (request['turns']! as List).cast<Map<String, Object?>>())
    if (t['role'] == role) t['text']! as String,
];

void main() {
  for (final questions in [1, 2]) {
    test('a builder that keeps asking $questions question(s) ends in a draft, '
        'never in an error', () async {
      final server = _InterviewServer(questionsPerAnswer: questions);
      final interview = _interview(server);
      addTearDown(interview.dispose);
      await interview.start();
      final answers = <String>[];
      // Far more answers than the limit holds: the builder never drafts by itself.
      for (var n = 1; n <= _maxTurns && interview.draft == null; n++) {
        answers.add('Answer $n');
        await interview.answer(answers.last);
        expect(interview.error, isNull, reason: 'after answer $n');
        expect(interview.pending, isFalse);
      }
      expect(interview.draft, isNotNull);
      expect(interview.draft!.draft!['handle'], 'notes');

      final last = server.requests.last;
      expect(last['want_draft'], isTrue);
      expect((last['turns']! as List).length, lessThanOrEqualTo(_maxTurns));
      // Every answer of the person reaches the builder; only its oldest questions are left out.
      expect(_texts(last, 'owner'), answers);
      expect(_texts(last, 'builder'), isNot(contains('Question 1.1?')));
      // The person still sees the whole interview.
      expect([
        for (final t in interview.turns)
          if (t.role == InterviewRole.owner) t.text,
      ], answers);
      expect(interview.turns.first.text, 'Question 1.1?');
    });
  }

  test(
    '"Use what we have" works after the builder asked past the limit',
    () async {
      // Six answers to one question each fill the limit; the builder's next question is turn 13.
      final server = _InterviewServer(questionsPerAnswer: 1);
      final interview = _interview(server);
      addTearDown(interview.dispose);
      await interview.start();
      for (var n = 1; n <= 6; n++) {
        await interview.answer('Answer $n');
      }
      expect(interview.turns, hasLength(_maxTurns + 1));
      expect(interview.draft, isNull);

      await interview.useWhatWeHave();
      expect(interview.error, isNull);
      expect(interview.draft, isNotNull);
      final last = server.requests.last;
      expect(last['want_draft'], isTrue);
      // The question nobody answered is the turn that is left out.
      expect(last['turns'], hasLength(_maxTurns));
      expect(_texts(last, 'builder'), isNot(contains('Question 7.1?')));
      expect(_texts(last, 'owner'), [for (var n = 1; n <= 6; n++) 'Answer $n']);
    },
  );

  test('a lower limit of the server is kept too', () async {
    final server = _InterviewServer(maxTurns: 5);
    final interview = _interview(server);
    addTearDown(interview.dispose);
    await interview.start();
    await interview.answer('Answer 1');
    await interview.answer('Answer 2');
    expect(interview.error, isNull);
    expect(interview.draft, isNotNull);
    final last = server.requests.last;
    expect((last['turns']! as List).length, lessThanOrEqualTo(5));
    expect(_texts(last, 'owner'), ['Answer 1', 'Answer 2']);
  });

  test(
    'another refusal of the turns still shows and gives the answer back',
    () async {
      final server = _InterviewServer();
      final interview = _interview(server);
      addTearDown(interview.dispose);
      await interview.start();
      server.failNextWith = {
        'type': '/api/v1/problems/validation_failed',
        'title': 'Validation failed',
        'status': 400,
        'code': 'validation_failed',
        'errors': [
          {'pointer': '/turns/2/text', 'code': 'too_long'},
        ],
      };
      await interview.answer('An answer the server finds too long');
      expect(interview.error?.errors, [
        const FieldIssue('/turns/2/text', 'too_long'),
      ]);
      expect(interview.draft, isNull);
      expect(
        [for (final t in interview.turns) t.role],
        [InterviewRole.builder, InterviewRole.builder],
      );
      // Nothing was sent a second time.
      expect(server.requests, hasLength(2));
    },
  );

  test('the turns that fit: unanswered questions go first, then the oldest '
      'questions, the answers last', () {
    const q = InterviewRole.builder;
    const a = InterviewRole.owner;
    List<String> fit(List<(InterviewRole, String)> turns, int count) => [
      for (final t in fittingTurns([
        for (final (role, text) in turns) InterviewTurn(role, text),
      ], count))
        t.text,
    ];
    const interview = [(q, 'q1'), (q, 'q2'), (a, 'a1'), (q, 'q3'), (a, 'a2')];

    expect(fit(interview, 5), ['q1', 'q2', 'a1', 'q3', 'a2']);
    expect(fit(interview, 3), ['a1', 'q3', 'a2']);
    // No question is left to drop: the oldest answer goes.
    expect(fit(interview, 1), ['a2']);
    // A question nobody answered carries nothing for a draft.
    expect(fit([...interview, (q, 'q4'), (q, 'q5')], 6), [
      'q1',
      'q2',
      'a1',
      'q3',
      'a2',
      'q4',
    ]);
    expect(fit([...interview, (q, 'q4'), (q, 'q5')], 4), [
      'q2',
      'a1',
      'q3',
      'a2',
    ]);
    expect(fit(const [], 3), isEmpty);
  });
}
