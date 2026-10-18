// SPDX-License-Identifier: Apache-2.0

/// Riverpod providers of the sidebar over the local database and the engine's counters
/// (docs/spec/15-client-architektur.md 4: one provider per query; brief M4-19 Design note 1;
/// docs/DECISIONS.md D-M4-19-1). Widgets never query the database themselves (R-CL-03).
library;

import 'package:domain/domain.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../app/providers/ports.dart';
import '../app/providers/session.dart';
import '../app/router.dart';
import 'sidebar_model.dart';
import 'sidebar_order.dart';

part 'providers.g.dart';

/// Every stored space.
@riverpod
Stream<List<Space>> storedSpaces(Ref ref) {
  final db = ref.watch(localDatabaseProvider);
  return db == null ? Stream.value(const []) : db.structureDao.watchSpaces();
}

/// The person's sidebar state of every space (03 `user_space_states`).
@riverpod
Stream<List<UserSpaceState>> userSpaceStates(Ref ref) {
  final db = ref.watch(localDatabaseProvider);
  return db == null
      ? Stream.value(const [])
      : db.structureDao.watchUserSpaceStates();
}

/// The spaces with the person's state, in the personal order with the global order as the
/// fallback (`orderSpaces`); empty while either source loads.
@riverpod
List<SidebarSpace> spacesWithState(Ref ref) => orderSpaces(
  ref.watch(storedSpacesProvider).value ?? const [],
  ref.watch(userSpaceStatesProvider).value ?? const [],
);

/// The stored channels of [spaceId] in their order.
@riverpod
Stream<List<Channel>> spaceChannels(Ref ref, String spaceId) {
  final db = ref.watch(localDatabaseProvider);
  return db == null
      ? Stream.value(const [])
      : db.structureDao.watchChannels(SpaceId.parse(spaceId));
}

/// Every stored read state (the server's counters per thread, 04 14).
@riverpod
Stream<List<ReadState>> allReadStates(Ref ref) {
  final db = ref.watch(localDatabaseProvider);
  return db == null
      ? Stream.value(const [])
      : db.personalDao.watchAllReadStates();
}

/// Every notification preference of the person.
@riverpod
Stream<List<NotificationPreference>> notificationPreferences(Ref ref) {
  final db = ref.watch(localDatabaseProvider);
  return db == null
      ? Stream.value(const [])
      : db.personalDao.watchNotificationPreferences();
}

/// The runs that have not ended.
@riverpod
Stream<List<RunSummary>> activeRuns(Ref ref) {
  final db = ref.watch(localDatabaseProvider);
  return db == null ? Stream.value(const []) : db.runsDao.watchActive();
}

/// A number that changes whenever the engine's counters change (04 14: its own counting of
/// `message.*` between two server values); the counter providers watch it.
@riverpod
Stream<int> counterTicks(Ref ref) async* {
  final engine = ref.watch(syncEngineProvider);
  var tick = 0;
  yield tick;
  if (engine == null) return;
  await for (final _ in engine.catchUp.countChanges) {
    yield ++tick;
  }
}

/// The counters of the threads of [channelId]: the engine's values, else the stored read
/// state (`unread_count` where delivered, `mention_count`).
@riverpod
Map<ThreadId, ThreadCounts> threadCountsOf(Ref ref, String channelId) {
  ref.watch(counterTicksProvider);
  final id = ChannelId.parse(channelId);
  final engine =
      ref.watch(syncEngineProvider)?.catchUp.countsOfChannel(id) ?? const {};
  return {
    for (final r
        in ref.watch(allReadStatesProvider).value ?? const <ReadState>[])
      if (r.channelId == id)
        r.threadId: ThreadCounts(
          unread: r.unreadCount ?? 0,
          mentions: r.mentionCount,
        ),
    ...engine,
  };
}

