// SPDX-License-Identifier: Apache-2.0

/// Riverpod providers of the sidebar over the local database and the engine's counters
/// (docs/spec/15-client-architecture.md 4: one provider per query; brief M4-19 Design note 1;
/// docs/DECISIONS.md D-M4-19-1). Widgets never query the database themselves (R-CL-03).
library;

import 'dart:async';

import 'package:design_system/design_system.dart' show DsChatStatus;
import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart' show ThreadSeenState;
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../app/providers/ports.dart';
import '../app/providers/session.dart';
import '../app/router.dart';
import '../forum/providers.dart'
    show
        channelDraftProvider,
        channelRunsProvider,
        forumThreadsProvider,
        PendingPost,
        pendingPostsProvider,
        unsentChannelDraftProvider;
import '../settings/settings_providers.dart'
    show showProjectStatusProvider, sidebarArrangementProvider;
import 'sidebar_arrange.dart';
import 'sidebar_model.dart';
import 'sidebar_order.dart';
import 'sidebar_status.dart';
import 'sidebar_threads.dart';

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

/// The spaces collapsed in the sidebar on this device (01 6.7, R-PR-24; kept in
/// `space_view_states`, never synchronized).
@riverpod
Stream<Set<SpaceId>> collapsedSpaces(Ref ref) {
  final db = ref.watch(localDatabaseProvider);
  return db == null
      ? Stream.value(const {})
      : db.forumViewDao.watchCollapsedSpaces();
}

/// The spaces with the person's state, collapsed as on this device; empty while either source
/// loads.
///
/// The order is the personal one with the global order as the fallback (`orderSpaces`), or, with
/// "Arrange spaces automatically" (01 6.16 item 4, D-M4-51-4), the newest activity of each
/// space's channels first with the personal order breaking ties; an automatic order holds while
/// [sidebarHoldProvider] holds, so no space moves under the pointer.
@riverpod
class SpacesWithState extends _$SpacesWithState {
  /// The spaces of the last build in their order; kept while the sidebar holds them.
  List<SpaceId> _shown = const [];

  @override
  List<SidebarSpace> build() {
    final manual = orderSpaces(
      ref.watch(storedSpacesProvider).value ?? const [],
      ref.watch(userSpaceStatesProvider).value ?? const [],
      collapsed: ref.watch(collapsedSpacesProvider).value ?? const {},
    );
    if (!ref.watch(sidebarArrangementProvider).autoSpaces) {
      _shown = const [];
      return manual;
    }
    final at = {
      for (final e in manual)
        e.space.id: spaceActivity(e.space, [
          for (final c
              in ref.watch(spaceChannelsProvider(e.space.id.value)).value ??
                  const <Channel>[])
            if (c.archivedAt == null && c.kind != ChannelKind.dm)
              ref.watch(channelActivityOfProvider(c.id.value)),
        ]),
    };
    final fresh = orderByActivity(manual, (e) => at[e.space.id]!);
    final out = ref.watch(sidebarHoldProvider)
        ? holdOrder(fresh, _shown, (e) => e.space.id)
        : fresh;
    _shown = [for (final e in out) e.space.id];
    return out;
  }
}

/// The activity of the channel [channelId] (01 6.16 item 4, D-M4-51-4): the newest
/// `last_activity_at` of its stored threads, the main history included, else its creation time;
/// the epoch while the channel is not stored.
@riverpod
DateTime channelActivityOf(Ref ref, String channelId) {
  final channel = ref.watch(storedChannelOfRouteProvider(channelId)).value;
  if (channel == null) {
    return DateTime.fromMillisecondsSinceEpoch(0, isUtc: true);
  }
  return channelActivity(
    channel,
    ref.watch(forumThreadsProvider(channelId)).value ?? const [],
  );
}

/// The person's sidebar state of every channel (03 `user_channel_states`): the manual order of
/// a forum's posts (01 6.16 item 3).
@riverpod
Stream<Map<ChannelId, UserChannelState>> userChannelStates(Ref ref) {
  final db = ref.watch(localDatabaseProvider);
  return db == null
      ? Stream.value(const {})
      : db.structureDao.watchUserChannelStates().map(
          (rows) => {for (final r in rows) r.channelId: r},
        );
}

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

