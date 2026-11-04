// SPDX-License-Identifier: Apache-2.0

/// The line beside the red X of a message that failed because the project or the thread's
/// agent was not ready (01 6.9 item 8; 09 3.8; docs/decisions.d/F-PROJEKT-BEREIT.md): the cause
/// and, for the author, "Trigger again", which puts one `retriggerMessage` into the outbox.
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';
import 'package:sync_engine/sync_engine.dart';

import '../app/providers/session.dart';
import '../sync/message_ops_app.dart';

/// The trigger reasons a message can be triggered again for (09 3.8).
const Set<String> retriggerReasons = {
  'project_not_ready',
  'operator_cannot_serve_project',
  'operator_not_in_channel',
};

/// Whether [message] failed for a reason "Trigger again" fixes once the cause is gone.
bool offersRetrigger(Message message) =>
    message.deletedAt == null &&
    message.triggerState == TriggerState.failed &&
    retriggerReasons.contains(message.triggerStateReason);

/// What became of one "Trigger again": null while it waits or after the server accepted it,
/// otherwise the code and reason of the refusal.
typedef RetriggerRefusal = ({String code, String? reason});

/// Sends "Trigger again" for a message and reports a refusal; the app's implementation goes
/// through the outbox ([outboxRetrigger]), tests pass their own.
typedef Retrigger = Future<RetriggerRefusal?> Function(MessageId message);

/// [Retrigger] over the outbox of [engine]: one `retriggerMessage` without a thread in the lane
/// `message:<id>`; a refused entry is discarded and its problem returned.
Retrigger outboxRetrigger(SyncEngine engine) => (message) async {
  final entry = await engine.outbox.enqueue(
    OutboxRequest(
      kind: AppMessageOps.retriggerMessage,
      payload: {'message_id': message.value},
    ),
  );
  await for (final views in engine.outbox.watchThread(null)) {
    final mine = views.where((v) => v.id == entry.id).firstOrNull;
    if (mine == null) return null;
    if (mine.stateLabel == OutboxStateLabel.failed) {
      unawaited(engine.outbox.discard(mine.id));
      return (code: mine.failureCode ?? 'unknown', reason: mine.failureReason);
    }
  }
  return null;
};

/// The notice of [message] for the reader whose author check is [mine]; nothing for a message
/// that [offersRetrigger] does not cover.
class RetriggerNotice extends ConsumerStatefulWidget {
  /// Creates the notice; [retrigger] replaces the outbox (tests).
  const RetriggerNotice({
    super.key,
    required this.message,
    required this.mine,
    this.retrigger,
  });

  /// The message.
  final Message message;

  /// Whether the reader wrote it (only the author may trigger it again).
  final bool mine;

  /// How "Trigger again" is sent; null uses the outbox of the signed-in session.
  final Retrigger? retrigger;

  @override
  ConsumerState<RetriggerNotice> createState() => _RetriggerNoticeState();
}

class _RetriggerNoticeState extends ConsumerState<RetriggerNotice> {
  bool _busy = false;
  RetriggerRefusal? _refusal;

  Future<void> _send() async {
    if (_busy) return;
    var send = widget.retrigger;
    if (send == null) {
      final engine = ref.read(syncEngineProvider);
      if (engine == null) return;
      send = outboxRetrigger(engine);
    }
    setState(() {
      _busy = true;
      _refusal = null;
    });
    final refusal = await send(widget.message.id);
    if (!mounted) return;
    setState(() {
      _refusal = refusal;
      // Accepted: the message's new trigger state replaces this notice; refused: offer again.
      _busy = refusal == null;
    });
  }

  @override
  Widget build(BuildContext context) {
    final message = widget.message;
    if (!offersRetrigger(message)) return const SizedBox.shrink();
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final reason = message.triggerStateReason;
    final cause = dsReasonText(l, reason) ?? reason ?? '';
    final refusal = _refusal;
    return Padding(
      key: ValueKey('retrigger.${message.id.value}'),
      padding: EdgeInsets.only(
        left: context.dsDensity.messageGutter + DsSpace.s8,
        right: DsSpace.s8,
        top: DsSpace.s2,
      ),
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  cause,
                  key: const ValueKey('retrigger.cause'),
                  style: dsTextStyle(DsType.bodySm)
                      .copyWith(color: colors.dangerFg),
                ),
                if (refusal != null)
                  Text(
                    l.retriggerRefused(
                      (refusal.reason == null
                              ? null
                              : dsReasonText(l, refusal.reason)) ??
                          problemText(l, refusal.code, reason: refusal.reason),
                    ),
                    key: const ValueKey('retrigger.refused'),
                    style: dsTextStyle(DsType.bodySm)
                        .copyWith(color: colors.textSecondary),
                  ),
              ],
            ),
          ),
          if (widget.mine)
            Semantics(
              label: l.retriggerMessageFor,
              excludeSemantics: true,
              button: true,
              child: DsButton(
                key: const ValueKey('retrigger.button'),
                label: l.retriggerMessage,
                loading: _busy,
                onPressed: _busy ? null : () => unawaited(_send()),
              ),
            ),
        ],
      ),
    );
  }
}
