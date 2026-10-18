// SPDX-License-Identifier: Apache-2.0

// Findings of the red team of M4-15: the claim of an entry checks lane head, dependencies and
// due time on fresh data under the lock; a pause during the first start holds; an edit of a
// failed create drops its older follow-up edits; a discarded entry fails its whole chain; and
// reaction emoji are percent-encoded.

import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:api_client/api_client.dart';
import 'package:dio/dio.dart';
import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

import 'support.dart';

final ThreadId threadC = ThreadId.parse('0192f4a0-7b1c-7000-8a00-00000000000c');

void main() {
  group('claim on fresh data', () {
    for (var k = 0; k <= 30; k++) {
      test(
        'a message never goes before the upload that replaced its failed one, k=$k',
        () async {
          final h = Harness.open();
          addTearDown(h.close);
          final uploads = h.fake('upload', lane: (e) => 'upload:${e.id}');
          uploads.then([
            problem(413, 'payload_too_large'),
            Completer<OutboxResponse>(),
          ]);
          final sentM = <String>[];
          var bCalls = 0;
          h.executors.register('createMessage', (e, ctx) async {
            if (e.threadId == threadB) {
              bCalls++;
              if (bCalls == 1) {
                throw problem(
                  503,
                  'unavailable',
                  retryAfter: const Duration(seconds: 10),
                );
              }
              return Completer<OutboxResponse>().future;
            }
            if (e.threadId == threadA) sentM.add(e.id);
            return Completer<OutboxResponse>().future;
          });
          final u = await h.outbox.enqueue(
            const OutboxRequest(kind: 'upload', payload: {'name': 'a.png'}),
          );
          await h.add('one file', thread: threadA, dependsOn: [u.id]);
          await h.add('b', thread: threadB);
          for (var i = 0; i < 8; i++) {
            await h.add(
              'other',
              thread: ThreadId.parse('0192f4a0-7b1c-7000-8a00-0000000001${i}0'),
            );
          }
          await h.outbox.start();
          await settle();
          final retried = h.outbox.retry(u.id);
          for (var i = 0; i < k; i++) {
            await Future<void>.microtask(() {});
          }
          unawaited(h.time.advance(const Duration(seconds: 11)));
          await retried;
          await settle();
          expect(sentM, isEmpty, reason: 'the new upload is not complete');
        },
      );
    }

    for (var k = 0; k <= 12; k += 2) {
      test(
        'Retry-After holds while the person sends elsewhere, k=$k',
        () async {
          final h = Harness.open();
          addTearDown(h.close);
          final create = h.fake('createMessage');
          final held = Completer<OutboxResponse>();
          create.then([
            problem(503, 'unavailable', retryAfter: const Duration(minutes: 2)),
            held,
          ]);
          await h.add('a1', thread: threadA);
          final b = await h.add('b1', thread: threadB);
          await h.outbox.start();
          await settle();
          unawaited(h.add('c1', thread: threadC));
          for (var i = 0; i < k; i++) {
            await Future<void>.microtask(() {});
          }
          held.completeError(
            problem(
              429,
              'rate_limited',
              retryAfter: const Duration(seconds: 60),
            ),
          );
          await settle();
          expect(create.calls.where((c) => c.id == b.id), hasLength(1));
        },
      );
    }
  });

  group('pause during the first start', () {
    test('a pause right after start holds', () async {
      final h = Harness.open();
      addTearDown(h.close);
      final exec = h.fake('createMessage');
      await h.add('hello', thread: threadA);
      final starting = h.outbox.start();
      await h.outbox.pause();
      await starting;
      await settle();
      expect(exec.calls, isEmpty);
      await h.add('later', thread: threadB);
      await settle();
      expect(exec.calls, isEmpty);
      await h.outbox.start();
      await settle();
      expect(exec.calls, hasLength(2));
    });
  });

  group('follow-up chains', () {
    late Harness h;
    late ScriptedExecutor creates;
    late ScriptedExecutor updates;
    late ScriptedExecutor deletes;
    const serverId = '0192f4a0-7b1c-7000-8a00-0000000000c1';

    setUp(() {
      h = Harness.open();
      creates = ScriptedExecutor();
      updates = ScriptedExecutor();
      deletes = ScriptedExecutor();
      for (final (kind, exec) in [
        (MessageOps.createMessage, creates),
        (MessageOps.updateMessage, updates),
        (MessageOps.deleteMessage, deletes),
      ]) {
        h.executors.register(
          kind,
          exec.call,
          followUpEdit: MessageOps.followUpEdit,
          followUpRemove: MessageOps.followUpRemove,
        );
      }
    });
    tearDown(() => h.close());

    test('editing a failed create drops its older follow-up edit; the latest text wins', () async {
      creates.then([
        problem(503, 'unavailable'),
        problem(422, 'validation_failed'),
        const OutboxResponse(result: {'message_id': serverId, 'version': 1}),
      ]);
      final c = await h.add('v1', thread: threadA);
      await h.outbox.start();
      await settle();
      final u1 = await h.outbox.edit(c.id, {'content_md': 'v2'});
      await h.time.advance(const Duration(minutes: 10));
      expect((await h.entry(c.id))!.state, OutboxState.failed);
      final c2 = await h.outbox.edit(c.id, {'content_md': 'v3'});
      await settle();
      expect(await h.entry(u1), isNull);
      expect(creates.calls.last.id, c2);
      expect(creates.calls.last.payload['content_md'], 'v3');
      expect(updates.calls, isEmpty);
      expect(await h.db.outboxDao.entriesFor(threadA), isEmpty);
    });

    test(
      'removing a failed create fails its whole chain, so the thread goes on',
      () async {
        creates.then([
          problem(503, 'unavailable'),
          problem(422, 'validation_failed'),
        ]);
        final c = await h.add('v1', thread: threadA);
        await h.outbox.start();
        await settle();
        final u1 = await h.outbox.edit(c.id, {'content_md': 'v2'});
        final d = await h.outbox.remove(c.id);
        await h.time.advance(const Duration(minutes: 10));
        expect((await h.entry(c.id))!.state, OutboxState.failed);
        await h.outbox.remove(c.id);
        expect((await h.entry(u1))!.state, OutboxState.failed);
        expect((await h.entry(d!))!.state, OutboxState.failed);
        final next = await h.add('next', thread: threadA);
        await settle();
        expect(
          creates.calls.last.id,
          next.id,
          reason: 'the thread is not blocked',
        );
      },
    );
  });

  test('reaction emoji are percent-encoded in the path', () async {
    final h = Harness.open();
    addTearDown(h.close);
    final adapter = _Adapter();
    MessageOps.register(
      h.executors,
      createApiClient(
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
      ),
    );
    for (final emoji in ['#⃣', '?', '+1']) {
      await h.outbox.enqueue(
        OutboxRequest(
          kind: MessageOps.addReaction,
          threadId: threadA,
          payload: {'message_id': 'm1', 'emoji': emoji},
        ),
      );
    }
    await h.outbox.start();
    await settle();
    expect(adapter.uris, hasLength(3));
    for (final (i, emoji) in ['#⃣', '?', '+1'].indexed) {
      expect(adapter.uris[i].pathSegments.last, emoji);
      expect(adapter.uris[i].query, isEmpty);
      expect(adapter.uris[i].fragment, isEmpty);
    }
  });
}

final class _Adapter implements HttpClientAdapter {
  final List<Uri> uris = [];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    uris.add(options.uri);
    return ResponseBody.fromString(
      '',
      204,
      headers: {
        Headers.contentTypeHeader: [jsonEncode('text/plain')],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}
