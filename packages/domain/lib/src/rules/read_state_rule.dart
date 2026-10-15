// SPDX-License-Identifier: Apache-2.0

/// The read state rule of 04 14 (R-SY-13): the read position only increases; moving it back
/// ("mark as unread") is a separate operation.
library;

import '../entities/personal.dart';

/// Merges an [incoming] read state of the same thread into the [local] one.
///
/// An [incoming] state with an older version than [local] is stale (a REST page or snapshot
/// loaded before a later change) and changes nothing (04 9 item 3: the version of the
/// personal sub-object decides). Otherwise the update keeps `lastReadSeq` monotonic: the
/// result has the greater of both positions, the newer version and the counters the server
/// computed for [incoming] (04 14).
///
/// [reset] marks [incoming] as the result of the separate reset operation
/// (`markThreadUnread`); with a newer version it replaces [local] entirely, so the position
/// can go down. A reset with an older or equal version is stale and changes nothing.
ReadState mergeReadState(
  ReadState local,
  ReadState incoming, {
  bool reset = false,
}) {
  final newer = incoming.version > local.version;
  if (reset) return newer ? incoming : local;
  if (incoming.version < local.version) return local;
  if (incoming.lastReadSeq >= local.lastReadSeq) return incoming;
  // A newer state with a lower position: keep the higher position; the server's counters
  // refer to the lower one, so the unread count is left to the caller to recompute.
  return ReadState(
    threadId: local.threadId,
    channelId: incoming.channelId,
    lastReadSeq: local.lastReadSeq,
    mentionCount: incoming.mentionCount,
    version: incoming.version,
    updatedByDeviceId: incoming.updatedByDeviceId,
    updatedAt: incoming.updatedAt,
  );
}
