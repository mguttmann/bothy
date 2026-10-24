// SPDX-License-Identifier: Apache-2.0

// The transcript rows of the offline search (14 6; brief M4-23): the voice files with a done
// transcript that a stored message attaches and that have no row yet, and the pruning of
// transcript rows whose file or attaching message is gone (04 2.2 item 5).

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:test/test.dart';

import 'support/fixtures.dart';

void main() {
  late LocalDatabase db;
  setUp(() => db = openInMemoryForTests());
  tearDown(() => db.close());

  /// Voice file [n] of [channelN] with the transcript status [status].
  FileMeta voice(
    int n, {
    int channelN = 1,
    TranscriptStatus status = TranscriptStatus.done,
  }) => FileMeta(
    id: FileId.parse(uuid(0x500 + n)),
    sizeBytes: 2048,
    mimeType: 'audio/ogg',
    originalName: 'voice-$n.ogg',
    uploadedBy: alex,
    purpose: FilePurpose.attachment,
    scopeChannelId: ChannelId.parse(uuid(0x200 + channelN)),
    uploadState: UploadState.complete,
    scanStatus: ScanStatus.clean,
    transcriptStatus: status,
    thumbnails: const [],
    createdAt: at(0),
    durationMs: 4000,
  );

  /// Stores space [s], channel [c] and thread [c] with message 1 attaching [file].
  Future<void> attach(
    FileMeta file, {
    int c = 1,
    int s = 1,
    int seq = 1,
  }) async {
    await db.structureDao.upsertSpaceIfNewer(space(s));
    await db.structureDao.upsertChannelIfNewer(channel(c, spaceN: s));
    await db.threadsDao.upsertIfNewer(thread(c, channelN: c));
    await db.messagesDao.upsertIfNewer(message(c, seq, channelN: c));
    await db.messagesDao.putFile(file);
    await db.messagesDao.putAttachments(messageId(c, seq), [
      MessageAttachment(
        messageId: messageId(c, seq),
        fileId: file.id,
        position: 0,
        threadId: threadId(c),
        channelId: channel(c).id,
      ),
    ]);
  }

  Future<int> transcriptRows() async =>
      (await db
              .customSelect(
                "SELECT count(*) AS n FROM search_index WHERE kind = 'transcript'",
              )
              .getSingle())
          .read<int>('n');

  test('a done transcript of an attached file is pending with its thread, channel and space', () async {
    await attach(voice(1), c: 1, s: 1);

    final pending = await db.searchDao.watchTranscriptsToIndex().first;
    expect(pending, [
      PendingTranscript(
        fileId: voice(1).id,
        threadId: threadId(1),
        channelId: channel(1).id,
        spaceId: space(1).id,
      ),
    ]);
  });

  test('a file without a done transcript, unattached or already indexed is not pending', () async {
    await attach(voice(1, status: TranscriptStatus.pending), c: 1);
    await db.messagesDao.putFile(voice(2, channelN: 1));
    await attach(voice(3, channelN: 3), c: 3, s: 3);
    await db.searchDao.indexTranscript(
      fileId: voice(3).id,
      threadId: threadId(3),
      channelId: channel(3).id,
      spaceId: space(3).id,
      text: 'Robin summarised the release plan',
    );

    expect(await db.searchDao.watchTranscriptsToIndex().first, isEmpty);
  });

  test('the pending list emits again when a transcript becomes done', () async {
    await attach(voice(1, status: TranscriptStatus.pending));
    final emissions = <List<FileId>>[];
    final sub = db.searchDao.watchTranscriptsToIndex().listen(
      (rows) => emissions.add([for (final r in rows) r.fileId]),
    );
    addTearDown(sub.cancel);
    await pumpEventQueue();
    await db.messagesDao.putFile(voice(1));
    await pumpEventQueue();
    expect(emissions, [
      <FileId>[],
      [voice(1).id],
    ]);
  });

  test('pruneTranscripts deletes the rows of deleted messages, dropped attachments and '
      'unknown files, and keeps the others', () async {
    Future<void> index(int n) => db.searchDao.indexTranscript(
      fileId: voice(n).id,
      threadId: threadId(n),
      channelId: channel(n).id,
      spaceId: space(n).id,
      text: 'Sam asked about the budget $n',
    );
    // 1 stays; the message of 2 is deleted; the edit of 3 drops the attachment; 4 has no
    // file row.
    for (final n in [1, 2, 3]) {
      await attach(
        voice(n, channelN: n),
        c: n,
        s: n,
      );
      await index(n);
    }
    await index(4);
    expect(await transcriptRows(), 4);

    await db.messagesDao.tombstone(
      messageId(2, 1),
      deletedAt: at(9),
      version: const ResourceVersion(2),
    );
    await db.messagesDao.putAttachments(messageId(3, 1), const []);

    expect(await db.searchDao.pruneTranscripts(), 3);
    expect(await transcriptRows(), 1);
    final hits = await db.searchDao.search('budget', limit: 10);
    expect([for (final h in hits) h.targetId], [voice(1).id.value]);
    expect(await db.searchDao.pruneTranscripts(), 0);
  });

  test('pruneTranscripts leaves message and thread title rows alone', () async {
    await attach(voice(1));
    await db.searchDao.indexMessage(message(1, 1), spaceId: space(1).id);
    await db.searchDao.indexThreadTitle(thread(1), spaceId: space(1).id);
    await db.messagesDao.putAttachments(messageId(1, 1), const []);

    expect(await db.searchDao.pruneTranscripts(), 0);
    final rows = await db
        .customSelect('SELECT count(*) AS n FROM search_index')
        .getSingle();
    expect(rows.read<int>('n'), 2);
  });
}
