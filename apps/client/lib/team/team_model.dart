// SPDX-License-Identifier: Apache-2.0

/// What the app derives from a team (09 3.9; design cross-project-handoff.md MVP item 6): the
/// state of a member in the header, who a message in the team channel speaks for or goes to
/// (the server's mention rule of 09 3.9.4), the parsing of the client command
/// `/team add|remove|list|stop` (09 3.9.1, reserved name 10.1), the home channels an agent may
/// still be added with and the handle the add form suggests.
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

/// An owner's text message without a mention of a member, which reaches every member (09 3.9.3;
/// owner ruling of 2026-11-22).
final class ToAllMembers extends TeamMessageLabel {
  /// Creates the label.
  const ToAllMembers();
}

/// An owner's text message that mentions members: "to @a, @b". It reaches only the members it
/// mentions (owner ruling of 2026-11-22, docs/decisions.d/FIX-TEAM-CHANNEL-APP.md).
final class ToMembers extends TeamMessageLabel {
  /// Creates the label with the mentioned member [handles].
  const ToMembers(this.handles);

  /// The handles of the mentioned members, in the order of their first mention.
  final List<String> handles;
}

/// The label of [message] in the team channel of [team], or null for none: a text message of
/// an agent that is a member is a mirror, labelled [FromMember] only when it carries no
/// `team_handle` (with one, the member is its author, FIX-TEAM-MEMBER-AUTHOR-DISPLAY); a text message of a person is an
/// owner's message, because only owners read a team channel (readers rule, 09 3.9.1). It goes
/// to the members it mentions ([ToMembers], the mention rule of [mentionedHandles]) and to every
/// member without such a mention ([ToAllMembers]). A voice message, a deleted message and every
/// other origin get no label, since they trigger nothing.
TeamMessageLabel? teamMessageLabel(TeamView team, Message message) {
  if (message.deletedAt != null || message.kind != MessageKind.text) {
    return null;
  }
  switch (message.origin) {
    case MessageOrigin.agent:
      // A mirror with its member's handle shows that member as its author, so it needs no
      // line (FIX-TEAM-MEMBER-AUTHOR-DISPLAY); a copy stored without it names every member
      // that agent is.
      if (message.teamHandle != null) return null;
      final handles = teamHandlesOf(team, message.authorId);
      return handles.isEmpty ? null : FromMember(handles);
    case MessageOrigin.human:
      if (message.contentMd.trim().isEmpty) return null;
      final handles = [
        for (final h in mentionedHandles(message.contentMd))
          if (team.byHandle(h) case final m?) m.handle,
      ];
      return handles.isEmpty ? const ToAllMembers() : ToMembers(handles);
    case _:
      return null;
  }
}

/// The handles [text] mentions, lower case, in the order of their first mention and without
/// duplicates: the server's rule of 09 3.9.4 (Go `MentionedHandles` in
/// services/server/internal/runs/domain/team.go). `@handle` counts at the start or after a
/// character that is no ASCII letter, digit, `-`, `_` or `@`, and runs up to the first
/// character that cannot be part of a handle; text inside a code span or a fenced code block
/// does not count. The caller keeps only the handles of members.
List<String> mentionedHandles(String text) {
  final out = <String>[];
  final seen = <String>{};
  for (final part in _outsideCode(text)) {
    for (final h in _handlesIn(part)) {
      if (seen.add(h)) out.add(h);
    }
  }
  return out;
}

bool _handleChar(int c) =>
    (c >= 0x61 && c <= 0x7a) || // a-z
    (c >= 0x41 && c <= 0x5a) || // A-Z
    (c >= 0x30 && c <= 0x39) || // 0-9
    c == 0x2d || // -
    c == 0x5f; // _

const int _at = 0x40;
const int _backtick = 0x60;

List<String> _handlesIn(String part) {
  final out = <String>[];
  for (var i = 0; i < part.length; i++) {
    final c = part.codeUnitAt(i);
    if (c != _at) continue;
    if (i > 0) {
      final before = part.codeUnitAt(i - 1);
      if (_handleChar(before) || before == _at) continue;
    }
    var j = i + 1;
    while (j < part.length && _handleChar(part.codeUnitAt(j))) {
      j++;
    }
    if (j > i + 1) out.add(part.substring(i + 1, j).toLowerCase());
    i = j - 1;
  }
  return out;
}

