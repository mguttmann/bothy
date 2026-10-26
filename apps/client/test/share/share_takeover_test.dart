// SPDX-License-Identifier: Apache-2.0

// The takeover of the share inbox (brief M5-06 Design note 5; 15 6.1, 7.5; 04 13.2) against the
// fake server: each entry of the signed-in instance goes into `share_inbox` and the outbox with
// the extension's `client_msg_id`, uploads before the message; the App Group folder goes once
// the outbox holds its own copies; a second takeover sends nothing twice; completed uploads
// are attached as they are; a forum post carries its files on the first message; a replay the
// server answers with the original completes the entry; entries of another instance stay.

import 'dart:convert';
import 'dart:io';

import 'package:client/share/share_apple.dart';
import 'package:client/share/share_outbox.dart';
import 'package:client/share/share_takeover.dart';
import 'package:client/sync/thread_ops.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

import '../support/fake_instance.dart' show eventually;
import 'share_fakes.dart';
import 'share_harness.dart';

const String _id1 = '0192f4a0-7b1c-7000-8000-000000000101';
const String _id2 = '0192f4a0-7b1c-7000-8000-000000000102';

void main() {
  late ShareHarness h;
  late FakeShareHost host;
  late ShareTakeover takeover;

  setUp(() async {
    h = await ShareHarness.start();
    host = FakeShareHost();
    takeover = ShareTakeover(
      host: host,
      instanceId: h.instanceId,
      outbox: h.shares,
      db: h.db,
    );
  });

  tearDown(() => h.stop());

  Future<List<OutboxEntry>> entries() => h.db.outboxDao.allEntries();

  test('an entry goes into share_inbox and the outbox with the extension\'s '
      'client_msg_id, uploads first; the folder goes after the outbox took '
      'its copy', () async {
    final file = await h.inboxFile(_id1, 1, 'notes.txt', bytesOf('hello Sam'));
    host.inbox[_id1] = inboxEntry(
      _id1,
      instanceId: h.instanceId,
      target: targetRecord(
        'thread',
        h.seed.general,
        threadId: h.seed.releaseNotes,
      ),
      files: [file],
    );

    expect(await takeover.run(), 1);

    final rows = await h.db.shareInboxDao.all();
    expect(rows.single.id, _id1);
    expect(rows.single.targetThreadId?.value, h.seed.releaseNotes);
    final payload = jsonDecode(rows.single.payload) as Map<String, Object?>;
    expect(payload['kind'], 'thread');
    expect(payload['channel_id'], h.seed.general);
    expect(payload['path'], 'general > Release notes');
    expect(payload['comment'], 'See error above');
    final stored = await entries();
    final upload = stored.singleWhere((e) => e.kind == 'upload');
    final message = stored.singleWhere(
      (e) => e.kind == MessageOps.createMessage,
    );
    expect(message.id, _id1);
    expect(message.dependsOn, upload.id);
    final path = decodePayload(upload.payload)['path']! as String;
    expect(path, startsWith(h.outgoing.path), reason: "the outbox's own copy");
    expect(rows.single.filePaths, [path]);
    expect(File(path).readAsStringSync(), 'hello Sam');
    expect(decodePayload(message.payload)['clear_draft'], isNot(true));
    expect(host.deletedEntries, [_id1]);
    expect(host.inbox, isEmpty);

    await h.outbox.start();
    await eventually(
      'sent',
      () => h.sent(h.seed.releaseNotes, _id1).length == 1,
    );
    final sent = h.sent(h.seed.releaseNotes, _id1).single;
    expect(sent['content_md'], 'See error above');
    expect(sent['attachments'], hasLength(1));
    // The row goes once the outbox entry is confirmed.
    await eventually('confirmed', () async => (await entries()).isEmpty);
    await takeover.prune();
    expect(await h.db.shareInboxDao.all(), isEmpty);
  });

  test('a second takeover enqueues nothing again and deletes the folder', () async {
    host.inbox[_id1] = inboxEntry(
      _id1,
      instanceId: h.instanceId,
      target: targetRecord(
        'channel',
        h.seed.general,
        threadId: h.seed.releaseNotes,
        path: 'general',
      ),
    );
    // The first deletion of the folder fails; the entry stays in the App Group.
    host.failDeleteOnce = true;
    expect(await takeover.run(), 1);
    expect(host.inbox.keys, [_id1]);
    final first = await entries();

    expect(await takeover.run(), 0);
    expect(await entries(), first, reason: 'nothing is enqueued twice');
    expect(host.inbox, isEmpty);
    expect((await h.db.shareInboxDao.all()).single.id, _id1);

    await h.outbox.start();
    await eventually('confirmed', () async => (await entries()).isEmpty);
    expect(h.sent(h.seed.releaseNotes, _id1), hasLength(1));
  });

  test(
    'an upload the extension completed is attached without a new upload',
    () async {
      final done = h.completedFile(h.seed.general, 'done.txt', bytesOf('done'));
      final fresh = await h.inboxFile(
        _id1,
        2,
        'new.txt',
        bytesOf('fresh bytes'),
      );
      host.inbox[_id1] = inboxEntry(
        _id1,
        instanceId: h.instanceId,
        target: targetRecord(
          'thread',
          h.seed.general,
          threadId: h.seed.releaseNotes,
        ),
        files: [
          await h.inboxFile(
            _id1,
            1,
            'done.txt',
            bytesOf('done'),
            completedFileId: done,
          ),
          fresh,
        ],
      );

      await takeover.run();

      final stored = await entries();
      expect(stored.where((e) => e.kind == 'upload'), hasLength(1));
      final message = decodePayload(
        stored.singleWhere((e) => e.id == _id1).payload,
      );
      final keys = (message['attachment_keys']! as List).cast<String>();
      expect(keys, hasLength(2));
      expect(message['file:${keys.first}'], done);
      expect(message['file:${keys.last}'], isNull);

      await h.outbox.start();
      await eventually('confirmed', () async => (await entries()).isEmpty);
      expect(
        h.sent(h.seed.releaseNotes, _id1).single['attachments'],
        hasLength(2),
      );
    },
  );

  test('a new post carries its files on the first message under the same '
      'client_msg_id', () async {
    final forum = h.server.world.addForum(h.seed.space, 'requests', {
      h.seed.alex.id: 'write',
    });
    host.inbox[_id2] = inboxEntry(
      _id2,
      instanceId: h.instanceId,
      target: targetRecord('new_post', forum, path: 'requests'),
      title: 'Printer error',
      files: [await h.inboxFile(_id2, 1, 'log.txt', bytesOf('error 42'))],
    );

    await takeover.run();
    final post = (await entries()).singleWhere((e) => e.id == _id2);
    expect(post.kind, ThreadOps.createThread);
    expect(post.threadId, isNull);

    await h.outbox.start();
    await eventually('confirmed', () async => (await entries()).isEmpty);
    final thread = h.server.world.threads.values.singleWhere(
      (t) => t['channel_id'] == forum,
    );
    expect(thread['title'], 'Printer error');
    final first = h.sent(thread['id']! as String, _id2).single;
    expect(first['content_md'], 'See error above');
    expect(first['attachments'], hasLength(1));
  });

  test(
    'a replay the server answers with the original completes the entry',
    () async {
      // The extension's message reached the server; its answer was lost.
      h.server.world.createMessage(h.seed.alex.id, h.seed.releaseNotes, {
        'client_msg_id': _id1,
        'content_md': 'See error above',
      });
      host.inbox[_id1] = inboxEntry(
        _id1,
        instanceId: h.instanceId,
        target: targetRecord(
          'thread',
          h.seed.general,
          threadId: h.seed.releaseNotes,
        ),
      );
      await takeover.run();
      await h.outbox.start();
      await eventually('confirmed', () async => (await entries()).isEmpty);
      await takeover.prune();
      expect(await h.db.shareInboxDao.all(), isEmpty);
      expect(h.sent(h.seed.releaseNotes, _id1), hasLength(1));
    },
  );

  test(
    'entries of another instance and broken entries stay untouched',
    () async {
      host.inbox[_id1] = inboxEntry(
        _id1,
        instanceId: '0192a000-0000-7000-8000-0000000000ff',
        target: targetRecord(
          'thread',
          h.seed.general,
          threadId: h.seed.releaseNotes,
        ),
      );
      host.inbox[_id2] = inboxEntry(
        _id2,
        instanceId: h.instanceId,
        target: targetRecord(
          'thread',
          'not-a-uuid',
          threadId: h.seed.releaseNotes,
        ),
      );
      expect(await takeover.run(), 0);
      expect(await entries(), isEmpty);
      expect(await h.db.shareInboxDao.all(), isEmpty);
      expect(host.inbox.keys, unorderedEquals([_id1, _id2]));
      expect(host.deletedEntries, isEmpty);
    },
  );

  test('upload entry IDs follow from the client_msg_id', () {
    final a = ShareOutbox.uploadIdOf(_id1, 0);
    expect(a, ShareOutbox.uploadIdOf(_id1, 0));
    expect(a, isNot(ShareOutbox.uploadIdOf(_id1, 1)));
    expect(a, isNot(ShareOutbox.uploadIdOf(_id2, 0)));
    expect(
      RegExp(
        r'^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$',
      ).hasMatch(a),
      isTrue,
    );
    expect(a.substring(0, 13), _id1.substring(0, 13));
  });

  test('a takeover without the host does nothing', () async {
    host.failing = true;
    expect(await takeover.run(), 0);
    expect(await entries(), isEmpty);
  });

  test('records of the inbox map to drafts', () {
    final draft = ShareTakeover.draftOf(
      inboxEntry(
        _id1,
        target: targetRecord(
          'new_post',
          '0192a000-0000-7000-8000-0000000000c1',
        ),
        title: 'Printer error',
        files: [
          ShareInboxFileRecord(
            path: '/tmp/x',
            name: 'x.txt',
            mimeType: 'text/plain',
            sizeBytes: 1,
          ),
        ],
      ),
    );
    expect(draft.clientMsgId, _id1);
    expect(draft.title, 'Printer error');
    expect(draft.target.threadId, isNull);
    expect(draft.files.single.name, 'x.txt');
  });
}
