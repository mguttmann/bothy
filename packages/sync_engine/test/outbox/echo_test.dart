// SPDX-License-Identifier: Apache-2.0

// The echo `message.created` with a matching `client_msg_id` confirms an entry, also before the
// response (04 13.2 item 6, 04 16.2).

import 'dart:async';
import 'dart:convert';

import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late Harness h;
  setUp(() => h = Harness.open());
  tearDown(() => h.close());

  test('an echo before the response confirms the entry; the late response adds nothing', () async {
    final exec = h.fake('createMessage');
    final held = Completer<OutboxResponse>();
    exec.then([held]);
    final e = await h.add('hello', thread: threadA);
    final next = await h.add('next', thread: threadA);
    final views = <List<OutboxEntryView>>[];
    final sub = h.outbox.watchThread(threadA).listen(views.add);
    await h.outbox.start();
    await settle();
    expect(exec.ids, [e.id]);
    expect(views.last.map((v) => v.id), [e.id, next.id]);
    expect(views.last.first.stateLabel, OutboxStateLabel.sending);

    expect(await h.outbox.confirmByEcho(e.id), isTrue);
    await settle();
    expect(await h.entry(e.id), isNull);
    expect(views.last.map((v) => v.id), [
      next.id,
    ], reason: 'the server message replaces it');
    expect(exec.ids, [e.id], reason: 'FIFO: next waits for the response');

    held.complete(const OutboxResponse());
    await settle();
    expect(exec.ids, [e.id, next.id]);
    expect(views.last, isEmpty);
    expect(h.confirmed, [
      next.id,
    ], reason: 'no second display entry for the late response');
    await sub.cancel();
  });

  test(
    'an echo for an entry waiting for a retry confirms it without another call',
    () async {
      final exec = h.fake('createMessage')..then([problem(503, 'unavailable')]);
      final e = await h.add('hello', thread: threadA);
      await h.outbox.start();
      await settle();
      expect(await h.outbox.confirmByEcho(e.id), isTrue);
      await h.time.advance(const Duration(minutes: 10));
      expect(exec.ids, [e.id]);
    },
  );

  test('an echo with an unknown ID changes nothing', () async {
    h.fake('createMessage').then([Completer<OutboxResponse>()]);
    final e = await h.add('hello', thread: threadA);
    final before = await h.entry(e.id);
    expect(
      await h.outbox.confirmByEcho('0192f4a0-7b1c-7000-8a00-0000000000ff'),
      isFalse,
    );
    expect(await h.entry(e.id), before);
  });

  test(
    'the echo binds the server ID into a follow-up that waits for it',
    () async {
      final exec = h.fake('createMessage');
      final updates = h.fake('updateMessage');
      exec.then([Completer<OutboxResponse>()]);
      final e = await h.add('hello', thread: threadA);
      final follow = await h.outbox.enqueue(
        OutboxRequest(
          kind: 'updateMessage',
          threadId: threadA,
          dependsOn: [e.id],
          payload: const {
            'message_id': null,
            'version': null,
            'content_md': 'hello!',
          },
        ),
      );
      await h.outbox.start();
      await settle();
      await h.outbox.confirmByEcho(
        e.id,
        result: const {
          'message_id': '0192f4a0-7b1c-7000-8a00-0000000000c1',
          'version': 1,
        },
      );
      final bound = jsonDecode((await h.entry(follow.id))!.payload);
      expect(bound['message_id'], '0192f4a0-7b1c-7000-8a00-0000000000c1');
      expect(bound['version'], 1);
      expect(
        updates.ids,
        isEmpty,
        reason: 'FIFO: the create still waits for its response',
      );
    },
  );
}
