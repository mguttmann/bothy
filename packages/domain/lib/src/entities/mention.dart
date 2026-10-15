// SPDX-License-Identifier: Apache-2.0

/// Mentions of the person themselves (15 6.1 `my_mentions` of 03 `mentions`).
library;

import '../ids.dart';

/// A mention of the person in a message (03 `mentions`).
final class Mention {
  /// Creates a [Mention].
  const Mention({
    required this.messageId,
    required this.principalId,
    required this.threadId,
  });

  /// Message.
  final MessageId messageId;

  /// Mentioned principal.
  final PrincipalId principalId;

  /// Thread of the message.
  final ThreadId threadId;

  /// Returns a copy with the given fields replaced; a nullable field cannot be cleared
  /// through it (build a new object for that).
  Mention copyWith({ThreadId? threadId}) => Mention(
    messageId: messageId,
    principalId: principalId,
    threadId: threadId ?? this.threadId,
  );

  @override
  bool operator ==(Object other) =>
      other is Mention &&
      other.messageId == messageId &&
      other.principalId == principalId &&
      other.threadId == threadId;

  @override
  int get hashCode => Object.hash(messageId, principalId, threadId);
}
