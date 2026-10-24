// SPDX-License-Identifier: Apache-2.0

/// Riverpod providers of the quick switcher D6 (docs/spec/15-client-architecture.md 4: one
/// provider per query; brief M4-23 Design note 3). The overlay reads only these (R-CL-03).
library;

import 'package:domain/domain.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:sync_engine/sync_engine.dart';

import '../app/providers/ports.dart';
import '../app/providers/session.dart';
import '../app/router.dart';
import 'recent_targets.dart';
import 'switcher_sources.dart';

part 'providers.g.dart';

/// The recent targets of this device, or null without a database; keeps and lists at most
/// `ui.quick_switch_max_results` targets, timed by the app's clock port.
@riverpod
RecentTargets? recentTargets(Ref ref) {
  final db = ref.watch(localDatabaseProvider);
  if (db == null) return null;
  final ports = ref.watch(appPortsProvider);
  return RecentTargets(
    db.recentTargetsDao,
    clock: ports.clock,
    keep: ref.watch(appConfigProvider).quickSwitchMaxResults,
  );
}

/// The recent targets, newest first.
@riverpod
Stream<List<RecentTarget>> recentTargetList(Ref ref) {
  final recent = ref.watch(recentTargetsProvider);
  return recent == null ? Stream.value(const []) : recent.watch();
}

/// Records a visit of [path] (path and query of the router) once; the shell watches it with
/// the location it shows, so every new route is recorded when it appears. Returns whether the
/// path named a space, channel or thread.
@riverpod
Future<bool> recordVisit(Ref ref, String path) async {
  final recent = ref.watch(recentTargetsProvider);
  if (recent == null) return false;
  return recent.recordLocation(Uri.parse(path));
}

/// The channel the route [path] shows: the channel of `/c/<id>` or the stored channel of the
/// thread of `/t/<id>`; null for other paths. The commands of the switcher are those of this
/// channel.
@riverpod
Future<String?> switcherCurrentChannel(Ref ref, String path) async {
  final db = ref.watch(localDatabaseProvider);
  return switch (parseLocation(Uri.parse(path))) {
    ChannelLocation(:final channelId) => channelId.value,
    ThreadLocation(:final threadId) when db != null =>
      (await db.threadsDao.byId(threadId))?.channelId.value,
    _ => null,
  };
}

/// The stored spaces, on every change.
@riverpod
Stream<List<Space>> switcherSpaces(Ref ref) {
  final db = ref.watch(localDatabaseProvider);
  return db == null ? Stream.value(const []) : db.structureDao.watchSpaces();
}

/// The stored channels of [spaceId], or the direct messages for null, on every change.
@riverpod
Stream<List<Channel>> switcherChannels(Ref ref, String? spaceId) {
  final db = ref.watch(localDatabaseProvider);
  return db == null
      ? Stream.value(const [])
      : db.structureDao.watchChannels(
          spaceId == null ? null : SpaceId.parse(spaceId),
        );
}

/// The stored threads of [channelId], on every change.
@riverpod
Stream<List<Thread>> switcherThreads(Ref ref, String channelId) {
  final db = ref.watch(localDatabaseProvider);
  return db == null
      ? Stream.value(const [])
      : db.threadsDao.watchForChannel(ChannelId.parse(channelId));
}

/// The stored principals, on every change.
@riverpod
Stream<List<Principal>> switcherPrincipals(Ref ref) {
  final db = ref.watch(localDatabaseProvider);
  return db == null ? Stream.value(const []) : db.principalsDao.watchAll();
}

/// The stored agent cards, on every change.
@riverpod
Stream<List<AgentCard>> switcherAgents(Ref ref) {
  final db = ref.watch(localDatabaseProvider);
  return db == null ? Stream.value(const []) : db.principalsDao.watchAgents();
}

/// Every stored command, on every change.
@riverpod
Stream<List<Command>> switcherCommands(Ref ref) {
  final db = ref.watch(localDatabaseProvider);
  return db == null ? Stream.value(const []) : db.commandsDao.watchAll();
}

/// What the local database holds for the switcher while it is open: every stored space,
/// channel (direct messages included), thread, principal, agent card and command, the
/// signed-in person and [currentChannel]; null while the spaces still load. The rows follow
/// changes of the database while the switcher is open.
@riverpod
SwitcherLocalData? switcherLocalData(Ref ref, String? currentChannel) {
  final spaces = ref.watch(switcherSpacesProvider).value;
  if (spaces == null) return null;
  final channels = [
    for (final s in spaces)
      ...?ref.watch(switcherChannelsProvider(s.id.value)).value,
    ...?ref.watch(switcherChannelsProvider(null)).value,
  ];
  return SwitcherLocalData(
    spaces: spaces,
    channels: channels,
    threads: [
      for (final c in channels)
        ...?ref.watch(switcherThreadsProvider(c.id.value)).value,
    ],
    principals: ref.watch(switcherPrincipalsProvider).value ?? const [],
    agents: ref.watch(switcherAgentsProvider).value ?? const [],
    commands: ref.watch(switcherCommandsProvider).value ?? const [],
    me: ref.watch(syncEngineProvider)?.catchUp.me,
    currentChannel: currentChannel == null
        ? null
        : ChannelId.parse(currentChannel),
  );
}

/// The server's prefix search over thread titles, or null before the login (tests override
/// it).
@riverpod
ThreadTitleSearch? threadTitleSearch(Ref ref) {
  final services = ref.watch(signedInServicesProvider);
  return services == null ? null : RestThreadTitleSearch(services.auth.api);
}

/// The server's thread titles for [query] (14 4.9), at most `ui.quick_switch_max_results`.
///
/// Asked only while the engine is connected; offline, before the login, for an empty query
/// and when the request fails the list is empty, so the switcher shows its local rows alone
/// (01 6.6: the app stays usable offline).
@riverpod
Future<List<ThreadTitleHit>> serverThreadTitles(Ref ref, String query) async {
  final q = query.trim();
  final search = ref.watch(threadTitleSearchProvider);
  final phase = ref.watch(connectionStateProvider).value?.phase;
  final online = switch (phase) {
    null ||
    ConnectionPhase.offline ||
    ConnectionPhase.backoff ||
    ConnectionPhase.suspended ||
    ConnectionPhase.updateRequired => false,
    _ => true,
  };
  if (q.isEmpty || search == null || !online) return const [];
  try {
    return await search.prefix(
      q,
      limit: ref.watch(appConfigProvider).quickSwitchMaxResults,
    );
  } on Object {
    return const [];
  }
}

/// The rows of the switcher for [query] under [filter] in the route's [currentChannel]:
/// local rows and server titles ranked by `rankSwitcher`, at most
/// `ui.quick_switch_max_results`.
@riverpod
List<SwitcherEntry> switcherRows(
  Ref ref,
  String query,
  SwitcherFilter filter,
  String? currentChannel,
) {
  final data =
      ref.watch(switcherLocalDataProvider(currentChannel)) ??
      SwitcherLocalData.empty;
  final hits = ref.watch(serverThreadTitlesProvider(query)).value ?? const [];
  return rankSwitcher(
    local: localEntries(data),
    server: serverEntries(hits, data),
    recent: ref.watch(recentTargetListProvider).value ?? const [],
    query: query,
    filter: filter,
    max: ref.watch(appConfigProvider).quickSwitchMaxResults,
  );
}
