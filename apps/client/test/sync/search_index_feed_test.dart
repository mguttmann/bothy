// SPDX-License-Identifier: Apache-2.0

// The transcript feed of the offline search (14 6; brief M4-23): a done transcript of a voice
// file that a stored message attaches is fetched once and indexed with the location of the
// message; a failed fetch is retried on the next change of the database; the row of a deleted
// voice message or a dropped attachment goes (04 2.2 item 5).

import 'dart:async';

import 'package:client/sync/search_index_feed.dart';
import 'package:domain/domain.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';

import '../support/structure.dart';
import '../support/thread_fixtures.dart';

/// Voice file [n] of channel [channelN] with the transcript status [status].
FileMeta voice(
  int n, {
  int channelN = 1,
  TranscriptStatus status = TranscriptStatus.done,
}) => FileMeta(
  id: FileId.parse(uuid(0x500 + n)),
  sizeBytes: 2048,
  mimeType: 'audio/ogg',
  originalName: 'voice-$n.ogg',
  uploadedBy: sam,
  purpose: FilePurpose.attachment,
  scopeChannelId: channelId(channelN),
  uploadState: UploadState.complete,
  scanStatus: ScanStatus.clean,
  transcriptStatus: status,
  thumbnails: const [],
  createdAt: t0,
  durationMs: 4000,
);

/// A transcript source that records each request and answers from [answer].
final class FakeTranscripts {
  /// The texts by file; a missing file answers null.
  final Map<FileId, String> texts = {};

  /// Requests per file.
  final Map<FileId, int> attempts = {};

  /// Files whose next request fails once.
  final Set<FileId> failOnce = {};

  /// When set, every request waits for it.
  Completer<void>? gate;

  final StreamController<FileId> _requests = StreamController.broadcast();

  /// Each request as it starts.
  Stream<FileId> get requests => _requests.stream;

  /// The source the feed calls.
  Future<String?> answer(FileId file) async {
    attempts[file] = (attempts[file] ?? 0) + 1;
    _requests.add(file);
    await gate?.future;
    if (failOnce.remove(file)) throw StateError('offline');
    return texts[file];
  }
}

