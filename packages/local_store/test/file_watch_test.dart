// SPDX-License-Identifier: Apache-2.0

// The file reads M4-22 added: one file's metadata, the files of a message in their order and
// the files of a thread (the section "Files" of the session column), each as a stream that
// follows `file.updated` (12 2).

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:test/test.dart';

import 'support/fixtures.dart';

void main() {
  late LocalDatabase db;
  setUp(() => db = openInMemoryForTests());
  tearDown(() => db.close());

  MessageAttachment attach(int threadN, int seq, int fileN, int position) =>
      MessageAttachment(
        messageId: messageId(threadN, seq),
        fileId: file(fileN).id,
        position: position,
        threadId: threadId(threadN),
        channelId: ChannelId.parse(uuid(0x201)),
      );

  test('watchFile follows the stored metadata', () async {
    final dao = db.messagesDao;
    final id = file(1).id;
    final seen = <ScanStatus?>[];
    final sub = dao.watchFile(id).listen((f) => seen.add(f?.scanStatus));
    await pumpEventQueue();
    await dao.putFile(file(1).copyWith(scanStatus: ScanStatus.pending));
    await pumpEventQueue();
    await dao.putFile(file(1));
    await pumpEventQueue();
    await sub.cancel();
    expect(seen, [null, ScanStatus.pending, ScanStatus.clean]);
  });

  test('watchAttachedFiles lists the files of a message by position', () async {
    final dao = db.messagesDao;
    await dao.upsertIfNewer(message(1, 1));
    await dao.putFile(file(1));
    await dao.putFile(file(2));
    await dao.putAttachments(messageId(1, 1), [
      attach(1, 1, 2, 0),
      attach(1, 1, 1, 1),
    ]);
    final first = await dao.watchAttachedFiles(messageId(1, 1)).first;
    expect([for (final f in first) f.id], [file(2).id, file(1).id]);

    final later = dao.watchAttachedFiles(messageId(1, 1)).skip(1).first;
    await dao.putFile(file(1).copyWith(scanStatus: ScanStatus.infected));
    final next = await later;
    expect(next.last.scanStatus, ScanStatus.infected);
  });

  test('a file without its metadata row is left out', () async {
    final dao = db.messagesDao;
    await dao.upsertIfNewer(message(1, 1));
    await dao.putAttachments(messageId(1, 1), [attach(1, 1, 3, 0)]);
    expect(await dao.watchAttachedFiles(messageId(1, 1)).first, isEmpty);
  });

  test('watchThreadFiles lists the files of a thread, newest first', () async {
    final dao = db.messagesDao;
    await dao.upsertIfNewer(message(1, 1));
    await dao.upsertIfNewer(message(1, 2));
    await dao.upsertIfNewer(message(2, 1));
    for (final n in [1, 2, 3, 4]) {
      await dao.putFile(file(n));
    }
    await dao.putAttachments(messageId(1, 1), [
      attach(1, 1, 1, 0),
      attach(1, 1, 2, 1),
    ]);
    await dao.putAttachments(messageId(1, 2), [attach(1, 2, 3, 0)]);
    await dao.putAttachments(messageId(2, 1), [attach(2, 1, 4, 0)]);

    final files = await dao.watchThreadFiles(threadId(1)).first;
    expect(
      [for (final f in files) f.file.id],
      [file(3).id, file(1).id, file(2).id],
    );
    expect(files.first.messageId, messageId(1, 2));
    expect(files.first.threadSeq, const ThreadSeq(2));
    expect(files[1].position, 0);
  });

  test('the files of a deleted message leave the thread list', () async {
    final dao = db.messagesDao;
    await dao.upsertIfNewer(message(1, 1));
    await dao.putFile(file(1));
    await dao.putAttachments(messageId(1, 1), [attach(1, 1, 1, 0)]);
    expect(await dao.watchThreadFiles(threadId(1)).first, hasLength(1));
    await dao.putAttachments(messageId(1, 1), const []);
    expect(await dao.watchThreadFiles(threadId(1)).first, isEmpty);
  });
}
