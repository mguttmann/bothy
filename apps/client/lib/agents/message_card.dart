// SPDX-License-Identifier: Apache-2.0

/// The body of a card message (`kind = card`, 09 11; 01 6.5, D4; components.md 8; SEC-OUT-4,
/// SEC-T14; brief M4-21 Design note 3): `DsMessageComponents` with the header from server
/// fields, the agent's text below the divider, the components of the local table, the
/// reader's permission and the state of the card's interactions in the outbox.
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';

import '../app/providers/ports.dart';
import '../forum/forum_model.dart' show timeLabel;
import '../notifications_settings/providers.dart' show isOwnerProvider;
import '../thread/providers.dart';
import 'component_actions.dart';
import 'components_mapping.dart';
import 'providers.dart';

/// The components of the card [message], or null while it has none stored.
class MessageCard extends ConsumerWidget {
  /// Creates the card body; [actions] sends the interactions of the thread.
  const MessageCard({
    super.key,
    required this.message,
    required this.actions,
    required this.onOpenLink,
  });

  /// The card message.
  final Message message;

  /// The interaction sender of the thread, or null without an engine.
  final ComponentActions? actions;

  /// Opens a link of the agent's text.
  final void Function(Uri) onOpenLink;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context);
    final components =
        ref
            .watch(
              messageComponentsProvider(
                message.id.value,
                message.version.value,
              ),
            )
            .value ??
        const <MessageComponent>[];
    if (components.isEmpty) return const SizedBox.shrink();
    final principals = ref.watch(principalsByIdProvider).value ?? const {};
    final outbox =
        ref.watch(threadOutboxProvider(message.threadId.value)).value ??
        const [];
    final access = ref
        .watch(channelAccessProvider(message.channelId.value))
        .value;
    final me = ref.watch(myPrincipalIdProvider);
    final owner = ref.watch(isOwnerProvider).value ?? false;
    final runs =
        ref.watch(threadRunsProvider(message.threadId.value)).value ?? const [];
    final runId = components.first.runId;
    RunSummary? run;
    for (final r in runs) {
      if (r.id == runId) run = r;
    }
    final starter = run == null
        ? null
        : principals[run.triggeredBy]?.displayName ?? run.triggeredBy.value;
    final now = ref.watch(appPortsProvider).clock.now();
    final format = L10nFormat(Localizations.localeOf(context));
    final permission = cardPermission(
      l,
      components,
      (access: access, me: me, owner: owner),
      run: run,
      runStarterName: starter,
    );
    final state = cardOutboxState(l, message.id, outbox);
    return DsMessageComponents(
      componentData(
        message,
        components,
        principals: principals,
        now: now,
        timeLabel: (at) => timeLabel(at, now: now, format: format),
        pendingCustomId: state.pendingCustomId,
        errorText: state.errorText,
      ),
      key: ValueKey('card.${message.id.value}'),
      permitted: permission.permitted,
      deniedReasonText: permission.reason,
      header: componentHeader(
        message,
        components,
        principals: principals,
        // The run is named by its start time, a server field (never a text of the agent).
        runLabel: run == null
            ? null
            : timeLabel(run.queuedAt, now: now, format: format),
        runStarterName: starter,
      ),
      agentText: message.contentMd,
      onOpenLink: onOpenLink,
      onActivate: (customId, value) {
        final actions = this.actions;
        if (actions == null) return;
        for (final c in components) {
          if (c.customId == customId) {
            unawaited(actions.activate(message, c, value, outbox: outbox));
            return;
          }
        }
      },
    );
  }
}
