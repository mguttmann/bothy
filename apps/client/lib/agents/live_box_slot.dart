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
///
/// On the phone (01 5.3 M3; brief M5-02 Design note 5) the slot always shows the compact row:
/// state, duration, progress and Stop at the top right; a tap on the row opens the full box as
/// a modal sheet.
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter/semantics.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';

import '../app/providers/ports.dart';
import '../app/providers/session.dart';
import '../keyboard/app_shortcuts.dart';
import '../keyboard/intents.dart';
import '../running/providers.dart' show mayStop, stopRun;
import '../thread/providers.dart';
import 'followups.dart';
import 'providers.dart';
import 'run_mapping.dart';

/// The pinned live box of [threadId] in [channelId], or nothing while no run is active.
///
/// It offers Stop (only where the box shows it) and the toggle of the box to the shortcut
/// layer (01 7: Cmd+. and Cmd+Shift+L; M4-24) with [keyboardPriority].
class LiveBoxSlot extends ConsumerStatefulWidget {
  /// Creates the slot.
  const LiveBoxSlot({
    super.key,
    required this.threadId,
    required this.channelId,
    this.keyboardPriority = 0,
  });

  /// The thread.
  final ThreadId threadId;

  /// Its channel (the access that decides Stop).
  final ChannelId channelId;

  /// The priority of the slot's shortcut handlers against another open history
  /// ([KeyboardActions.priority]).
  final int keyboardPriority;

  @override
  ConsumerState<LiveBoxSlot> createState() => _LiveBoxSlotState();
}

class _LiveBoxSlotState extends ConsumerState<LiveBoxSlot> {
  Timer? _second;

  /// The announcer of the thread's runs (01 8; components.md 2.4): the boxes observe it, so
  /// the throttle and a merged announcement survive a collapse and the end of a run, and the
  /// slot announces the final state when the box goes.
  LiveAnnouncer? _announcer;
  Timer? _flush;

  /// The run the box showed last, until its end was announced.
  RunSummary? _shown;

  @override
  void dispose() {
    _second?.cancel();
    _flush?.cancel();
    super.dispose();
  }

  /// Rebuilds once a second while the run is `running`: the duration counts locally (Design
  /// note 1); a timer, not an animation, so it also counts with reduced motion.
  void _ticking(bool active) {
    if (active && _second == null) {
      _second = Timer.periodic(const Duration(seconds: 1), (_) {
        if (mounted) setState(() {});
      });
    } else if (!active) {
      _second?.cancel();
      _second = null;
    }
  }

  LiveAnnouncer _announcerOf(BuildContext context, Duration interval) {
    final view = View.of(context);
    final direction = Directionality.of(context);
    return _announcer ??= LiveAnnouncer(
      minInterval: interval,
      clock: DateTime.now,
      sink: (message) => unawaited(
        SemanticsService.sendAnnouncement(
          view,
          message,
          direction,
          assertiveness: Assertiveness.polite,
        ),
      ),
    );
  }

  /// Announces the final state of [ended] after its box went (the box of an active run never
  /// sees the end), through the same throttle as the box.
  void _announceEnd(LiveAnnouncer announcer, String text) {
    announcer.observe(state: text, questionId: null);
    _scheduleFlush(announcer);
  }

  void _scheduleFlush(LiveAnnouncer announcer) {
    _flush?.cancel();
    final due = announcer.pendingDue;
    if (due == null) return;
    var wait = due.difference(DateTime.now());
    if (wait < const Duration(milliseconds: 1)) {
      wait = const Duration(milliseconds: 1);
    }
    _flush = Timer(wait, () {
      announcer.flush();
      if (mounted) _scheduleFlush(announcer);
    });
  }

  @override
  Widget build(BuildContext context) {
    final thread = widget.threadId.value;
    final runs = ref.watch(threadRunsProvider(thread)).value ?? const [];
    final run = newestActiveRun(runs);
    _ticking(run?.state == RunState.running);
    final config = ref.watch(appConfigProvider);
    final announcer = _announcerOf(context, config.liveAnnounceMinInterval);
    final principals = ref.watch(principalsByIdProvider).value ?? const {};
    String name(PrincipalId id) => principals[id]?.displayName ?? id.value;
    final shown = _shown;
    if (shown != null && shown.id != run?.id) {
      RunSummary? ended;
      for (final r in runs) {
        if (r.id == shown.id && !isActiveRun(r.state)) ended = r;
      }
      _shown = null;
      if (ended != null) {
        final l = AppLocalizations.of(context);
        final data = runFromSummary(
          ended,
          agentName: name(ended.agentId),
          now: DateTime.now(),
        );
        final text = l.liveBoxAnnounceState(
          data.agentName,
          dsRunStateWord(l, data),
        );
        WidgetsBinding.instance.addPostFrameCallback(
          (_) => _announceEnd(announcer, text),
        );
      }
    }
    if (run == null) return const SizedBox.shrink();
    _shown = run;
    final doc = ref.watch(runDocumentProvider(run.id.value, thread)).value;
    final me = ref.watch(myPrincipalIdProvider);
    final access = ref
        .watch(channelAccessProvider(widget.channelId.value))
        .value;
    final now = ref.watch(appPortsProvider).clock.now();
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
    if (ref.watch(appPortsProvider).formFactor.isPhone) {
      return Padding(
        key: ValueKey('livebox.slot.${run.id.value}'),
        padding: const EdgeInsets.fromLTRB(
          DsSpace.s8,
          DsSpace.s4,
          DsSpace.s8,
          DsSpace.s4,
        ),
        child: DsLiveBox.mobile(
          data,
          key: ValueKey('livebox.line.${run.id.value}'),
          announceMinInterval: config.liveAnnounceMinInterval,
          onOpen: () => unawaited(
            showLiveBoxSheet(
              context,
              threadId: widget.threadId,
              channelId: widget.channelId,
              runId: run.id,
            ),
          ),
          onStop: canStop ? stop : null,
          announcer: announcer,
        ),
      );
    }
    return KeyboardActions(
      priority: widget.keyboardPriority,
      handlers: {
        ShortcutAction.stopRun: KeyboardHandler(stop, enabled: canStop),
        ShortcutAction.toggleLiveBox: KeyboardHandler(() => toggle(!expanded)),
      },
      child: Padding(
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
                announcer: announcer,
              )
            : DsLiveBox.mobile(
                data,
                key: ValueKey('livebox.line.${run.id.value}'),
                announceMinInterval: config.liveAnnounceMinInterval,
                onOpen: () => toggle(true),
                onStop: canStop ? stop : null,
                announcer: announcer,
              ),
      ),
    );
  }
}

