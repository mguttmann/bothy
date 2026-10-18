// SPDX-License-Identifier: Apache-2.0

// Personal data (04 14, 15 7.3 item 5, brief M4-14 Tests): the echo of this device is ignored,
// the read state never decreases, a draft being typed here is not overwritten, settings.updated
// with deleted removes the key, overwrote_newer raises a notice, and the counters run between
// two server values.

import 'package:domain/domain.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late World w;
  setUp(() => w = World());
  tearDown(() => w.dispose());

  final t1 = ThreadId.parse(threadId(1));
  final ch1 = ChannelId.parse(channelId(1));

  Map<String, Object?> draftUpdated(int seq, Map<String, Object?> draft) =>
      envelope(
        'draft.updated',
        seq: seq,
        payload: {'draft': draft},
        resourceKind: 'draft',
        resourceId: draft['id']! as String,
        version: draft['version']! as int,
        personal: true,
      );

  Map<String, Object?> settingsUpdated(
    int seq,
    Map<String, Object?> setting, {
    bool deleted = false,
  }) => envelope(
    'settings.updated',
    seq: seq,
    payload: {'setting': setting, 'deleted': ?(deleted ? true : null)},
    resourceKind: 'user',
    resourceId: alexId,
    personal: true,
  );

  Map<String, Object?> readStateUpdated(int seq, Map<String, Object?> state) =>
      envelope(
        'read_state.updated',
        seq: seq,
        payload: {'read_state': state},
        resourceKind: 'thread',
        resourceId: threadId(1),
        personal: true,
      );

  Future<Draft?> storedDraft() =>
      w.db.personalDao.watchDraft(channelId: ch1, threadId: t1).first;

  test('the echo of this device is ignored', () async {
    await w.connect(cursor: 30);
    await w.dispatch([
      draftUpdated(
        31,
        draftJson(1, text: 'typed here', version: 2, device: deviceIdOfThis),
      ),
      settingsUpdated(
        32,
        settingJson(
          'composer.enter_sends',
          false,
          version: 2,
          device: deviceIdOfThis,
        ),
      ),
    ]);
    await w.push('cursor', {'seq': 32});
    expect(await storedDraft(), isNull);
    expect(await w.db.personalDao.watchSettings().first, isEmpty);
    await w.dispatch([
      draftUpdated(
        33,
        draftJson(
          1,
          text: 'typed on the phone',
          version: 3,
          device: otherDevice,
        ),
      ),
    ]);
    await w.push('cursor', {'seq': 33});
    expect((await storedDraft())!.contentMd, 'typed on the phone');
  });

  test('the read state never decreases', () async {
    await w.connect(cursor: 30);
    await w.dispatch([
      readStateUpdated(31, readStateJson(1, lastRead: 12, version: 3)),
      readStateUpdated(32, readStateJson(1, lastRead: 9, version: 4)),
    ]);
    await w.push('cursor', {'seq': 32});
    final state = await w.db.personalDao.readState(t1);
    expect(state!.lastReadSeq, const ThreadSeq(12));
    expect(state.version, const ResourceVersion(4));
  });

  test('a draft being typed on this device is not overwritten', () async {
    await w.connect(cursor: 30);
    await w.dispatch([
      draftUpdated(
        31,
        draftJson(1, text: 'first', version: 1, device: otherDevice),
      ),
    ]);
    await w.push('cursor', {'seq': 31});
    w.editing.editingThread = threadId(1);
    await w.dispatch([
      draftUpdated(
        32,
        draftJson(1, text: 'from the phone', version: 2, device: otherDevice),
      ),
      envelope(
        'draft.deleted',
        seq: 33,
        payload: {
          'draft_id': draftId(1),
          'channel_id': channelId(1),
          'thread_id': threadId(1),
        },
        resourceKind: 'draft',
        resourceId: draftId(1),
        personal: true,
      ),
    ]);
    await w.push('cursor', {'seq': 33});
    expect((await storedDraft())!.contentMd, 'first');
  });

  test('settings.updated with deleted removes the key', () async {
    await w.connect(cursor: 30);
    await w.dispatch([
      settingsUpdated(
        31,
        settingJson(
          'composer.enter_sends',
          true,
          version: 1,
          device: otherDevice,
        ),
      ),
    ]);
    await w.push('cursor', {'seq': 31});
    expect(await w.db.personalDao.watchSettings().first, hasLength(1));
    await w.dispatch([
      settingsUpdated(
        32,
        settingJson(
          'composer.enter_sends',
          true,
          version: 2,
          device: otherDevice,
        ),
        deleted: true,
      ),
    ]);
    await w.push('cursor', {'seq': 32});
    expect(await w.db.personalDao.watchSettings().first, isEmpty);
  });

  test('overwrote_newer of the write path raises a notice', () async {
    await w.connect(cursor: 30);
    await w.catchUp.applyDraftPutResult({
      'draft': draftJson(
        1,
        text: 'mine wins',
        version: 5,
        device: deviceIdOfThis,
      ),
      'overwrote_newer': true,
    });
    expect(
      w.notices.whereType<DraftOverwritten>().single.draft.contentMd,
      'mine wins',
    );
    expect((await storedDraft())!.version, const ResourceVersion(5));
  });

  test(
    'the counters run from the snapshot until the next server value',
    () async {
      await w.connect(
        snapshot: snapshotJson(
          unread: [
            {
              'thread_id': threadId(1),
              'channel_id': channelId(1),
              'unread_count': 2,
              'mention_count': 1,
              'last_message_seq': 5,
              'last_read_seq': 3,
            },
          ],
        ),
      );
      w.rest.routes['GET /me/read-states'] = (_) =>
          const FakeReply(200, {'items': <Object?>[]});
      await w.idle();
      expect(
        w.catchUp.countsOf(t1),
        const ThreadCounts(unread: 2, mentions: 1),
      );
      await w.push('catchup.complete', {'head_seq': 40});
      await w.dispatch([
        messageCreated(41, 6, mentions: [alexId]),
      ]);
      expect(
        w.catchUp.countsOf(t1),
        const ThreadCounts(unread: 3, mentions: 2),
      );
      await w.dispatch([messageCreated(42, 7, author: alexId)]);
      expect(
        w.catchUp.countsOf(t1).unread,
        3,
        reason: 'own messages never count',
      );
      await w.dispatch([
        readStateUpdated(
          43,
          readStateJson(1, lastRead: 7, version: 2, device: otherDevice),
        ),
      ]);
      expect(w.catchUp.countsOf(t1), ThreadCounts.zero);
    },
  );
}

/// This device as text.
String get deviceIdOfThis => thisDevice.value;
