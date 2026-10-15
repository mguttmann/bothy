// SPDX-License-Identifier: Apache-2.0

import 'dart:io';

import 'package:domain/domain.dart';

/// The repository root: the first directory upwards that holds `docs/spec`.
Directory repoRoot() {
  var dir = Directory.current.absolute;
  while (!Directory('${dir.path}/docs/spec').existsSync()) {
    final parent = dir.parent;
    if (parent.path == dir.path) {
      throw StateError('repository root not found');
    }
    dir = parent;
  }
  return dir;
}

/// Reads a file relative to the repository root.
String readRepoFile(String path) =>
    File('${repoRoot().path}/$path').readAsStringSync();

/// A UUID whose last group ends in [n].
String uuid(int n) =>
    '0192f4a0-7b1c-7000-8000-${n.toRadixString(16).padLeft(12, '0')}';

final PrincipalId alex = PrincipalId.parse(uuid(1));
final PrincipalId sam = PrincipalId.parse(uuid(2));
final PrincipalId robin = PrincipalId.parse(uuid(3));
final ThreadId thread1 = ThreadId.parse(uuid(10));
final ChannelId channel1 = ChannelId.parse(uuid(20));
final DateTime t0 = DateTime.utc(2026, 10, 15, 12);

/// A message with sensible defaults.
Message message(
  int seq, {
  PrincipalId? author,
  List<PrincipalId> mentions = const [],
  MessageKind kind = MessageKind.text,
  DateTime? createdAt,
  DateTime? deletedAt,
  ThreadId? thread,
  int version = 1,
}) => Message(
  id: MessageId.parse(uuid(1000 + seq)),
  threadId: thread ?? thread1,
  channelId: channel1,
  threadSeq: ThreadSeq(seq),
  authorId: author ?? sam,
  clientMsgId: MessageId.parse(uuid(5000 + seq)),
  origin: MessageOrigin.human,
  kind: kind,
  contentMd: 'hello',
  contentFormat: 1,
  contentTruncated: false,
  silent: false,
  mentionIds: mentions,
  pinned: false,
  version: ResourceVersion(version),
  createdAt: createdAt ?? t0,
  deletedAt: deletedAt,
);

/// A thread with sensible defaults.
Thread thread(
  int n, {
  ThreadState state = ThreadState.open,
  DateTime? lastActivityAt,
  DateTime? createdAt,
}) => Thread(
  id: ThreadId.parse(uuid(100 + n)),
  channelId: channel1,
  kind: ThreadKind.thread,
  title: 'Thread $n',
  titleSource: TitleSource.manual,
  state: state,
  lastMessageSeq: const ThreadSeq(0),
  unreadFloorSeq: const ThreadSeq(0),
  lastActivityAt: lastActivityAt ?? t0,
  messageCount: 0,
  createdBy: alex,
  createdAt: createdAt ?? t0,
  version: const ResourceVersion(1),
  tagIds: const [],
);

/// A read state of [thread1].
ReadState readState(
  int lastRead, {
  int version = 1,
  int mentions = 0,
  int? unread,
}) => ReadState(
  threadId: thread1,
  channelId: channel1,
  lastReadSeq: ThreadSeq(lastRead),
  mentionCount: mentions,
  unreadCount: unread,
  version: ResourceVersion(version),
);
