// SPDX-License-Identifier: Apache-2.0

// The files of a new chat against the fake server that checks every request and answer
// against the API contract (FEAT-NEW-CHAT-ATTACH; rulings FNCA/2 to FNCA/4; 12 3, 12 10, 15 7.5):
// files staged in the new chat view go out on the send as their uploads, then `createThread`
// whose first message attaches them; offline the batch waits; the agent run of the first
// message gets the files as the run of a message in an existing chat does; an upload that
// fails after the send holds the post, "Try again" sends it again, "Discard" leaves no file and
// no open session on the server; a refused post leaves none either; the app quitting after the
// send resumes the uploads with their sessions and creates the post.

import 'dart:convert';
import 'dart:io';

import 'package:client/composer/draft_sync.dart';
import 'package:client/files/attach_controller.dart';
import 'package:client/files/file_ports.dart';
import 'package:client/files/upload_executor.dart';
import 'package:client/forum/new_chat_controller.dart';
import 'package:client/forum/post_actions.dart';
import 'package:client/sync/thread_ops.dart';
import 'package:client/thread/thread_actions.dart';
import 'package:domain/domain.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:sync_engine/testing.dart';

import '../support/fake_instance.dart' show eventually;
import '../support/file_fakes.dart';

void main() {
  // flutter_test answers every HttpClient request with 400; these tests talk to the fake.
  HttpOverrides? saved;
  late UploadHarness h;
  late String forum;
  late Uuid7Generator ids;

  setUp(() async {
    saved = HttpOverrides.current;
    HttpOverrides.global = null;
    h = await UploadHarness.start(posts: true);
    forum = h.server.world.addForum(h.seed.space, 'support', {
      h.seed.alex.id: 'trigger',
      h.seed.robin.id: 'write',
    });
    var n = 0;
    ids = Uuid7Generator(
      nowMillis: () => DateTime.now().millisecondsSinceEpoch,
      random: (bits) => (n++ * 104729) % (1 << (bits > 30 ? 30 : bits)),
    );
  });
  tearDown(() async {
    await h.stop();
    HttpOverrides.global = saved;
  });

  FakeWorld world() => h.server.world;

  Directory staging() => Directory('${h.folder.path}/staged/$forum');

  AttachController attach() => AttachController(
    thread: null,
    channel: ChannelId.parse(forum),
    outbox: h.outbox,
    progress: h.progress,
    newKey: ids.next,
    outgoing: () async => Directory('${h.folder.path}/outgoing'),
    staging: () async => staging(),
  );

  NewChatController chat(AttachController files) => NewChatController(
    forum: ChannelId.parse(forum),
    outbox: h.outbox,
    dao: h.db.outboxDao,
    editing: DraftEditingRegistry(),
    dropStoredDraft: () async {},
    attach: files,
  );

  Future<PickedFile> file(String name, List<int> bytes) async {
    final f = await h.file(name, bytes);
    return PickedFile(path: f.path, name: name);
  }

  Future<List<OutboxEntry>> entries() => h.db.outboxDao.allEntries();

  /// The posts the fake created in the forum, with their first message.
  List<JsonMap> firstMessages() => [
    for (final t in world().threads.values)
      if (t['channel_id'] == forum)
        world().messagesOf(t['id']! as String).first,
  ];

  List<String> attachedNames(JsonMap message) => [
    for (final a in message['attachments']! as List)
      ((a as Map)['file']! as Map)['original_name']! as String,
  ];

  List<String> ops() => [
    for (final r in h.server.requests)
      if (r.status >= 200 && r.status < 300) r.operationId,
  ];

  test('a new chat whose first message is only a pasted image', () async {
    final files = attach();
    final c = chat(files);
    expect(
      await files.add([
        PickedFile(
          path: (await h.file('paste.png', utf8.encode('png bytes'))).path,
          name: 'screenshot.png',
          owned: true,
        ),
      ]),
      isTrue,
    );
    // Staged: nothing reached the outbox or the server.
    expect(await entries(), isEmpty);
    expect(c.canSend, isTrue);
    final entry = await c.send('');
    expect(entry, isNotNull);
    await h.outbox.start();
    await eventually('the post', () async => firstMessages().isNotEmpty);
    final first = firstMessages().single;
    expect(first['content_md'], '');
    expect(attachedNames(first), ['screenshot.png']);
    final request = h.server.requestsOf('createThread').single;
    final ids = (request.body! as Map)['first_message']! as Map;
    expect(ids['attachment_file_ids'], [world().files.keys.single]);
    expect(
      ops().indexOf('completeUpload'),
      lessThan(ops().indexOf('createThread')),
    );
    await eventually(
      'the outbox is empty',
      () async => (await entries()).isEmpty,
    );
    expect(h.server.violations, isEmpty);
    c.dispose();
  });

  test('a new chat with text and two files, sent offline, goes out after the '
      'reconnect with both files in their order', () async {
    final files = attach();
    final c = chat(files);
    await files.add([
      await file('server.log', utf8.encode('the server log')),
      await file('client.log', utf8.encode('the client log')),
    ]);
    await c.send('Both logs of the outage');
    // Offline (the outbox is not started): the batch waits, nothing went out.
    expect(h.server.requestsOf('createUpload'), isEmpty);
    expect(h.server.requestsOf('createThread'), isEmpty);
    final waiting = await entries();
    expect(waiting.where((e) => e.kind == UploadOps.upload), hasLength(2));
    expect(
      waiting.where((e) => e.kind == ThreadOps.createThread),
      hasLength(1),
    );
    await h.outbox.start();
    await eventually('the post', () async => firstMessages().isNotEmpty);
    final first = firstMessages().single;
    expect(first['content_md'], 'Both logs of the outage');
    expect(attachedNames(first), ['server.log', 'client.log']);
    expect(h.server.requestsOf('createUpload'), hasLength(2));
    expect(h.server.violations, isEmpty);
    c.dispose();
  });

  test('the agent run of the first message receives the files, as the run of '
      'a message in an existing chat does', () async {
    world().resolveMentions = true;
    final agent = ScriptedAgent(
      world(),
      agentId: h.seed.robin.id,
      script: const AgentScript([AgentStep.queue()]),
    )..attach();
    addTearDown(agent.detach);
    final existing = world().addThread(
      forum,
      'Earlier',
      createdBy: h.seed.alex.id,
    );

    // The new chat: two files and a mention of the agent on its first message.
    final files = attach();
    final c = chat(files);
    await files.add([
      await file('a.png', utf8.encode('first image')),
      await file('b.png', utf8.encode('second image')),
    ]);
    await c.send('@robin what do these show?');

    // An existing chat: the same files through the composer of its thread.
    final inThread = AttachController(
      thread: ThreadId.parse(existing),
      channel: ChannelId.parse(forum),
      outbox: h.outbox,
      progress: h.progress,
      newKey: ids.next,
      outgoing: () async => Directory('${h.folder.path}/outgoing'),
    );
    await inThread.add([
      await file('c.png', utf8.encode('first image')),
      await file('d.png', utf8.encode('second image')),
    ]);
    await h.outbox.enqueue(
      ThreadActions.messageRequest(
        ThreadId.parse(existing),
        '@robin what do these show?',
        attachments: inThread.take(),
      ),
    );
    await h.outbox.start();
    await eventually('two runs', () async => agent.runIds.length == 2);

    List<String> triggering(String runId) {
      final run = world().runs[runId]!;
      final message = world().messages[run['triggering_message_id']]!;
      return attachedNames(message);
    }

    final runs = {
      for (final id in agent.runIds) world().runs[id]!['thread_id']: id,
    };
    final newThread = runs.keys.firstWhere((t) => t != existing)! as String;
    expect(world().threads[newThread]!['channel_id'], forum);
    expect(triggering(runs[newThread]!), ['a.png', 'b.png']);
    expect(triggering(runs[existing]!), ['c.png', 'd.png']);
    expect(h.server.violations, isEmpty);
    c.dispose();
  });

  test('an upload that fails after the send holds the post; "Try again" sends '
      'it again with a new session and the post follows', () async {
    world().fileSettings.partBytes = 4;
    h.server.faults.onRequest(
      'uploadPart',
      status: 409,
      code: 'conflict',
      reason: 'part_conflict',
    );
    final files = attach();
    final c = chat(files);
    await files.add([await file('trace.txt', utf8.encode('trace of it'))]);
    final post = (await c.send('The trace'))!;
    await h.outbox.start();
    await eventually('the upload failed', () async {
      final all = await entries();
      return all.any(
        (e) => e.kind == UploadOps.upload && e.state == OutboxState.failed,
      );
    });
    expect(firstMessages(), isEmpty, reason: 'the post waits for its file');
    final failed = (await entries()).firstWhere(
      (e) => e.kind == UploadOps.upload,
    );
    final session =
        decodePayload(failed.payload)[UploadOps.sessionMember]! as Map;
    await retryPost(h.outbox, h.db.outboxDao, post);
    await eventually('the post', () async => firstMessages().isNotEmpty);
    expect(attachedNames(firstMessages().single), ['trace.txt']);
    // The session of the failed upload was released; only the attached file is left.
    await eventually(
      'the outbox is empty',
      () async => (await entries()).isEmpty,
    );
    expect(world().files.keys, isNot(contains(session['file_id'])));
    expect(world().files, hasLength(1));
    expect(h.server.violations, isEmpty);
    c.dispose();
  });

  test('"Discard" of a post whose upload failed leaves no file and no open '
      'session on the server', () async {
    world().fileSettings.partBytes = 4;
    final files = attach();
    final c = chat(files);
    await files.add([
      await file('a.txt', utf8.encode('the first file')),
      await file('b.txt', utf8.encode('the second file')),
    ]);
    final post = (await c.send(''))!;
    // One upload opens its session and then fails for good; the other completes.
    h.server.faults.onRequest(
      'uploadPart',
      status: 409,
      code: 'conflict',
      reason: 'part_conflict',
    );
    await h.outbox.start();
    await eventually('one upload failed', () async {
      final all = await entries();
      return all.any((e) => e.state == OutboxState.failed);
    });
    expect(world().files, isNotEmpty, reason: 'sessions on the server');
    await discardPost(h.outbox, h.db.outboxDao, post);
    await eventually(
      'the outbox is empty',
      () async => (await entries()).isEmpty,
    );
    expect(world().files, isEmpty, reason: 'no orphaned upload');
    expect(firstMessages(), isEmpty, reason: 'no chat was created');
    expect(h.server.violations, isEmpty);
    c.dispose();
  });

  test(
    '"Discard" of a post the server refused deletes its uploaded files',
    () async {
      h.server.faults.onRequest('createThread', status: 403, code: 'forbidden');
      final files = attach();
      final c = chat(files);
      await files.add([
        await file('a.txt', utf8.encode('one')),
        await file('b.txt', utf8.encode('two')),
      ]);
      final post = (await c.send('Refused'))!;
      await h.outbox.start();
      await eventually('the post failed', () async {
        return (await h.db.outboxDao.byId(post))?.state == OutboxState.failed;
      });
      expect(world().files, hasLength(2), reason: 'uploaded, not attached');
      await discardPost(h.outbox, h.db.outboxDao, post);
      await eventually(
        'the outbox is empty',
        () async => (await entries()).isEmpty,
      );
      expect(world().files, isEmpty);
      expect(h.server.violations, isEmpty);
      c.dispose();
    },
  );

  test(
    'a release of a file the server no longer has ends without a failure',
    () async {
      await h.outbox.enqueue(
        UploadOps.releaseRequest(
          channelId: forum,
          key: 'k1',
          fileId: ids.next(),
        ),
      );
      await h.outbox.start();
      await eventually(
        'the outbox is empty',
        () async => (await entries()).isEmpty,
      );
      expect(h.server.requestsOf('deleteFile').single.status, 404);
      // Without a file ID and without a session of this run there is nothing to delete.
      await h.outbox.enqueue(
        UploadOps.releaseRequest(channelId: forum, key: 'k2'),
      );
      await eventually(
        'the outbox is empty',
        () async => (await entries()).isEmpty,
      );
      expect(h.server.requestsOf('deleteFile'), hasLength(1));
      expect(h.server.violations, isEmpty);
    },
  );

  test(
    'the app quitting after the send resumes the uploads with their sessions '
    'and creates the post',
    () async {
      world().fileSettings.partBytes = 4;
      h.server.faults.delay(
        const Duration(milliseconds: 300),
        operationId: 'uploadPart',
      );
      final files = attach();
      final c = chat(files);
      await files.add([
        await file('a.txt', utf8.encode('first file of two')),
        await file('b.txt', utf8.encode('second file of two')),
      ]);
      await c.send('Two files');
      await h.outbox.start();
      await eventually(
        'the first part on its way',
        () async =>
            h.server.requestsOf('uploadPart').isNotEmpty ||
            h.server.requestsOf('createUpload').isNotEmpty,
      );
      // The app quits: the outbox stops, the entries stay in the database.
      await h.outbox.close();
      c.dispose();
      final next = h.reopen();
      addTearDown(next.close);
      await next.start();
      await eventually('the post', () async => firstMessages().isNotEmpty);
      expect(attachedNames(firstMessages().single), ['a.txt', 'b.txt']);
      expect(
        h.server
            .requestsOf('createUpload')
            .map((r) => r.headers['idempotency-key'])
            .toSet(),
        hasLength(2),
        reason: 'one session per file',
      );
      expect(world().files, hasLength(2));
      expect(h.server.violations, isEmpty);
    },
  );
}
