// SPDX-License-Identifier: Apache-2.0

// The executors of the message and personal operations call the generated client with the
// entry's ID (04 5.1, 5.2) over a fake network.

import 'dart:convert';
import 'dart:typed_data';

import 'package:api_client/api_client.dart';
import 'package:dio/dio.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

import 'support.dart';

const _messageId = '0192f4a0-7b1c-7353-8d8c-dea659467151';

Map<String, Object?> _message(String clientMsgId, {int version = 1}) => {
  'id': _messageId,
  'thread_id': threadA.value,
  'channel_id': '0192f4a0-7b1c-72d4-8bae-7661d2b2f203',
  'thread_seq': 42,
  'author': {
    'id': '0192f4a0-7b1c-7f89-8d6b-6960453241bc',
    'kind': 'user',
    'handle': 'alex',
    'display_name': 'Alex',
  },
  'client_msg_id': clientMsgId,
  'origin': 'human',
  'kind': 'text',
  'content_md': 'Hello Sam',
  'content_format': 1,
  'silent': false,
  'attachments': <Object>[],
  'mention_ids': <Object>[],
  'reactions': <Object>[],
  'pinned': false,
  'components': <Object>[],
  'version': version,
  'created_at': '2026-10-10T09:30:00.000Z',
};

/// One request the client sent.
final class _Seen {
  _Seen(this.method, this.path, this.headers, this.body);
  final String method;
  final String path;
  final Map<String, dynamic> headers;
  final Object? body;
}

