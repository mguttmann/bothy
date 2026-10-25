// SPDX-License-Identifier: Apache-2.0

/// The run graph D14 (docs/spec/01-product.md 5.1 D14; 09 5.6; brief M4-39): opened from the
/// button "Graph" of the live box or from a control room card, route
/// `/running/{run_id}?t={thread_id}`.
library;

import 'dart:async';

import 'package:bothy_viz/bothy_viz.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:l10n/l10n.dart';

import '../agents/providers.dart';
import '../agents/run_mapping.dart' show isActiveRun;
import '../app/providers/ports.dart';
import '../app/providers/session.dart';
import '../app/router.dart';
import '../thread/providers.dart';
import 'providers.dart';
import 'run_api.dart';

/// The run graph of [location].
///
/// The run is focused with its thread in `background_thread_ids` while the graph shows (the
/// graph shows no messages, 04 6.2), so `run.updated` grows the graph live; the graph is
/// derived again from the document on every applied patch and once a second while the run is
/// active (the bars of running nodes end at now). When the run ends the store drops its
/// document and the graph shows the final document from `getRun`. Stop shows only where the
/// server's answers allow `run.cancel` (15 1 principle 1).
class RunGraphScreen extends ConsumerWidget {
  /// Creates the screen.
  const RunGraphScreen({super.key, required this.location});

  /// The run, its thread and where "back" goes.
  final RunGraphLocation location;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context);
    final c = context.dsColors;
    final runId = location.runId.value;
    final threadId = location.threadId.value;
    final live = ref.watch(runGraphDocumentProvider(runId, threadId));
    final summaries = ref.watch(threadRunsProvider(threadId)).value ?? const [];
    RunSummary? summary;
    for (final s in summaries) {
      if (s.id == location.runId) summary = s;
    }
    final liveDoc = live.value;
    final ended =
        liveDoc == null && summary != null && !isActiveRun(summary.state);
    final doc =
        liveDoc ??
        (ended ? ref.watch(endedRunDocumentProvider(runId)).value : null);
    final run = doc == null ? null : apiRunOf(doc);
    final principals = ref.watch(principalsByIdProvider).value ?? const {};
    final agentId = summary?.agentId;
    final agentName = agentId == null
        ? null
        : dsIsolate(principals[agentId]?.displayName ?? agentId.value);
    final config = ref.watch(appConfigProvider);
    final clock = ref.watch(appPortsProvider).clock;
    // The bars of running nodes end at now: the clock of "Running now" ticks once a second.
    final now = ref.watch(runningClockProvider).value ?? clock.now();
    final me = ref.watch(myPrincipalIdProvider);
    final access = summary == null
        ? null
        : ref.watch(channelAccessProvider(summary.channelId.value)).value;
    final engine = ref.watch(syncEngineProvider);
    final canStop =
        engine != null &&
        summary != null &&
        isActiveRun(summary.state) &&
        mayStop(summary, access, me);
    final back = location.fromThread
        ? (label: l.runGraphBackToThread, path: '/t/$threadId')
        : (label: l.runGraphBackToRunning, path: '/running');
    return Material(
      key: const ValueKey('runGraph.screen'),
      color: c.surfaceCanvas,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(
              DsSpace.s16,
              DsSpace.s12,
              DsSpace.s16,
              DsSpace.s8,
            ),
            child: Wrap(
              spacing: DsSpace.s12,
              runSpacing: DsSpace.s6,
              crossAxisAlignment: WrapCrossAlignment.center,
              children: [
                DsButton(
                  key: const ValueKey('runGraph.back'),
                  label: back.label,
                  style: DsButtonStyle.ghost,
                  onPressed: () => context.go(back.path),
                ),
                Semantics(
                  header: true,
                  child: Text(
                    l.runGraphTitle,
                    style: dsTextStyle(DsType.heading)
                        .copyWith(color: c.textPrimary),
                  ),
                ),
                if (agentName != null)
                  Text(
                    agentName,
                    style: dsTextStyle(DsType.bodyStrong)
                        .copyWith(color: c.textPrimary),
                  ),
                if (canStop)
                  DsButton(
                    key: const ValueKey('runGraph.stop'),
                    label: l.runningStop,
                    onPressed: () => unawaited(stopRun(engine, summary!)),
                  ),
              ],
            ),
          ),
          Divider(
            height: DsBorderWidth.hairline,
            thickness: DsBorderWidth.hairline,
            color: c.borderSubtle,
          ),
          Expanded(
            child: run == null
                ? Center(
                    child: Text(
                      live.isLoading || (ended && doc == null)
                          ? l.runGraphLoading
                          : l.runGraphUnavailable,
                      key: const ValueKey('runGraph.empty'),
                      style: dsTextStyle(DsType.body)
                          .copyWith(color: c.textSecondary),
                    ),
                  )
                : Padding(
                    padding: const EdgeInsets.symmetric(
                      horizontal: DsSpace.s16,
                    ),
                    child: RunGraphView(
                      key: ValueKey('runGraph.view.$runId'),
                      graph: RunGraphModel.fromRun(
                        run,
                        now,
                        agentName: agentName,
                      ),
                      visibleSteps: config.liveboxVisibleSteps,
                      announceMinInterval: config.liveAnnounceMinInterval,
                      clock: clock.now,
                    ),
                  ),
          ),
        ],
      ),
    );
  }
}
