// SPDX-License-Identifier: Apache-2.0

/// The rules of the status dots of the sidebar (docs/spec/01-product.md 6.17; brief M4-52 Design
/// notes 1, 2 and 4; docs/decisions.d/M4-52.md): the status of a chat from its newest run,
/// whether the person has seen it, and the most urgent unseen status of a channel. Pure
/// functions over the run, read and seen state the client already holds; the providers and the
/// rows of the sidebar use them on the desktop and the phone alike.
library;

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart' show ThreadSeenState;

/// The status of a chat whose newest run is in [state] (Design note 1): `queued` and `running`
/// are working, `waiting_input` is waiting (a question or approval card came first, 09 5),
/// `completed` is done and `failed` is failed; `cancelled` and a state of a newer server give
/// none.
DsChatStatus? chatStatusOf(RunState state) => switch (state) {
  RunState.queued || RunState.running => DsChatStatus.working,
  RunState.waitingInput => DsChatStatus.waiting,
  RunState.completed => DsChatStatus.done,
  RunState.failed => DsChatStatus.failed,
  RunState.cancelled || RunState.unknown => null,
};

/// The newest run of every thread among [runs], by `queued_at`; the run ID breaks ties, so the
/// result never depends on the input order.
Map<ThreadId, RunSummary> newestRuns(Iterable<RunSummary> runs) {
  final out = <ThreadId, RunSummary>{};
  for (final r in runs) {
    final previous = out[r.threadId];
    if (previous == null ||
        r.queuedAt.isAfter(previous.queuedAt) ||
        (r.queuedAt == previous.queuedAt &&
            r.id.value.compareTo(previous.id.value) > 0)) {
      out[r.threadId] = r;
    }
  }
  return out;
}

/// When [run] entered its current status, on the server's clock, or null when the summary does
/// not tell: working since `queued_at` (the live box anchor appears then; `started_at` keeps its
/// first value, so a run that resumes after an answer has no time of its own), done and failed
/// at `finished_at`, waiting never.
DateTime? statusChangedAt(RunSummary run) => switch (chatStatusOf(run.state)) {
  DsChatStatus.working => run.queuedAt,
  DsChatStatus.done || DsChatStatus.failed => run.finishedAt,
  DsChatStatus.waiting || null => null,
};

/// The status of the chat whose newest run is [newestRun] when the person has not seen it yet,
/// else null (01 6.17 items 2 and 3, Design note 2).
///
/// The status is seen when any of these holds:
/// 1. [open]: the chat shows on this device;
/// 2. [seen], this device's mark of the chat, names the same run in the same status;
/// 3. [readState] was written (`updated_at`, on any device) at or after [statusChangedAt];
/// 4. the status is not failed, there is no time to compare (no change time or no read time),
///    and [counts] has nothing unread and no unread mention: working, waiting and done come
///    with a message (the live box anchor, the question card, the answer before `run.finish`),
///    which the person has then read. A failure can come without a message.
///
/// Example:
/// ```dart
/// unseenChatStatus(newestRun: run, open: false, seen: mark, readState: read,
///     counts: counts); // DsChatStatus.done
/// ```
DsChatStatus? unseenChatStatus({
  required RunSummary? newestRun,
  required bool open,
  ThreadSeenState? seen,
  ReadState? readState,
  ThreadCounts counts = ThreadCounts.zero,
}) {
  final run = newestRun;
  if (run == null || open) return null;
  final status = chatStatusOf(run.state);
  if (status == null) return null;
  if (seen != null &&
      seen.runId == run.id &&
      chatStatusOf(seen.runState) == status) {
    return null;
  }
  final changed = statusChangedAt(run);
  final read = readState?.updatedAt;
  if (changed != null && read != null) {
    return read.isBefore(changed) ? status : null;
  }
  final nothingNew = counts.unread == 0 && counts.mentions == 0;
  if (status != DsChatStatus.failed && nothingNew) return null;
  return status;
}

/// The unseen status of every chat among [runs] that has one (Design note 2): [open] are the
/// chats on screen, [seen] the marks of this device, [readStates] the person's read states and
/// [counts] the counters per chat.
Map<ThreadId, DsChatStatus> unseenStatuses({
  required Iterable<RunSummary> runs,
  required Set<ThreadId> open,
  required Map<ThreadId, ThreadSeenState> seen,
  required Iterable<ReadState> readStates,
  required Map<ThreadId, ThreadCounts> counts,
}) {
  final reads = {for (final r in readStates) r.threadId: r};
  final out = <ThreadId, DsChatStatus>{};
  for (final MapEntry(key: thread, value: run) in newestRuns(runs).entries) {
    final status = unseenChatStatus(
      newestRun: run,
      open: open.contains(thread),
      seen: seen[thread],
      readState: reads[thread],
      counts: counts[thread] ?? ThreadCounts.zero,
    );
    if (status != null) out[thread] = status;
  }
  return out;
}

/// The most urgent of [statuses] for a forum or channel row (01 6.17 item 6): red over blue
/// over yellow over green; null without any.
DsChatStatus? aggregateStatus(Iterable<DsChatStatus?> statuses) =>
    DsChatStatus.mostUrgent(statuses);
