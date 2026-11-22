// SPDX-License-Identifier: Apache-2.0

/// What the app derives from a team (09 3.9; design cross-project-handoff.md MVP item 6): the
/// state of a member in the header, who a message in the team channel speaks for, and the
/// parsing of the client command `/team add|remove|list|stop` (09 3.9.1, reserved name 10.1).
library;

import 'package:domain/domain.dart';

import '../agents/run_mapping.dart' show isActiveRun;
import 'team_port.dart';

/// The state of a member in the header of a team channel.
enum TeamMemberState {
  /// No active run in the member's work thread and member inputs flow.
  idle,

  /// The member's agent has an active run (queued, running or waiting) in its work thread.
  running,

  /// No active run and member inputs of the channel are paused (brake or stop all).
  paused,
}

/// The state of [member] from the runs of its work thread [runs] and the channel's [pause]:
/// an active run of the member's agent is [TeamMemberState.running] also while the channel is
/// paused (the pause holds back new member inputs, not a running run).
TeamMemberState teamMemberState(
  TeamMemberView member,
  Iterable<RunSummary> runs,
  TeamPause? pause,
) {
  for (final r in runs) {
    if (r.threadId == member.workThreadId &&
        r.agentId == member.agentId &&
        isActiveRun(r.state)) {
      return TeamMemberState.running;
    }
  }
  return pause == null ? TeamMemberState.idle : TeamMemberState.paused;
}

/// The handles of the members whose agent is [author], in the team's order. A mirror of a
/// member's reply is a message of that member's agent in the team channel (09 3.9.4); the same
/// agent may be a member twice with two home channels, then both handles are named.
List<String> teamHandlesOf(TeamView team, PrincipalId author) => [
  for (final m in team.members)
    if (m.agentId == author) m.handle,
];

/// How a message in a team channel is labelled.
sealed class TeamMessageLabel {
  const TeamMessageLabel();
}

/// A member's mirrored reply: "from @handle".
final class FromMember extends TeamMessageLabel {
  /// Creates the label with the member [handles].
  const FromMember(this.handles);

  /// The handles of the members whose agent wrote the message.
  final List<String> handles;
}

/// An owner's text message, which reaches every member (09 3.9.3).
final class ToAllMembers extends TeamMessageLabel {
  /// Creates the label.
  const ToAllMembers();
}

/// The label of [message] in the team channel of [team], or null for none: a text message of
/// an agent that is a member is a mirror ([FromMember]); a text message of a person is an
/// owner's broadcast ([ToAllMembers]), because only owners read a team channel (readers rule,
/// 09 3.9.1). A voice message, a deleted message and every other origin get no label, since
/// they trigger nothing.
TeamMessageLabel? teamMessageLabel(TeamView team, Message message) {
  if (message.deletedAt != null || message.kind != MessageKind.text) {
    return null;
  }
  switch (message.origin) {
    case MessageOrigin.agent:
      final handles = teamHandlesOf(team, message.authorId);
      return handles.isEmpty ? null : FromMember(handles);
    case MessageOrigin.human:
      return message.contentMd.trim().isEmpty ? null : const ToAllMembers();
    case _:
      return null;
  }
}

/// The pattern of a handle (09 3.9.1); the length limit is the server's.
final RegExp teamHandlePattern = RegExp(r'^[a-z0-9][a-z0-9_-]*$');

/// One use of `/team`.
sealed class TeamCommand {
  const TeamCommand();
}

/// `/team` or `/team list`: shows the team.
final class TeamList extends TeamCommand {
  /// Creates the command.
  const TeamList();
}

/// `/team add [handle]`: opens the member dialog's add form, with [handle] filled in.
final class TeamAdd extends TeamCommand {
  /// Creates the command.
  const TeamAdd([this.handle]);

  /// The handle to fill in, without `@`, or null.
  final String? handle;
}

/// `/team remove @handle`: removes the member [handle].
final class TeamRemove extends TeamCommand {
  /// Creates the command.
  const TeamRemove(this.handle);

  /// The handle, without `@`.
  final String handle;
}

/// `/team stop`: stops every member run.
final class TeamStop extends TeamCommand {
  /// Creates the command.
  const TeamStop();
}

/// An unknown sub-command or a missing handle: the composer shows the usage.
final class TeamUsage extends TeamCommand {
  /// Creates the command.
  const TeamUsage();
}

/// The command of the text after `/team` ([argument]); sub-commands in any letter case, a
/// handle with or without `@` and lower-cased.
TeamCommand parseTeamCommand(String argument) {
  final words = argument.trim().split(RegExp(r'\s+'))
    ..removeWhere((w) => w.isEmpty);
  if (words.isEmpty) return const TeamList();
  String? handle(int i) {
    if (words.length <= i) return null;
    final w = words[i].startsWith('@') ? words[i].substring(1) : words[i];
    return w.isEmpty ? null : w.toLowerCase();
  }

  return switch (words.first.toLowerCase()) {
    'list' when words.length == 1 => const TeamList(),
    'add' when words.length <= 2 => TeamAdd(handle(1)),
    'remove' when words.length == 2 && handle(1) != null => TeamRemove(
      handle(1)!,
    ),
    'stop' when words.length == 1 => const TeamStop(),
    _ => const TeamUsage(),
  };
}

/// The channels [agent] may have as home channel among [channels] (09 3.9.1): operated by the
/// agent (its effective operator), not a direct message, not archived, not read-only and not
/// the team channel [team] itself. The server checks again, also that no home channel is a
/// team channel.
List<Channel> teamHomeCandidates(
  Iterable<Channel> channels,
  PrincipalId agent, {
  required ChannelId team,
}) => [
  for (final c in channels)
    if (c.id != team &&
        c.kind != ChannelKind.dm &&
        c.archivedAt == null &&
        !c.readOnly &&
        c.operator.effectiveAgentId == agent)
      c,
];