/// The parts of [text] outside fenced code blocks (a line starting with three or more
/// backticks or tildes opens one, the same fence closes it) and outside code spans.
List<String> _outsideCode(String text) {
  final parts = <String>[];
  var fence = '';
  for (final line in text.split('\n')) {
    final trimmed = line.replaceFirst(RegExp('^ +'), '');
    final f = _fenceOf(trimmed);
    if (f.isNotEmpty) {
      if (fence.isEmpty) {
        fence = f;
        continue;
      }
      if (trimmed.startsWith(fence) &&
          trimmed
              .replaceFirst(RegExp('^${RegExp.escape(fence[0])}+'), '')
              .trim()
              .isEmpty) {
        fence = '';
        continue;
      }
    }
    if (fence.isNotEmpty) continue;
    parts.addAll(_outsideSpans(line));
  }
  return parts;
}

String _fenceOf(String line) {
  for (final c in ['`', '~']) {
    var n = 0;
    while (n < line.length && line[n] == c) {
      n++;
    }
    if (n >= 3) return line.substring(0, n);
  }
  return '';
}

/// The parts of one [line] outside its code spans: a run of backticks opens a span that the
/// next run of the same length closes; an unclosed run is text.
List<String> _outsideSpans(String line) {
  final parts = <String>[];
  var start = 0;
  var i = 0;
  while (i < line.length) {
    if (line.codeUnitAt(i) != _backtick) {
      i++;
      continue;
    }
    final n = _backticks(line, i);
    final end = _closingRun(line, i + n, n);
    if (end < 0) {
      i += n;
      continue;
    }
    parts.add(line.substring(start, i));
    i = end + n;
    start = i;
  }
  return parts..add(line.substring(start));
}

int _backticks(String s, int i) {
  var n = 0;
  while (i + n < s.length && s.codeUnitAt(i + n) == _backtick) {
    n++;
  }
  return n;
}

int _closingRun(String s, int from, int n) {
  for (var j = from; j < s.length;) {
    if (s.codeUnitAt(j) != _backtick) {
      j++;
      continue;
    }
    final m = _backticks(s, j);
    if (m == n) return j;
    j += m;
  }
  return -1;
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

/// The home channels [agent] may still be added with to [team]: [teamHomeCandidates] without
/// the channels in which the agent is already a member (`team_member_exists`, 09 3.9.1).
List<Channel> teamFreeHomeCandidates(
  Iterable<Channel> channels,
  PrincipalId agent,
  TeamView team,
) => [
  for (final c in teamHomeCandidates(channels, agent, team: team.channelId))
    if (!team.members.any((m) => m.agentId == agent && m.homeChannelId == c.id))
      c,
];

/// The longest handle the add form suggests: the default of `team.handle_max_chars` (09 3.9.1,
/// 09 config table). The server checks its own value.
const int teamHandleSuggestMax = 32;

/// The handle the add form suggests for [name] (the home channel's name), or for [fallback]
/// (the agent's handle) when [name] leaves nothing: lower case, every run of characters outside
/// the handle pattern a `-`, without leading characters other than a letter or digit and
/// without trailing `-`, at most [teamHandleSuggestMax] characters, and unique among the
/// [taken] handles in any letter case by a suffix `-2`, `-3` and so on. Empty when neither
/// name leaves a character.
String suggestTeamHandle(
  String name, {
  String? fallback,
  required Iterable<String> taken,
}) {
  String reduce(String s) => s
      .toLowerCase()
      .replaceAll(RegExp('[^a-z0-9_-]+'), '-')
      .replaceFirst(RegExp('^[^a-z0-9]+'), '')
      .replaceFirst(RegExp(r'-+$'), '');
  String cut(String s, int max) {
    final c = s.length > max ? s.substring(0, max) : s;
    return c.replaceFirst(RegExp(r'-+$'), '');
  }

  var base = reduce(name);
  if (base.isEmpty && fallback != null) base = reduce(fallback);
  if (base.isEmpty) return '';
  final used = {for (final t in taken) t.toLowerCase()};
  var handle = cut(base, teamHandleSuggestMax);
  for (var n = 2; used.contains(handle); n++) {
    final suffix = '-$n';
    handle = '${cut(base, teamHandleSuggestMax - suffix.length)}$suffix';
  }
  return handle;
}
