// SPDX-License-Identifier: Apache-2.0

// An entry the outbox creates never waits for an entry stored as failed: a failed entry is
// never sent, so its dependent would block the lane for good (04 13.2 item 2). The class
// reached the thread three times (a delete behind a discarded edit, a retry after a discarded
// create, a retry after a discarded edit); these tests hold it for retry and edit after a
// discard.

import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

import 'support.dart';

/// The IDs [e] waits for.
List<String> dependenciesOf(OutboxEntry e) =>
    e.dependsOn == null || e.dependsOn!.isEmpty
    ? const []
    : e.dependsOn!.split(',');

void main() {
  const serverId = '0192f4a0-7b1c-7000-8a00-0000000000c1';

  group('message chains', () {
    late Harness h;
    late ScriptedExecutor creates;
    late ScriptedExecutor updates;
    late ScriptedExecutor deletes;

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

    test(
      'retry of a delete failed with its chain after a discarded edit is sent',
      () async {
        creates.then([
          const OutboxResponse(result: {'message_id': serverId, 'version': 1}),
        ]);
        updates.then([
          problem(503, 'unavailable'),
          problem(412, 'version_conflict', current: {'version': 2}),
        ]);
        await h.add('v1', thread: threadA);
        await h.outbox.start();
        await settle();
        final u1 = await h.outbox.enqueue(
          OutboxRequest(
            kind: MessageOps.updateMessage,
            threadId: threadA,
            payload: {'message_id': serverId, 'version': 1, 'content_md': 'v2'},
          ),
        );
        await settle();
        final u2 = await h.outbox.edit(u1.id, {'content_md': 'v3'});
        final d = await h.outbox.remove(u1.id);
        await h.time.advance(const Duration(minutes: 10));
        expect((await h.entry(u1.id))!.state, OutboxState.failed);

        await h.outbox.discard(u1.id);
        expect((await h.entry(u2))!.lastErrorCode, 'dependency_discarded');
        final d2 = await h.outbox.retry(d!);
        expect(
          dependenciesOf((await h.entry(d2))!),
          isNot(contains(u2)),
          reason: 'the new delete never waits for the failed edit',
        );
        final next = await h.add('next', thread: threadA);
        await settle();
        await h.time.advance(const Duration(hours: 24));

        expect(deletes.ids, [d2], reason: 'the delete is sent');
        expect(deletes.calls.single.payload['message_id'], serverId);
        expect(creates.ids, contains(next.id), reason: 'the thread goes on');
      },
    );

    test('retry of a delete failed with its chain after a discarded create unblocks the thread', () async {
      creates.then([
        problem(503, 'unavailable'),
        problem(422, 'validation_failed'),
      ]);
      final c = await h.add('v1', thread: threadA);
      await h.outbox.start();
      await settle();
      final u = await h.outbox.edit(c.id, {'content_md': 'v2'});
      final d = await h.outbox.remove(c.id);
      await h.time.advance(const Duration(minutes: 10));
      expect((await h.entry(c.id))!.state, OutboxState.failed);

      await h.outbox.remove(c.id); // discards the failed create
      final d2 = await h.outbox.retry(d!);
      expect(dependenciesOf((await h.entry(d2))!), isNot(contains(u)));
      final next = await h.add('next', thread: threadA);
      await h.time.advance(const Duration(hours: 24));

      expect(creates.ids, contains(next.id), reason: 'the thread goes on');
    });
  });

  group('generic chains', () {
    late Harness h;
    late ScriptedExecutor ops;

    setUp(() {
      h = Harness.open();
      ops = h.fake('op');
    });
    tearDown(() => h.close());

    Future<OutboxEntry> op(String name, {List<String> after = const []}) =>
        h.outbox.enqueue(
          OutboxRequest(
            kind: 'op',
            threadId: threadA,
            payload: {'name': name},
            dependsOn: after,
          ),
        );

    // a fails; b waits for a, e waits for b; discarding a fails b and e with their chain.
    Future<(OutboxEntry, OutboxEntry)> discardedChain() async {
      ops.then([problem(422, 'validation_failed')]);
      final a = await op('a');
      final b = await op('b', after: [a.id]);
      final e = await op('e', after: [b.id]);
      await h.outbox.start();
      await settle();
      await h.outbox.discard(a.id);
      expect((await h.entry(b.id))!.state, OutboxState.failed);
      expect((await h.entry(e.id))!.state, OutboxState.failed);
      return (b, e);
    }

    test('retry after a discard never waits for a failed entry', () async {
      final (b, e) = await discardedChain();
      final fresh = await h.outbox.retry(e.id);
      expect(dependenciesOf((await h.entry(fresh))!), isNot(contains(b.id)));
      final next = await op('next');
      await settle();
      expect(ops.ids, containsAllInOrder([fresh, next.id]));
      expect((await h.entry(b.id))!.state, OutboxState.failed);
    });

    test('edit after a discard never waits for a failed entry', () async {
      final (b, e) = await discardedChain();
      final fresh = await h.outbox.edit(e.id, {'name': 'e2'});
      expect(dependenciesOf((await h.entry(fresh))!), isNot(contains(b.id)));
      await settle();
      expect(ops.calls.last.id, fresh);
      expect(ops.calls.last.payload['name'], 'e2');
    });

    test('retry keeps waiting for a parent that is not failed', () async {
      ops.then([problem(422, 'validation_failed')]);
      final a = await op('a');
      final b = await op('b', after: [a.id]);
      final e = await op('e', after: [b.id]);
      await h.outbox.start();
      await settle();
      // b and e wait behind the failed a; retrying a re-points b to the new entry.
      final a2 = await h.outbox.retry(a.id);
      expect(dependenciesOf((await h.entry(b.id))!), [a2]);
      await settle();
      expect(ops.ids, [a.id, a2, b.id, e.id]);
    });
  });
}
