// SPDX-License-Identifier: Apache-2.0

/// The selection of the thread rows under a channel of the sidebar and the counters around
/// them (docs/spec/01-product.md 6.8 items 2 to 6; brief M4-40 Design notes 1 and 3; rulings
/// D-SB-2 to D-SB-5; brief M4-46, D-M4-46-1 to D-M4-46-4): a pure function the desktop sidebar
/// uses and the phone layout of M5-15 can reuse.
library;

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';

import 'sidebar_arrange.dart' show applyPersonalOrder;

/// The limits of the selection, from `AppConfig`: never a literal in code (brief M4-40 rule 9).
final class SidebarThreadLimits {
  /// Creates the limits.
  const SidebarThreadLimits({
    required this.window,
    required this.perChannel,
    required this.forumPostsVisible,
  });

  /// The limits of [config]: `ui.sidebar_thread_window`, `ui.sidebar_threads_per_channel` and
  /// `ui.sidebar_forum_posts_visible`.
  factory SidebarThreadLimits.of(AppConfig config) => SidebarThreadLimits(
    window: config.sidebarThreadWindow,
    perChannel: config.sidebarThreadsPerChannel,
    forumPostsVisible: config.sidebarForumPostsVisible,
  );

  /// `ui.sidebar_thread_window`: an open thread of a text channel with activity within it
  /// qualifies.
  final Duration window;

  /// `ui.sidebar_threads_per_channel`: at most this many rows per text channel.
  final int perChannel;

  /// `ui.sidebar_forum_posts_visible`: a forum group shows this many open posts before "Show
  /// more" (01 6.8 item 3, brief M4-46).
  final int forumPostsVisible;
}

/// The "Show more" row of a forum group (01 6.8 item 3, brief M4-46): the forum has more open
/// posts than `ui.sidebar_forum_posts_visible`.
final class SidebarMorePosts {
  /// Creates the row state.
  const SidebarMorePosts({required this.hidden, required this.expanded});

  /// While collapsed, the open posts the group does not show; while expanded, the open posts
  /// beyond the limit that "Show less" hides again.
  final int hidden;

  /// Whether the person expanded the group to every open post on this device (the row reads
  /// "Show less").
  final bool expanded;

  @override
  bool operator ==(Object other) =>
      other is SidebarMorePosts &&
      other.hidden == hidden &&
      other.expanded == expanded;

  @override
  int get hashCode => Object.hash(hidden, expanded);

  @override
  String toString() => 'SidebarMorePosts(hidden: $hidden, expanded: $expanded)';
}

/// A thread of a sidebar group with what its row shows.
final class SidebarThread {
  /// Creates the entry.
  const SidebarThread({
    required this.thread,
    required this.counts,
    this.activeRun,
    this.lastRun,
    this.waitingForMe = false,
    this.muted = false,
  });

  /// The thread (a forum post or a thread of a text channel).
  final Thread thread;

  /// Unread messages and unread mentions of the person (04 14).
  final ThreadCounts counts;

  /// The run of the thread that has not ended, or null.
  final RunSummary? activeRun;

  /// The newest run of the thread, or null.
  final RunSummary? lastRun;

  /// "Waiting for me" as the post list defines it (D-M4-19-3): a run the person triggered
  /// waits for input, or the thread holds an unread mention of the person.
  final bool waitingForMe;

  /// Whether the person muted the thread or its channel.
  final bool muted;

  /// The row data of `DsSidebarThreadRow`: the run state icon is the active run's state, else
  /// `failed` when the newest run failed, else none (01 6.8 item 6). An empty title stays empty;
  /// the row reads it as "New chat" with the "automatic" mark.
  DsSidebarThreadRowData get rowData {
    final run = activeRun;
    final failed = run == null && lastRun?.state == RunState.failed;
    return DsSidebarThreadRowData(
      id: thread.id.value,
      title: thread.title,
      runState: run != null
          ? DsRunState.fromCode(run.state.code)
          : (failed ? DsRunState.failed : null),
      unreadCount: counts.unread,
      mentionCount: counts.mentions,
      muted: muted,
      done: thread.state == ThreadState.closed,
    );
  }
}

