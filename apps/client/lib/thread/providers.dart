// SPDX-License-Identifier: Apache-2.0

/// Riverpod providers of the thread view over the local database, the outbox and the engine
/// (docs/spec/15-client-architecture.md 4: one provider per query; brief M4-20 Design notes 1 to
/// 6). Widgets never query the database themselves (R-CL-03).
library;

import 'dart:async';

import 'package:domain/domain.dart';
import 'package:flutter/widgets.dart';
import 'package:local_store/local_store.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:sync_engine/sync_engine.dart';

import '../app/providers/ports.dart';
import '../app/providers/session.dart';
import '../composer/draft_sync.dart';
import '../desktop/providers.dart';
import '../sidebar/providers.dart';
import 'thread_reads.dart';

part 'providers.g.dart';

/// The messages of a page of the history: the first window and every page loaded when the
/// person scrolls up (a page of `listThreadMessages` without `limit` has the server's size).
const int historyPageSize = 50;

/// The REST reads of the thread view; tests override it.
@riverpod
ThreadReads threadReads(Ref ref) {
  final engine = ref.watch(syncEngineProvider);
  return engine == null ? const _NoReads() : EngineThreadReads(engine);
}

/// The signed-in person, or null before the first snapshot.
@riverpod
PrincipalId? myPrincipalId(Ref ref) =>
    ref.watch(ownPrincipalProvider).value?.id;

/// The caller's level and capabilities in [channelId] as the server reported them.
@riverpod
Stream<AccessView?> channelAccess(Ref ref, String channelId) {
  final db = ref.watch(localDatabaseProvider);
  return db == null
      ? Stream.value(null)
      : db.structureDao.watchChannelAccess(ChannelId.parse(channelId));
}

/// Every stored principal by ID (names, handles and the mention suggestions).
@riverpod
Stream<Map<PrincipalId, Principal>> principalsById(Ref ref) {
  final db = ref.watch(localDatabaseProvider);
  if (db == null) return Stream.value(const {});
  return db.principalsDao.watchAll().map(
    (all) => {for (final p in all) p.id: p},
  );
}

/// The limits of the instance (`getInstance`, D-W-115), or null while unknown.
@riverpod
Stream<InstanceLimits?> instanceLimits(Ref ref) {
  final db = ref.watch(localDatabaseProvider);
  return db == null
      ? Stream.value(null)
      : db.instanceDao.watch().map((i) => i?.limits);
}

/// How many messages the history of [threadId] shows: one page, more after paging up or a
/// jump; it lasts while the app runs, so coming back keeps the loaded range in view.
@Riverpod(keepAlive: true)
class HistoryLimit extends _$HistoryLimit {
  @override
  int build(String threadId) => historyPageSize;

  /// Shows at least [count] messages.
  void atLeast(int count) {
    if (count > state) state = count;
  }
}

/// The contiguous window of [threadId] (15 7.3 item 4): the newest loaded messages and the
/// gaps before and after them.
@riverpod
Stream<ContiguousWindow> threadWindow(Ref ref, String threadId) {
  final db = ref.watch(localDatabaseProvider);
  if (db == null) return Stream.value(const ContiguousWindow(messages: []));
  return db.messagesDao.watchContiguous(
    ThreadId.parse(threadId),
    limit: ref.watch(historyLimitProvider(threadId)),
  );
}

/// The outbox entries of [threadId] that are not confirmed, with their display state
/// (04 13.2 item 3).
@riverpod
Stream<List<OutboxEntryView>> threadOutbox(Ref ref, String threadId) {
  final engine = ref.watch(syncEngineProvider);
  return engine == null
      ? Stream.value(const [])
      : engine.outbox.watchThread(ThreadId.parse(threadId));
}

/// The pinned messages of [threadId] (the section "Pinned" of the session column).
@riverpod
Stream<List<Message>> pinnedMessages(Ref ref, String threadId) {
  final db = ref.watch(localDatabaseProvider);
  return db == null
      ? Stream.value(const [])
      : db.messagesDao.watchPinned(ThreadId.parse(threadId));
}

/// The people typing in [threadId] (04 12 item 1), from the engine's ephemeral store.
@riverpod
Stream<Set<PrincipalId>> typingIn(Ref ref, String threadId) {
  final engine = ref.watch(syncEngineProvider);
  return engine == null
      ? Stream.value(const {})
      : engine.ephemeral.watchTyping(ThreadId.parse(threadId));
}

