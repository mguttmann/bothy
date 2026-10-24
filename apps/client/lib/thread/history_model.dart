// SPDX-License-Identifier: Apache-2.0

/// The rows of the history (brief M4-20 Design note 1; 04 4, 04 13.2 item 3; 15 7.3 item 4):
/// the confirmed messages of the contiguous window in `thread_seq` order with their grouping,
/// a loading row for each gap, and the writes of the outbox after the last confirmed message.
library;

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

import '../files/upload_executor.dart';
import '../sync/agent_ops.dart';

/// One row of the history.
sealed class HistoryItem {
  const HistoryItem();

  /// A key that stays the same while the row is the same (the list keeps its state).
  String get key;
}

/// A confirmed message.
final class MessageItem extends HistoryItem {
  /// Creates the row.
  const MessageItem(
    this.message, {
    required this.continuation,
    this.started = const [],
  });

  /// The message.
  final Message message;

  /// Whether it continues the group of the message before (no avatar and header).
  final bool continuation;

  /// The threads started from this message (the main history of a text channel, 01 5.1 D2).
  final List<Thread> started;

  @override
  String get key => 'message.${message.id.value}';
}

/// Missing positions: a loading row until the engine filled them (never messages out of
/// order, 04 4).
final class GapItem extends HistoryItem {
  /// Creates the row.
  const GapItem(this.range, {required this.before});

  /// The missing positions.
  final SeqRange range;

  /// Whether the gap lies before the window (older messages), else after it.
  final bool before;

  @override
  String get key => before ? 'history.gapBefore' : 'history.gapAfter';
}

/// A new message of the outbox: sending, delayed or failed (04 13.2 items 3 and 5).
final class PendingItem extends HistoryItem {
  /// Creates the row.
  const PendingItem(this.view);

  /// The entry with its display state.
  final OutboxEntryView view;

  @override
  String get key => 'outbox.${view.id}';
}

/// Another write of the thread the server refused (an edit, a delete, a reaction, a pin):
/// the reason with the actions of 04 13.2 item 5 and 04 13.3.
final class FailedWriteItem extends HistoryItem {
  /// Creates the row.
  const FailedWriteItem(this.view);

  /// The failed entry.
  final OutboxEntryView view;

  @override
  String get key => 'outbox.${view.id}';
}

/// The rows of the history, oldest first: the gap before the window, the messages of [window]
/// grouped by [groupWindow] (`ui.message_group_window`) with the threads [threads] started from
/// them, the gap after the window, then the new messages of [outbox] in FIFO order and the
/// refused other writes.
///
/// Example: `buildHistory(window: w, outbox: views, groupWindow: config.messageGroupWindow)`.
List<HistoryItem> buildHistory({
  required ContiguousWindow window,
  required List<OutboxEntryView> outbox,
  required Duration groupWindow,
  List<Thread> threads = const [],
}) {
  final started = <MessageId, List<Thread>>{};
  for (final t in threads) {
    final parent = t.parentMessageId;
    if (parent != null) started.putIfAbsent(parent, () => []).add(t);
  }
  final items = <HistoryItem>[];
  final before = window.gapBefore;
  if (before != null) items.add(GapItem(before, before: true));
  Message? previous;
  for (final m in window.messages) {
    items.add(
      MessageItem(
        m,
        continuation:
            previous != null &&
            m.deletedAt == null &&
            previous.deletedAt == null &&
            isContinuation(previous, m, groupWindow),
        started: started[m.id] ?? const [],
      ),
    );
    previous = m;
  }
  final after = window.gapAfter;
  if (after != null) items.add(GapItem(after, before: false));
  for (final v in outbox) {
    if (v.entry.kind == MessageOps.createMessage) {
      items.add(PendingItem(v));
    } else if (v.stateLabel == OutboxStateLabel.failed &&
        v.entry.kind != PersonalOps.putThreadDraft &&
        v.entry.kind != PersonalOps.putReadState &&
        // A failed upload shows on the file of its message and in the composer (M4-22).
        v.entry.kind != UploadOps.upload &&
        // A refused interaction shows as the error of its card (M4-21, components.md 8).
        v.entry.kind != AgentOps.createInteraction) {
      items.add(FailedWriteItem(v));
    }
  }
  return items;
}

/// The last own message of [window] that can be edited (arrow up in the empty composer,
/// 01 7): a text message of [me] that is not deleted.
Message? lastOwnMessage(ContiguousWindow window, PrincipalId? me) {
  if (me == null) return null;
  for (final m in window.messages.reversed) {
    if (m.authorId == me && m.deletedAt == null && m.kind == MessageKind.text) {
      return m;
    }
  }
  return null;
}
