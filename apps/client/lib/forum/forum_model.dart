// SPDX-License-Identifier: Apache-2.0

/// The view model of the forum post list and its pure rules (components.md 6; 01 6.1; brief
/// M4-19 Design notes 1 and 3; docs/DECISIONS.md D-M4-19-3): filters, order with the Done
/// section, "Waiting for me", the row data of M4-07 and the time label.
library;

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:l10n/l10n.dart';

/// The view chips of the post list: every open post, or only those waiting for the reader.
enum PostView {
  /// "Open" (the default).
  open,

  /// "Waiting for me".
  waitingForMe,
}

/// The filters of the post list header (components.md 6).
final class PostListFilter {
  /// Creates a filter; the default shows every open post in the forum's default order.
  const PostListFilter({
    this.view = PostView.open,
    this.tags = const {},
    this.sort,
  });

  /// The view chip.
  final PostView view;

  /// The selected tags; a post matches when it carries at least one of them. Empty: no tag
  /// filter.
  final Set<ForumTagId> tags;

  /// The chosen sort; null is the forum's `forum_default_sort`.
  final ForumSort? sort;

  /// A copy with the given fields replaced; [clearSort] goes back to the forum's default.
  PostListFilter copyWith({
    PostView? view,
    Set<ForumTagId>? tags,
    ForumSort? sort,
    bool clearSort = false,
  }) => PostListFilter(
    view: view ?? this.view,
    tags: tags ?? this.tags,
    sort: clearSort ? null : sort ?? this.sort,
  );

  /// The sort in effect for [forum]: the chosen one, else `forum_default_sort`, else last
  /// activity.
  ForumSort sortFor(Channel forum) {
    final s = sort ?? forum.forumDefaultSort;
    return s == ForumSort.created ? ForumSort.created : ForumSort.lastActivity;
  }

  @override
  bool operator ==(Object other) =>
      other is PostListFilter &&
      other.view == view &&
      other.sort == sort &&
      other.tags.length == tags.length &&
      other.tags.containsAll(tags);

  @override
  int get hashCode => Object.hash(
    view,
    sort,
    Object.hashAllUnordered(tags.map((t) => t.value)),
  );
}

/// A post of the list with what its row shows.
final class PostItem {
  /// Creates the item.
  const PostItem({
    required this.thread,
    required this.counts,
    this.activeRun,
    this.lastRun,
    this.waitingForMe = false,
  });

  /// The thread (forum post).
  final Thread thread;

  /// Unread messages and unread mentions of the reader (04 14).
  final ThreadCounts counts;

  /// The run of the thread that has not ended, or null.
  final RunSummary? activeRun;

  /// The newest run of the thread, or null.
  final RunSummary? lastRun;

  /// Whether a run the reader started asks a question here (the badge "Waiting for you").
  final bool waitingForMe;

  /// The row data of `DsThreadRow` (M4-07).
  ///
  /// [tags] are the tags of the forum by ID (unknown IDs are left out), [names] the display
  /// names of agents for the operator, [timeLabel] the formatted last activity ([timeLabel]).
  /// The failure reason shows when the newest run failed and none is active.
  DsThreadRowData rowData(
    AppLocalizations l, {
    required Map<ForumTagId, ForumTag> tags,
    required Map<PrincipalId, String> names,
    required String timeLabel,
  }) {
    final run = activeRun;
    final last = lastRun;
    final failed = run == null && last?.state == RunState.failed;
    final operator = thread.operatorAgentId;
    return DsThreadRowData(
      id: thread.id.value,
      title: thread.title,
      timeLabel: timeLabel,
      unreadCount: counts.unread,
      mentionCount: counts.mentions,
      runState: run == null ? null : DsRunState.fromCode(run.state.code),
      waitingForMe: waitingForMe,
      operatorName: operator == null ? null : names[operator],
      tags: [
        for (final id in thread.tagIds)
          if (tags[id] case final tag?)
            DsTagData(tag.name, DsCategory.fromCode(tag.color)),
      ],
      errorText: failed
          ? dsReasonText(l, last!.stateReason) ?? l.liveBoxStateFailed
          : null,
      done: thread.state == ThreadState.closed,
    );
  }
}