/// Whether the app window has the focus: in the foreground (`AppLifecycle`), not `inactive`
/// for Flutter, which is how macOS and Windows report a window behind another app, and active
/// for the desktop window (shown, not minimized, the foreground window; brief M4-30 Design
/// note 3, docs/DECISIONS.md D-M4-30-3). The history marks read and focuses only then, and the
/// focus reports `idle` otherwise (brief M4-20 Design note 2).
@riverpod
Stream<bool> windowFocused(Ref ref) {
  final lifecycle = ref.watch(appPortsProvider).lifecycle;
  final desktop = ref.watch(windowBehaviorProvider);
  final controller = StreamController<bool>();
  bool active(AppLifecycleState? s) =>
      s == null || s == AppLifecycleState.resumed;
  var window = active(WidgetsBinding.instance.lifecycleState);
  var foreground = lifecycle.current == AppActivity.foreground;
  var shown = desktop.active;
  void emit() {
    if (!controller.isClosed) controller.add(foreground && window && shown);
  }

  final shownSub = desktop.activeChanges.listen((a) {
    shown = a;
    emit();
  });

  final listener = AppLifecycleListener(
    onStateChange: (s) {
      window = active(s);
      emit();
    },
  );
  final sub = lifecycle.changes.listen((a) {
    foreground = a == AppActivity.foreground;
    emit();
  });
  emit();
  ref.onDispose(() {
    listener.dispose();
    unawaited(sub.cancel());
    unawaited(shownSub.cancel());
    unawaited(controller.close());
  });
  return controller.stream;
}

/// The engine's notices to the app (removed targets, an overwritten draft).
@riverpod
Stream<SyncNotice> syncNotices(Ref ref) {
  final engine = ref.watch(syncEngineProvider);
  return engine == null ? const Stream.empty() : engine.notices;
}

/// The stored read state of [threadId], or null.
@riverpod
ReadState? threadReadState(Ref ref, String threadId) {
  final id = ThreadId.parse(threadId);
  for (final r
      in ref.watch(allReadStatesProvider).value ?? const <ReadState>[]) {
    if (r.threadId == id) return r;
  }
  return null;
}

/// The stored draft of [threadId] in [channelId], or null.
@riverpod
Stream<Draft?> threadDraft(Ref ref, String channelId, String threadId) {
  final db = ref.watch(localDatabaseProvider);
  return db == null
      ? Stream.value(null)
      : db.personalDao.watchDraft(
          channelId: ChannelId.parse(channelId),
          threadId: ThreadId.parse(threadId),
        );
}

/// The personal templates and those of [channelId] (01 3.3), read when the menu opens.
@riverpod
Future<TemplateLists> templateLists(Ref ref, String channelId) =>
    ref.watch(threadReadsProvider).templates(ChannelId.parse(channelId));

/// The drafts being edited on this device: the registry of the signed-in instance, or a
/// registry of its own without one (tests).
@Riverpod(keepAlive: true)
DraftEditingRegistry draftEditing(Ref ref) =>
    ref.watch(signedInServicesProvider)?.draftEditing ?? DraftEditingRegistry();

/// Whether the session column shows (the header buttons "Column" and "Pin", 01 5.1 D1): next to
/// a forum thread it shows unless the person hid it; next to a thread of a text channel it is
/// collapsed until the person shows it (D2). It lasts while the app runs.
@Riverpod(keepAlive: true)
class SessionColumnShown extends _$SessionColumnShown {
  @override
  ({bool forum, bool textThread}) build() => (forum: true, textThread: false);

  /// Shows or hides the column of the layout ([textThread]: a thread of a text channel).
  void toggle({required bool textThread}) => state = textThread
      ? (forum: state.forum, textThread: !state.textThread)
      : (forum: !state.forum, textThread: state.textThread);

  /// Shows the column of the layout.
  void show({required bool textThread}) => state = textThread
      ? (forum: state.forum, textThread: true)
      : (forum: true, textThread: state.textThread);
}

/// What the server lets the person do in a channel, from its level, capabilities and the
/// read-only flag (05 action table; docs/DECISIONS.md D-M4-20-4); no derivation from grants.
final class ThreadPermissions {
  /// Creates the permissions.
  const ThreadPermissions({
    required this.canWrite,
    required this.canManage,
    required this.readOnly,
  });

  /// Nothing allowed: the channel or its access is not known yet.
  static const ThreadPermissions none = ThreadPermissions(
    canWrite: false,
    canManage: false,
    readOnly: false,
  );

  /// Level `write` or above and the channel is not read-only: send, reply, react, edit and
  /// delete own messages, Done, reopen, start a thread.
  final bool canWrite;