/// The thread group of one channel: its rows and the counters of the channel row and of "All
/// posts".
final class SidebarThreadGroup {
  /// Creates the group.
  const SidebarThreadGroup({
    required this.rows,
    required this.candidates,
    required this.channelCounts,
    required this.allPostsCounts,
    this.more,
  });

  /// The rows in their order (01 6.8 item 4): in a text channel by creation time, in a forum by
  /// last activity, newest first.
  final List<SidebarThread> rows;

  /// Every thread that qualifies (01 6.8 item 2), before the bound: in a text channel in the
  /// order of the priority of item 3, in a forum by last activity; the rows are taken from its
  /// start.
  final List<SidebarThread> candidates;

  /// The "Show more" row of a forum with more open posts than the limit, or null (text
  /// channels, and forums within the limit).
  final SidebarMorePosts? more;

  /// The counters of the channel row (D-SB-5): every thread of the channel when the group is
  /// collapsed or has no rows; when expanded, a text channel counts its main thread and the
  /// threads not shown, and a forum nothing, because its rows and "All posts" show the rest.
  final ThreadCounts channelCounts;

  /// The counters of "All posts" of an expanded forum group with rows: the posts not shown
  /// (D-SB-4, never a total); null for text channels, a collapsed group and a group without rows.
  final ThreadCounts? allPostsCounts;

  /// Whether the group has thread rows; a channel without rows shows no arrow and no "All
  /// posts" (01 6.8 item 7).
  bool get hasRows => rows.isNotEmpty;
}

const Set<RunState> _activeStates = {
  RunState.queued,
  RunState.running,
  RunState.waitingInput,
};

