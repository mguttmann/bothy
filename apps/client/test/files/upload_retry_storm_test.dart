// SPDX-License-Identifier: Apache-2.0

// FIX-upload-retry-storm (live report of 2026-10-30): one message with four attachments hung
// for minutes because every retry of an upload started at `createUpload`, the status read was
// retried twice (the read retry inside one attempt and the outbox around it), every lane woke
// at the exact `Retry-After` and all uploads ran at once against one bucket of `rl.upload`.
// Against a fake server that limits the upload operations like the server (one bucket,
// `429 rate_limited` with `Retry-After`), a message with four files must finish with one
// session per file, and no retry may announce a new upload.

import 'dart:convert';
import 'dart:io';

import 'package:api_client/api_client.dart' show ReadRetryPolicy;
import 'package:client/files/upload_executor.dart';
import 'package:client/files/upload_planner.dart';
import 'package:client/thread/thread_actions.dart';
import 'package:domain/domain.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart' show decodePayload;

import '../support/fake_instance.dart' show eventually;
import '../support/file_fakes.dart';

/// The operations of `rl.upload` the client sends (12 13).
const _uploadClass = {
  'createUpload',
  'getUpload',
  'uploadPart',
  'completeUpload',
};

void main() {
  HttpOverrides? saved;
  late UploadHarness h;

  setUp(() async {
    saved = HttpOverrides.current;
    HttpOverrides.global = null;
    // The read retry of the app (base and cap from the outbox values), here short, so a
    // second retry layer inside one attempt would show in the request log.
    h = await UploadHarness.start(
      partBytes: 1024,
      readRetry: ReadRetryPolicy.fromBackoff(
        base: const Duration(milliseconds: 20),
        cap: const Duration(milliseconds: 200),
      ),
    );
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

  test('four attachments under the rate limit: one session per file, the '
      'message finishes, no retry announces a new upload', () async {
    h.server.faults.rateLimit(
      _uploadClass,
      burst: 4,
      refill: const Duration(milliseconds: 250),
    );
    const keys = ['k1', 'k2', 'k3', 'k4'];
    final uploads = [
      for (final k in keys)
        await enqueueUpload(
          await h.file('$k.jpg', utf8.encode('photo $k ' * 20)),
          k,
        ),
    ];
    final message = await h.outbox.enqueue(
      ThreadActions.messageRequest(
        thread(),
        'go',
        attachments: MessageAttachments(keys: keys, uploads: uploads),
      ),
    );
    await h.outbox.start();
    await eventually(
      'message sent',
      () => gone(message.id),
      timeout: const Duration(seconds: 60),
    );

    expect(
      h.server.faults.limited,
      isNotEmpty,
      reason: 'the test must hit the limit',
    );
    final creates = h.server.requestsOf('createUpload');
    final created = [
      for (final r in creates)
        if (r.status == 201) r,
    ];
    // One announcement per file that the server accepted; every refused one carried the key
    // of its entry, and no file announced again after its session existed.
    expect(created, hasLength(4));
    expect(
      {for (final r in created) r.headers['idempotency-key']},
      {...uploads},
    );
    for (final id in uploads) {
      final ofEntry = [
        for (final r in creates)
          if (r.headers['idempotency-key'] == id) r.status,
      ];
      expect(ofEntry.last, 201, reason: 'nothing after the session existed');
    }
    expect(h.server.world.files, hasLength(4), reason: 'one file per upload');
    final ids = {for (final f in h.server.world.files.values) f.id};
    final sent = h.server.requestsOf('createMessage').single.body! as Map;
    expect((sent['attachment_file_ids'] as List).toSet(), ids);
    for (final k in keys) {
      expect(ids, contains(h.progress.completedFileOf(k)));
    }
    expect(h.server.violations, isEmpty);
  });

  test('a retry resumes the kept session: no createUpload, one status read, '
      'and a refused status read goes back to the outbox', () async {
    // The first part fails after the session exists; the status read of the next attempt is
    // refused once.
    h.server.faults
      ..onRequest('uploadPart', status: 503, code: 'unavailable')
      ..onRequest(
        'getUpload',
        status: 429,
        code: 'rate_limited',
        retryAfter: const Duration(seconds: 1),
      );
    final id = await enqueueUpload(
      await h.file('a.jpg', utf8.encode('one photo')),
      'k1',
    );
    await h.outbox.start();
    await eventually(
      'status read refused',
      () => h.server.requestsOf('getUpload').isNotEmpty,
    );
    // The read retry of the client would have repeated the read within milliseconds.
    await Future<void>.delayed(const Duration(milliseconds: 400));
    expect(h.server.requestsOf('getUpload'), hasLength(1));
    final waiting = (await h.db.outboxDao.byId(id))!;
    expect(waiting.state, OutboxState.waiting);
    final kept =
        decodePayload(waiting.payload)[UploadOps.sessionMember]! as Map;
    expect(kept['entry'], id);
    expect(kept['file_id'], h.server.world.files.values.single.id);

    await eventually(
      'upload confirmed',
      () => gone(id),
      timeout: const Duration(seconds: 20),
    );
    expect(h.server.requestsOf('createUpload'), hasLength(1));
    expect(h.server.requestsOf('getUpload'), hasLength(2));
    expect(h.server.world.files, hasLength(1));
  });

  test('a fresh session sends its parts without a status read', () async {
    final id = await enqueueUpload(
      await h.file('a.jpg', utf8.encode('one photo')),
      'k1',
    );
    await h.outbox.start();
    await eventually('upload confirmed', () => gone(id));
    expect(
      [
        for (final r in h.server.requests)
          if (_uploadClass.contains(r.operationId)) r.operationId,
      ],
      ['createUpload', 'uploadPart', 'completeUpload'],
    );
  });

  test('Upload again after a final failure starts a new session', () async {
    h.server.faults.onRequest(
      'uploadPart',
      status: 409,
      code: 'conflict',
      reason: 'part_conflict',
    );
    final id = await enqueueUpload(
      await h.file('a.jpg', utf8.encode('one photo')),
      'k1',
    );
    await h.outbox.start();
    await eventually(
      'failed',
      () async => (await h.db.outboxDao.byId(id))?.state == OutboxState.failed,
    );
    final again = await h.outbox.retry(id);
    await eventually('upload confirmed', () => gone(again));
    final creates = h.server.requestsOf('createUpload');
    expect(creates, hasLength(2));
    expect(creates.last.headers['idempotency-key'], again);
    expect(h.server.world.files, hasLength(2));
  });

  test('the uploads of one conversation go one at a time', () async {
    final uploads = [
      for (final k in ['k1', 'k2', 'k3'])
        await enqueueUpload(await h.file('$k.jpg', utf8.encode('photo $k')), k),
    ];
    await h.outbox.start();
    for (final id in uploads) {
      await eventually('upload confirmed', () => gone(id));
    }
    final ops = [
      for (final r in h.server.requests)
        if (_uploadClass.contains(r.operationId)) r.operationId,
    ];
    expect(ops, [
      for (var i = 0; i < 3; i++) ...[
        'createUpload',
        'uploadPart',
        'completeUpload',
      ],
    ]);
    final keys = [
      for (final r in h.server.requestsOf('createUpload'))
        r.headers['idempotency-key'],
    ];
    expect(keys, uploads, reason: 'in the order they were attached');
  });
}
