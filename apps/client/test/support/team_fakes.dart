// SPDX-License-Identifier: Apache-2.0

// A scripted team API and team fixtures for the team channel tests (09 3.9).

import 'package:client/team/team_port.dart';
import 'package:domain/domain.dart';

import 'structure.dart';

/// The agent Nova (a second agent beside Robin).
final PrincipalId nova = PrincipalId.parse(uuid(0xa4));

/// Member [n] with [handle], [agent] (Robin) working in channel [homeN] with work thread
/// [threadN].
TeamMemberView member(
  int n,
  String handle, {
  PrincipalId? agent,
  int homeN = 2,
  int threadN = 2,
}) => TeamMemberView(
  id: uuid(0xe00 + n),
  handle: handle,
  agentId: agent ?? robin,
  homeChannelId: channelId(homeN),
  workThreadId: threadId(threadN),
);

/// The team of channel [channelN] with [members], sponsored by Alex.
TeamView team({
  int channelN = 1,
  List<TeamMemberView> members = const [],
  TeamPause? pause,
}) => TeamView(
  channelId: channelId(channelN),
  sponsorId: alex,
  memberTurns: 0,
  turnBudget: 200,
  pause: pause,
  members: members,
);

/// A [TeamPort] that records its calls and answers from [current]; a failure in [failures]
/// is thrown by the next call instead.
final class FakeTeamPort implements TeamPort {
  /// Creates the port with the team [current] (null: no team channel).
  FakeTeamPort([this.current]);

  /// The team the server holds, or null.
  TeamView? current;

  /// The calls in order, for example `add coding key-1`.
  final List<String> calls = [];

  /// The failure every read throws, or null; reads never take one of [failures].
  TeamFailure? readFailure;

  /// Failures thrown by the next changing calls, in order.
  final List<TeamFailure> failures = [];

  /// The runs [stopAll] answers.
  List<String> stopped = const ['run-1'];

  /// The channel [createTextChannel] answers.
  ChannelId created = channelId(7);

  /// The failure the next [createTextChannel] throws, or null; it never takes one of
  /// [failures].
  TeamFailure? createFailure;

  /// The channels [enable] was called for, in order.
  final List<ChannelId> enabled = [];

  void _fail() {
    if (failures.isNotEmpty) throw failures.removeAt(0);
  }

  @override
  Future<TeamView?> read(ChannelId channel) async {
    calls.add('read');
    final failure = readFailure;
    if (failure != null) throw failure;
    return current;
  }

  @override
  Future<TeamView> enable(ChannelId channel) async {
    calls.add('enable');
    enabled.add(channel);
    _fail();
    return current ??= team();
  }

  @override
  Future<ChannelId> createTextChannel(
    SpaceId space,
    String name, {
    required String idempotencyKey,
  }) async {
    calls.add('create ${space.value} $name $idempotencyKey');
    final failure = createFailure;
    createFailure = null;
    if (failure != null) throw failure;
    return created;
  }

  @override
  Future<void> disable(ChannelId channel) async {
    calls.add('disable');
    _fail();
    current = null;
  }

  @override
  Future<TeamMemberView> add(
    ChannelId channel, {
    required PrincipalId agent,
    required ChannelId homeChannel,
    required String handle,
    required String idempotencyKey,
  }) async {
    calls.add('add $handle $idempotencyKey');
    _fail();
    final m = TeamMemberView(
      id: 'member-$handle',
      handle: handle,
      agentId: agent,
      homeChannelId: homeChannel,
      workThreadId: threadId(9),
    );
    current = current?.withMember(m);
    return m;
  }

  @override
  Future<void> remove(ChannelId channel, String memberId) async {
    calls.add('remove $memberId');
    _fail();
    current = current?.without(memberId);
  }

  @override
  Future<List<String>> stopAll(
    ChannelId channel, {
    required String idempotencyKey,
  }) async {
    calls.add('stop $idempotencyKey');
    _fail();
    current = current?.paused(TeamPause.stopped);
    return stopped;
  }

  /// The agents [agents] answers; null fails the read, so the dialog offers the stored agents.
  List<TeamAgent>? agentList;

  @override
  Future<List<TeamAgent>> agents() async =>
      agentList ?? (throw const TeamFailure.offline());
}