/// The channels whose thread group the person collapsed on this device (01 6.8 item 7; kept in
/// `channel_view_states`, never synchronized, D-SB-6).
@riverpod
Stream<Set<ChannelId>> collapsedGroups(Ref ref) {
  final db = ref.watch(localDatabaseProvider);
  return db == null
      ? Stream.value(const {})
      : db.forumViewDao.watchCollapsedGroups();
}

/// The forums whose group lists every open post on this device ("Show more", 01 6.8 item 3;
/// kept in `channel_view_states`, never synchronized, D-M4-46-2).
@riverpod
Stream<Set<ChannelId>> postListsShownInFull(Ref ref) {
  final db = ref.watch(localDatabaseProvider);
  return db == null
      ? Stream.value(const {})
      : db.forumViewDao.watchPostListsShownInFull();
}

/// Whether the pointer is over the sidebar or the keyboard focus is in it: while it holds, a
/// thread row that no longer qualifies stays in place (01 6.8 item 4; brief M4-40 Design note
/// 2). The sidebar sets it ([SidebarHold.set]).
@Riverpod(keepAlive: true)
class SidebarHold extends _$SidebarHold {
  @override
  bool build() => false;

  /// Sets whether the sidebar holds its rows; does nothing once the provider is gone (a
  /// sidebar that leaves with the whole app reports after it).
  void set({required bool held}) {
    if (!ref.mounted) return;
    if (state != held) state = held;
  }
}

/// The seen marks of this device (01 6.17 item 3, `thread_seen_states`): the run and its state
/// each chat showed when the person last had it open here.
@riverpod
Stream<Map<ThreadId, ThreadSeenState>> seenStates(Ref ref) {
  final db = ref.watch(localDatabaseProvider);
  return db == null
      ? Stream.value(const {})
      : db.forumViewDao.watchSeenStates();
}

/// Writes the seen mark of a chat on screen ([ChatSeenMarker]): the newest run and its state;
/// null before the login. A failed write leaves the dot, which the next open clears.
@riverpod
void Function(ThreadId thread, RunSummary run)? chatSeenWriter(Ref ref) {
  final db = ref.watch(localDatabaseProvider);
  if (db == null) return null;
  return (thread, run) {
    db.forumViewDao.markSeen(thread, run.id, run.state).ignore();
  };
}

/// The chats of [channel] open on this device by [route] (01 6.17 item 2): the thread of the
/// route, or the main history of a text channel its own route shows.
Set<ThreadId> openChatsOf(RouteSelection route, Channel channel) => {
  ?route.thread,
  if (route.thread == null &&
      route.channel == channel.id &&
      !route.newChat &&
      channel.mainThreadId != null)
    channel.mainThreadId!,
};

/// The unseen status of every chat of [channelId] that has one (01 6.17; `unseenStatuses`): its
/// stored runs, the chats open on this device, the seen marks of this device, the person's read
/// states and the counters per chat.
@riverpod
Map<ThreadId, DsChatStatus> channelStatuses(Ref ref, String channelId) {
  final channel = ref.watch(storedChannelOfRouteProvider(channelId)).value;
  if (channel == null) return const {};
  return unseenStatuses(
    runs: ref.watch(channelRunsProvider(channelId)).value ?? const [],
    open: openChatsOf(ref.watch(lastRouteSelectionProvider), channel),
    seen: ref.watch(seenStatesProvider).value ?? const {},
    readStates: [
      for (final r
          in ref.watch(allReadStatesProvider).value ?? const <ReadState>[])
        if (r.channelId == channel.id) r,
    ],
    counts: ref.watch(threadCountsOfProvider(channelId)),
  );
}

/// The thread group of [channelId] (01 6.8 items 2 to 6; `selectSidebarThreads`): its stored
/// threads, the counters per thread, its runs, the selected thread of the route, the person,
/// the clock, the limits of `AppConfig` and the "Show more" state of this device. While
/// [sidebarHoldProvider] holds, the rows shown last stay, and in a forum in their order (01 6.8
/// item 4, D-M4-46-3); additions and changes apply at once. "Show more" and "Show less" apply at
/// once too: the person asked for them.
@riverpod
class ChannelThreads extends _$ChannelThreads {
  /// The rows of the last build in their order; kept while the sidebar holds them.
  List<ThreadId> _shown = const [];

  /// The "Show more" state of the last build.
  bool? _inFull;

