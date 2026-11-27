// SPDX-License-Identifier: Apache-2.0

/// The status chip of computer use in the thread of a run (docs/spec/27-computer-use.md 11
/// items 1 and 2; 01 3.9; brief CU-07 requirement 13): "Controls `desktop` · `frontmost app`
/// · `n` actions" or "`desktop` locked, waiting", with Stop, while the run's computer session is
/// not closed; once it closes, the reason once, until dismissed.
///
/// The same chip on the desktop and the phone, under the live box of the thread (ruling
/// D-CU-07-2). Metadata only: no view of computer use requests, caches or shows an image
/// (R-CL-18).
library;

import 'package:api_client/api_client.dart' show ApiProblem;
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart' show ThreadId;
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';

import '../agents/providers.dart' show threadRunsProvider;
import 'computer_status.dart';
import 'providers.dart';

/// The text of a failed stop or switch-off: the server's problem, or "offline" without one.
String computerProblemText(AppLocalizations l, Object error) {
  final problem = ApiProblem.of(error);
  return problem == null
      ? l.selfOffline
      : problemText(l, problem.code, reason: problem.reason);
}

/// The chip of one computer session [status].
///
/// [onStop] shows Stop while the session is not closed; [onDismiss] hides a closed one;
/// [error] is a line under the chip (a refused stop). The chip is a live region, so a screen
/// reader hears a change of state.
class ComputerStatusChip extends StatelessWidget {
  /// Creates the chip.
  const ComputerStatusChip({
    super.key,
    required this.status,
    this.onStop,
    this.onDismiss,
    this.error,
    this.stopping = false,
  });

  /// The session.
  final ComputerStatus status;

  /// Stops the session; null shows no Stop.
  final VoidCallback? onStop;

  /// Hides the ended session; null shows no Dismiss.
  final VoidCallback? onDismiss;

  /// The text of a failed stop, or null.
  final String? error;

  /// Whether a stop is on its way (Stop is disabled meanwhile).
  final bool stopping;

  /// The chip's text in [l].
  static String textOf(AppLocalizations l, ComputerStatus s) =>
      switch (s.phase) {
        ComputerPhase.opening => l.computerChipOpening(s.desktop),
        ComputerPhase.paused => l.computerChipLocked(s.desktop),
        ComputerPhase.active => switch (s.frontApp) {
          final app? => l.computerChipActive(s.desktop, app, s.actionCount),
          null => l.computerChipActiveNoApp(s.desktop, s.actionCount),
        },
        ComputerPhase.closed => l.computerChipEnded(
          s.desktop,
          l.computerCloseReason(s.closeReason ?? ''),
        ),
      };

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final c = context.dsColors;
    final s = status;
    final background = switch (s.phase) {
      ComputerPhase.paused => c.attentionSubtle,
      ComputerPhase.closed => c.surfaceSunken,
      _ => c.accentSubtle,
    };
    final text = textOf(l, s);
    return Padding(
      padding: const EdgeInsets.symmetric(
        horizontal: DsSpace.s8,
        vertical: DsSpace.s4,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          DecoratedBox(
            decoration: BoxDecoration(
              color: background,
              borderRadius: BorderRadius.circular(DsRadius.md),
              border: Border.all(
                color: c.borderSubtle,
                width: DsBorderWidth.hairline,
              ),
            ),
            child: Padding(
              padding: const EdgeInsets.symmetric(horizontal: DsSpace.s8),
              child: Row(
                children: [
                  Expanded(
                    child: Semantics(
                      liveRegion: true,
                      child: Text(
                        text,
                        style: dsTextStyle(DsType.bodySm)
                            .copyWith(color: c.textPrimary),
                      ),
                    ),
                  ),
                  if (!s.closed && onStop != null)
                    ComputerActionButton(
                      key: ValueKey('computer.stop.${s.sessionId}'),
                      text: l.computerStop,
                      label: l.computerStopLabel(s.desktop),
                      onPressed: stopping ? null : onStop,
                    ),
                  if (s.closed && onDismiss != null)
                    ComputerActionButton(
                      key: ValueKey('computer.dismiss.${s.sessionId}'),
                      text: l.computerDismiss,
                      label: l.computerDismiss,
                      onPressed: onDismiss,
                    ),
                ],
              ),
            ),
          ),
          if (error case final e?)
            Padding(
              padding: const EdgeInsets.only(top: DsSpace.s4),
              child: Text(
                e,
                style: dsTextStyle(DsType.caption).copyWith(color: c.dangerFg),
              ),
            ),
        ],
      ),
    );
  }
}

