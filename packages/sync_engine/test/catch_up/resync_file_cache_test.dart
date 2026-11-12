// SPDX-License-Identifier: Apache-2.0

// A snapshot that replaces the mirror names what the person sees now (04 7.3) and brings no
// `access.changed` for a channel lost in the meantime: the cached files of a stored channel the
// snapshot no longer lists go with the mirror and their bytes after the commit (04 8.3 item 3),
// the cached files of the channels it lists stay, and so do the drafts (04 7.3). Both ways to a
// whole snapshot are covered: `resync_required` and a `ready` without a stored cursor.

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late World w;
  setUp(() => w = World());
  tearDown(() => w.dispose());

  FileId file(int n) => FileId.parse(id(0x7000 + n));
  String cached(int n) => '/cache/files/$n/original/report-$n.pdf';

  /// Stores file [n] with the scope channel [channelN], attached to message 1 of thread
  /// [channelN], and a cached original of it.
  Future<void> cacheFileOf(int n, {required int channelN}) async {
    final dao = w.db.messagesDao;
    await dao.putFile(
      FileMeta(
        id: file(n),
        sizeBytes: 10,
        mimeType: 'application/pdf',
        originalName: 'report-$n.pdf',
        uploadedBy: PrincipalId.parse(samId),
        purpose: FilePurpose.attachment,
        scopeChannelId: ChannelId.parse(channelId(channelN)),
        uploadState: UploadState.complete,
        scanStatus: ScanStatus.clean,
        transcriptStatus: TranscriptStatus.none,
        thumbnails: const [],
        createdAt: DateTime.utc(2026, 10, 17),
      ),
    );
    await dao.putAttachments(MessageId.parse(messageId(channelN, 1)), [
      MessageAttachment(
        messageId: MessageId.parse(messageId(channelN, 1)),
        fileId: file(n),
        position: 1,
        threadId: ThreadId.parse(threadId(channelN)),
        channelId: ChannelId.parse(channelId(channelN)),
      ),
    ]);
    await w.db.fileCacheDao.put(
      FileCacheEntry(
        fileId: file(n),
        variant: 'original',
        path: cached(n),
        sizeBytes: 10,
        lastAccessedAt: DateTime.utc(2026, 10, 17),
      ),
    );
  }

  Future<void> draftIn(int channelN) => w.db.personalDao.upsertDraftIfNewer(
    Draft(
      id: DraftId.parse(draftId(channelN)),
      threadId: ThreadId.parse(threadId(channelN)),
      channelId: ChannelId.parse(channelId(channelN)),
      contentMd: 'half a sentence',
      fileIds: const [],
      version: const ResourceVersion(2),
    ),
  );

  /// The device holds channel 1 and channel 2, each with a thread, a message, a cached file
  /// and a draft.
  Future<void> holdBoth() async {
    await w.dispatch([
      threadCreated(31, 1),
      messageCreated(32, 1),
      threadCreated(33, 2, channelN: 2),
      messageCreated(34, 1, threadN: 2, channelN: 2),
    ]);
    await cacheFileOf(1, channelN: 1);
    await cacheFileOf(2, channelN: 2);
    await draftIn(1);
    await draftIn(2);
  }

  Future<void> expectOnlyChannelTwoLeft() async {
    expect(w.eraser.erased, [cached(1)]);
    expect(await w.db.fileCacheDao.entry(file(1), 'original'), isNull);
    expect(await w.db.fileCacheDao.entry(file(2), 'original'), isNotNull);
    expect(
      await w.db.personalDao.allDrafts(),
      hasLength(2),
      reason: 'a resync keeps the drafts (04 7.3)',
    );
  }

  test('resync_required: the cached file of a channel the snapshot no longer '
      'lists is erased, the other stays', () async {
    await w.connect(cursor: 30);
    await holdBoth();
    await w.push('cursor', {'seq': 34});
    // Alex lost channel 1 while the device was too far behind: no access.changed arrives.
    w.rest.routes['GET /sync/snapshot'] = (_) => FakeReply(
      200,
      snapshotJson(headSeq: 90, epoch: 'epoch-2', channels: [channelView(2)]),
    );

    await w.push('resync_required', {
      'reason': 'too_far_behind',
      'head_seq': 90,
      'sync_epoch': 'epoch-2',
    });

    expect(w.notices.whereType<Resynced>(), hasLength(1));
    expect(
      await w.db.structureDao.channelById(ChannelId.parse(channelId(1))),
      isNull,
    );
    await expectOnlyChannelTwoLeft();
  });

  test('a ready without a stored cursor: the same rule for the mirror it '
      'replaces', () async {
    // The device holds data of an earlier session but no cursor (a migration step cleared
    // it, 15 6.2), so the snapshot of `ready` replaces the mirror.
    await cacheFileOf(1, channelN: 1);
    await cacheFileOf(2, channelN: 2);
    await draftIn(1);
    await draftIn(2);

    await w.connect(
      snapshot: snapshotJson(headSeq: 90, channels: [channelView(2)]),
      headSeq: 90,
    );

    expect(await w.cursor(), 90);
    await expectOnlyChannelTwoLeft();
  });

  test('a resync that still lists every channel erases nothing', () async {
    await w.connect(cursor: 30);
    await holdBoth();
    await w.push('cursor', {'seq': 34});
    w.rest.routes['GET /sync/snapshot'] = (_) =>
        FakeReply(200, snapshotJson(headSeq: 90, epoch: 'epoch-2'));

    await w.push('resync_required', {
      'reason': 'retention',
      'head_seq': 90,
      'sync_epoch': 'epoch-2',
    });

    expect(w.eraser.erased, isEmpty);
    expect(await w.db.fileCacheDao.entry(file(1), 'original'), isNotNull);
    expect(await w.db.fileCacheDao.entry(file(2), 'original'), isNotNull);
  });
}