void main() {
  late LocalDatabase db;
  late FakeTranscripts transcripts;
  late SearchIndexFeed feed;

  setUp(() {
    db = openInMemoryForTests();
    transcripts = FakeTranscripts();
    feed = SearchIndexFeed(database: db, fetchTranscript: transcripts.answer);
    addTearDown(() async {
      await feed.stop();
      await db.close();
    });
  });

  /// Stores space 1, channel [c] of it, thread [c] and message [seq] attaching [file].
  Future<void> attach(FileMeta file, {int c = 1, int seq = 1}) async {
    await db.structureDao.upsertSpaceIfNewer(space(1));
    await db.structureDao.upsertChannelIfNewer(channel(c));
    await db.threadsDao.upsertIfNewer(thread(c, channelN: c));
    await db.messagesDao.upsertIfNewer(
      msg(seq, threadN: c, channelN: c, kind: MessageKind.voice, text: ''),
    );
    await db.messagesDao.putFile(file);
    await db.messagesDao.putAttachments(messageId(seq, threadN: c), [
      MessageAttachment(
        messageId: messageId(seq, threadN: c),
        fileId: file.id,
        position: 0,
        threadId: threadId(c),
        channelId: channelId(c),
      ),
    ]);
  }

  /// Completes once the transcript row of [file] exists ([present]) or is gone.
  Future<void> indexed(FileMeta file, {bool present = true}) async {
    await db
        .customSelect(
          // The ID is a fixed UUID of the test data, so it can stand in the text.
          'SELECT count(*) AS n FROM search_index '
          "WHERE kind = 'transcript' AND target_id = '${file.id.value}'",
          readsFrom: {db.searchIndex},
        )
        .watchSingle()
        .firstWhere((row) => (row.read<int>('n') > 0) == present);
  }

  test('a done transcript is fetched once and found offline with the location of its '
      'message', () async {
    transcripts.texts[voice(1).id] = 'Robin summarised the quarterly budget';
    await attach(voice(1));
    feed.start();

    await indexed(voice(1));
    final hits = await db.searchDao.search('budget', limit: 10);
    expect(hits, hasLength(1));
    expect(hits.single.kind, SearchKind.transcript);
    expect(hits.single.targetId, voice(1).id.value);
    expect(hits.single.threadId, threadId(1));
    expect(hits.single.channelId, channelId(1));
    expect(hits.single.spaceId, spaceId(1));

    // Later changes of the database do not fetch the indexed text again.
    transcripts.texts[voice(2).id] = 'Sam asked about the roadmap';
    await attach(voice(2), c: 2);
    await indexed(voice(2));
    expect(transcripts.attempts, {voice(1).id: 1, voice(2).id: 1});
  });

  test('a transcript that is not done yet is fetched when file.updated reports done', () async {
    transcripts.texts[voice(1).id] = 'Alex will send the invoice';
    await attach(voice(1, status: TranscriptStatus.pending));
    feed.start();
    await pumpEventQueue();
    expect(transcripts.attempts, isEmpty);

    await db.messagesDao.putFile(voice(1));
    await indexed(voice(1));
    expect(transcripts.attempts, {voice(1).id: 1});
  });

  test('a failed fetch is retried on the next change, not in a loop', () async {
    transcripts.texts[voice(1).id] = 'Robin needs the release notes';
    transcripts.failOnce.add(voice(1).id);
    await attach(voice(1));
    final first = transcripts.requests.first;
    feed.start();
    await first;
    await pumpEventQueue();
    expect(transcripts.attempts, {voice(1).id: 1});

    // Any change of the watched tables, here a further message, retries.
    await db.messagesDao.upsertIfNewer(msg(2, text: 'Hello from Alex'));
    await indexed(voice(1));
    expect(transcripts.attempts, {voice(1).id: 2});
  });

  test('a transcript without text is fetched once', () async {
    await attach(voice(1));
    transcripts.texts[voice(2).id] = 'Sam booked the venue';
    feed.start();
    await transcripts.requests.firstWhere((f) => f == voice(1).id);

    // The new pending file comes after voice 1 in the same pass, so its row proves the pass
    // that skipped voice 1 is over.
    await attach(voice(2), c: 2);
    await indexed(voice(2));
    expect(transcripts.attempts, {voice(1).id: 1, voice(2).id: 1});
  });

  test('the row of a deleted voice message goes, so the offline search no longer finds it', () async {
    transcripts.texts[voice(1).id] = 'Alex shared the secret launch date';
    await attach(voice(1));
    feed.start();
    await indexed(voice(1));
    expect(await db.searchDao.search('launch', limit: 10), hasLength(1));

    await db.messagesDao.tombstone(
      messageId(1),
      deletedAt: t0.add(const Duration(hours: 1)),
      version: const ResourceVersion(2),
    );

    await indexed(voice(1), present: false);
    expect(await db.searchDao.search('launch', limit: 10), isEmpty);
  });

  test('the row goes when an edit drops the attachment', () async {
    transcripts.texts[voice(1).id] = 'Robin drafted the contract';
    await attach(voice(1));
    feed.start();
    await indexed(voice(1));

    await db.messagesDao.putAttachments(messageId(1), const []);

    await indexed(voice(1), present: false);
    expect(await db.searchDao.search('contract', limit: 10), isEmpty);
    expect(transcripts.attempts, {voice(1).id: 1});
  });

  test('a fetch that ends after stop writes nothing', () async {
    transcripts.texts[voice(1).id] = 'Sam will call back';
    transcripts.gate = Completer<void>();
    await attach(voice(1));
    final first = transcripts.requests.first;
    feed.start();
    await first;

    await feed.stop();
    transcripts.gate!.complete();
    await pumpEventQueue();

    expect(await db.searchDao.search('call', limit: 10), isEmpty);
    await db.messagesDao.upsertIfNewer(msg(2));
    await pumpEventQueue();
    expect(transcripts.attempts, {voice(1).id: 1});
  });
}