/// The thread rows of [channel] and its counters (01 6.8 items 2 to 6).
///
/// [threads] are the stored threads (others than [channel]'s are ignored), [counts] the person's
/// counters per thread of the channel (the engine's values, else the stored read states; threads
/// that are not stored count too), [runs] the stored runs of the channel, [selected] the thread
/// the route shows, [me] the person, [now] the clock, [limits] from `AppConfig`.
///
/// In a text channel a thread qualifies when it is of kind `thread`, open (or selected) and
/// selected, has an active run, waits for [me], is unread or was active within
/// [SidebarThreadLimits.window]. When more qualify than [SidebarThreadLimits.perChannel], the
/// priority selected, waiting, active run, unread mentions, unread, newest activity decides; rows
/// are ordered by creation time, newest first.
///
/// In a forum every open post qualifies, and the selected one also when done (brief M4-46).
/// Rows are ordered by last activity, newest first; with more open posts than
/// [SidebarThreadLimits.forumPostsVisible] only the newest that many and the selected post show
/// until [showAllPosts] (the "Show more" state of this device) lists them all, and
/// [SidebarThreadGroup.more] describes the row.
///
/// [manualOrder] is the person's order of the forum's posts when "Arrange chats automatically" is
/// off (01 6.16, brief M4-51): the posts follow it, posts not in it come first by activity
/// (D-M4-51-3), the limit takes the first posts of that order, and [heldOrder] does not apply,
/// because activity never moves a row of a manual group and a drag must show at once. Null is the
/// order by activity. Text channels ignore it.
///
/// [keep] holds rows that stay although they no longer qualify (the pointer or the focus is in
/// the sidebar, 01 6.8 item 4); they stay while stored. In a forum [heldOrder] is the order of
/// the rows shown last while the sidebar holds them: those rows keep it, and rows new to the
/// group come first, so no row moves under the pointer. [expanded] is the state of the group,
/// [mutedThreads] the threads the person muted (a muted channel mutes its threads). Never
/// throws.
///
/// Example:
/// ```dart
/// final group = selectSidebarThreads(channel: forum, threads: stored, counts: counts,
///     runs: runs, selected: route.thread, me: me, now: clock.now(),
///     limits: SidebarThreadLimits.of(config));
/// ```
SidebarThreadGroup selectSidebarThreads({
  required Channel channel,
  required Iterable<Thread> threads,
  required Map<ThreadId, ThreadCounts> counts,
  required Iterable<RunSummary> runs,
  required ThreadId? selected,
  required PrincipalId? me,
  required DateTime now,
  required SidebarThreadLimits limits,
  bool expanded = true,
  Set<ThreadId> keep = const {},
  Set<ThreadId> mutedThreads = const {},
  bool channelMuted = false,
  bool showAllPosts = false,
  List<ThreadId> heldOrder = const [],
  List<ThreadId>? manualOrder,
}) {
  final active = <ThreadId, RunSummary>{};
  final last = <ThreadId, RunSummary>{};
  for (final r in runs) {
    if (r.channelId != channel.id) continue;
    final previous = last[r.threadId];
    if (previous == null || r.queuedAt.isAfter(previous.queuedAt)) {
      last[r.threadId] = r;
    }
    if (_activeStates.contains(r.state)) {
      final a = active[r.threadId];
      if (a == null || r.queuedAt.isAfter(a.queuedAt)) active[r.threadId] = r;
    }
  }
  SidebarThread entry(Thread t) {
    final run = active[t.id];
    final c = counts[t.id] ?? ThreadCounts.zero;
    return SidebarThread(
      thread: t,
      counts: c,
      activeRun: run,
      lastRun: last[t.id],
      waitingForMe:
          (run != null &&
              me != null &&
              run.state == RunState.waitingInput &&
              run.triggeredBy == me) ||
          c.mentions > 0,
      muted: channelMuted || mutedThreads.contains(t.id),
    );
  }

  final forum = channel.kind == ChannelKind.forum;
  final manual = forum ? manualOrder : null;
  final since = now.subtract(limits.window);
  bool qualifies(SidebarThread e) {
    final t = e.thread;
    if (t.id == selected) return true;
    // A done thread shows only while it is selected (01 6.8 item 2).
    if (t.state == ThreadState.closed) return false;
    // Every open post of a forum shows (brief M4-46, D-M4-46-1).
    if (forum) return true;
    return e.activeRun != null ||
        e.waitingForMe ||
        e.counts.unread > 0 ||
        e.counts.mentions > 0 ||
        !t.lastActivityAt.isBefore(since);
  }

  final own = [
    for (final t in threads)
      if (t.channelId == channel.id && t.kind == ThreadKind.thread) entry(t),
  ];
  final sorted = [
    for (final e in own)
      if (qualifies(e)) e,
  ]..sort(forum ? _byActivity : (a, b) => _byPriority(a, b, selected));
  final candidates = manual == null
      ? sorted
      : applyPersonalOrder(sorted, manual, (e) => e.thread.id);
  final Set<ThreadId> chosen;
  SidebarMorePosts? more;
  if (forum) {
    final visible = limits.forumPostsVisible < 0 ? 0 : limits.forumPostsVisible;
    final open = [
      for (final e in candidates)
        if (e.thread.state == ThreadState.open) e,
    ];
    chosen = {
      for (final e in showAllPosts ? open : open.take(visible)) e.thread.id,
      ?selected,
    };
    if (open.length > visible) {
      more = SidebarMorePosts(
        hidden: showAllPosts
            ? open.length - visible
            : open.where((e) => !_shownIn(e, chosen, keep)).length,
        expanded: showAllPosts,
      );
      // Kept rows can cover every post beyond the limit for a moment; then no row is hidden.
      if (!showAllPosts && more.hidden == 0) more = null;
    }
  } else {
    final bound = limits.perChannel < 0 ? 0 : limits.perChannel;
    chosen = {for (final e in candidates.take(bound)) e.thread.id};
  }
  final List<SidebarThread> rows;
  if (manual != null) {
    // A manual group keeps the order of [candidates]; a kept row that no longer qualifies
    // (selected before, done now) stays where it was by activity, after them.
    rows = [
      for (final e in candidates)
        if (_shownIn(e, chosen, keep)) e,
      for (final e in own)
        if (!candidates.contains(e) && keep.contains(e.thread.id)) e,
    ];
  } else {
    rows = [
      for (final e in own)
        if (_shownIn(e, chosen, keep)) e,
    ]..sort(forum ? _heldThenActivity(heldOrder) : _byCreation);
  }

  var total = ThreadCounts.zero;
  for (final c in counts.values) {
    total = _plus(total, c);
  }
  var shown = ThreadCounts.zero;
  for (final r in rows) {
    shown = _plus(shown, r.counts);
  }
  final grouped = expanded && rows.isNotEmpty;
  final rest = _minus(total, shown);
  return SidebarThreadGroup(
    rows: rows,
    candidates: candidates,
    channelCounts: !grouped ? total : (forum ? ThreadCounts.zero : rest),
    allPostsCounts: grouped && forum ? rest : null,
    more: more,
  );
}

