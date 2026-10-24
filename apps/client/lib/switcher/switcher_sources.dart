// SPDX-License-Identifier: Apache-2.0

/// The sources and the ranking of the quick switcher D6 (docs/spec/01-product.md 5.1 D6;
/// docs/spec/14-search.md 4.9; brief M4-23 Design note 3): spaces, channels, people, agents
/// and the commands of the current channel from the local database, thread titles by a local
/// prefix match on stored threads plus the server's `searchThreadTitles`, merged by thread ID.
/// Only what the local database and the server deliver is listed (15 1 principle 1).
library;

import 'package:api_client/api_client.dart' show ApiClient;
import 'package:domain/domain.dart';

import 'recent_targets.dart';

/// The kind of a switcher row; it decides the icon, the kind label and the filter.
enum SwitcherKind {
  /// A space (`/s/<id>`).
  space,

  /// A text or forum channel (`/c/<id>`).
  channel,

  /// A thread (`/t/<id>`).
  thread,

  /// A person; opens the stored DM with them, if any.
  person,

  /// An agent; opens the stored DM with it, if any.
  agent,

  /// A command of the current channel; commands run from the composer (M4-21).
  command,
}

/// The filter row of D6: All, Channels, Threads, People, Agents, Commands.
enum SwitcherFilter {
  /// Every kind.
  all,

  /// Spaces and channels (D6 has no own filter for spaces).
  channels,

  /// Threads.
  threads,

  /// People.
  people,

  /// Agents.
  agents,

  /// Commands.
  commands;

  /// Whether a row of [kind] shows under this filter.
  bool admits(SwitcherKind kind) => switch (this) {
    all => true,
    channels => kind == SwitcherKind.space || kind == SwitcherKind.channel,
    threads => kind == SwitcherKind.thread,
    people => kind == SwitcherKind.person,
    agents => kind == SwitcherKind.agent,
    commands => kind == SwitcherKind.command,
  };

  /// The filter after this one (Tab), wrapping to [all].
  SwitcherFilter get next => values[(index + 1) % values.length];

  /// The filter before this one (Shift+Tab), wrapping to [commands].
  SwitcherFilter get previous =>
      values[(index - 1 + values.length) % values.length];
}

/// One row of the switcher.
final class SwitcherEntry {
  /// Creates a row.
  const SwitcherEntry({
    required this.kind,
    required this.id,
    required this.name,
    this.context,
    this.path,
  });

  /// The kind.
  final SwitcherKind kind;

  /// The ID of the target (space, channel, thread, principal or command).
  final String id;

  /// The name shown and matched: a space or channel name, a thread title, a display name, or
  /// a command with its slash (`/status`).
  final String name;

  /// What the row sits in, shown on the right: the space of a channel, the channel of a
  /// thread, the owner of a command; null when there is none.
  final String? context;

  /// The path Enter goes to, or null for a row that does not navigate (a person or agent
  /// without a stored DM, a command).
  final String? path;
}

/// What the local database holds for the switcher, read in one place so the ranking stays a
/// pure function of it.
final class SwitcherLocalData {
  /// Creates the data.
  const SwitcherLocalData({
    required this.spaces,
    required this.channels,
    required this.threads,
    required this.principals,
    required this.agents,
    required this.commands,
    this.me,
    this.currentChannel,
  });

  /// No data (no database).
  static const SwitcherLocalData empty = SwitcherLocalData(
    spaces: [],
    channels: [],
    threads: [],
    principals: [],
    agents: [],
    commands: [],
  );

  /// The stored spaces.
  final List<Space> spaces;

  /// Every stored channel, direct messages included.
  final List<Channel> channels;

  /// The stored threads.
  final List<Thread> threads;

  /// The stored principals.
  final List<Principal> principals;

  /// The stored agent cards.
  final List<AgentCard> agents;

  /// Every stored command.
  final List<Command> commands;

  /// The signed-in person, or null before the first snapshot.
  final PrincipalId? me;

  /// The channel the route shows (the channel of a thread route included), or null.
  final ChannelId? currentChannel;
}

