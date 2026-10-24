// SPDX-License-Identifier: Apache-2.0

/// The run at its anchor message (`kind = run`, 09 5.4; 01 6.2, D3; components.md 2.2; brief
/// M4-21 Design note 1): nothing while the run is active (its box is pinned over the history),
/// then the collapsed line built from the summary of `run.state_changed` and the optional
/// `summary`, never from a server sentence. Expanding loads the document once (`getRun`).
/// After `interrupted` or `stale` the line offers "Send again" (09 5.5).
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';

import '../app/providers/ports.dart';
import '../app/providers/session.dart';
import '../thread/providers.dart';
import '../thread/thread_actions.dart';
import 'providers.dart';
import 'run_mapping.dart';

/// The line of the run of the anchor [message], or nothing while the run is active or its
/// summary is not stored yet.
class RunLine extends ConsumerWidget {
  /// Creates the line.
  const RunLine({super.key, required this.message});

  /// The anchor message (`kind = run`, `run_id` set).
  final Message message;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final runId = message.runId;
    if (runId == null) return const SizedBox.shrink();
    final thread = message.threadId.value;
    final runs = ref.watch(threadRunsProvider(thread)).value ?? const [];
    RunSummary? run;
    for (final r in runs) {
      if (r.id == runId) run = r;
    }
    if (run == null || isActiveRun(run.state)) return const SizedBox.shrink();
    final ended = run;
    final principals = ref.watch(principalsByIdProvider).value ?? const {};
    final config = ref.watch(appConfigProvider);
    final now = ref.watch(appPortsProvider).clock.now();
    final agentName =
        principals[ended.agentId]?.displayName ?? ended.agentId.value;
    final expanded = ref.watch(runExpansionProvider)[runId.value] ?? false;
    void toggle(bool open) => ref
        .read(runExpansionProvider.notifier)
        .set(runId.value, expanded: open);
    final doc = expanded
        ? ref.watch(endedRunDocumentProvider(runId.value)).value
        : null;
    final Widget box;
    if (expanded && doc != null) {
      box = DsLiveBox(
        runFromDocument(doc, agentName: agentName, now: now),
        key: ValueKey('runline.open.${runId.value}'),
        visibleSteps: config.liveboxVisibleSteps,
        announceMinInterval: config.liveAnnounceMinInterval,
        onCollapse: () => toggle(false),
      );
    } else {
      box = DsLiveBox.collapsed(
        runFromSummary(ended, agentName: agentName, now: now),
        key: ValueKey('runline.${runId.value}'),
        announceMinInterval: config.liveAnnounceMinInterval,
        onExpand: () => toggle(true),
      );
    }
    if (!offersSendAgain(ended)) return box;
    final l = AppLocalizations.of(context);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        box,
        const SizedBox(height: DsSpace.s4),
        Row(
          children: [
            Expanded(
              child: Text(
                l.liveBoxReasonInterrupted,
                style: dsTextStyle(DsType.bodySm)
                    .copyWith(color: context.dsColors.textSecondary),
              ),
            ),
            DsButton(
              key: ValueKey('runline.sendAgain.${runId.value}'),
              label: l.liveBoxSendAgain,
              onPressed: () => unawaited(sendAgain(ref, ended)),
            ),
          ],
        ),
      ],
    );
  }
}

/// "Send again" after `interrupted` or `stale` (09 5.5): a new message with the text of the
/// triggering message of [run] goes into the outbox; there is no automatic retry, because the
/// runtime may have caused side effects already. Does nothing when the triggering message is
/// unknown or the engine is gone.
Future<void> sendAgain(WidgetRef ref, RunSummary run) async {
  final engine = ref.read(syncEngineProvider);
  final db = ref.read(localDatabaseProvider);
  if (engine == null || db == null) return;
  final json = await ref.read(agentReadsProvider).run(run.id);
  final triggering = json?['triggering_message_id'];
  if (triggering is! String) return;
  final MessageId id;
  try {
    id = MessageId.parse(triggering);
  } on FormatException {
    return;
  }
  final message = await db.messagesDao.byId(id);
  if (message == null || message.contentMd.trim().isEmpty) return;
  await ThreadActions(engine).send(run.threadId, message.contentMd);
}