  /// Capability `manage`: pin, move, delete foreign messages.
  final bool canManage;

  /// Whether the channel is read-only for everyone (the composer shows the note).
  final bool readOnly;
}

/// The [ThreadPermissions] of [channelId].
@riverpod
ThreadPermissions threadPermissions(Ref ref, String channelId) {
  final channel = ref.watch(storedChannelOfRouteProvider(channelId)).value;
  final access = ref.watch(channelAccessProvider(channelId)).value;
  if (channel == null || access == null) return ThreadPermissions.none;
  final readOnly = channel.readOnly;
  final writes =
      access.level == AccessLevel.write || access.level == AccessLevel.trigger;
  return ThreadPermissions(
    canWrite: writes && !readOnly,
    canManage: access.capabilities.contains(Capability.manage),
    readOnly: readOnly || !writes,
  );
}

/// The visible channels that are no direct message and not archived, from the sidebar's
/// tree (move targets, 01 6.7).
@riverpod
List<Channel> visibleChannels(Ref ref) => [
  for (final node in ref.watch(sidebarTreeProvider))
    for (final c in node.channels) c.channel,
];

/// The threads of [channelId] (the thread starter lines of the main history, 01 5.1 D2).
@riverpod
Stream<List<Thread>> threadsOfChannel(Ref ref, String channelId) {
  final db = ref.watch(localDatabaseProvider);
  return db == null
      ? Stream.value(const [])
      : db.threadsDao.watchForChannel(ChannelId.parse(channelId));
}

/// The reactions of [messageId], ordered by emoji.
@riverpod
Stream<List<ReactionSummary>> messageReactions(Ref ref, String messageId) {
  final db = ref.watch(localDatabaseProvider);
  return db == null
      ? Stream.value(const [])
      : db.messagesDao.watchReactions(MessageId.parse(messageId));
}

/// The stored message [messageId] (the quote of a reply), or null.
@riverpod
Future<Message?> storedMessage(Ref ref, String messageId) async {
  final db = ref.watch(localDatabaseProvider);
  return db?.messagesDao.byId(MessageId.parse(messageId));
}

/// The threads on screen and whether the window is idle, sent to the gateway as `focus`
/// (04 11.3 item 2; brief M4-20 Design note 2): every history column adds its thread while it
/// shows and removes it when it closes. The state stays empty; the set lives in the notifier
/// and goes to `EngineFocus.setVisible`.
@Riverpod(keepAlive: true)
class VisibleThreads extends _$VisibleThreads {
  final Map<ThreadId, int> _shown = {};

  @override
  Set<ThreadId> build() {
    final engine = ref.watch(syncEngineProvider);
    ref.listen(windowFocusedProvider, (_, next) {
      final focused = next.value;
      if (focused != null) _push(engine, idle: !focused);
    });
    return const {};
  }

  /// [thread] is on screen.
  void add(ThreadId thread) {
    _shown.update(thread, (n) => n + 1, ifAbsent: () => 1);
    _changed();
  }

  /// [thread] left the screen.
  void remove(ThreadId thread) {
    final n = _shown[thread];
    if (n == null) return;
    if (n <= 1) {
      _shown.remove(thread);
    } else {
      _shown[thread] = n - 1;
    }
    _changed();
  }

  void _changed() {
    // No state change: the set goes to the gateway only, so a column may call this while the
    // widget tree builds.
    final focused = ref.read(windowFocusedProvider).value ?? true;
    _push(ref.read(syncEngineProvider), idle: !focused);
  }

  void _push(SyncEngine? engine, {required bool idle}) =>
      engine?.focus.setVisible(Set.of(_shown.keys), idle: idle);
}

/// Reads that answer nothing (no signed-in engine).
final class _NoReads implements ThreadReads {
  const _NoReads();

  @override
  Future<bool> loadOlder(ThreadId thread, ThreadSeq before) async => false;

  @override
  Future<void> loadRange(ThreadId thread, ThreadSeq from, ThreadSeq to) async {}

  @override
  Future<void> loadThread(ThreadId thread) async {}

  @override
  Future<void> loadChannelThreads(ChannelId channel) async {}

  @override
  Future<ThreadSeq?> positionOf(MessageId id) async => null;

  @override
  Future<List<Revision>> revisions(MessageId id) async => const [];

  @override
  Future<TemplateLists> templates(ChannelId channelId) async =>
      const TemplateLists(personal: [], channel: []);

  @override
  Future<void> loadPins(ThreadId thread) async {}
}