/// The local rows of [data] in the order spaces, channels, threads (latest activity first),
/// people, agents, commands; names in their stored order or alphabetically.
///
/// Archived spaces and channels, direct-message channels (their participant is the row) and
/// main threads (the channel is the row) are left out; so is the signed-in person. A person or
/// agent gets the path of the stored DM with exactly them; a person with no stored DM is listed
/// without a path (the client never creates a DM from the switcher). Commands are those of the
/// instance, of the space of the current channel and of the current channel.
List<SwitcherEntry> localEntries(SwitcherLocalData data) {
  final spaceNames = {for (final s in data.spaces) s.id: s.name};
  final channels = {for (final c in data.channels) c.id: c};
  final names = {for (final p in data.principals) p.id: p.displayName};
  final dmOf = <PrincipalId, ChannelId>{};
  for (final c in data.channels) {
    if (c.kind != ChannelKind.dm || c.archivedAt != null) continue;
    final others = c.dmParticipantIds.where((p) => p != data.me).toSet();
    if (others.length == 1) dmOf.putIfAbsent(others.single, () => c.id);
  }
  String? channelPath(PrincipalId id) =>
      dmOf[id] == null ? null : '/c/${dmOf[id]!.value}';
  String? nonEmpty(String? s) => s == null || s.isEmpty ? null : s;
  final current = data.currentChannel == null
      ? null
      : channels[data.currentChannel];
  final agentIds = {for (final a in data.agents) a.id};
  final threads = [...data.threads]
    ..sort((a, b) => b.lastActivityAt.compareTo(a.lastActivityAt));
  final commands = [
    for (final c in data.commands)
      if (c.scopeKind == TargetKind.instance ||
          (c.scopeKind == TargetKind.channel &&
              current != null &&
              c.scopeId == current.id.value) ||
          (c.scopeKind == TargetKind.space &&
              current?.spaceId != null &&
              c.scopeId == current!.spaceId!.value))
        c,
  ]..sort((a, b) => a.name.compareTo(b.name));
  return [
    for (final s in data.spaces)
      if (s.archivedAt == null)
        SwitcherEntry(
          kind: SwitcherKind.space,
          id: s.id.value,
          name: s.name,
          path: '/s/${s.id.value}',
        ),
    for (final c in data.channels)
      if (c.kind != ChannelKind.dm && c.archivedAt == null)
        SwitcherEntry(
          kind: SwitcherKind.channel,
          id: c.id.value,
          name: c.name,
          context: nonEmpty(spaceNames[c.spaceId]),
          path: '/c/${c.id.value}',
        ),
    for (final t in threads)
      if (t.kind != ThreadKind.main && t.title.isNotEmpty)
        SwitcherEntry(
          kind: SwitcherKind.thread,
          id: t.id.value,
          name: t.title,
          context: nonEmpty(channels[t.channelId]?.name),
          path: '/t/${t.id.value}',
        ),
    for (final p in data.principals)
      if (p.kind == PrincipalKind.user && p.id != data.me)
        SwitcherEntry(
          kind: SwitcherKind.person,
          id: p.id.value,
          name: p.displayName,
          path: channelPath(p.id),
        ),
    for (final a in data.agents)
      SwitcherEntry(
        kind: SwitcherKind.agent,
        id: a.id.value,
        name: a.displayName,
        path: channelPath(a.id),
      ),
    for (final p in data.principals)
      if (p.kind == PrincipalKind.agent && !agentIds.contains(p.id))
        SwitcherEntry(
          kind: SwitcherKind.agent,
          id: p.id.value,
          name: p.displayName,
          path: channelPath(p.id),
        ),
    for (final c in commands)
      SwitcherEntry(
        kind: SwitcherKind.command,
        id: c.id.value,
        name: '/${c.name}',
        context: nonEmpty(names[c.ownerPrincipalId]),
      ),
  ];
}

/// A thread title the server found by prefix (API `ThreadTitleHitList` item, 14 4.9).
final class ThreadTitleHit {
  /// Creates a hit.
  const ThreadTitleHit({
    required this.threadId,
    required this.channelId,
    required this.title,
    required this.lastActivityAt,
  });

  /// The thread.
  final ThreadId threadId;

  /// Its channel.
  final ChannelId channelId;

  /// Its title.
  final String title;

  /// Its latest activity; the server sorts by it.
  final DateTime lastActivityAt;
}

/// The server's prefix search over thread titles (`searchThreadTitles`, 14 4.9); a port so
/// tests replace the REST call.
abstract interface class ThreadTitleSearch {
  /// Up to [limit] titles whose words start with the words of [q], latest activity first.
  ///
  /// Throws the error of the request (network, problem response).
  Future<List<ThreadTitleHit>> prefix(String q, {required int limit});
}

/// [ThreadTitleSearch] over `api_client`.
///
/// Example: `await RestThreadTitleSearch(services.auth.api).prefix('gate', limit: 20);`
final class RestThreadTitleSearch implements ThreadTitleSearch {
  /// Creates the search on [api].
  RestThreadTitleSearch(this._api);

