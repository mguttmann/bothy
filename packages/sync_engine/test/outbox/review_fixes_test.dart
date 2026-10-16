// SPDX-License-Identifier: Apache-2.0

// Findings of the code review of M4-15: a dependent never blocks its parent in the same lane,
// a replaced entry hands its dependents to the replacement, an edit merges into the stored
// payload, and a broken TLS handshake is retried.

import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:dio/dio.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
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

  Map<String, Object?> payloadOf(OutboxEntry e) =>
      (jsonDecode(e.payload) as Map).cast<String, Object?>();

  test('a follow-up written after the clock stepped back does not block its parent', () async {
    creates.then([
      problem(503, 'unavailable', retryAfter: const Duration(minutes: 1)),
    ]);
    final c = await h.add('hello', thread: threadA);
    await h.outbox.start();
    await settle();
    h.time.now = h.time.now.subtract(const Duration(seconds: 10));
    final u = await h.outbox.edit(c.id, {'content_md': 'hello!'});
    expect((await h.entry(u))!.createdAt.isBefore(c.createdAt), isTrue);
    creates.then([
      const OutboxResponse(result: {'message_id': serverId, 'version': 1}),
    ]);
    await h.time.advance(const Duration(minutes: 2));
    expect(creates.ids, [c.id, c.id]);
    expect(updates.ids, [u]);
  });

  test(
    'a failed upload sent again keeps its message waiting for the new upload',
    () async {
      final uploads = h.fake('upload', lane: (e) => 'upload:${e.id}');
      final second = Completer<OutboxResponse>();
      uploads.then([problem(503, 'unavailable'), second]);
      // The first attempt fails for good, the second runs and is held.
      final u = await h.outbox.enqueue(
        const OutboxRequest(kind: 'upload', payload: {'name': 'a.png'}),
      );
      await h.db.outboxDao.recordFailure(u.id, code: 'payload_too_large');
      final m = await h.add('one file', thread: threadA, dependsOn: [u.id]);
      await h.outbox.start();
      await settle();
      final u2 = await h.outbox.retry(u.id);
      await settle();
      final waiting = (await h.entry(m.id))!;
      expect(waiting.state, OutboxState.pending, reason: 'not failed');
      expect(waiting.dependsOn, u2);
      expect(creates.ids, isEmpty);
      await h.time.advance(const Duration(minutes: 10));
      expect(creates.ids, isEmpty, reason: 'the new upload is not complete');
      second.complete(const OutboxResponse());
      await settle();
      expect(creates.ids, [m.id]);
    },
  );

  test(
    'a delete behind an edit that failed with 412 is still delivered',
    () async {
      final held = Completer<OutboxResponse>();
      creates.then([held]);
      updates.then([
        problem(
          412,
          'version_conflict',
          current: {'id': serverId, 'version': 3},
        ),
        const OutboxResponse(result: {'message_id': serverId, 'version': 4}),
      ]);
      final c = await h.add('hello', thread: threadA);
      await h.outbox.start();
      await settle();
      final u = await h.outbox.edit(c.id, {'content_md': 'edited'});
      final d = await h.outbox.remove(c.id);
      held.complete(
        const OutboxResponse(result: {'message_id': serverId, 'version': 1}),
      );
      await settle();
      expect((await h.entry(u))!.state, OutboxState.failed);
      expect((await h.entry(d!))!.state, OutboxState.pending);

      final u2 = await h.outbox.edit(u, {'content_md': 'edited again'});
      await settle();
      expect(updates.calls.last.id, u2);
      expect(updates.calls.last.payload, {
        'message_id': serverId,
        'version': 3,
        'content_md': 'edited again',
      });
      expect(deletes.calls.single.payload, {'message_id': serverId});
    },
  );

  test('editing a follow-up again keeps the slots the outbox fills', () async {
    creates.then([Completer<OutboxResponse>()]);
    final c = await h.add('hello', thread: threadA);
    await h.outbox.start();
    await settle();
    final u = await h.outbox.edit(c.id, {'content_md': 'one'});
    expect(await h.outbox.edit(u, {'content_md': 'two'}), u);
    expect(payloadOf((await h.entry(u))!), {
      'message_id': null,
      'version': null,
      'content_md': 'two',
    });
  });

  test('a broken TLS handshake is retried', () {
    final c = classify(
      DioException(
        requestOptions: RequestOptions(path: '/x'),
        error: const HandshakeException(
          'Connection terminated during handshake',
        ),
      ),
    );
    expect(c.outcome, OutcomeClass.retry);
    expect(classify(const TlsException('reset')).outcome, OutcomeClass.retry);
  });
}
