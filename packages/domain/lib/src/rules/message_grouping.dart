// SPDX-License-Identifier: Apache-2.0

/// Message grouping of the history (01 14 `ui.message_group_window`): a message of the same
/// author shortly after the previous one gets no header of its own.
library;

import '../entities/message.dart';

/// Whether [next] continues the group of [previous] and is shown without its own header.
///
/// True when both have the same author and the same kind class (text and voice are one class;
/// run anchors and cards each their own), neither is a system line, and [next] was created
/// less than [window] after [previous]. [window] is `ui.message_group_window`
/// (`AppConfig.messageGroupWindow`).
bool isContinuation(Message previous, Message next, Duration window) {
  if (previous.kind == MessageKind.system || next.kind == MessageKind.system) {
    return false;
  }
  if (previous.authorId != next.authorId) return false;
  if (_kindClass(previous.kind) != _kindClass(next.kind)) return false;
  final gap = next.createdAt.difference(previous.createdAt);
  return !gap.isNegative && gap < window;
}

int _kindClass(MessageKind kind) => switch (kind) {
  MessageKind.text || MessageKind.voice => 0,
  MessageKind.run => 1,
  MessageKind.card => 2,
  MessageKind.system || MessageKind.unknown => 3,
};
