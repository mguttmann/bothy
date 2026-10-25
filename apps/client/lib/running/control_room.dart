// SPDX-License-Identifier: Apache-2.0

/// The control room: the cards of "Running now" (docs/spec/01-product.md 5.1 D10; 09 5.6;
/// design note run-visualization D-RV-7, D-RV-8; brief M4-39 Design note 4).
library;

import 'dart:async';

import 'package:bothy_viz/bothy_viz.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../app/providers/session.dart';
import '../app/router.dart';
import 'providers.dart';
import 'run_api.dart';

/// The cards of [rows] in a wrapping grid.
///
/// After every layout and scroll the grid measures which cards are on screen and hands them
/// in on-screen order (top to bottom, then start to end) to [ControlRoomFocus]: their threads
/// go into `background_thread_ids` within `sync.max_focused_threads`, and a card with a place
/// shows the run document live; the others show the summary with "live details paused".
class ControlRoomCards extends ConsumerStatefulWidget {
  /// Creates the grid.
  const ControlRoomCards({super.key, required this.rows});

  /// The rows of D10 after the filters.
  final List<RunningRow> rows;

  @override
  ConsumerState<ControlRoomCards> createState() => _ControlRoomCardsState();
}

class _ControlRoomCardsState extends ConsumerState<ControlRoomCards> {
  final Map<RunId, GlobalKey> _keys = {};
  final GlobalKey _viewport = GlobalKey();
  bool _measureScheduled = false;

  GlobalKey _keyOf(RunId run) => _keys.putIfAbsent(run, GlobalKey.new);

  void _scheduleMeasure() {
    if (_measureScheduled) return;
    _measureScheduled = true;
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _measureScheduled = false;
      if (mounted) _measure();
    });
  }

  /// The cards whose box meets the viewport, in on-screen order.
  void _measure() {
    final viewport = _viewport.currentContext?.findRenderObject();
    if (viewport is! RenderBox || !viewport.hasSize) return;
    final area = viewport.localToGlobal(Offset.zero) & viewport.size;
    final shown = <({RunId run, ThreadId thread, Offset at})>[];
    for (final row in widget.rows) {
      final box = _keys[row.run.id]?.currentContext?.findRenderObject();
      if (box is! RenderBox || !box.hasSize) continue;
      final rect = box.localToGlobal(Offset.zero) & box.size;
      if (!rect.overlaps(area)) continue;
      shown.add((run: row.run.id, thread: row.run.threadId, at: rect.topLeft));
    }
    shown.sort((a, b) {
      final dy = a.at.dy.compareTo(b.at.dy);
      return dy != 0 ? dy : a.at.dx.compareTo(b.at.dx);
    });
    ref.read(controlRoomFocusProvider.notifier).show([
      for (final s in shown) (run: s.run, thread: s.thread),
    ]);
  }

  @override
  Widget build(BuildContext context) {
    final live = ref.watch(controlRoomFocusProvider);
    _keys.removeWhere((run, _) => !widget.rows.any((r) => r.run.id == run));
    _scheduleMeasure();
    return NotificationListener<ScrollNotification>(
      onNotification: (_) {
        _scheduleMeasure();
        return false;
      },
      child: SingleChildScrollView(
        key: _viewport,
        padding: const EdgeInsets.all(DsSpace.s16),
        child: Wrap(
          key: const ValueKey('running.cards'),
          spacing: DsSpace.s12,
          runSpacing: DsSpace.s12,
          children: [
            for (final row in widget.rows)
              KeyedSubtree(
                key: _keyOf(row.run.id),
                child: RunningCard(row: row, live: live.contains(row.run.id)),
              ),
          ],
        ),
      ),
    );
  }
}

/// One card of the control room: [RunControlCard] of `bothy_viz` with the run document while
/// [live], else the summary.
class RunningCard extends ConsumerWidget {
  /// Creates the card of [row].
  const RunningCard({super.key, required this.row, required this.live});

  /// The row of D10.
  final RunningRow row;

  /// Whether the card's thread has a focus place.
  final bool live;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final run = row.run;
    final doc = live
        ? ref.watch(cardDocumentProvider(run.id.value)).value
        : null;
    final now = ref.watch(runningClockProvider).value ?? run.queuedAt;
    final stall = ref.watch(activeRunStallsProvider)[run.id];
    final summary = apiSummaryOf(run, stallWarnedAt: stall);
    if (summary == null) return const SizedBox.shrink();
    final model = RunControlCardModel.from(
      summary,
      document: doc == null ? null : apiRunOf(doc),
      now: now,
      agentName: dsIsolate(row.agentName),
      path: [for (final p in row.path) dsIsolate(p)],
      canStop: row.canStop,
    );
    return RunControlCard(
      key: ValueKey('running.card.${run.id.value}'),
      model: model,
      onOpenGraph: () =>
          context.go(RunGraphLocation(run.id, run.threadId).path),
      onStop: () {
        final engine = ref.read(syncEngineProvider);
        if (engine != null) unawaited(stopRun(engine, run));
      },
      onOpen: () => context.go('/t/${run.threadId.value}'),
    );
  }
}
