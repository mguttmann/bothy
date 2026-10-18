// SPDX-License-Identifier: Apache-2.0

// Messages, people and a stored thread for the thread view tests (white label: Alex, Sam,
// Robin and `example.com`).

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';

import 'structure.dart';

/// The ID of message [seq] of thread [threadN].
MessageId messageId(int seq, {int threadN = 1}) =>
    MessageId.parse(uuid(0x10000 * (threadN + 1) + seq));

/// Message [seq] of thread [threadN], written [minutes] after [t0] by [author] (Sam).
Message msg(
  int seq, {
  int threadN = 1,
  int channelN = 1,
  PrincipalId? author,
  String? text,
  num minutes = 0,
  MessageKind kind = MessageKind.text,
  MessageOrigin origin = MessageOrigin.human,
  SystemCode? systemCode,
  bool edited = false,
  bool deleted = false,
  bool pinned = false,
  List<PrincipalId> mentions = const [],
  MessageId? replyTo,
  int version = 1,
}) => Message(
  id: messageId(seq, threadN: threadN),
  threadId: threadId(threadN),
  channelId: channelId(channelN),
  threadSeq: ThreadSeq(seq),
  authorId: author ?? sam,
  clientMsgId: messageId(seq, threadN: threadN),
  origin: origin,
  kind: kind,
  systemCode: systemCode,
  contentMd: deleted ? '' : text ?? 'Message $seq',
  contentFormat: 1,
  contentTruncated: false,
  silent: false,
  mentionIds: mentions,
  pinned: pinned,
  replyToMessageId: replyTo,
  version: ResourceVersion(version),
  createdAt: t0.add(Duration(seconds: (minutes * 60).round())),
  editedAt: edited ? t0.add(const Duration(hours: 1)) : null,
  deletedAt: deleted ? t0.add(const Duration(hours: 1)) : null,
);

/// Alex, Sam and the agent Robin as stored principals.
final List<Principal> people = [
  Principal(
    id: alex,
    kind: PrincipalKind.user,
    handle: 'alex',
    displayName: 'Alex',
  ),
  Principal(
    id: sam,
    kind: PrincipalKind.user,
    handle: 'sam',
    displayName: 'Sam',
  ),
  Principal(
    id: robin,
    kind: PrincipalKind.agent,
    handle: 'robin',
    displayName: 'Robin',
  ),
];

/// The limits of the test instance: [maxBytes] per message.
InstanceInfo instanceWith({int maxBytes = 1000}) => InstanceInfo(
  instanceId: 'instance-1',
  branding: const Branding(
    productName: 'ExampleChat',
    version: ResourceVersion(1),
  ),
  localeDefault: 'en',
  specVersion: '1',
  features: const {},
  limits: InstanceLimits(
    messageMaxContentBytes: maxBytes,
    messageMaxAttachments: 10,
    fileMaxUploadBytes: 1000000,
    threadTitleMaxChars: 200,
    voiceMaxDurationMs: 60000,
  ),
);

/// Stores [ch] (default: forum channel 1), [th] (default: thread 1 of it with the last
/// position of [messages]), [messages] with their positions as one loaded range, the people,
/// the instance and the caller's [level] and [capabilities] in the channel.
Future<void> seedThread(
  LocalDatabase db, {
  Channel? ch,
  Thread? th,
  List<Message> messages = const [],
  AccessLevel level = AccessLevel.write,
  List<Capability> capabilities = const [],
  int maxBytes = 1000,
  bool loaded = true,
}) async {
  final c = ch ?? channel(1, kind: ChannelKind.forum, name: 'ideas');
  final last = messages.isEmpty ? 0 : messages.last.threadSeq.value;
  final t = th ?? thread(1, title: 'Printer jam', lastSeq: last);
  await db.structureDao.upsertSpaceIfNewer(space(1));
  await db.structureDao.upsertChannelIfNewer(c);
  await db.structureDao.putChannelAccess(
    c.id,
    AccessView(level: level, capabilities: capabilities),
  );
  await db.threadsDao.upsertIfNewer(t);
  for (final p in people) {
    await db.principalsDao.put(p);
  }
  await db.instanceDao.put(instanceWith(maxBytes: maxBytes));
  for (final m in messages) {
    await db.messagesDao.upsertIfNewer(m);
  }
  if (loaded && messages.isNotEmpty) {
    await db.messagesDao.addLoadedRange(
      t.id,
      messages.first.threadSeq,
      messages.last.threadSeq,
    );
  }
}
