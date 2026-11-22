// SPDX-License-Identifier: Apache-2.0

/// The six owner-only operations of a team channel (09 3.9.1, 3.9.7) as the client's port:
/// `getChannelTeam`, `enableChannelTeam`, `disableChannelTeam`, `addTeamMember`,
/// `removeTeamMember` and `stopChannelTeam`, with the team as the app reads it ([TeamView]).
/// They are online operations, never outbox entries: each answers the team's new state or a
/// problem whose `reason` the dialog shows (ruling D-TEAM-APP-2). Tests replace the port.
library;

import 'package:api_client/api_client.dart' as wire;
import 'package:dio/dio.dart';
import 'package:domain/domain.dart';

/// Why member inputs of a team channel are paused (`TeamPauseReason`, 09 3.9.6, 3.9.7).
enum TeamPause {
  /// The turn budget since an owner's last message is used up.
  turnBudget,

  /// Two members triggered each other too often in a row.
  pingpong,

  /// An owner stopped the team.
  stopped,

  /// A reason of a newer server.
  unknown,
}

/// A member of a team channel (`TeamMember`).
final class TeamMemberView {
  /// Creates the member.
  const TeamMemberView({
    required this.id,
    required this.handle,
    required this.agentId,
    required this.homeChannelId,
    required this.workThreadId,
  });

  /// The member's ID.
  final String id;

  /// The handle unique in the team channel, without `@`.
  final String handle;

  /// The member's agent.
  final PrincipalId agentId;

  /// The channel the agent works in.
  final ChannelId homeChannelId;

  /// The member's work thread in its home channel, where its inputs and runs live.
  final ThreadId workThreadId;
}

/// The team of a team channel (`Team`).
final class TeamView {
  /// Creates the team.
  const TeamView({
    required this.channelId,
    required this.sponsorId,
    required this.memberTurns,
    required this.turnBudget,
    required this.members,
    this.pause,
  });

  /// The team channel.
  final ChannelId channelId;

  /// The owner on whose behalf member inputs run.
  final PrincipalId sponsorId;

  /// Member inputs since an owner's last message.
  final int memberTurns;

  /// The turn budget of the server now.
  final int turnBudget;

  /// Why member inputs are paused, or null while they flow.
  final TeamPause? pause;

  /// The members in the server's order.
  final List<TeamMemberView> members;

  /// The member with [handle] in any letter case, or null.
  TeamMemberView? byHandle(String handle) {
    final lower = handle.toLowerCase();
    for (final m in members) {
      if (m.handle.toLowerCase() == lower) return m;
    }
    return null;
  }

  /// A copy without the member [memberId].
  TeamView without(String memberId) => TeamView(
    channelId: channelId,
    sponsorId: sponsorId,
    memberTurns: memberTurns,
    turnBudget: turnBudget,
    pause: pause,
    members: [
      for (final m in members)
        if (m.id != memberId) m,
    ],
  );

  /// A copy with [member] added at the end.
  TeamView withMember(TeamMemberView member) => TeamView(
    channelId: channelId,
    sponsorId: sponsorId,
    memberTurns: memberTurns,
    turnBudget: turnBudget,
    pause: pause,
    members: [...members, member],
  );

  /// A copy paused for [reason].
  TeamView paused(TeamPause reason) => TeamView(
    channelId: channelId,
    sponsorId: sponsorId,
    memberTurns: memberTurns,
    turnBudget: turnBudget,
    pause: reason,
    members: members,
  );
}

/// A refused or failed team operation: the server's problem [code] and [reason], or
/// [offline] without an answer.
final class TeamFailure implements Exception {
  /// A problem of the server.
  const TeamFailure.problem(String this.code, {this.reason}) : offline = false;

  /// No answer (offline, timeout).
  const TeamFailure.offline() : code = null, reason = null, offline = true;

  /// The problem code, null when [offline].
  final String? code;

  /// The problem's `reason`, if any.
  final String? reason;

  /// Whether no answer arrived.
  final bool offline;

  @override
  String toString() =>
      offline ? 'TeamFailure(offline)' : 'TeamFailure($code, $reason)';
}

/// The team operations of the signed-in owner. Every method throws [TeamFailure] when the
/// server refuses or does not answer.
abstract interface class TeamPort {
  /// The team of [channel], or null when the channel is no team channel (`404`).
  Future<TeamView?> read(ChannelId channel);

  /// Switches [channel] to team mode with the caller as sponsor.
  Future<TeamView> enable(ChannelId channel);

