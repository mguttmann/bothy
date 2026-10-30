// SPDX-License-Identifier: Apache-2.0

/// The run at its anchor message (`kind = run`, 09 5.4; 01 6.2, D3; components.md 2.2; brief
/// M4-21 Design note 1): nothing while the run is active (its box is pinned over the history),
/// then the collapsed line built from the summary of `run.state_changed` and the optional
/// `summary`, never from a server sentence. Expanding loads the document once (`getRun`).
/// After `interrupted` or `stale` the line offers "Send again" (09 5.5); the newest run of the
/// thread that ended `failed` or `cancelled` otherwise offers "Retry" (`retryRun`, 09 5.7).
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';
import 'package:sync_engine/sync_engine.dart';

import '../app/providers/ports.dart';
import '../app/providers/session.dart';
import '../sync/message_ops_app.dart';
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
    if (offersRetry(ended, runs)) {
      return Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          box,
          const SizedBox(height: DsSpace.s4),
          _RetryButton(
            key: ValueKey('runline.retry.${runId.value}'),
            run: ended,
            agentName: agentName,
          ),
        ],
      );
    }
    // Only the person who triggered the run decides whether to send again (09 5.5).
    final me = ref.watch(myPrincipalIdProvider);
    if (!offersSendAgain(ended) || ended.triggeredBy != me) return box;
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
            _SendAgainButton(
              key: ValueKey('runline.sendAgain.${runId.value}'),
              run: ended,
            ),
          ],
        ),
      ],
    );
  }
}

/// Whether the line of [run] offers "Retry" (09 5.7): it ended `failed` or `cancelled`, it is the
/// newest of the [runs] of its thread (`queued_at`, then the ID, as the server orders them), and
/// it is no `interrupted` or `stale` run, which keeps "Send again" (09 5.5).
bool offersRetry(RunSummary run, List<RunSummary> runs) {
  if (run.state != RunState.failed && run.state != RunState.cancelled) {
    return false;
  }
  if (offersSendAgain(run)) return false;
  for (final other in runs) {
    if (other.threadId != run.threadId || other.id == run.id) continue;
    final order = other.queuedAt.compareTo(run.queuedAt);
    if (order > 0 ||
        (order == 0 && other.id.value.compareTo(run.id.value) > 0)) {
      return false;
    }
  }
  return true;
}

/// The button "Retry" with the reason of a refusal: one activation puts one `retryRun` into the
/// outbox (lane of the run, no thread); it stays disabled while the entry waits and after the
/// server accepted it (the new run then replaces this line as the newest). A refused entry is
/// discarded, its reason shown, and the button offered again.
class _RetryButton extends ConsumerStatefulWidget {
  const _RetryButton({super.key, required this.run, required this.agentName});

  final RunSummary run;
  final String agentName;

  @override
  ConsumerState<_RetryButton> createState() => _RetryButtonState();
}

class _RetryButtonState extends ConsumerState<_RetryButton> {
  bool _busy = false;
  String? _entryId;
  String? _refusalCode;
  String? _refusalReason;
  StreamSubscription<List<OutboxEntryView>>? _watch;

  @override
  void dispose() {
    unawaited(_watch?.cancel());
    super.dispose();
  }

  Future<void> _retry() async {
    if (_busy) return;
    final engine = ref.read(syncEngineProvider);
    if (engine == null) return;
    setState(() {
      _busy = true;
      _refusalCode = _refusalReason = null;
    });
    final entry = await engine.outbox.enqueue(
      OutboxRequest(
        kind: AppMessageOps.retryRun,
        payload: {'run_id': widget.run.id.value},
      ),
    );
    if (!mounted) return;
    _entryId = entry.id;
    await _watch?.cancel();
    _watch = engine.outbox.watchThread(null).listen((views) {
      for (final v in views) {
        if (v.id != _entryId || v.stateLabel != OutboxStateLabel.failed) {
          continue;
        }
        _entryId = null;
        unawaited(engine.outbox.discard(v.id));
        if (!mounted) return;
        setState(() {
          _busy = false;
          _refusalCode = v.failureCode ?? 'unknown';
          _refusalReason = v.failureReason;
        });
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final code = _refusalCode;
    final reason = _refusalReason;
    final refusal = code == null
        ? null
        : l.liveBoxRetryRefused(
            (reason == null ? null : dsReasonText(l, reason)) ??
                problemText(l, code, reason: reason),
          );
    return Row(
      children: [
        Expanded(
          child: Text(
            refusal ?? '',
            style: dsTextStyle(DsType.bodySm)
                .copyWith(color: context.dsColors.textSecondary),
          ),
        ),
        Semantics(
          label: l.liveBoxRetryRun(widget.agentName),
          excludeSemantics: true,
          button: true,
          child: DsButton(
            label: l.liveBoxRetry,
            loading: _busy,
            onPressed: _busy ? null : () => unawaited(_retry()),
          ),
        ),
      ],
    );
  }
}

/// The button "Send again": one activation sends once; it stays disabled while the send runs
/// and after it succeeded, so a double click never starts two runs (R-AG-17).
class _SendAgainButton extends ConsumerStatefulWidget {
  const _SendAgainButton({super.key, required this.run});

  final RunSummary run;

  @override
  ConsumerState<_SendAgainButton> createState() => _SendAgainButtonState();
}

class _SendAgainButtonState extends ConsumerState<_SendAgainButton> {
  bool _busy = false;
  bool _sent = false;

  Future<void> _send() async {
    if (_busy || _sent) return;
    setState(() => _busy = true);
    final sent = await sendAgain(ref, widget.run);
    if (!mounted) return;
    setState(() {
      _busy = false;
      _sent = sent;
    });
  }

  @override
  Widget build(BuildContext context) => DsButton(
    label: AppLocalizations.of(context).liveBoxSendAgain,
    loading: _busy,
    onPressed: _busy || _sent ? null : () => unawaited(_send()),
  );
}

/// "Send again" after `interrupted` or `stale` (09 5.5): a new message with the text of the
/// triggering message of [run] goes into the outbox; there is no automatic retry, because the
/// runtime may have caused side effects already. Returns whether a message went into the
/// outbox; nothing is sent when the triggering message is unknown or the engine is gone.
Future<bool> sendAgain(WidgetRef ref, RunSummary run) async {
  final engine = ref.read(syncEngineProvider);
  final db = ref.read(localDatabaseProvider);
  if (engine == null || db == null) return false;
  final Map<String, Object?>? json;
  try {
    json = await ref.read(agentReadsProvider).run(run.id);
  } on Object {
    // Offline or refused: nothing is sent; the button stays for another try.
    return false;
  }
  final triggering = json?['triggering_message_id'];
  if (triggering is! String) return false;
  final MessageId id;
  try {
    id = MessageId.parse(triggering);
  } on FormatException {
    return false;
  }
  final message = await db.messagesDao.byId(id);
  if (message == null || message.contentMd.trim().isEmpty) return false;
  await ThreadActions(engine).send(run.threadId, message.contentMd);
  return true;
}