  @override
  SidebarThreadGroup? build(String channelId) {
    final channel = ref.watch(storedChannelOfRouteProvider(channelId)).value;
    if (channel == null) return null;
    final inFull = (ref.watch(postListsShownInFullProvider).value ?? const {})
        .contains(channel.id);
    final toggled = _inFull != null && _inFull != inFull;
    _inFull = inFull;
    final held = ref.watch(sidebarHoldProvider) && !toggled;
    // 01 6.16: with "Arrange chats automatically" off a forum follows the person's order; a
    // forum without one stays by activity, held as before.
    final order = ref.watch(sidebarArrangementProvider).autoChats
        ? null
        : ref.watch(userChannelStatesProvider).value?[channel.id]?.threadOrder;
    final manual = order == null || order.isEmpty ? null : order;
    final prefs = ref.watch(notificationPreferencesProvider).value ?? const [];
    final now = ref.watch(appPortsProvider).clock.now();
    final group = selectSidebarThreads(
      channel: channel,
      threads: ref.watch(forumThreadsProvider(channelId)).value ?? const [],
      counts: ref.watch(threadCountsOfProvider(channelId)),
      runs: ref.watch(channelRunsProvider(channelId)).value ?? const [],
      selected: ref.watch(lastRouteSelectionProvider).thread,
      me: ref.watch(ownPrincipalProvider).value?.id,
      now: now,
      limits: SidebarThreadLimits.of(ref.watch(appConfigProvider)),
      expanded: !(ref.watch(collapsedGroupsProvider).value ?? const {})
          .contains(channel.id),
      keep: held ? _shown.toSet() : const {},
      heldOrder: held ? _shown : const [],
      showAllPosts: inFull,
      manualOrder: manual,
      mutedThreads: {
        for (final p in prefs)
          if (p.targetKind == NotificationTargetKind.thread &&
              isMutedTarget(p, now: now))
            ThreadId.parse(p.targetId),
      },
      channelMuted: isMuted(channel.id, prefs, now: now),
      statuses: ref.watch(channelStatusesProvider(channelId)),
    );
    _shown = [for (final r in group.rows) r.thread.id];
    return group;
  }
}

/// The rows of the forum [channelId] above its thread rows (01 6.8 item 8, brief M4-41): the
/// draft row while the person's draft of a new post has text (the unsent write of this device,
/// else the stored draft) or the new chat view of the forum is open and writing, then the posts
/// sent from this device that the server has not created yet, newest first. Empty for other
/// channels.
@riverpod
List<SidebarNewChatRow> newChatRows(Ref ref, String channelId) {
  final channel = ref.watch(storedChannelOfRouteProvider(channelId)).value;
  if (channel == null || channel.kind != ChannelKind.forum) return const [];
  final route = ref.watch(lastRouteSelectionProvider);
  final writing =
      route.newChat && route.channel == channel.id && route.sentEntry == null;
  // A draft write this device has not had confirmed is newer than the stored draft (04 13).
  final text =
      ref.watch(unsentChannelDraftProvider(channelId)).value ??
      ref.watch(channelDraftProvider(channelId)).value?.contentMd ??
      '';
  final hasDraft = text.trim().isNotEmpty;
  return [
    if (writing || hasDraft) const SidebarNewChatRow.draft(),
    for (final p
        in ref.watch(pendingPostsProvider(channelId)).value ??
            const <PendingPost>[])
      SidebarNewChatRow.sent(
        entryId: p.entryId,
        title: p.title,
        failed: p.failed,
      ),
  ];
}

/// The channel rows of [spaceId]: counters ([channelCounts]), active run, muted state, the
/// thread group with its collapse state on this device (01 6.8) and the status mark (01 6.17
/// item 6: the most urgent unseen status of its chats while "Show status on projects" is on,
/// else that of its main history); direct messages and archived channels are not part of the
/// tree.
///
/// The order is the global one in the person's `channel_order` (rows not in it first, 01 6.16
/// item 3), or, with "Arrange channels and projects automatically", the newest activity first
/// with that order breaking ties, held while [sidebarHoldProvider] holds (D-M4-51-4).
@riverpod
class ChannelsOfSpace extends _$ChannelsOfSpace {
  /// The channels of the last build in their order; kept while the sidebar holds them.
  List<ChannelId> _shown = const [];