/// The post list: open posts and the Done section (01 6.1).
final class PostListModel {
  /// Creates the model.
  const PostListModel({required this.open, required this.done});

  /// The open posts matching the filters, in order.
  final List<PostItem> open;

  /// The done posts matching the filters, in the same order.
  final List<PostItem> done;
}

/// Builds the post list of [channel] from the stored [threads] (brief M4-19 Design note 1).
///
/// [counts] are the reader's counters per thread, [runs] the stored run summaries of the
/// channel, [me] the reader. The order is `ForumOrder` of M4-08 with [PostListFilter.sortFor];
/// the tag filter keeps posts with at least one selected tag; "Waiting for me" keeps posts
/// whose run the reader started waits for input (`waiting_input`) or that hold an unread
/// mention of the reader (docs/DECISIONS.md D-M4-19-3). Done posts follow the same filters.
PostListModel buildPostList({
  required List<Thread> threads,
  required Channel channel,
  required PostListFilter filter,
  required Map<ThreadId, ThreadCounts> counts,
  required List<RunSummary> runs,
  required PrincipalId? me,
}) {
  final active = <ThreadId, RunSummary>{};
  final last = <ThreadId, RunSummary>{};
  for (final r in runs) {
    final previous = last[r.threadId];
    if (previous == null || r.queuedAt.isAfter(previous.queuedAt)) {
      last[r.threadId] = r;
    }
    if (_activeStates.contains(r.state)) {
      final a = active[r.threadId];
      if (a == null || r.queuedAt.isAfter(a.queuedAt)) active[r.threadId] = r;
    }
  }
  PostItem item(Thread t) {
    final run = active[t.id];
    final c = counts[t.id] ?? ThreadCounts.zero;
    return PostItem(
      thread: t,
      counts: c,
      activeRun: run,
      lastRun: last[t.id],
      waitingForMe:
          run != null &&
          me != null &&
          run.state == RunState.waitingInput &&
          run.triggeredBy == me,
    );
  }

  bool matches(PostItem i) {
    if (filter.tags.isNotEmpty && !i.thread.tagIds.any(filter.tags.contains)) {
      return false;
    }
    if (filter.view == PostView.waitingForMe) {
      return i.waitingForMe || i.counts.mentions > 0;
    }
    return true;
  }

  final ordered = ForumOrder.sort([
    for (final t in threads)
      if (t.channelId == channel.id) t,
  ], filter.sortFor(channel));
  return PostListModel(
    open: [
      for (final t in ordered.open)
        if (item(t) case final i when matches(i)) i,
    ],
    done: [
      for (final t in ordered.done)
        if (item(t) case final i when matches(i)) i,
    ],
  );
}

const Set<RunState> _activeStates = {
  RunState.queued,
  RunState.running,
  RunState.waitingInput,
};

/// The time of a row: the clock time for [instant] on the day of [now], else the day
/// (`Yesterday`, the weekday, the date) through [format].
///
/// Both are seen in the device's time zone: the local store does not hold the person's
/// `users.time_zone`, so the wall clock of this device is formatted as UTC
/// (docs/DECISIONS.md D-M4-19-7).
String timeLabel(
  DateTime instant, {
  required DateTime now,
  required L10nFormat format,
}) {
  DateTime wall(DateTime t) {
    final local = t.toLocal();
    return DateTime.utc(
      local.year,
      local.month,
      local.day,
      local.hour,
      local.minute,
    );
  }

  final at = wall(instant);
  final today = wall(now);
  if (at.year == today.year && at.month == today.month && at.day == today.day) {
    return format.time(at, timeZone: 'UTC');
  }
  return format.relativeDay(at, now: today, timeZone: 'UTC');
}