/// The view model of the active [run] of [threadId] and its Stop, or null for Stop where the
/// server's answers do not allow `run.cancel` (shared by the slot and the phone's sheet).
({DsRunData data, VoidCallback? stop}) liveBoxView(
  WidgetRef ref,
  RunSummary run, {
  required ThreadId threadId,
  required ChannelId channelId,
}) {
  final thread = threadId.value;
  final principals = ref.watch(principalsByIdProvider).value ?? const {};
  String name(PrincipalId id) => principals[id]?.displayName ?? id.value;
  final doc = ref.watch(runDocumentProvider(run.id.value, thread)).value;
  final me = ref.watch(myPrincipalIdProvider);
  final access = ref.watch(channelAccessProvider(channelId.value)).value;
  final now = ref.watch(appPortsProvider).clock.now();
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
  final engine = ref.watch(syncEngineProvider);
  final canStop = engine != null && mayStop(run, access, me);
  return (
    data: data,
    stop: canStop ? () => unawaited(stopRun(engine, run)) : null,
  );
}

/// Opens the full live box of [runId] in [threadId] as a modal sheet (01 5.3 M3); the sheet
/// closes itself when the run is no longer active.
Future<void> showLiveBoxSheet(
  BuildContext context, {
  required ThreadId threadId,
  required ChannelId channelId,
  required RunId runId,
}) => showModalBottomSheet<void>(
  context: context,
  useSafeArea: true,
  isScrollControlled: true,
  builder: (_) =>
      LiveBoxSheet(threadId: threadId, channelId: channelId, runId: runId),
);

/// The full live box of one run in a sheet, with Stop and the steps (components.md 2); the
/// duration counts once a second like the slot's.
class LiveBoxSheet extends ConsumerStatefulWidget {
  /// Creates the sheet body.
  const LiveBoxSheet({
    super.key,
    required this.threadId,
    required this.channelId,
    required this.runId,
  });

  /// The thread of the run.
  final ThreadId threadId;

  /// Its channel.
  final ChannelId channelId;

  /// The run.
  final RunId runId;

  @override
  ConsumerState<LiveBoxSheet> createState() => _LiveBoxSheetState();
}

class _LiveBoxSheetState extends ConsumerState<LiveBoxSheet> {
  Timer? _second;
  bool _closing = false;

  @override
  void dispose() {
    _second?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final runs =
        ref.watch(threadRunsProvider(widget.threadId.value)).value ?? const [];
    RunSummary? run;
    for (final r in runs) {
      if (r.id == widget.runId && isActiveRun(r.state)) run = r;
    }
    if (run == null) {
      // The run ended while the sheet was open: the collapsed line in the history takes over.
      _second?.cancel();
      if (!_closing) {
        _closing = true;
        WidgetsBinding.instance.addPostFrameCallback((_) {
          if (mounted) unawaited(Navigator.of(context).maybePop());
        });
      }
      return const SizedBox.shrink();
    }
    if (run.state == RunState.running) {
      _second ??= Timer.periodic(const Duration(seconds: 1), (_) {
        if (mounted) setState(() {});
      });
    } else {
      _second?.cancel();
      _second = null;
    }
    final view = liveBoxView(
      ref,
      run,
      threadId: widget.threadId,
      channelId: widget.channelId,
    );
    final config = ref.watch(appConfigProvider);
    return SingleChildScrollView(
      key: ValueKey('livebox.sheet.${widget.runId.value}'),
      padding: const EdgeInsets.all(DsSpace.s16),
      child: DsLiveBox(
        view.data,
        visibleSteps: config.liveboxVisibleSteps,
        announceMinInterval: config.liveAnnounceMinInterval,
        onStop: view.stop,
        onCollapse: () => unawaited(Navigator.of(context).maybePop()),
      ),
    );
  }
}