  /// Switches team mode of [channel] off; the work threads stay.
  Future<void> disable(ChannelId channel);

  /// Adds [agent] working in [homeChannel] as [handle]; [idempotencyKey] makes a repeated
  /// press after a lost answer the same request.
  Future<TeamMemberView> add(
    ChannelId channel, {
    required PrincipalId agent,
    required ChannelId homeChannel,
    required String handle,
    required String idempotencyKey,
  });

  /// Removes the member [memberId]; its work thread stays and a running run keeps running.
  Future<void> remove(ChannelId channel, String memberId);

  /// Stops every active run of the members and pauses member inputs; answers the runs it
  /// stopped.
  Future<List<String>> stopAll(
    ChannelId channel, {
    required String idempotencyKey,
  });
}

/// [TeamPort] over the generated client of the signed-in session.
final class ApiTeamPort implements TeamPort {
  /// Creates the port over [_api].
  ApiTeamPort(this._api);

  final wire.ApiClient _api;

  wire.ChannelsApi get _channels => _api.getChannelsApi();

  @override
  Future<TeamView?> read(ChannelId channel) async {
    try {
      final team = (await _channels.getChannelTeam(channelId: channel.value))
          .data;
      return team == null ? null : teamOfWire(team);
    } on DioException catch (e) {
      if (e.response?.statusCode == 404) return null;
      throw teamFailureOf(e);
    }
  }

  @override
  Future<TeamView> enable(ChannelId channel) => _guard(() async {
    final team = (await _channels.enableChannelTeam(channelId: channel.value))
        .data!;
    return teamOfWire(team);
  });

  @override
  Future<void> disable(ChannelId channel) => _guard(() async {
    await _channels.disableChannelTeam(channelId: channel.value);
  });

  @override
  Future<TeamMemberView> add(
    ChannelId channel, {
    required PrincipalId agent,
    required ChannelId homeChannel,
    required String handle,
    required String idempotencyKey,
  }) => _guard(() async {
    final member = (await _channels.addTeamMember(
      channelId: channel.value,
      idempotencyKey: idempotencyKey,
      teamMemberCreate: wire.TeamMemberCreate(
        (b) => b
          ..agentId = agent.value
          ..homeChannelId = homeChannel.value
          ..handle = handle,
      ),
    )).data!;
    return memberOfWire(member);
  });

  @override
  Future<void> remove(ChannelId channel, String memberId) => _guard(() async {
    await _channels.removeTeamMember(
      channelId: channel.value,
      memberId: memberId,
    );
  });

  @override
  Future<List<String>> stopAll(
    ChannelId channel, {
    required String idempotencyKey,
  }) => _guard(() async {
    final result = (await _channels.stopChannelTeam(
      channelId: channel.value,
      idempotencyKey: idempotencyKey,
    )).data!;
    return result.stoppedRunIds.toList();
  });

  static Future<T> _guard<T>(Future<T> Function() call) async {
    try {
      return await call();
    } on DioException catch (e) {
      throw teamFailureOf(e);
    }
  }
}

/// The [TeamFailure] of [error]: the server's problem, or offline without an answer.
TeamFailure teamFailureOf(DioException error) {
  final problem = error.response == null ? null : wire.ApiProblem.of(error);
  if (problem == null) return const TeamFailure.offline();
  return TeamFailure.problem(problem.code, reason: problem.reason);
}

/// The [TeamView] of the wire [team].
TeamView teamOfWire(wire.Team team) => TeamView(
  channelId: ChannelId.parse(team.channelId),
  sponsorId: PrincipalId.parse(team.sponsorId),
  memberTurns: team.memberTurns,
  turnBudget: team.turnBudget,
  pause: switch (team.pauseReason) {
    null => null,
    wire.TeamPauseReason.turnBudget => TeamPause.turnBudget,
    wire.TeamPauseReason.pingpong => TeamPause.pingpong,
    wire.TeamPauseReason.stopped => TeamPause.stopped,
    _ => TeamPause.unknown,
  },
  members: [for (final m in team.members) memberOfWire(m)],
);

/// The [TeamMemberView] of the wire [member].
TeamMemberView memberOfWire(wire.TeamMember member) => TeamMemberView(
  id: member.id,
  handle: member.handle,
  agentId: PrincipalId.parse(member.agentId),
  homeChannelId: ChannelId.parse(member.homeChannelId),
  workThreadId: ThreadId.parse(member.workThreadId),
);