  @override
  List<SidebarChannel> build(String spaceId) {
    ref.watch(counterTicksProvider);
    final engine = ref.watch(syncEngineProvider);
    final reads = ref.watch(allReadStatesProvider).value ?? const [];
    final prefs = ref.watch(notificationPreferencesProvider).value ?? const [];
    final runs = ref.watch(activeRunsProvider).value ?? const [];
    final collapsed = ref.watch(collapsedGroupsProvider).value ?? const {};
    final now = ref.watch(appPortsProvider).clock.now();
    final space = SpaceId.parse(spaceId);
    final state = (ref.watch(userSpaceStatesProvider).value ?? const [])
        .where((s) => s.spaceId == space)
        .firstOrNull;
    final manual = applyPersonalOrder(
      [
        for (final c
            in ref.watch(spaceChannelsProvider(spaceId)).value ??
                const <Channel>[])
          if (c.archivedAt == null && c.kind != ChannelKind.dm) c,
      ],
      state?.channelOrder,
      (c) => c.id,
    );
    var ordered = manual;
    if (ref.watch(sidebarArrangementProvider).autoChannels) {
      final at = {
        for (final c in manual)
          c.id: ref.watch(channelActivityOfProvider(c.id.value)),
      };
      final fresh = orderByActivity(manual, (c) => at[c.id]!);
      ordered = ref.watch(sidebarHoldProvider)
          ? holdOrder(fresh, _shown, (c) => c.id)
          : fresh;
      _shown = [for (final c in ordered) c.id];
    } else {
      _shown = const [];
    }
    final showProjects = ref.watch(showProjectStatusProvider);
    DsChatStatus? statusOf(Channel c) {
      final statuses = ref.watch(channelStatusesProvider(c.id.value));
      // Off, a row shows only the status of its own chat: the main history of a text channel.
      return showProjects
          ? aggregateStatus(statuses.values)
          : statuses[c.mainThreadId];
    }

    return [
      for (final c in ordered)
        SidebarChannel(
          channel: c,
          counts: channelCounts(
            engine: engine?.catchUp.countsOfChannel(c.id) ?? const {},
            readStates: reads,
            channel: c.id,
          ),
          runActive: runs.any((r) => r.channelId == c.id),
          muted: isMuted(c.id, prefs, now: now),
          group: ref.watch(channelThreadsProvider(c.id.value)),
          expanded: !collapsed.contains(c.id),
          newChatRows: ref.watch(newChatRowsProvider(c.id.value)),
          status: statusOf(c),
        ),
    ];
  }
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
  const RouteSelection({
    this.channel,
    this.kind,
    this.thread,
    this.newChat = false,
    this.sentEntry,
    this.mainHistory = false,
  });

  /// The selected channel, or null.
  final ChannelId? channel;

  /// Its kind, or null.
  final ChannelKind? kind;

  /// The selected thread, or null.
  final ThreadId? thread;

  /// Whether [thread] is the main history of its channel (a `/t/` route a mention, a search
  /// hit or a push opens): the layout is the main history alone, not a thread beside it
  /// (01 D2, D-M4-45-2).
  final bool mainHistory;

  /// Whether the route is the new chat view of [channel] (01 6.8 item 8, M4-41): its draft
  /// row, or the row of [sentEntry], is the selected one.
  final bool newChat;

  /// The outbox entry of the post the new chat view sent and waits for, or null.
  final String? sentEntry;

  /// The forum whose post list shows, or null (brief M4-19 Design note 5).
  ChannelId? get forum => kind == ChannelKind.forum ? channel : null;
}

/// The last selection a route resolved to; the shell shows it while the selection of a new
/// route loads, so the post list does not leave and come back for a frame.
@Riverpod(keepAlive: true)
class LastRouteSelection extends _$LastRouteSelection {
  @override
  RouteSelection build() => const RouteSelection();

  /// Keeps [selection], right after the current refresh of the providers. The desktop
  /// shell calls this from its listener of the route selection, which runs inside such a
  /// refresh, and a state set there would rebuild a provider that watches it and was already
  /// rebuilt in that refresh a second time (`Tried to rebuild ... multiple times in the same
  /// frame`, main run 40691988401; PL/59).
  void keep(RouteSelection selection) => scheduleMicrotask(() {
    if (ref.mounted) state = selection;
  });
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
    case NewChatLocation(:final channelId, :final sentEntry):
      final channel = await ref.watch(
        storedChannelOfRouteProvider(channelId.value).future,
      );
      return RouteSelection(
        channel: channelId,
        kind: channel?.kind,
        newChat: true,
        sentEntry: sentEntry,
      );
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
    mainHistory: channel?.mainThreadId == thread.value,
  );
}