/// Answers in order and records every request; no socket is opened.
final class _Adapter implements HttpClientAdapter {
  final List<_Seen> seen = [];
  final List<(int, Object?)> answers = [];

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
    seen.add(
      _Seen(
        options.method,
        options.path,
        Map.of(options.headers),
        bytes.isEmpty ? null : jsonDecode(utf8.decode(bytes)),
      ),
    );
    final (status, body) = answers.removeAt(0);
    return ResponseBody.fromString(
      body == null ? '' : jsonEncode(body),
      status,
      headers: {
        if (body != null) Headers.contentTypeHeader: ['application/json'],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  late Harness h;
  late _Adapter adapter;

  setUp(() {
    h = Harness.open();
    adapter = _Adapter();
    final api = createApiClient(
      baseUrl: Uri.parse('https://chat.example.com'),
      traceContext: () => TraceContext(
        traceId: '4bf92f3577b34da6a3ce929d0e0e4736',
        sampled: true,
      ),
      readRetry: ReadRetryPolicy(
        base: Duration.zero,
        cap: Duration.zero,
        maxAttempts: 1,
      ),
      adapter: adapter,
    );
    MessageOps.register(h.executors, api);
    PersonalOps.register(h.executors, api);
  });
  tearDown(() => h.close());

  Future<OutboxEntry> send(
    String kind,
    Map<String, Object?> payload, {
    bool thread = true,
    List<String> dependsOn = const [],
  }) async {
    final e = await h.outbox.enqueue(
      OutboxRequest(
        kind: kind,
        threadId: thread ? threadA : null,
        payload: payload,
        dependsOn: dependsOn,
      ),
    );
    await h.outbox.start();
    await settle();
    return e;
  }

  test(
    'createMessage sends the entry ID as client_msg_id and binds the server ID',
    () async {
      final create = await h.outbox.enqueue(
        OutboxRequest(
          kind: MessageOps.createMessage,
          threadId: threadA,
          payload: const {'content_md': 'Hello Sam'},
        ),
      );
      final follow = await h.outbox.enqueue(
        MessageOps.followUpEdit(create, const {
          'content_md': 'Hello Sam!',
        }).withDependsOn([create.id]),
      );
      adapter.answers
        ..add((201, _message(create.id)))
        ..add((200, _message(create.id, version: 2)));
      await h.outbox.start();
      await settle();
      expect(adapter.seen[0].method, 'POST');
      expect(adapter.seen[0].path, '/threads/${threadA.value}/messages');
      expect((adapter.seen[0].body! as Map)['client_msg_id'], create.id);
      expect((adapter.seen[0].body! as Map)['content_md'], 'Hello Sam');
      expect(adapter.seen[1].method, 'PATCH');
      expect(adapter.seen[1].path, '/messages/$_messageId');
      expect(adapter.seen[1].headers['If-Match'], '"1"');
      expect(adapter.seen[1].headers['Idempotency-Key'], follow.id);
      expect(adapter.seen[1].body, {'content_md': 'Hello Sam!'});
      expect(await h.db.outboxDao.entriesFor(threadA), isEmpty);
    },
  );

  test('a 412 from the server keeps the current revision', () async {
    adapter.answers.add((
      412,
      {'code': 'version_conflict', 'current': _message('x', version: 5)},
    ));
    final e = await send(MessageOps.updateMessage, {
      'message_id': _messageId,
      'version': 4,
      'content_md': 'mine',
    });
    final failed = (await h.entry(e.id))!;
    expect(failed.state, OutboxState.failed);
    expect(jsonDecode(failed.conflictCurrent!)['version'], 5);
  });

  test(
    'a follow-up without a bound message ID fails as a programming error',
    () async {
      final e = await send(MessageOps.deleteMessage, {'message_id': null});
      expect((await h.entry(e.id))!.lastErrorCode, 'executor_error');
      expect(adapter.seen, isEmpty);
    },
  );

  test('deleteMessage, reactions and the personal writes carry the Idempotency-Key', () async {
    final cases =
        <(String, Map<String, Object?>, bool, String, String, Object?)>[
          (
            MessageOps.deleteMessage,
            {'message_id': _messageId},
            true,
            'DELETE',
            '/messages/$_messageId',
            null,
          ),
          (
            MessageOps.addReaction,
            {'message_id': _messageId, 'emoji': 'tada'},
            true,
            'PUT',
            '/messages/$_messageId/reactions/tada',
            null,
          ),
          (
            MessageOps.removeReaction,
            {'message_id': _messageId, 'emoji': 'tada'},
            false,
            'DELETE',
            '/messages/$_messageId/reactions/tada',
            null,
          ),
          (
            PersonalOps.putReadState,
            {'last_read_seq': 42},
            true,
            'PUT',
            '/threads/${threadA.value}/read-state',
            {
              'thread_id': threadA.value,
              'channel_id': _messageId,
              'last_read_seq': 42,
              'mention_count': 0,
              'version': 3,
            },
          ),
          (
            PersonalOps.putThreadDraft,
            {'content_md': 'Draft for Robin'},
            true,
            'PUT',
            '/threads/${threadA.value}/draft',
            {
              'draft': {
                'id': _messageId,
                'channel_id': _messageId,
                'content_md': 'Draft for Robin',
                'file_ids': <Object>[],
                'version': 1,
              },
              'overwrote_newer': false,
            },
          ),
          (
            PersonalOps.deleteThreadDraft,
            {},
            true,
            'DELETE',
            '/threads/${threadA.value}/draft',
            null,
          ),
          (
            PersonalOps.putMySetting,
            {'key': 'ui.theme', 'value': 'dark'},
            false,
            'PUT',
            '/me/settings/ui.theme',
            {'key': 'ui.theme', 'value': 'dark', 'version': 2},
          ),
          (
            PersonalOps.deleteMySetting,
            {'key': 'ui.theme'},
            false,
            'DELETE',
            '/me/settings/ui.theme',
            null,
          ),
        ];
    for (final (kind, payload, thread, method, path, answer) in cases) {
      adapter.answers.add((answer == null ? 204 : 200, answer));
      final e = await send(kind, payload, thread: thread);
      final seen = adapter.seen.last;
      expect(seen.method, method, reason: kind);
      expect(seen.path, path, reason: kind);
      expect(seen.headers['Idempotency-Key'], e.id, reason: kind);
      expect(await h.entry(e.id), isNull, reason: '$kind confirmed');
    }
    expect(adapter.seen, hasLength(cases.length));
  });

  test('every executor names its lane', () {
    OutboxEntry entry(String kind, String payload, {bool thread = true}) =>
        OutboxEntry(
          id: '0192f4a0-7b1c-7000-8a00-0000000000e1',
          kind: kind,
          payload: payload,
          state: OutboxState.pending,
          attempts: 0,
          createdAt: at(0),
          stateChangedAt: at(0),
          threadId: thread ? threadA : null,
        );
    final lanes = h.executors;
    expect(
      lanes.laneOf(entry(MessageOps.createMessage, '{}')),
      'thread:${threadA.value}',
    );
    expect(
      lanes.laneOf(entry(PersonalOps.putReadState, '{}')),
      'thread:${threadA.value}',
    );
    expect(
      lanes.laneOf(
        entry(PersonalOps.putMySetting, '{"key":"ui.theme"}', thread: false),
      ),
      'setting:ui.theme',
    );
    expect(
      lanes.laneOf(
        entry(PersonalOps.deleteMySetting, '{"key":"ui.theme"}', thread: false),
      ),
      'setting:ui.theme',
    );
    expect(
      lanes.laneOf(
        entry(MessageOps.addReaction, '{"message_id":"m1"}', thread: false),
      ),
      'message:m1',
    );
    expect(lanes.laneOf(entry('unknown', '{}', thread: false)), 'global');
  });
}
