// SPDX-License-Identifier: Apache-2.0

/// Unread and mention counters between two server values (04 14): the server delivers the
/// computed values in the snapshot, `listMyReadStates` and `read_state.updated`; in between the
/// client counts from `message.created`, `message.updated` and `message.deleted`.
library;

import '../entities/message.dart';
import '../entities/personal.dart';
import '../ids.dart';
import '../seq.dart';

/// A `message.*` event as the counter needs it.
sealed class MessageEvent {
  const MessageEvent();

  /// Thread of the message.
  ThreadId get threadId;
}

/// `message.created` with the new message.
final class MessageCreated extends MessageEvent {
  /// Creates the event.
  const MessageCreated(this.message);

  /// The message.
  final Message message;

  @override
  ThreadId get threadId => message.threadId;
}

/// `message.updated` with the message after the change (also a redacted tombstone).
final class MessageUpdated extends MessageEvent {
  /// Creates the event; [previous] is the locally stored message before the change, if any.
  const MessageUpdated(this.message, {this.previous});

  /// The message after the change.
  final Message message;

  /// The message before the change from the local store; without it the counter can only
  /// adjust messages it counted itself.
  final Message? previous;

  @override
  ThreadId get threadId => message.threadId;
}

/// `message.deleted` (payload `MessageDeletedPayload`).
final class MessageDeleted extends MessageEvent {
  /// Creates the event; [previous] is the locally stored message before the deletion, if any.
  const MessageDeleted({
    required this.messageId,
    required this.threadId,
    required this.threadSeq,
    this.previous,
  });

  /// The deleted message.
  final MessageId messageId;

  /// The message before the deletion from the local store; with it a mention the server
  /// counted is taken back too.
  final Message? previous;

  @override
  final ThreadId threadId;

  /// Its position, kept as a tombstone (04 4).
  final ThreadSeq threadSeq;
}

/// Unread messages and unread mentions of one thread.
final class ThreadCounts {
  /// Creates counts.
  const ThreadCounts({required this.unread, required this.mentions});

  /// No unread message.
  static const ThreadCounts zero = ThreadCounts(unread: 0, mentions: 0);

  /// Unread messages.
  final int unread;

  /// Unread mentions of the person.
  final int mentions;

  @override
  bool operator ==(Object other) =>
      other is ThreadCounts &&
      other.unread == unread &&
      other.mentions == mentions;

  @override
  int get hashCode => Object.hash(unread, mentions);

  @override
  String toString() => 'ThreadCounts(unread: $unread, mentions: $mentions)';
}

final class _Thread {
  int baseUnread = 0;
  int baseMentions = 0;
  final Map<MessageId, ThreadSeq> unread = {};
  final Map<MessageId, ThreadSeq> mentions = {};
}

/// Counts unread messages and mentions per thread from message events (04 14).
///
/// The counter starts from the server's values ([resetFrom]) and adds what it saw itself; it
/// can take back only what it counted itself (an edit that removes a mention, a deletion).
/// Messages of the person, messages at or below the read position and imported messages at or
/// below `unread_floor_seq` never count.
final class UnreadCounter {
  final Map<ThreadId, _Thread> _threads = {};

  /// The counts of [thread]; zero for a thread without data.
  ThreadCounts countsOf(ThreadId thread) {
    final t = _threads[thread];
    if (t == null) return ThreadCounts.zero;
    return ThreadCounts(
      unread: t.baseUnread + t.unread.length,
      mentions: t.baseMentions + t.mentions.length,
    );
  }

  /// Applies one message event for the person [me] whose read position in the thread is
  /// [lastReadSeq]; [unreadFloorSeq] is the thread's `unread_floor_seq`.
  void apply(
    MessageEvent event, {
    required PrincipalId me,
    required ThreadSeq lastReadSeq,
    ThreadSeq unreadFloorSeq = const ThreadSeq(0),
  }) {
    final t = _threads.putIfAbsent(event.threadId, _Thread.new);
    bool counts(ThreadSeq seq) => seq > lastReadSeq && seq > unreadFloorSeq;
    switch (event) {
      case MessageCreated(:final message):
        if (message.authorId == me ||
            message.deletedAt != null ||
            !counts(message.threadSeq)) {
          return;
        }
        t.unread[message.id] = message.threadSeq;
        if (message.mentionIds.contains(me)) {
          t.mentions[message.id] = message.threadSeq;
        }
      case MessageUpdated(:final message, :final previous):
        if (message.deletedAt != null) {
          _forget(t, message.id, previous, me, counts);
          return;
        }
        if (message.authorId == me || !counts(message.threadSeq)) return;
        final isMention = message.mentionIds.contains(me);
        // Only a change of the mention counts (04 14); whether it changed is known from the
        // previous local copy or from what this counter counted itself.
        final wasMention = previous != null
            ? previous.mentionIds.contains(me)
            : t.mentions.containsKey(message.id);
        if (previous == null && !t.unread.containsKey(message.id)) return;
        if (isMention && !wasMention) {
          t.mentions[message.id] = message.threadSeq;
        } else if (!isMention && wasMention) {
          if (t.mentions.remove(message.id) == null && t.baseMentions > 0) {
            t.baseMentions--;
          }
        }
      case MessageDeleted(:final messageId, :final previous):
        _forget(t, messageId, previous, me, counts);
    }
  }

  /// Takes a deleted message out of the counts. A mention of a deleted message never counts
  /// (03 `read_states`); if this counter did not count it itself, the server did, as long as
  /// the [previous] copy mentioned [me] above the read position.
  void _forget(
    _Thread t,
    MessageId id,
    Message? previous,
    PrincipalId me,
    bool Function(ThreadSeq) counts,
  ) {
    t.unread.remove(id);
    if (t.mentions.remove(id) != null) return;
    if (previous != null &&
        previous.deletedAt == null &&
        previous.authorId != me &&
        previous.mentionIds.contains(me) &&
        counts(previous.threadSeq) &&
        t.baseMentions > 0) {
      t.baseMentions--;
    }
  }

  /// Takes the server's computed values from [state] (snapshot, `listMyReadStates`,
  /// `read_state.updated`) for a thread whose stored `last_message_seq` is [lastMessageSeq]
  /// and whose `unread_floor_seq` is [unreadFloorSeq].
  ///
  /// Mentions come from [ReadState.mentionCount]. Unread messages come from
  /// [ReadState.unreadCount] where the server delivered it, otherwise from the rule of 03
  /// `read_states`: `last_message_seq - GREATEST(last_read_seq, unread_floor_seq)`. Messages
  /// this counter saw above [lastMessageSeq] stay counted on top.
  void resetFrom(
    ReadState state, {
    required ThreadSeq lastMessageSeq,
    ThreadSeq unreadFloorSeq = const ThreadSeq(0),
  }) {
    final t = _threads.putIfAbsent(state.threadId, _Thread.new);
    t.baseMentions = state.mentionCount;
    t.mentions.removeWhere((_, seq) => seq <= lastMessageSeq);
    final floor = state.lastReadSeq > unreadFloorSeq
        ? state.lastReadSeq
        : unreadFloorSeq;
    final computed = lastMessageSeq.value - floor.value;
    t.baseUnread = state.unreadCount ?? (computed > 0 ? computed : 0);
    t.unread.removeWhere(
      (_, seq) => seq <= lastMessageSeq || seq <= state.lastReadSeq,
    );
  }
}