/// A text button of computer use with its own screen reader [label] (the visible [text] is
/// short; the label names the desktop). Focusable and activated by Enter or Space through
/// [DsTappable].
class ComputerActionButton extends StatelessWidget {
  /// Creates the button.
  const ComputerActionButton({
    super.key,
    required this.text,
    required this.label,
    required this.onPressed,
  });

  /// The visible text.
  final String text;

  /// The name for screen readers.
  final String label;

  /// The action; null disables the button.
  final VoidCallback? onPressed;

  @override
  Widget build(BuildContext context) {
    final c = context.dsColors;
    return DsTappable(
      onTap: onPressed,
      label: label,
      radius: DsRadius.md,
      excludeChildSemantics: true,
      child: Container(
        constraints: BoxConstraints(minHeight: context.dsDensity.targetMin),
        padding: const EdgeInsets.symmetric(horizontal: DsSpace.s12),
        alignment: Alignment.center,
        child: Text(
          text,
          style: dsTextStyle(DsType.label)
              .copyWith(color: onPressed == null ? c.textMuted : c.textPrimary),
        ),
      ),
    );
  }
}

/// The chips of the computer sessions of the runs of [threadId] (27 11 item 1), or nothing.
///
/// Nothing for a person who is no owner: [computerSessionsProvider] stays empty for them.
class ComputerChipSlot extends ConsumerStatefulWidget {
  /// Creates the slot.
  const ComputerChipSlot({super.key, required this.threadId});

  /// The thread.
  final ThreadId threadId;

  @override
  ConsumerState<ComputerChipSlot> createState() => _ComputerChipSlotState();
}

class _ComputerChipSlotState extends ConsumerState<ComputerChipSlot> {
  /// The text of a failed stop per session.
  final Map<String, String> _errors = {};
  final Set<String> _stopping = {};

  Future<void> _stop(ComputerStatus s) async {
    final l = AppLocalizations.of(context);
    final sessions = ref.read(computerSessionsProvider.notifier);
    setState(() {
      _stopping.add(s.sessionId);
      _errors.remove(s.sessionId);
    });
    try {
      await sessions.stop(s.sessionId);
    } on Object catch (e) {
      if (mounted) {
        setState(
          () => _errors[s.sessionId] = l.computerStopFailed(
            computerProblemText(l, e),
          ),
        );
      }
    } finally {
      if (mounted) setState(() => _stopping.remove(s.sessionId));
    }
  }

  @override
  Widget build(BuildContext context) {
    final sessions = ref.watch(computerSessionsProvider);
    if (sessions.live.isEmpty && sessions.ended.isEmpty) {
      return const SizedBox.shrink();
    }
    final runs = ref.watch(threadRunsProvider(widget.threadId.value)).value;
    if (runs == null || runs.isEmpty) return const SizedBox.shrink();
    final shown = [for (final r in runs) ?sessions.ofRun(r.id.value)];
    if (shown.isEmpty) return const SizedBox.shrink();
    return Column(
      key: const ValueKey('computer.slot'),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        for (final s in shown)
          ComputerStatusChip(
            key: ValueKey('computer.chip.${s.sessionId}'),
            status: s,
            stopping: _stopping.contains(s.sessionId),
            error: _errors[s.sessionId],
            onStop: () => _stop(s),
            onDismiss: () => ref
                .read(computerSessionsProvider.notifier)
                .dismiss(s.sessionId),
          ),
      ],
    );
  }
}
