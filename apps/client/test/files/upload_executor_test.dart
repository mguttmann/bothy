// SPDX-License-Identifier: Apache-2.0

// The upload executor against the fake server (brief M4-22 Tests; 12 3; 15 7.5, 15 14): the
// session with `Idempotency-Key`, the parts with `Content-Digest`, the completion; an abort
// after part 2 resumes with the parts `getUpload` did not confirm; `part_conflict` fails the
// entry; the message waits for its uploads and attaches their file IDs; files over the limits
// are refused before any entry is written.

import 'dart:convert';
import 'dart:io';

import 'package:client/files/attach_controller.dart';
import 'package:client/files/file_ports.dart';
import 'package:client/files/upload_executor.dart';
import 'package:client/files/upload_planner.dart';
import 'package:client/thread/thread_actions.dart';
import 'package:crypto/crypto.dart';
import 'package:domain/domain.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart' show decodePayload;
import 'package:sync_engine/testing.dart' show FilesWorld;

import '../support/fake_instance.dart' show eventually;
import '../support/file_fakes.dart';

void main() {
  // flutter_test answers every HttpClient request with 400; these tests talk to the fake.
  HttpOverrides? saved;
  late UploadHarness h;

  setUp(() async {
    saved = HttpOverrides.current;
    HttpOverrides.global = null;
    h = await UploadHarness.start();
  });
  tearDown(() async {
    await h.stop();
    HttpOverrides.global = saved;
  });

  ThreadId thread() => ThreadId.parse(h.seed.releaseNotes);
  ChannelId channel() => ChannelId.parse(h.seed.general);

  Future<String> enqueueUpload(File f, String key) async {
    final source = await prepareUpload(f.path);
    final e = await h.outbox.enqueue(
      UploadOps.request(
        threadId: thread(),
        channelId: channel(),
        source: source,
        key: key,
      ),
    );
    return e.id;
  }

  Future<bool> gone(String id) async => await h.db.outboxDao.byId(id) == null;

  test('session, parts with digests, completion', () async {
    final bytes = utf8.encode('hello world');
    final f = await h.file('notes.txt', bytes);
    final id = await enqueueUpload(f, 'k1');
    await h.outbox.start();
    await eventually('upload confirmed', () => gone(id));

    final create = h.server.requestsOf('createUpload').single;
    expect(create.headers['idempotency-key'], id);
    expect((create.body! as Map)['sha256'], sha256.convert(bytes).toString());
    final parts = h.server.requestsOf('uploadPart');
    expect(parts, hasLength(3));
    for (final p in parts) {
      final n = int.parse(p.path.split('/').last);
      final slice = bytes.sublist((n - 1) * 4, n * 4 > 11 ? 11 : n * 4);
      expect(p.headers['content-digest'], contentDigestOf(slice));
    }
    expect(h.server.requestsOf('completeUpload').single.status, 200);
    final file = h.server.world.files.values.single;
    expect(file.json['upload_state'], 'complete');
    expect(h.progress.completedFileOf('k1'), file.id);
    expect(h.server.violations, isEmpty);
  });

  test(
    'an abort after part 2 resumes with the parts getUpload did not confirm',
    () async {
      h.server.world.fileSettings.abortAfterParts = 2;
      final bytes = utf8.encode('twenty bytes of text');
      final f = await h.file('long.txt', bytes);
      final id = await enqueueUpload(f, 'k1');
      await h.outbox.start();
      await eventually('upload confirmed', () => gone(id));

      final parts = h.server.requestsOf('uploadPart');
      expect(
        [for (final p in parts) p.status].where((s) => s == 503),
        hasLength(1),
      );
      final stored = <String>[
        for (final p in parts)
          if (p.status == 200) p.path.split('/').last,
      ];
      // Every part reached the server exactly once; the confirmed ones were not sent again.
      expect(stored.toSet(), {'1', '2', '3', '4', '5'});
      expect(stored, hasLength(5));
      // The first attempt announced the session and needed no status read; the retry
      // resumed the kept session with one status read and no new announcement
      // (FIX-upload-retry-storm).
      expect(h.server.requestsOf('getUpload'), hasLength(1));
      expect(h.server.requestsOf('createUpload'), hasLength(1));
      expect(h.server.world.files, hasLength(1), reason: 'the same session');
      expect(
        h.server.world.files.values.single.json['upload_state'],
        'complete',
      );
    },
  );

  test('an upload completed while the app quit completes again without its '
      'file', () async {
    // The app quits while completeUpload is on its way: the server completes the upload, the
    // executor deletes the copy, but the closed outbox never records the confirmation.
    h.server.faults.delay(
      const Duration(milliseconds: 300),
      operationId: 'completeUpload',
    );
    final f = await h.file('voice.m4a', utf8.encode('recorded audio'));
    final source = await prepareUpload(f.path);
    final e = await h.outbox.enqueue(
      UploadOps.request(
        threadId: thread(),
        channelId: channel(),
        source: source,
        key: 'k1',
        owned: true,
      ),
    );
    await h.outbox.start();
    await eventually(
      'completing',
      () => h.server.world.files.values.any(
        (x) => x.json['upload_state'] == 'complete',
      ),
    );
    await h.outbox.close();
    await eventually('copy deleted', () => !f.existsSync());

    final next = h.reopen();
    addTearDown(next.close);
    await next.start();
    await eventually('confirmed after the restart', () => gone(e.id));
    expect(h.server.world.files, hasLength(1));
  });

  test(
    'a file that cannot be read fails the entry instead of retrying',
    () async {
      final f = await h.file('locked.txt', utf8.encode('secret'));
      final id = await enqueueUpload(f, 'k1');
      await Process.run('chmod', ['000', f.path]);
      addTearDown(() => Process.run('chmod', ['644', f.path]));
      try {
        await f.openRead().first;
        // A privileged process still reads a mode-000 file: nothing to check here.
        markTestSkipped('this process can read a file with mode 000');
        return;
      } on FileSystemException {
        // Unreadable, as the test needs.
      }
      await h.outbox.start();
      await eventually(
        'failed',
        () async =>
            (await h.db.outboxDao.byId(id))?.state == OutboxState.failed,
      );
      expect((await h.db.outboxDao.byId(id))!.lastErrorCode, 'file_unreadable');
    },
    skip: Platform.isWindows ? 'chmod does not exist on Windows' : false,
  );

  test('part_conflict fails the entry with its reason', () async {
    h.server.faults.onRequest(
      'uploadPart',
      status: 409,
      code: 'conflict',
      reason: 'part_conflict',
    );
    final f = await h.file('a.txt', utf8.encode('abcd'));
    final id = await enqueueUpload(f, 'k1');
    await h.outbox.start();
    await eventually(
      'failed',
      () async => (await h.db.outboxDao.byId(id))?.state == OutboxState.failed,
    );
    final e = (await h.db.outboxDao.byId(id))!;
    expect(e.lastErrorCode, 'conflict');
    expect(e.lastErrorReason, 'part_conflict');
    expect(h.server.requestsOf('completeUpload'), isEmpty);
  });

  test(
    'the message waits for its uploads and attaches their file IDs',
    () async {
      final a = await enqueueUpload(
        await h.file('a.txt', utf8.encode('first file')),
        'ka',
      );
      final b = await enqueueUpload(
        await h.file('b.txt', utf8.encode('second')),
        'kb',
      );
      final message = await h.outbox.enqueue(
        ThreadActions.messageRequest(
          thread(),
          'Two files',
          attachments: MessageAttachments(
            keys: const ['ka', 'kb'],
            uploads: [a, b],
          ),
        ),
      );
      await h.outbox.start();
      await eventually('message sent', () => gone(message.id));

      final ops = [for (final r in h.server.requests) r.operationId];
      expect(
        ops.lastIndexOf('completeUpload'),
        lessThan(ops.indexOf('createMessage')),
      );
      final sent = h.server.requestsOf('createMessage').single.body! as Map;
      final ids = [
        for (final k in ['ka', 'kb']) h.progress.completedFileOf(k),
      ];
      expect(sent['attachment_file_ids'], ids);
      expect(sent.keys, isNot(contains('attachment_keys')));
      expect(sent.keys.where((k) => '$k'.startsWith('file:')), isEmpty);
      final stored = h.server.world.messagesOf(h.seed.releaseNotes).last;
      expect((stored['attachments']! as List), hasLength(2));
      expect(h.server.violations, isEmpty);
    },
  );

  test(
    'an upload confirmed before the message still attaches its file',
    () async {
      final a = await enqueueUpload(
        await h.file('a.txt', utf8.encode('early')),
        'ka',
      );
      await h.outbox.start();
      await eventually('upload confirmed', () => gone(a));
      final message = await h.outbox.enqueue(
        ThreadActions.messageRequest(
          thread(),
          'Late text',
          attachments: MessageAttachments(
            keys: const ['ka'],
            uploads: [a],
            fileIds: {'ka': h.progress.completedFileOf('ka')!},
          ),
        ),
      );
      await eventually('message sent', () => gone(message.id));
      final sent = h.server.requestsOf('createMessage').single.body! as Map;
      expect(sent['attachment_file_ids'], [h.progress.completedFileOf('ka')]);
    },
  );

  // FIX-voice-send: the server answers a voice message whose file is still being checked with
  // `409 file_not_ready` (12 8.1 rule 1); the message waits in the outbox until the file is
  // ready instead of failing (15 13).
  test('a voice message waits while its file is being checked', () async {
    h.server.world.fileSettings.holdScan = true;
    final up = await enqueueUpload(
      await h.file('voice.m4a', utf8.encode('recorded audio')),
      'kv',
    );
    final message = await h.outbox.enqueue(
      ThreadActions.messageRequest(
        thread(),
        '',
        kind: MessageKind.voice,
        attachments: MessageAttachments(keys: const ['kv'], uploads: [up]),
      ),
    );
    await h.outbox.start();
    await eventually(
      'refused while being checked',
      () => h.server.requestsOf('createMessage').any((r) => r.status == 409),
    );
    final waiting = (await h.db.outboxDao.byId(message.id))!;
    expect(waiting.state, isNot(OutboxState.failed));

    h.server.world.finishScan(h.progress.completedFileOf('kv')!);
    await eventually('message sent', () => gone(message.id));
    final sends = h.server.requestsOf('createMessage');
    expect(sends.first.status, 409);
    expect(sends.last.status, 201);
    expect({for (final r in sends) r.headers['idempotency-key']}, hasLength(1));
    final stored = h.server.world.messagesOf(h.seed.releaseNotes).last;
    expect(stored['kind'], 'voice');
    expect((stored['attachments']! as List), hasLength(1));
    expect(h.server.violations, isEmpty);
  });

  // "Send again" of a voice message that failed (here as the server answered the Windows
  // recording before FIX-voice-send) sends the same file in a new entry and is stored.
  test('send again of a failed voice message attaches its file', () async {
    final up = await enqueueUpload(
      await h.file('voice.m4a', utf8.encode('recorded audio')),
      'kv',
    );
    h.server.faults.onRequest(
      'createMessage',
      status: 400,
      code: 'validation_failed',
    );
    final message = await h.outbox.enqueue(
      ThreadActions.messageRequest(
        thread(),
        '',
        kind: MessageKind.voice,
        attachments: MessageAttachments(keys: const ['kv'], uploads: [up]),
      ),
    );
    await h.outbox.start();
    await eventually(
      'failed',
      () async =>
          (await h.db.outboxDao.byId(message.id))?.state == OutboxState.failed,
    );
    final again = await h.outbox.retry(message.id);
    await eventually('sent again', () => gone(again));
    final sends = h.server.requestsOf('createMessage');
    expect([for (final r in sends) r.status], [400, 201]);
    expect((sends.last.body! as Map)['attachment_file_ids'], [
      h.progress.completedFileOf('kv'),
    ]);
    expect(
      h.server.world.messagesOf(h.seed.releaseNotes).last['kind'],
      'voice',
    );
  });

  group('limits before any entry', () {
    late AttachController attach;
    var keys = 0;

    setUp(() {
      attach = AttachController(
        thread: ThreadId.parse(h.seed.releaseNotes),
        channel: ChannelId.parse(h.seed.general),
        outbox: h.outbox,
        progress: h.progress,
        newKey: () => 'key${keys++}',
        outgoing: () async => Directory('${h.folder.path}/outgoing'),
      );
    });
    tearDown(() => attach.dispose());

    Future<PickedFile> picked(String name, int size) async {
      final f = await h.file(name, List.filled(size, 0x41));
      return PickedFile(path: f.path, name: name);
    }

    test('a file over file_max_upload_bytes is refused', () async {
      attach.limits = const InstanceLimits(
        messageMaxContentBytes: 1000,
        messageMaxAttachments: 10,
        fileMaxUploadBytes: 8,
        threadTitleMaxChars: 100,
        voiceMaxDurationMs: 1000,
      );
      final ok = await attach.add([await picked('big.bin', 9)]);
      expect(ok, isFalse);
      expect(
        attach.refusal,
        isA<FileTooLarge>()
            .having((r) => r.name, 'name', 'big.bin')
            .having((r) => r.limitBytes, 'limit', 8),
      );
      expect(await h.db.outboxDao.allEntries(), isEmpty);
    });

    test('more files than message_max_attachments are refused', () async {
      attach.limits = const InstanceLimits(
        messageMaxContentBytes: 1000,
        messageMaxAttachments: 1,
        fileMaxUploadBytes: 100,
        threadTitleMaxChars: 100,
        voiceMaxDurationMs: 1000,
      );
      final ok = await attach.add([
        await picked('a.bin', 3),
        await picked('b.bin', 3),
      ]);
      expect(ok, isFalse);
      expect(attach.refusal, isA<TooManyFiles>());
      expect(await h.db.outboxDao.allEntries(), isEmpty);
    });

    test('an attached file is copied, so the upload does not need the '
        'original (macOS sandbox after a restart)', () async {
      final original = await picked('a.bin', 5);
      expect(await attach.add([original]), isTrue);
      final entry = (await h.db.outboxDao.allEntries()).single;
      final path = decodePayload(entry.payload)['path']! as String;
      expect(path, startsWith('${h.folder.path}/outgoing'));
      expect(decodePayload(entry.payload)['delete_after'], isTrue);
      await File(original.path).delete();
      await h.outbox.start();
      await eventually('upload confirmed', () => gone(entry.id));
      expect(
        File(path).existsSync(),
        isFalse,
        reason: 'deleted after the upload',
      );
    });

    test('Upload again gives a failed attachment a new entry', () async {
      h.server.faults.onRequest(
        'uploadPart',
        status: 409,
        code: 'conflict',
        reason: 'part_conflict',
      );
      expect(await attach.add([await picked('a.bin', 4)]), isTrue);
      final first = attach.attachments.single;
      await h.outbox.start();
      await eventually(
        'failed',
        () async =>
            (await h.db.outboxDao.byId(first.entryId))?.state ==
            OutboxState.failed,
      );
      await attach.retry(first.key);
      final again = attach.attachments.single;
      expect(again.key, first.key);
      expect(again.entryId, isNot(first.entryId));
      expect(await h.db.outboxDao.byId(first.entryId), isNull);
      await attach.retry('unknown');
      expect(attach.attachments.single.entryId, again.entryId);
    });

    test('dragging over the composer notifies once per change', () {
      var calls = 0;
      attach.addListener(() => calls++);
      attach.setDragging(true);
      attach.setDragging(true);
      expect(attach.dragging, isTrue);
      attach.setDragging(false);
      expect(attach.dragging, isFalse);
      expect(calls, 2);
    });

    test('a removed attachment leaves no copy behind', () async {
      expect(await attach.add([await picked('a.bin', 5)]), isTrue);
      final key = attach.attachments.single.key;
      final copy = Directory('${h.folder.path}/outgoing/$key');
      expect(copy.existsSync(), isTrue);
      await attach.remove(key);
      expect(await h.db.outboxDao.allEntries(), isEmpty);
      expect(copy.existsSync(), isFalse);
    });

    test('files within the limits become upload entries', () async {
      final ok = await attach.add([await picked('a.bin', 3)]);
      expect(ok, isTrue);
      expect(attach.attachments.single.name, 'a.bin');
      final entries = await h.db.outboxDao.allEntries();
      expect(entries.single.kind, UploadOps.upload);
      expect(entries.single.id, attach.attachments.single.entryId);
    });
  });
}
