// SPDX-License-Identifier: Apache-2.0

// Access removal and revocation (04 8.3, 8.5, R-SY-14, brief M4-14 Tests): access.changed with
// removed in the middle of a catch-up page deletes the channel's rows, search entries and
// drafts, applies the events of other channels after it, commits the page with its cursor,
// emits targetsRemoved and keeps outbox entries; 4012 during a catch-up and session.revoked
// wipe, stop the page and emit signedOut.

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late World w;
  setUp(() => w = World());
  tearDown(() => w.dispose());

  final ch1 = ChannelId.parse(channelId(1));
  final t1 = ThreadId.parse(threadId(1));
  final t2 = ThreadId.parse(threadId(2));

  Future<void> withContent() async {
    await w.connect(cursor: 30);
    await w.dispatch([
      threadCreated(31, 1),
      messageCreated(32, 1, content: 'deploy window agreed'),
      threadCreated(33, 2, channelN: 2),
    ]);
    await w.push('cursor', {'seq': 33});
    await w.db.personalDao.upsertDraftIfNewer(
      Draft(
        id: DraftId.parse(draftId(1)),
        threadId: t1,
        channelId: ch1,
        contentMd: 'reply in progress',
        fileIds: const [],
        version: const ResourceVersion(1),
      ),
    );
    await w.db.outboxDao.enqueue(
      id: id(0x801),
      kind: 'sendMessage',
      threadId: t1,
      payload: '{"content_md":"queued"}',
    );
    expect(await w.db.searchDao.search('deploy', limit: 10), hasLength(1));
  }

  test('access.changed removed in the middle of a page', () async {
    await withContent();
    await w.dispatch([
      messageCreated(34, 2, content: 'deploy before removal'),
      accessRemoved(35, 1),
      messageCreated(36, 1, threadN: 2, channelN: 2, content: 'other channel'),
    ]);
    await w.push('cursor', {'seq': 37});

    expect(await w.cursor(), 37);
    expect(await w.db.threadsDao.byId(t1), isNull);
    expect(await w.storedSeqs(1), isEmpty);
    expect(await w.db.structureDao.channelById(ch1), isNull);
    expect(await w.db.searchDao.search('deploy', limit: 10), isEmpty);
    expect(await w.db.personalDao.allDrafts(), isEmpty);
    expect(await w.db.outboxDao.byId(id(0x801)), isNotNull);
    expect(await w.db.threadsDao.byId(t2), isNotNull);
    expect(await w.storedSeqs(2), [1], reason: 'the later event of channel 2');
    final removed = w.notices.whereType<TargetsRemoved>().single;
    expect(removed.targets, [TargetRef.channel(ch1)]);
    expect((await w.db.syncStateDao.read()).permissionVersion?.value, 4);
  });

  test('4012 during a catch-up wipes, stops the page and signs out', () async {
    await withContent();
    await w.dispatch([threadCreated(34, 3)]);
    w.ch.serverClose(4012, 'session revoked');
    await w.idle();
    expect(w.catchUp.wiped, isTrue);
    expect(w.notices.last, isA<SignedOutNotice>());
    expect((w.notices.last as SignedOutNotice).deleteLocalData, isTrue);
    expect(w.h.phase, ConnectionPhase.signedOut);
  });

  test('session.revoked of this session wipes and stops the page', () async {
    await withContent();
    await w.dispatch([
      envelope(
        'session.revoked',
        seq: 34,
        payload: {'session_id': 'sid-alex-laptop', 'reason': 'admin'},
        resourceKind: 'auth_session',
        resourceId: id(0x5e),
        personal: true,
      ),
      threadCreated(35, 3),
    ]);
    await w.push('cursor', {'seq': 35});
    expect(w.catchUp.wiped, isTrue);
    expect(w.notices.whereType<SignedOutNotice>(), hasLength(1));
  });

  test(
    'session.revoked of another session of the person changes nothing',
    () async {
      await withContent();
      await w.dispatch([
        envelope(
          'session.revoked',
          seq: 34,
          payload: {'session_id': 'sid-alex-phone', 'reason': 'logout'},
          resourceKind: 'auth_session',
          resourceId: id(0x5f),
          personal: true,
        ),
        threadCreated(35, 3),
      ]);
      await w.push('cursor', {'seq': 35});
      expect(w.catchUp.wiped, isFalse);
      expect(
        await w.db.threadsDao.byId(ThreadId.parse(threadId(3))),
        isNotNull,
      );
    },
  );

  test(
    'access.changed added loads the structure without older events',
    () async {
      await w.connect(cursor: 30);
      w.rest.routes['GET /channels/${channelId(7)}'] = (_) =>
          FakeReply(200, channelView(7, level: 'read'));
      w.rest.routes['GET /channels/${channelId(7)}/threads'] = (_) =>
          FakeReply(200, {
            'items': [threadJson(70, channelN: 7, lastSeq: 4)],
          });
      await w.dispatch([
        envelope(
          'access.changed',
          seq: 31,
          payload: {
            'permission_version': 5,
            'added': [
              {'kind': 'channel', 'id': channelId(7), 'level': 'read'},
            ],
            'removed': <Object?>[],
            'changed': <Object?>[],
          },
          resourceKind: 'principal',
          resourceId: alexId,
          personal: true,
        ),
      ]);
      await w.push('cursor', {'seq': 31});
      final access = await w.db.structureDao
          .watchChannelAccess(ChannelId.parse(channelId(7)))
          .first;
      expect(access!.level, AccessLevel.read);
      expect(
        await w.db.threadsDao.byId(ThreadId.parse(threadId(70))),
        isNotNull,
      );
      expect(w.rest.requests.where((r) => r.path == '/events'), isEmpty);
    },
  );
}
