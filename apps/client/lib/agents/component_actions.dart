// SPDX-License-Identifier: Apache-2.0

/// The use of a component through the outbox (09 11.3, 11.4; 01 6.5; components.md 8; brief
/// M4-21 Design note 3): one activation is one `createInteraction` with a new
/// `client_interaction_id` (the outbox entry's ID, which never changes on retry); the card
/// shows "sent and waiting" until the echo `message.updated` changes it, and a second
/// activation of a card while one of its interactions waits is ignored.
library;

import 'package:domain/domain.dart';
import 'package:l10n/l10n.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

import '../sync/agent_ops.dart';
import 'components_mapping.dart';

/// The pending and the refused interaction of one card, read from the thread's outbox.
typedef CardOutboxState = ({String? pendingCustomId, String? errorText});

/// The state of the card [messageId] in the thread's [outbox]: the `custom_id` of an
/// interaction that is not confirmed and not failed, and the translated problem of the newest
/// failed one (components.md 8 "error": the card is operable again while it is `active`).
CardOutboxState cardOutboxState(
  AppLocalizations l,
  MessageId messageId,
  List<OutboxEntryView> outbox,
) {
  String? pending;
  String? error;
  for (final v in outbox) {
    if (v.entry.kind != AgentOps.createInteraction) continue;
    final p = v.payload;
    if (p['message_id'] != messageId.value) continue;
    if (v.stateLabel == OutboxStateLabel.failed) {
      final code = v.failureCode;
      error = code == null
          ? null
          : problemText(l, code, reason: v.failureReason);
    } else {
      pending = p['custom_id'] as String?;
      error = null;
    }
  }
  return (pendingCustomId: pending, errorText: error);
}

/// Sends interactions of the cards of one thread through the outbox of [engine].
///
/// Example: `ComponentActions(engine).activate(card, component, value, outbox: views)`.
final class ComponentActions {
  /// Creates the actions on [engine].
  ComponentActions(this.engine);

  /// The engine whose outbox takes the interactions.
  final SyncEngine engine;

  /// Cards with an activation between the tap and the stored entry; the outbox view does not
  /// show the entry before it is stored.
  final Set<MessageId> _inFlight = {};

  /// Puts one interaction with [component] of [message] and the activation's [value] into
  /// the outbox and returns the entry, or null when the card already waits for one (in
  /// [outbox], the thread's entries, or an activation not stored yet): a double click creates
  /// nothing twice (01 6.5). Failed entries do not block; the person may try again.
  Future<OutboxEntry?> activate(
    Message message,
    MessageComponent component,
    Object? value, {
    required List<OutboxEntryView> outbox,
  }) async {
    final waiting = outbox.any(
      (v) =>
          v.entry.kind == AgentOps.createInteraction &&
          v.stateLabel != OutboxStateLabel.failed &&
          v.payload['message_id'] == message.id.value,
    );
    if (waiting || !_inFlight.add(message.id)) return null;
    try {
      return await AgentOps.enqueueInteraction(
        engine,
        thread: message.threadId,
        message: message.id,
        component: component,
        values: interactionValues(component, value),
      );
    } finally {
      _inFlight.remove(message.id);
    }
  }
}
