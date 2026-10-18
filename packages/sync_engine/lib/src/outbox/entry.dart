// SPDX-License-Identifier: Apache-2.0

/// The display view of an outbox entry (04 13.2 item 3, 15 7.5).
library;

import 'package:local_store/local_store.dart';

import 'executors.dart';

/// What the interface shows for an entry.
enum OutboxStateLabel {
  /// Waits for its turn in the thread or for an entry it depends on.
  pending,

  /// A request is in flight.
  sending,

  /// Backing off before the next attempt.
  waiting,

  /// Waiting longer than `client.outbox_stuck_after` since it was written: "Sending delayed"
  /// (15 7.5, docs/DECISIONS.md D-M4-15-7).
  delayed,

  /// Refused by the server; the person edits, sends again or discards it (04 13.2 item 5).
  failed,
}

/// An outbox entry with its display state, as `Outbox.watchThread` publishes it.
final class OutboxEntryView {
  /// Creates a view.
  const OutboxEntryView({
    required this.entry,
    required this.stateLabel,
    this.blockedByCode,
    this.blockedByReason,
  });

  /// The stored entry.
  final OutboxEntry entry;

  /// The display state.
  final OutboxStateLabel stateLabel;

  /// The problem code of a failed entry this one depends on (a failed upload), if any.
  final String? blockedByCode;

  /// The reason of that failure, if any.
  final String? blockedByReason;

  /// The entry's ID.
  String get id => entry.id;

  /// The problem code of this entry's own failure, if any.
  String? get failureCode => entry.lastErrorCode;

  /// The reason of this entry's own failure, if any.
  String? get failureReason => entry.lastErrorReason;

  /// The decoded request body, for example the text to show or copy.
  Map<String, Object?> get payload => decodePayload(entry.payload);

  @override
  bool operator ==(Object other) =>
      other is OutboxEntryView &&
      other.entry == entry &&
      other.stateLabel == stateLabel &&
      other.blockedByCode == blockedByCode &&
      other.blockedByReason == blockedByReason;

  @override
  int get hashCode =>
      Object.hash(entry, stateLabel, blockedByCode, blockedByReason);
}
