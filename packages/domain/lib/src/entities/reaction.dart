// SPDX-License-Identifier: Apache-2.0

/// Reaction summaries (15 6.1 `reactions_summary` of 03 `reactions`).
library;

import '../ids.dart';

/// Count of one emoji on one message and whether the person reacted with it. `reaction.*` carry absolute values and apply in `seq` order (04 9 item 3).
final class ReactionSummary {
  /// Creates a [ReactionSummary].
  const ReactionSummary({
    required this.messageId,
    required this.emoji,
    required this.count,
    required this.me,
  });

  /// Message.
  final MessageId messageId;

  /// One grapheme cluster.
  final String emoji;

  /// Number of reactions.
  final int count;

  /// Whether the person reacted with this emoji.
  final bool me;

  /// Returns a copy with the given fields replaced; a nullable field cannot be cleared
  /// through it (build a new object for that).
  ReactionSummary copyWith({int? count, bool? me}) => ReactionSummary(
    messageId: messageId,
    emoji: emoji,
    count: count ?? this.count,
    me: me ?? this.me,
  );

  @override
  bool operator ==(Object other) =>
      other is ReactionSummary &&
      other.messageId == messageId &&
      other.emoji == emoji &&
      other.count == count &&
      other.me == me;

  @override
  int get hashCode => Object.hash(messageId, emoji, count, me);
}
