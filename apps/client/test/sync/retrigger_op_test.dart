// SPDX-License-Identifier: Apache-2.0

// The executor of `retriggerMessage` (09 3.8, F-PROJEKT-BEREIT): the server answers 200 also
// when the cause is still there, with the message failed again. That answer must fail the
// outbox entry with the message's reason, so "Trigger again" shows it and offers the button
// again; an answer with another trigger state confirms the entry.

import 'dart:convert';
import 'dart:typed_data';

import 'package:api_client/api_client.dart' show ApiClient, ApiProblem;
import 'package:client/sync/message_ops_app.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

String _id(int n) =>
    '0192f4a0-7b1c-78f9-8074-${n.toRadixString(16).padLeft(12, '0')}';

/// Answers every request with 200 and [body].
class _Adapter implements HttpClientAdapter {
  _Adapter(this.body);

  final Map<String, Object?> body;
  final List<String> seen = [];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    seen.add('${options.method} ${options.uri.path}');
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

Map<String, Object?> _message(String state, {String? reason}) => {
  'id': _id(0x701),
  'thread_id': _id(0x301),
  'channel_id': _id(0x201),
  'thread_seq': 1,
  'author': {
    'id': _id(0x901),
    'kind': 'user',
    'handle': 'sam',
    'display_name': 'Sam',
  },
  'client_msg_id': _id(0x801),
  'origin': 'human',
  'kind': 'text',
  'content_md': 'build it',
  'content_format': 1,
  'trigger_state': state,
  'trigger_state_reason': ?reason,
  'silent': false,
  'attachments': <Object?>[],
  'mention_ids': <Object?>[],
  'reactions': <Object?>[],
  'pinned': false,
  'components': <Object?>[],
  'version': 2,
  'created_at': '2026-11-04T10:14:00Z',
};

void main() {
  Future<OutboxResponse> send(Map<String, Object?> body, _Adapter adapter) {
    final executors = OutboxExecutors();
    AppMessageOps.register(
      executors,
      ApiClient(
        dio: Dio(BaseOptions(baseUrl: 'https://chat.example.com'))
          ..httpClientAdapter = adapter,
      ),
      AppOpsStore(
        storeThread: (_) async {},
        storeReadState: (_, {required reset}) async {},
        applyPin: (_, {required pinned}) async {},
        storeDraftResult: (_) async {},
      ),
    );
    final entry = OutboxEntry(
      id: _id(0xe01),
      kind: AppMessageOps.retriggerMessage,
      payload: jsonEncode({'message_id': _id(0x701)}),
      state: OutboxState.sending,
      attempts: 0,
      createdAt: DateTime.utc(2026, 11, 4),
      stateChangedAt: DateTime.utc(2026, 11, 4),
    );
    return executors
        .lookup(entry.kind)!
        .send(entry, ApiCallContext(idempotencyKey: entry.id));
  }

  test(
    '200 with the message failed again fails the entry with its reason',
    () async {
      final adapter = _Adapter(
        _message('failed', reason: 'operator_cannot_serve_project'),
      );
      Object? error;
      try {
        await send(adapter.body, adapter);
      } on Object catch (e) {
        error = e;
      }
      expect(adapter.seen, ['POST /messages/${_id(0x701)}/retrigger']);
      expect(error, isA<ApiProblem>());
      final c = classify(error!);
      expect(c.outcome, OutcomeClass.failed);
      expect(c.code, AppMessageOps.retriggerStillFailed);
      expect(c.reason, 'operator_cannot_serve_project');
    },
  );

  test('200 with the message received again confirms the entry', () async {
    final adapter = _Adapter(_message('received'));
    final response = await send(adapter.body, adapter);
    expect(response.status, 200);
  });
}
