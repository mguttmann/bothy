// SPDX-License-Identifier: Apache-2.0

/// Test data: fixed IDs, a fake clock and entity builders with the people Alex, Sam and Robin.
library;

import 'dart:io';
import 'dart:typed_data';

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';

/// A UUID whose last group is [n] in hex.
String uuid(int n) =>
    '0192f4a0-7b1c-78f9-8074-${n.toRadixString(16).padLeft(12, '0')}';

/// Alex, a person.
final PrincipalId alex = PrincipalId.parse(uuid(0xa1));

/// Sam, a person.
final PrincipalId sam = PrincipalId.parse(uuid(0xa2));

/// Robin, an agent.
final PrincipalId robin = PrincipalId.parse(uuid(0xa3));

/// A UTC time [minutes] after a fixed start.
DateTime at(int minutes) =>
    DateTime.utc(2026, 10, 1, 9).add(Duration(minutes: minutes));

/// A clock that tests move by hand.
final class FakeClock {
  /// Starts at [at] of 0.
  DateTime now = at(0);

  /// Moves the clock by [d].
  void advance(Duration d) => now = now.add(d);

  /// The time source for the database.
  DateTime call() => now;
}

/// The fixed test key: 32 bytes `0x11`. Not a secret; it only opens test databases.
final Uint8List testKeyA = Uint8List.fromList(List.filled(32, 0x11));

/// A second fixed test key: 32 bytes `0x22`.
final Uint8List testKeyB = Uint8List.fromList(List.filled(32, 0x22));

/// A key source that returns a copy of [key].
final class FixedKeySource implements DatabaseKeySource {
  /// Creates the source.
  FixedKeySource(this.key);

  /// The key.
  final Uint8List key;

  @override
  Future<Uint8List> obtainKey() async => Uint8List.fromList(key);
}

/// A temporary directory that [Directory.deleteSync] removes after the test.
Future<Directory> tempDir() =>
    Directory.systemTemp.createTemp('local_store_test_');

/// A space.
Space space(int n, {int version = 1}) => Space(
  id: SpaceId.parse(uuid(0x100 + n)),
  name: 'Space $n',
  sortKey: 'a$n',
  createdAt: at(0),
  version: ResourceVersion(version),
);

/// A text channel of [spaceN].
Channel channel(int n, {int spaceN = 1, int version = 1}) => Channel(
  id: ChannelId.parse(uuid(0x200 + n)),
  spaceId: SpaceId.parse(uuid(0x100 + spaceN)),
  kind: ChannelKind.text,
  name: 'channel-$n',
  topic: '',
  sortKey: 'c$n',
  operator: ChannelOperator(
    mode: OperatorMode.agent,
    agentId: robin,
    effectiveAgentId: robin,
  ),
  readOnly: false,
  dmParticipantIds: const [],
  createdAt: at(0),
  version: ResourceVersion(version),
  settingsVersion: const ResourceVersion(1),
);

/// The ID of thread [n].
ThreadId threadId(int n) => ThreadId.parse(uuid(0x300 + n));

/// A thread of [channelN].
Thread thread(
  int n, {
  int channelN = 1,
  int version = 1,
  int lastSeq = 0,
  String title = 'Thread',
}) => Thread(
  id: threadId(n),
  channelId: ChannelId.parse(uuid(0x200 + channelN)),
  kind: ThreadKind.thread,
  title: title,
  titleSource: TitleSource.manual,
  state: ThreadState.open,
  lastMessageSeq: ThreadSeq(lastSeq),
  unreadFloorSeq: const ThreadSeq(0),
  lastActivityAt: at(1),
  messageCount: lastSeq,
  createdBy: alex,
  createdAt: at(0),
  version: ResourceVersion(version),
  tagIds: const [],
);

/// The ID of message [seq] of thread [threadN].
MessageId messageId(int threadN, int seq) =>
    MessageId.parse(uuid(0x10000 * threadN + seq));

/// Message [seq] of thread [threadN] in [channelN].
Message message(
  int threadN,
  int seq, {
  int channelN = 1,
  int version = 1,
  String text = 'Hello from Alex',
}) => Message(
  id: messageId(threadN, seq),
  threadId: threadId(threadN),
  channelId: ChannelId.parse(uuid(0x200 + channelN)),
  threadSeq: ThreadSeq(seq),
  authorId: alex,
  clientMsgId: messageId(threadN, seq),
  origin: MessageOrigin.human,
  kind: MessageKind.text,
  contentMd: text,
  contentFormat: 1,
  contentTruncated: false,
  silent: false,
  mentionIds: [sam],
  pinned: false,
  version: ResourceVersion(version),
  createdAt: at(seq),
);

/// A draft in thread [threadN] of [channelN].
Draft draft(int n, {int? threadN = 1, int channelN = 1}) => Draft(
  id: DraftId.parse(uuid(0x400 + n)),
  threadId: threadN == null ? null : threadId(threadN),
  channelId: ChannelId.parse(uuid(0x200 + channelN)),
  contentMd: 'Draft $n by Sam',
  fileIds: const [],
  version: const ResourceVersion(1),
);

/// File metadata [n] in [channelN].
FileMeta file(int n, {int channelN = 1}) => FileMeta(
  id: FileId.parse(uuid(0x500 + n)),
  sizeBytes: 100,
  mimeType: 'image/png',
  originalName: 'photo-$n.png',
  uploadedBy: alex,
  purpose: FilePurpose.attachment,
  scopeChannelId: ChannelId.parse(uuid(0x200 + channelN)),
  uploadState: UploadState.complete,
  scanStatus: ScanStatus.clean,
  transcriptStatus: TranscriptStatus.none,
  thumbnails: const [
    Thumbnail(
      variant: ThumbnailVariant.small,
      mimeType: 'image/webp',
      width: 64,
      height: 48,
    ),
  ],
  createdAt: at(0),
);