/// The channel rows of [spaceId]: counters ([channelCounts]), active run and muted state;
/// direct messages and archived channels are not part of the tree.
@riverpod
List<SidebarChannel> channelsOfSpace(Ref ref, String spaceId) {
  ref.watch(counterTicksProvider);
  final engine = ref.watch(syncEngineProvider);
  final reads = ref.watch(allReadStatesProvider).value ?? const [];
  final prefs = ref.watch(notificationPreferencesProvider).value ?? const [];
  final runs = ref.watch(activeRunsProvider).value ?? const [];
  final now = ref.watch(appPortsProvider).clock.now();
  return [
    for (final c
        in ref.watch(spaceChannelsProvider(spaceId)).value ?? const <Channel>[])
      if (c.archivedAt == null && c.kind != ChannelKind.dm)
        SidebarChannel(
          channel: c,
          counts: channelCounts(
            engine: engine?.catchUp.countsOfChannel(c.id) ?? const {},
            readStates: reads,
            channel: c.id,
          ),
          runActive: runs.any((r) => r.channelId == c.id),
          muted: isMuted(c.id, prefs, now: now),
        ),
  ];
}

/// The whole tree: the spaces in the person's order with their channel rows.
@riverpod
List<SidebarSpaceNode> sidebarTree(Ref ref) => [
  for (final entry in ref.watch(spacesWithStateProvider))
    SidebarSpaceNode(
      entry: entry,
      channels: ref.watch(channelsOfSpaceProvider(entry.space.id.value)),
    ),
];

/// The signed-in person as the engine's last snapshot named it, with name and avatar from
/// the local database; null before the first snapshot of this process.
@riverpod
Future<Principal?> ownPrincipal(Ref ref) async {
  ref.watch(connectionStateProvider);
  final services = ref.watch(signedInServicesProvider);
  final me = services?.engine.catchUp.me;
  if (services == null || me == null) return null;
  return services.database.principalsDao.byId(me);
}

/// The stored thread [threadId], emitted on every change; null while it is not stored.
@riverpod
Stream<Thread?> storedThread(Ref ref, String threadId) {
  final db = ref.watch(localDatabaseProvider);
  return db == null
      ? Stream.value(null)
      : db.threadsDao.watchThread(ThreadId.parse(threadId));
}

/// The stored channel [channelId], emitted on every change; null while it is not stored.
@riverpod
Stream<Channel?> storedChannelOfRoute(Ref ref, String channelId) {
  final db = ref.watch(localDatabaseProvider);
  return db == null
      ? Stream.value(null)
      : db.structureDao.watchChannel(ChannelId.parse(channelId));
}

/// What the route selects: the channel (of `/c/<id>`, or the channel of the thread of
/// `/t/<id>`), its kind and the thread; empty for other paths or targets not stored here.
final class RouteSelection {
  /// Creates the selection.
  const RouteSelection({this.channel, this.kind, this.thread});

  /// The selected channel, or null.
  final ChannelId? channel;

  /// Its kind, or null.
  final ChannelKind? kind;

  /// The selected thread, or null.
  final ThreadId? thread;

  /// The forum whose post list shows, or null (brief M4-19 Design note 5).
  ChannelId? get forum => kind == ChannelKind.forum ? channel : null;
}

/// The last selection a route resolved to; the shell shows it while the selection of a new
/// route loads, so the post list does not leave and come back for a frame.
@Riverpod(keepAlive: true)
class LastRouteSelection extends _$LastRouteSelection {
  @override
  RouteSelection build() => const RouteSelection();

  /// Keeps [selection].
  void keep(RouteSelection selection) => state = selection;
}

/// The selection of [path] (path and query of the router), read from the local database; the
/// selected sidebar row and the post list follow it (brief M4-19 Design note 5).
@riverpod
Future<RouteSelection> routeSelection(Ref ref, String path) async {
  final db = ref.watch(localDatabaseProvider);
  if (db == null) return const RouteSelection();
  final location = parseLocation(Uri.parse(path));
  ThreadId? thread;
  ChannelId? channelId;
  switch (location) {
    case ChannelLocation(:final channelId):
      // The channel may arrive later (a link opened before the first snapshot).
      final channel = await ref.watch(
        storedChannelOfRouteProvider(channelId.value).future,
      );
      return RouteSelection(channel: channelId, kind: channel?.kind);
    case ThreadLocation(:final threadId):
      thread = threadId;
      // The thread may arrive later (a deep link); its post list shows once it is stored.
      channelId = (await ref.watch(storedThreadProvider(threadId.value).future))
          ?.channelId;
    default:
      return const RouteSelection();
  }
  final channel = channelId == null
      ? null
      : await db.structureDao.channelById(channelId);
  return RouteSelection(
    channel: channelId,
    kind: channel?.kind,
    thread: thread,
  );
}
