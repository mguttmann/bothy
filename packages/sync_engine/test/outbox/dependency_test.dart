// SPDX-License-Identifier: Apache-2.0

// Uploads are entries before their message; the message waits until every file it attaches is
// confirmed (15 7.5, 15 14).

import 'dart:async';

import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late Harness h;
  late ScriptedExecutor uploads;
  late ScriptedExecutor messages;
  setUp(() {
    h = Harness.open();
    // Each upload runs in its own lane (M4-22 registers the real executor).
    uploads = h.fake('upload', lane: (e) => 'upload:${e.id}');
    messages = h.fake('createMessage');
  });
  tearDown(() => h.close());

  Future<OutboxEntry> upload(String name) =>
      h.outbox.enqueue(OutboxRequest(kind: 'upload', payload: {'name': name}));

  test('a message waits until both of its uploads are confirmed', () async {
    final first = Completer<OutboxResponse>();
    final second = Completer<OutboxResponse>();
    uploads.then([first, second]);
    final u1 = await upload('a.png');
    final u2 = await upload('b.png');
    final m = await h.add(
      'two files',
      thread: threadA,
      dependsOn: [u1.id, u2.id],
    );
    await h.outbox.start();
    await settle();
    expect(uploads.ids, [u1.id, u2.id], reason: 'uploads run in parallel');
    expect(messages.ids, isEmpty);

    first.complete(const OutboxResponse());
    await settle();
    expect(messages.ids, isEmpty, reason: 'one file is still uploading');
    final views = await h.outbox.watchThread(threadA).first;
    expect(views.single.id, m.id);
    expect(views.single.stateLabel, OutboxStateLabel.pending);

    second.complete(const OutboxResponse());
    await settle();
    expect(messages.ids, [m.id]);
  });

  test(
    'a failed upload leaves the message waiting and visible with the reason',
    () async {
      final first = Completer<OutboxResponse>();
      uploads.then([
        first,
        problem(413, 'payload_too_large', reason: 'file_too_large'),
      ]);
      final u1 = await upload('a.png');
      final u2 = await upload('huge.mov');
      final m = await h.add(
        'two files',
        thread: threadA,
        dependsOn: [u1.id, u2.id],
      );
      final later = await h.add('later', thread: threadA);
      await h.outbox.start();
      await settle();
      first.complete(const OutboxResponse());
      await settle();
      await h.time.advance(const Duration(hours: 1));
      expect(
        messages.ids,
        isEmpty,
        reason: 'FIFO: later does not overtake the message',
      );
      expect((await h.entry(u2.id))!.state, OutboxState.failed);
      final views = await h.outbox.watchThread(threadA).first;
      expect(views.map((v) => v.id), [m.id, later.id]);
      expect(views.first.stateLabel, OutboxStateLabel.pending);
      expect(views.first.blockedByCode, 'payload_too_large');
      expect(views.first.blockedByReason, 'file_too_large');
    },
  );

  test('discarding a failed upload fails the message that needs it', () async {
    uploads.then([problem(413, 'payload_too_large')]);
    final u1 = await upload('huge.mov');
    final m = await h.add('one file', thread: threadA, dependsOn: [u1.id]);
    await h.outbox.start();
    await settle();
    await h.outbox.discard(u1.id);
    await settle();
    expect(await h.entry(u1.id), isNull);
    final failed = (await h.entry(m.id))!;
    expect(failed.state, OutboxState.failed);
    expect(failed.lastErrorCode, 'dependency_discarded');
    expect(messages.ids, isEmpty, reason: 'nothing is sent without its file');
  });
}