bool _shownIn(SidebarThread e, Set<ThreadId> chosen, Set<ThreadId> keep) =>
    chosen.contains(e.thread.id) || keep.contains(e.thread.id);

/// Last activity, newest first; the creation time, newest first, then the ID break ties, so the
/// result never depends on the input order (brief M4-46).
int _byActivity(SidebarThread a, SidebarThread b) {
  final byActivity = b.thread.lastActivityAt.compareTo(a.thread.lastActivityAt);
  if (byActivity != 0) return byActivity;
  return _byCreation(a, b);
}

/// The order of forum rows while the sidebar holds them (01 6.8 item 4, D-M4-46-3): rows not in
/// [held] first by [_byActivity], then the held rows in their held order. Without [held] it is
/// [_byActivity].
Comparator<SidebarThread> _heldThenActivity(List<ThreadId> held) {
  if (held.isEmpty) return _byActivity;
  final place = {for (final (i, id) in held.indexed) id: i};
  return (a, b) {
    final pa = place[a.thread.id];
    final pb = place[b.thread.id];
    if (pa == null && pb == null) return _byActivity(a, b);
    if (pa == null) return -1;
    if (pb == null) return 1;
    return pa.compareTo(pb);
  };
}

/// The priority of 01 6.8 item 3: selected, waiting for the person, active run, unread
/// mentions, unread, newest activity; the thread ID breaks ties, so the result never depends on
/// the input order.
int _byPriority(SidebarThread a, SidebarThread b, ThreadId? selected) {
  int rank(SidebarThread e) {
    if (e.thread.id == selected) return 0;
    if (e.waitingForMe) return 1;
    if (e.activeRun != null) return 2;
    if (e.counts.mentions > 0) return 3;
    if (e.counts.unread > 0) return 4;
    return 5;
  }

  final byRank = rank(a).compareTo(rank(b));
  if (byRank != 0) return byRank;
  final byActivity = b.thread.lastActivityAt.compareTo(a.thread.lastActivityAt);
  if (byActivity != 0) return byActivity;
  return b.thread.id.value.compareTo(a.thread.id.value);
}

/// Creation time, newest first; the ID (UUIDv7, time ordered) breaks ties.
int _byCreation(SidebarThread a, SidebarThread b) {
  final byCreated = b.thread.createdAt.compareTo(a.thread.createdAt);
  if (byCreated != 0) return byCreated;
  return b.thread.id.value.compareTo(a.thread.id.value);
}

ThreadCounts _plus(ThreadCounts a, ThreadCounts b) => ThreadCounts(
  unread: a.unread + b.unread,
  mentions: a.mentions + b.mentions,
);

/// [a] minus [b], never below zero (an engine value and a read state can briefly disagree).
ThreadCounts _minus(ThreadCounts a, ThreadCounts b) => ThreadCounts(
  unread: a.unread - b.unread < 0 ? 0 : a.unread - b.unread,
  mentions: a.mentions - b.mentions < 0 ? 0 : a.mentions - b.mentions,
);
