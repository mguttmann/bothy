// SPDX-License-Identifier: Apache-2.0

// Builders of spaces, channels, threads, runs and read states for the sidebar and post list
// tests (white label: Alex, Sam, Robin and `example.com`).

import 'package:domain/domain.dart';

/// A UUIDv7 whose last group is [n] in hex.
String uuid(int n) =>
    '0193f2a7-0000-7000-8000-${n.toRadixString(16).padLeft(12, '0')}';

/// Alex, the person of the tests.
final PrincipalId alex = PrincipalId.parse(uuid(0xa1));

/// Sam, another person.
final PrincipalId sam = PrincipalId.parse(uuid(0xa2));

/// Robin, an agent.
final PrincipalId robin = PrincipalId.parse(uuid(0xa3));

/// A fixed time of the tests.
final DateTime t0 = DateTime.utc(2026, 10, 17, 9);

/// Space [n] with the global key [key].
Space space(int n, {String? key, String? name, String? icon, String? color}) =>
    Space(
      id: SpaceId.parse(uuid(0x100 + n)),
      name: name ?? 'Space $n',
      icon: icon,
      color: color,
      sortKey: key ?? 'a$n',
      createdAt: t0,
      version: const ResourceVersion(1),
    );

/// The ID of space [n].
SpaceId spaceId(int n) => SpaceId.parse(uuid(0x100 + n));

/// The ID of channel [n].
ChannelId channelId(int n) => ChannelId.parse(uuid(0x200 + n));

/// The ID of thread [n].
ThreadId threadId(int n) => ThreadId.parse(uuid(0x300 + n));

/// Channel [n] of space [spaceN].
Channel channel(
  int n, {
  int spaceN = 1,
  ChannelKind kind = ChannelKind.text,
  String? name,
  String? key,
  ForumSort? sort,
  PrincipalId? operator,
}) => Channel(
  id: channelId(n),
  spaceId: spaceId(spaceN),
  kind: kind,
  name: name ?? 'channel-$n',
  topic: '',
  sortKey: key ?? 'a$n',
  operator: ChannelOperator(
    mode: OperatorMode.inherit,
    effectiveAgentId: operator,
  ),
  forumDefaultSort: sort,
  readOnly: false,
  mainThreadId: kind == ChannelKind.text ? threadId(0x80 + n) : null,
  dmParticipantIds: const [],
  createdAt: t0,
  version: const ResourceVersion(1),
  settingsVersion: const ResourceVersion(1),
);

/// Thread [n] of channel [channelN], last active [activeMinutes] after [t0].
Thread thread(
  int n, {
  int channelN = 1,
  String? title,
  ThreadState state = ThreadState.open,
  int activeMinutes = 0,
  int createdMinutes = 0,
  List<ForumTagId> tags = const [],
  int lastSeq = 0,
  PrincipalId? operator,
}) => Thread(
  id: threadId(n),
  channelId: channelId(channelN),
  kind: ThreadKind.thread,
  title: title ?? 'Post $n',
  titleSource: TitleSource.manual,
  state: state,
  operatorAgentId: operator,
  lastMessageSeq: ThreadSeq(lastSeq),
  unreadFloorSeq: const ThreadSeq(0),
  lastActivityAt: t0.add(Duration(minutes: activeMinutes)),
  messageCount: lastSeq,
  createdBy: sam,
  createdAt: t0.add(Duration(minutes: createdMinutes)),
  version: const ResourceVersion(1),
  tagIds: tags,
);

/// The ID of forum tag [n].
ForumTagId tagId(int n) => ForumTagId.parse(uuid(0x600 + n));

/// Forum tag [n] of channel [channelN].
ForumTag tag(int n, String name, {int channelN = 1, String? color}) => ForumTag(
  id: tagId(n),
  channelId: channelId(channelN),
  name: name,
  color: color,
  sortKey: 'a$n',
  restricted: false,
);

/// Run [n] of thread [threadN] in channel [channelN].
RunSummary run(
  int n,
  RunState state, {
  int threadN = 1,
  int channelN = 1,
  PrincipalId? triggeredBy,
  String? reason,
}) => RunSummary(
  id: RunId.parse(uuid(0x700 + n)),
  threadId: threadId(threadN),
  channelId: channelId(channelN),
  agentId: robin,
  triggeredBy: triggeredBy ?? alex,
  state: state,
  stateReason: reason,
  version: const ResourceVersion(1),
  billingModeExpected: BillingMode.organization,
  cost: const Money(amountMicros: 0, currency: 'USD', isEstimate: true),
  stepCount: 0,
  queuedAt: t0.add(Duration(minutes: n)),
);

/// The read state of thread [threadN] in channel [channelN].
ReadState readState(
  int threadN, {
  int channelN = 1,
  int lastRead = 0,
  int mentions = 0,
  int? unread,
}) => ReadState(
  threadId: threadId(threadN),
  channelId: channelId(channelN),
  lastReadSeq: ThreadSeq(lastRead),
  mentionCount: mentions,
  unreadCount: unread,
  version: const ResourceVersion(1),
);
