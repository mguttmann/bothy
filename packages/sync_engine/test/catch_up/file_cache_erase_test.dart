// SPDX-License-Identifier: Apache-2.0

// A deleted message takes its cached file bytes along (brief M4-22; 20 4.7 "client caches:
// delete on message.deleted"; 04 2.2 rule 5): `message.deleted` and a redacted
// `message.updated` delete the file cache rows of the message's files and hand their paths to
// the `FileCacheEraser` after the commit.

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late World w;
  setUp(() => w = World());
  tearDown(() => w.dispose());

  final file = FileId.parse(id(0x7001));
  const cached = '/cache/files/0193/original/report.pdf';

  Future<void> attachCached() async {
    final dao = w.db.messagesDao;
    await dao.putFile(
      FileMeta(
        id: file,
        sizeBytes: 10,
        mimeType: 'application/pdf',
        originalName: 'report.pdf',
        uploadedBy: PrincipalId.parse(samId),
        purpose: FilePurpose.attachment,
        uploadState: UploadState.complete,
        scanStatus: ScanStatus.clean,
        transcriptStatus: TranscriptStatus.none,
        thumbnails: const [],
        createdAt: DateTime.utc(2026, 10, 17),
      ),
    );
    await dao.putAttachments(MessageId.parse(messageId(1, 1)), [
      MessageAttachment(
        messageId: MessageId.parse(messageId(1, 1)),
        fileId: file,
        position: 1,
        threadId: ThreadId.parse(threadId(1)),
        channelId: ChannelId.parse(channelId(1)),
      ),
    ]);
    await w.db.fileCacheDao.put(
      FileCacheEntry(
        fileId: file,
        variant: 'original',
        path: cached,
        sizeBytes: 10,
        lastAccessedAt: DateTime.utc(2026, 10, 17),
      ),
    );
  }

  Future<void> created() async {
    await w.connect(cursor: 30);
    await w.push('catchup.complete', {'head_seq': 30});
    await w.dispatch([threadCreated(31, 1), messageCreated(32, 1)]);
    await attachCached();
  }

  test('message.deleted erases the cached bytes of its files', () async {
    await created();
    await w.dispatch([
      envelope(
        'message.deleted',
        seq: 33,
        payload: {
          'message_id': messageId(1, 1),
          'thread_id': threadId(1),
          'thread_seq': 1,
          'deleted_at': '2026-10-17T10:00:00.000Z',
          'deleted_by': samId,
        },
        resourceKind: 'message',
        resourceId: messageId(1, 1),
        version: 2,
        scope: {'channel_id': channelId(1), 'thread_id': threadId(1)},
      ),
    ]);
    expect(w.eraser.erased, [cached]);
    expect(await w.db.fileCacheDao.entry(file, 'original'), isNull);
  });

  test('a redacted message.updated erases them too', () async {
    await created();
    await w.dispatch([messageUpdated(33, 1, version: 2, redacted: true)]);
    expect(w.eraser.erased, [cached]);
    expect(await w.db.fileCacheDao.entry(file, 'original'), isNull);
  });
}