  final ApiClient _api;

  @override
  Future<List<ThreadTitleHit>> prefix(String q, {required int limit}) async {
    final list = (await _api.getSearchApi().searchThreadTitles(
      q: q,
      limit: limit,
    )).data;
    return [
      if (list != null)
        for (final i in list.items)
          ThreadTitleHit(
            threadId: ThreadId.parse(i.threadId),
            channelId: ChannelId.parse(i.channelId),
            title: i.title,
            lastActivityAt: i.lastActivityAt,
          ),
    ];
  }
}

/// The rows of the server [hits], with the channel name from [data] as the context when the
/// channel is stored. A hit for a thread that is not stored still navigates to `/t/<id>`;
/// the target resolver loads it (15 11).
List<SwitcherEntry> serverEntries(
  List<ThreadTitleHit> hits,
  SwitcherLocalData data,
) {
  final channels = {for (final c in data.channels) c.id: c.name};
  return [
    for (final h in hits)
      SwitcherEntry(
        kind: SwitcherKind.thread,
        id: h.threadId.value,
        name: h.title,
        context: switch (channels[h.channelId]) {
          final String n when n.isNotEmpty => n,
          _ => null,
        },
        path: '/t/${h.threadId.value}',
      ),
  ];
}

/// Splits [text] into lower-case words on everything that is not a letter or a digit.
List<String> _words(String text) => text
    .toLowerCase()
    .split(RegExp(r'[^\p{L}\p{N}]+', unicode: true))
    .where((w) => w.isNotEmpty)
    .toList();

/// Whether every word of [query] is the start of a word of [name], ignoring case: the local
/// counterpart of the server's prefix search (14 4.9, `rechnungs okt` finds "Rechnungslauf
/// Oktober"). A query without words matches nothing.
bool matchesWordPrefixes(String name, String query) {
  final wanted = _words(query);
  if (wanted.isEmpty) return false;
  final words = _words(name);
  return wanted.every((q) => words.any((w) => w.startsWith(q)));
}

/// The rows of the switcher for [query] under [filter], at most [max] (the value of
/// `ui.quick_switch_max_results`).
///
/// Ranking (01 D6, Design note 3): first the targets of [recent] in their order (newest
/// first), then the rows whose name starts with the query, then the other matches. A row
/// matches when every word of the query starts a word of its name; server rows matched on the
/// server and always count. A server row for a thread that is also stored locally is dropped
/// (merged by ID). An empty query lists only the recent targets, unless [showAllWhenEmpty].
///
/// A recent target is found by its path, so a visit of a DM channel ranks the person or agent
/// of that DM; a recent target that is no longer stored is not listed.
List<SwitcherEntry> rankSwitcher({
  required List<SwitcherEntry> local,
  required List<SwitcherEntry> server,
  required List<RecentTarget> recent,
  required String query,
  required SwitcherFilter filter,
  required int max,
  bool showAllWhenEmpty = false,
}) {
  final localThreads = {
    for (final e in local)
      if (e.kind == SwitcherKind.thread) e.id,
  };
  final fromServer = [
    for (final e in server)
      if (!localThreads.contains(e.id)) e,
  ];
  final candidates = [
    for (final e in [...local, ...fromServer])
      if (filter.admits(e.kind)) e,
  ];
  final q = query.trim();
  final serverIds = {for (final e in fromServer) e.id};
  final matched = q.isEmpty
      ? (showAllWhenEmpty ? candidates : const <SwitcherEntry>[])
      : [
          for (final e in candidates)
            if ((e.kind == SwitcherKind.thread && serverIds.contains(e.id)) ||
                matchesWordPrefixes(e.name, q))
              e,
        ];
  final byPath = <String, SwitcherEntry>{
    for (final e in q.isEmpty ? candidates : matched)
      if (e.path != null) e.path!: e,
  };
  final recentRows = <SwitcherEntry>[];
  final sorted = [...recent]
    ..sort((a, b) => b.visitedAt.compareTo(a.visitedAt));
  for (final r in sorted) {
    final e = byPath[recentPath(r)];
    if (e != null && !recentRows.contains(e)) recentRows.add(e);
  }
  final lower = q.toLowerCase();
  final rest = [
    for (final e in matched)
      if (!recentRows.contains(e)) e,
  ];
  final starts = [
    for (final e in rest)
      if (e.name.toLowerCase().startsWith(lower)) e,
  ];
  final others = [
    for (final e in rest)
      if (!starts.contains(e)) e,
  ];
  return [...recentRows, ...starts, ...others].take(max).toList();
}
