// SPDX-License-Identifier: Apache-2.0

// Editing and deleting (04 13.3, brief M4-15 Design note 6): "not sent yet" is `attempts == 0`.

import 'dart:async';
import 'dart:convert';

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

  test('an entry never attempted is edited and removed in place', () async {
    final e = await h.add('draft', thread: threadA);
    final id = await h.outbox.edit(e.id, {'content_md': 'edited'});
    expect(id, e.id);
    final edited = (await h.entry(e.id))!;
    expect(payloadOf(edited), {'content_md': 'edited'});
    expect(await h.db.outboxDao.entriesFor(threadA), hasLength(1));

    await h.outbox.remove(e.id);
    expect(await h.entry(e.id), isNull);
    await h.outbox.start();
    await settle();
    expect(creates.calls, isEmpty, reason: 'no server request');
  });

  test(
    'an attempted entry keeps ID and payload; edit and delete follow it',
    () async {
      final held = Completer<OutboxResponse>();
      creates.then([held]);
      updates.then([
        const OutboxResponse(result: {'message_id': serverId, 'version': 2}),
      ]);
      final e = await h.add('hello', thread: threadA);
      await h.outbox.start();
      await settle();
      expect(creates.ids, [e.id]);

      final editId = await h.outbox.edit(e.id, {'content_md': 'hello, edited'});
      expect(editId, isNot(e.id));
      expect(payloadOf((await h.entry(e.id))!), {'content_md': 'hello'});
      final edit = (await h.entry(editId))!;
      expect(edit.kind, MessageOps.updateMessage);
      expect(edit.dependsOn, e.id);

      final removeId = await h.outbox.remove(e.id);
      final delete = (await h.entry(removeId!))!;
      expect(delete.kind, MessageOps.deleteMessage);
      expect(delete.dependsOn, editId, reason: 'the delete follows the edit');

      held.complete(
        const OutboxResponse(result: {'message_id': serverId, 'version': 1}),
      );
      await settle();
      expect(updates.calls.single.payload, {
        'message_id': serverId,
        'version': 1,
        'content_md': 'hello, edited',
      });
      expect(deletes.calls.single.payload, {'message_id': serverId});
      expect(await h.db.outboxDao.entriesFor(threadA), isEmpty);
    },
  );

  test(
    'a second edit changes the follow-up that has not been sent yet',
    () async {
      creates.then([Completer<OutboxResponse>()]);
      final e = await h.add('hello', thread: threadA);
      await h.outbox.start();
      await settle();
      final first = await h.outbox.edit(e.id, {'content_md': 'one'});
      final second = await h.outbox.edit(e.id, {'content_md': 'two'});
      expect(second, first);
      expect(payloadOf((await h.entry(first))!)['content_md'], 'two');
      expect(await h.db.outboxDao.entriesFor(threadA), hasLength(2));
    },
  );

  test(
    'an entry reset to pending after a restart still counts as attempted',
    () async {
      final e = await h.add('hello', thread: threadA);
      await h.db.outboxDao.markState(e.id, OutboxState.sending);
      await h.db.outboxDao.markState(e.id, OutboxState.pending);
      final editId = await h.outbox.edit(e.id, {'content_md': 'edited'});
      expect(editId, isNot(e.id));
      expect(payloadOf((await h.entry(e.id))!), {'content_md': 'hello'});
      final removeId = await h.outbox.remove(e.id);
      expect(removeId, isNotNull);
      expect(await h.entry(e.id), isNotNull);
    },
  );

  test('a failed entry edited becomes a new entry with a new ID', () async {
    creates.then([problem(400, 'validation_failed')]);
    final e = await h.add('bad', thread: threadA);
    await h.outbox.start();
    await settle();
    expect((await h.entry(e.id))!.state, OutboxState.failed);
    final id = await h.outbox.edit(e.id, {'content_md': 'good'});
    expect(id, isNot(e.id));
    expect(await h.entry(e.id), isNull);
    await settle();
    expect(creates.ids, [e.id, id]);
    expect(creates.calls.last.payload, {'content_md': 'good'});
  });

  test(
    'send again takes a new ID; discard and remove of a failed entry delete it',
    () async {
      creates.then([problem(422, 'validation_failed'), const OutboxResponse()]);
      final e = await h.add('hello', thread: threadA);
      await h.outbox.start();
      await settle();
      final again = await h.outbox.retry(e.id);
      expect(again, isNot(e.id));
      await settle();
      expect(creates.ids, [e.id, again]);
      expect(await h.entry(e.id), isNull);

      creates.then([problem(400, 'validation_failed')]);
      final f = await h.add('other', thread: threadA);
      await settle();
      expect((await h.entry(f.id))!.state, OutboxState.failed);
      expect(await h.outbox.remove(f.id), isNull);
      expect(await h.entry(f.id), isNull);
    },
  );

  test('retry of an entry that has not failed is refused', () async {
    final e = await h.add('hello', thread: threadA);
    expect(() => h.outbox.retry(e.id), throwsStateError);
    expect(
      () => h.outbox.edit('0192f4a0-7b1c-7000-8a00-0000000000ff', {}),
      throwsStateError,
    );
  });

  test(
    'a kind without follow-ups cannot be changed after an attempt',
    () async {
      final settings = h.fake('putMySetting')
        ..then([Completer<OutboxResponse>()]);
      final e = await h.outbox.enqueue(
        const OutboxRequest(kind: 'putMySetting', payload: {'key': 'ui.theme'}),
      );
      await h.outbox.start();
      await settle();
      expect(settings.ids, [e.id]);
      expect(() => h.outbox.edit(e.id, {'key': 'ui.theme'}), throwsStateError);
      expect(() => h.outbox.remove(e.id), throwsStateError);
    },
  );
}
