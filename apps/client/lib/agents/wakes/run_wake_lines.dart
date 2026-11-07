// SPDX-License-Identifier: Apache-2.0

/// What the run lines of a thread show of autonomous runs (01 6.21 items 2 to 4, 09 18.19.10,
/// 18.19.11; brief AR-08): at the anchor of a wake run "Woke up: <label>" instead of a
/// triggering message, at a recovery run "Continued after lost work"; under the collapsed line
/// of a finished run its end state ("Done", "Waiting", "Needs you") and, for the newest run of
/// the thread while a wake-up is pending, when the thread continues; the note "Ended waiting,
/// but nothing will wake this thread" for a waiting end without a pending wake-up; and the
/// usage-limit notice of a run the usage window of the account ended. Every note is drawn as a
/// note of the thread, never as a message.
library;

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';
import 'package:sync_engine/sync_engine.dart';

import '../../app/providers/ports.dart';
import '../../settings/settings_providers.dart';
import '../../thread/thread_note.dart';
import '../providers.dart';
import 'pending_wakes.dart';
import 'wake_view.dart';

/// The line at the anchor of [runId] in [threadId] that names the wake-up of a wake run, or
/// nothing for a run of a message, a command or an interaction.
class RunTriggerLine extends ConsumerWidget {
  /// Creates the line.
  const RunTriggerLine({
    super.key,
    required this.runId,
    required this.threadId,
  });

  /// The run of the anchor.
  final RunId runId;

  /// Its thread.
  final ThreadId threadId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final wakes =
        ref.watch(threadWakesProvider(threadId.value)).value ?? const [];
    final wake = wakeOfRun(wakes, runId);
    if (wake == null) return const SizedBox.shrink();
    final l = AppLocalizations.of(context);
    final text = switch (wake) {
      RunWake(reason: RunWakeReason.recovery) => l.runContinuedAfterLostWork,
      RunWake(label: '') => l.runWokeUpUnnamed,
      _ => l.runWokeUp(dsIsolate(wake.label)),
    };
    return ThreadNoteLine(
      key: ValueKey('run.trigger.${runId.value}'),
      text: text,
    );
  }
}

/// The lines under the collapsed line of the ended [run]: its end state from [doc] (null while
/// the document loads or for a run without one), when the thread continues, and the notes.
/// [newest] marks the newest run of the thread with no active run after it.
class RunEndLines extends ConsumerWidget {
  /// Creates the lines.
  const RunEndLines({
    super.key,
    required this.run,
    required this.doc,
    required this.newest,
  });

  /// The ended run.
  final RunSummary run;

  /// Its document, or null.
  final RunDocument? doc;

  /// Whether it is the newest run of its thread and none runs now.
  final bool newest;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final wakes =
        ref.watch(threadWakesProvider(run.threadId.value)).value ?? const [];
    final zone = ref.watch(personTimeZoneProvider);
    final now = ref.watch(appPortsProvider).clock.now();
    final format = dsFormat(context);
    String when(DateTime at) => wakeTime(format, at, now: now, timeZone: zone);
    final endState = doc?.metrics.endState;
    final pending = pendingWakes(wakes);
    final children = <Widget>[];
    if (endState != null && endState != RunEndState.unknown) {
      final style = dsTextStyle(DsType.caption)
          .copyWith(color: colors.textSecondary);
      final next = newest && pending.isNotEmpty
          ? pending.first.notBefore
          : null;
      children.add(
        Padding(
          key: ValueKey('run.end.${run.id.value}'),
          padding: const EdgeInsets.symmetric(
            horizontal: DsSpace.s8,
            vertical: DsSpace.s2,
          ),
          child: Wrap(
            spacing: DsSpace.s8,
            children: [
              Text(
                l.runEndStateLabel(switch (endState) {
                  RunEndState.done => l.runEndStateDone,
                  RunEndState.blocked => l.runEndStateBlocked,
                  _ => l.runEndStateWaiting,
                }),
                style: dsTextStyle(DsType.caption),
              ),
              if (next != null)
                Text(
                  l.runContinuesAt(
                    format.relativeDay(next, now: now, timeZone: zone),
                    format.time(next, timeZone: zone),
                  ),
                  style: style,
                ),
            ],
          ),
        ),
      );
    }
    // 18.19.11: a waiting end without a pending wake-up at the end of the run.
    if (endState == RunEndState.waiting &&
        doc?.metrics.wakePendingUntil == null) {
      children.add(
        ThreadNoteLine(
          key: ValueKey('run.note.waiting.${run.id.value}'),
          text: l.noteEndedWaiting,
        ),
      );
    }
    // 18.19.10: the usage window of the account ended the run.
    if (run.state == RunState.failed && run.stateReason == usageLimitedReason) {
      final wake = usageWakeOf(wakes, run.id);
      final text = switch (wake) {
        RunWake(state: RunWakeState.pending) => l.wakesUsageLimitContinues(
          when(wake.notBefore),
        ),
        RunWake(state: RunWakeState.fired) => null,
        RunWake(state: RunWakeState.cancelled) => null,
        RunWake() => l.wakesUsageLimitReplyAfter(when(wake.notBefore)),
        null => l.wakesUsageLimitReplyLater,
      };
      if (text != null) {
        children.add(
          ThreadNoteLine(
            key: ValueKey('run.note.usage.${run.id.value}'),
            text: text,
          ),
        );
      }
    }
    if (children.isEmpty) return const SizedBox.shrink();
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: children,
    );
  }
}
