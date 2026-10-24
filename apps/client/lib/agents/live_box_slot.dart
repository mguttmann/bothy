// SPDX-License-Identifier: Apache-2.0

/// The live box pinned over the history while a run of the thread is active (01 6.2, D3;
/// 09 5.4; components.md 2; brief M4-21 Design note 1; docs/decisions.d/M4-21.md D-M4-21-1).
///
/// The slot watches the run summaries of the thread (the local `runs` table); the newest
/// active run gets the box. Its document comes from the `RunStore` (focused while the slot
/// shows, so patches arrive only for a visible thread, 04 11.3) at most once per frame; the
/// run time counts up locally from `started_at` with a ticker, without a patch. Stop shows
/// only where the server's answers allow `run.cancel` and asks for no confirmation. When the
/// run ends the slot empties and the collapsed line at the anchor message takes over.
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter/scheduler.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../app/providers/ports.dart';
import '../app/providers/session.dart';
import '../running/providers.dart' show mayStop, stopRun;
import '../thread/providers.dart';
import 'followups.dart';
import 'providers.dart';
import 'run_mapping.dart';

/// The pinned live box of [threadId] in [channelId], or nothing while no run is active.
class LiveBoxSlot extends ConsumerStatefulWidget {
  /// Creates the slot.
  const LiveBoxSlot({
    super.key,
    required this.threadId,
    required this.channelId,
  });

  /// The thread.
  final ThreadId threadId;

  /// Its channel (the access that decides Stop).
  final ChannelId channelId;

  @override
  ConsumerState<LiveBoxSlot> createState() => _LiveBoxSlotState();
}

class _LiveBoxSlotState extends ConsumerState<LiveBoxSlot>
    with SingleTickerProviderStateMixin {
  late final Ticker _ticker = createTicker(_tick);
  int _second = -1;

  @override
  void dispose() {
    _ticker.dispose();
    super.dispose();
  }

  /// Rebuilds once per second of run time: the duration counts locally (Design note 1), and
  /// a frame between two seconds changes nothing on screen.
  void _tick(Duration _) {
    final now = ref.read(appPortsProvider).clock.now();
    final second = now.millisecondsSinceEpoch ~/ 1000;
    if (second != _second) {
      _second = second;
      setState(() {});
    }
  }

  void _ticking(bool active) {
    final reduce = MediaQuery.maybeDisableAnimationsOf(context) ?? false;
    if (active && !reduce && !_ticker.isActive) {
      _ticker.start();
    } else if ((!active || reduce) && _ticker.isActive) {
      _ticker.stop();
    }
  }

  @override
  Widget build(BuildContext context) {
    final thread = widget.threadId.value;
    final runs = ref.watch(threadRunsProvider(thread)).value ?? const [];
    final run = newestActiveRun(runs);
    _ticking(run?.state == RunState.running);
    if (run == null) return const SizedBox.shrink();
    final doc = ref.watch(runDocumentProvider(run.id.value, thread)).value;
    final principals = ref.watch(principalsByIdProvider).value ?? const {};
    final me = ref.watch(myPrincipalIdProvider);
    final access = ref
        .watch(channelAccessProvider(widget.channelId.value))
        .value;
    final config = ref.watch(appConfigProvider);
    final now = ref.watch(appPortsProvider).clock.now();
    String name(PrincipalId id) => principals[id]?.displayName ?? id.value;
    // A question waits for the person who triggered the run; for that person it reads
    // "Waiting for you" (components.md 2.1).
    final waitingFor = run.triggeredBy == me ? null : name(run.triggeredBy);
    final data = doc == null
        ? runFromSummary(run, agentName: name(run.agentId), now: now)
        : runFromDocument(
            doc,
            agentName: name(run.agentId),
            now: now,
            waitingForName: waitingFor,
          );
    final expanded = ref.watch(runExpansionProvider)[run.id.value] ?? true;
    final engine = ref.watch(syncEngineProvider);
    final canStop = engine != null && mayStop(run, access, me);
    void stop() => unawaited(stopRun(engine!, run));
    void toggle(bool open) => ref
        .read(runExpansionProvider.notifier)
        .set(run.id.value, expanded: open);
    return Padding(
      key: ValueKey('livebox.slot.${run.id.value}'),
      padding: const EdgeInsets.fromLTRB(
        DsSpace.s16,
        DsSpace.s8,
        DsSpace.s16,
        DsSpace.s4,
      ),
      child: expanded
          ? DsLiveBox(
              data,
              key: ValueKey('livebox.${run.id.value}'),
              visibleSteps: config.liveboxVisibleSteps,
              announceMinInterval: config.liveAnnounceMinInterval,
              // Stop asks for no confirmation (01 6.2): it is harmless and must be fast.
              onStop: canStop ? stop : null,
              onCollapse: () => toggle(false),
            )
          : DsLiveBox.mobile(
              data,
              key: ValueKey('livebox.line.${run.id.value}'),
              announceMinInterval: config.liveAnnounceMinInterval,
              onOpen: () => toggle(true),
              onStop: canStop ? stop : null,
            ),
    );
  }
}
