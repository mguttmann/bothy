// SPDX-License-Identifier: Apache-2.0

/// The card of the control room "Running now" (docs/spec/01-product.md 5.1 D10;
/// docs/spec/09-agents.md 5.6; design note run-visualization D-RV-7, D-RV-8).
library;

import 'package:api_client/api_client.dart' as api;
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';

import '../marks/run_card_mark.dart' show RunCardPainter;
import '../model/costs.dart';
import 'run_graph_model.dart';
import 'run_graph_texts.dart';

/// One active run as a control room card.
///
/// A card is live while the host holds the run document (the card's thread is in the focus,
/// 04 6.2): then it shows the current tool, the open subagents, workflows and background jobs
/// and the stall notice of the document. Without the document it shows the summary of
/// `listActiveRuns` and `run.state_changed` with "live details paused"; its stall notice is
/// `stall_warned_at` as of the last `listActiveRuns` load (09 5.6).
final class RunControlCardModel {
  /// Creates a card; [RunControlCardModel.from] is the normal way.
  const RunControlCardModel({
    required this.runId,
    required this.threadId,
    required this.agentName,
    required this.path,
    required this.state,
    required this.live,
    required this.canStop,
    this.duration,
    this.cost,
    this.currentTool,
    this.openSubagents = 0,
    this.openWorkflows = 0,
    this.openBackgroundJobs = 0,
    this.stalledFor,
  });

  /// Derives the card of [summary] at [now]; [document] is the run document while the card
  /// is live (a document of another run is ignored).
  ///
  /// [agentName] and [path] (space, channel, thread title) come from the host's stored
  /// names; [canStop] is what the server's answers allow for `run.cancel` (15 1 principle 1:
  /// the card derives no permission). The cost follows the suffix rules of D10: an
  /// organization run is an estimate, a personal run "information only"; a queued run shows
  /// neither duration nor cost.
  factory RunControlCardModel.from(
    api.RunSummary summary, {
    required DateTime now,
    required String agentName,
    List<String> path = const [],
    bool canStop = false,
    api.Run? document,
  }) {
    final doc = document != null && document.id == summary.id ? document : null;
    final graph = doc == null
        ? null
        : RunGraphModel.fromRun(doc, now, agentName: agentName);
    final state = graph?.root.state ?? _summaryState(summary.state);
    final started = doc?.startedAt ?? summary.startedAt;
    final mode =
        doc?.billingModeUsed ??
        summary.billingModeUsed ??
        summary.billingModeExpected;
    final money = doc?.cost ?? summary.cost;
    final stalled = doc == null ? summary.stallWarnedAt : doc.stallWarnedAt;
    final queued = state == RunGraphState.queued || started == null;
    return RunControlCardModel(
      runId: summary.id,
      threadId: summary.threadId,
      agentName: agentName,
      path: List.unmodifiable(path),
      state: state,
      live: doc != null,
      canStop: canStop,
      duration: queued ? null : _between(started, summary.finishedAt ?? now),
      cost: queued
          ? null
          : VizCost.of(
              micros: money.amountMicros,
              currency: money.currency,
              mode: mode,
              isEstimate: money.isEstimate,
            ),
      currentTool: graph?.currentTool,
      openSubagents: graph?.openSubagents ?? 0,
      openWorkflows: graph?.openWorkflows ?? 0,
      openBackgroundJobs: graph?.openBackgroundJobs ?? 0,
      stalledFor: stalled == null || state != RunGraphState.running
          ? null
          : _between(stalled, now),
    );
  }

  /// Run ID.
  final String runId;

  /// Thread of the run.
  final String threadId;

  /// Display name of the agent.
  final String agentName;

  /// Space, channel and thread title, the parts the host knows.
  final List<String> path;

  /// State of the run.
  final RunGraphState state;

  /// Whether the card shows the run document (its thread has a focus slot).
  final bool live;

  /// Whether Stop shows.
  final bool canStop;

  /// Run time so far; null while queued.
  final Duration? duration;

  /// Cost with its label; null while queued.
  final VizCost? cost;

  /// The newest running tool call; only while [live].
  final String? currentTool;

  /// Open subagents; only while [live].
  final int openSubagents;

  /// Open workflows; only while [live].
  final int openWorkflows;

  /// Open background jobs; only while [live].
  final int openBackgroundJobs;

  /// Time since the server marked the stall (09 5.5); null without a mark.
  final Duration? stalledFor;
}

RunGraphState _summaryState(api.RunState state) =>
    <api.RunState, RunGraphState>{
      api.RunState.queued: RunGraphState.queued,
      api.RunState.running: RunGraphState.running,
      api.RunState.waitingInput: RunGraphState.waitingInput,
      api.RunState.completed: RunGraphState.completed,
      api.RunState.failed: RunGraphState.failed,
      api.RunState.cancelled: RunGraphState.cancelled,
    }[state] ??
    RunGraphState.unknown;

Duration _between(DateTime from, DateTime to) {
  final d = to.difference(from);
  return d.isNegative ? Duration.zero : d;
}

/// The control room card of [model]: agent, state with icon and word, duration, cost, path,
/// current tool, open subagents, workflows and jobs, the stall notice, "live details paused"
/// when not live, and the buttons Graph and Stop (only with [RunControlCardModel.canStop]) or
/// Open. A tap on the card opens the run graph as Graph does.
///
/// Screen readers read the card once as a button that opens the graph; the buttons stay
/// reachable on their own.
///
/// [expand] is the phone's card (01 5.3 M11): it takes the width it gets, one card per row,
/// and Graph and Stop (or Open) stand stacked at full width; else the card has the width of a
/// list column for the desktop's grid.
class RunControlCard extends StatelessWidget {
  /// Creates the card.
  const RunControlCard({
    super.key,
    required this.model,
    required this.onOpenGraph,
    this.onStop,
    this.onOpen,
    this.expand = false,
  });

  /// The card.
  final RunControlCardModel model;

  /// Opens the run graph.
  final VoidCallback onOpenGraph;

  /// Stops the run; the button shows only with [RunControlCardModel.canStop].
  final VoidCallback? onStop;

  /// Opens the thread; the button shows when Stop does not.
  final VoidCallback? onOpen;

  /// Whether the card fills the width with full-width buttons (the phone).
  final bool expand;

  /// The texts of the card in reading order (its semantics label).
  static List<String> parts(RunGraphTexts texts, RunControlCardModel model) {
    final l = texts.l10n;
    final f = texts.viz.format;
    final tool = model.currentTool;
    return [
      model.agentName,
      texts.state(model.state),
      if (model.duration != null) f.elapsed(model.duration!),
      if (model.cost != null) texts.viz.cost(model.cost!),
      if (model.path.isNotEmpty) model.path.join(' > '),
      if (tool != null) l.vizStepToolCall(tool),
      ...openWork(texts, model),
      if (model.stalledFor != null) l.liveBoxStall(model.stalledFor!.inMinutes),
      if (!model.live) l.runningLivePaused,
    ];
  }

  /// "2 subagents", "1 workflow", "1 background job", only the kinds that are open.
  static List<String> openWork(
    RunGraphTexts texts,
    RunControlCardModel model,
  ) => [
    if (model.openSubagents > 0)
      texts.l10n.runningSubagents(model.openSubagents),
    if (model.openWorkflows > 0)
      texts.l10n.runningWorkflows(model.openWorkflows),
    if (model.openBackgroundJobs > 0)
      texts.l10n.runningBackgroundJobs(model.openBackgroundJobs),
  ];

  @override
  Widget build(BuildContext context) {
    final texts = RunGraphTexts.of(context);
    final l = texts.l10n;
    final f = texts.viz.format;
    final colors = context.dsColors;
    final secondary = dsTextStyle(DsType.bodySm)
        .copyWith(color: colors.textSecondary);
    final stateColor = runGraphStateColor(context, model.state);
    final work = openWork(texts, model);
    final info = Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        Text(
          model.agentName,
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
          style: dsTextStyle(DsType.label).copyWith(color: colors.textPrimary),
        ),
        const SizedBox(height: DsSpace.s4),
        Wrap(
          spacing: DsSpace.s8,
          runSpacing: DsSpace.s4,
          crossAxisAlignment: WrapCrossAlignment.center,
          children: [
            Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                DsIcon(
                  runGraphStateGlyph(model.state),
                  size: DsIconSizeRole.sm,
                  color: stateColor,
                  decorative: true,
                ),
                const SizedBox(width: DsSpace.s4),
                Flexible(
                  child: Text(texts.state(model.state), style: secondary),
                ),
              ],
            ),
            if (model.duration != null)
              Text(
                f.elapsed(model.duration!),
                style: dsTextStyle(DsType.codeSm)
                    .copyWith(color: colors.textSecondary),
              ),
            if (model.cost != null)
              Text(texts.viz.cost(model.cost!), style: secondary),
          ],
        ),
        if (model.path.isNotEmpty) ...[
          const SizedBox(height: DsSpace.s4),
          Text(
            model.path.join(' > '),
            maxLines: 2,
            overflow: TextOverflow.ellipsis,
            style: dsTextStyle(DsType.bodySm)
                .copyWith(color: colors.textPrimary),
          ),
        ],
        if (model.currentTool != null) ...[
          const SizedBox(height: DsSpace.s4),
          Text(
            l.vizStepToolCall(model.currentTool!),
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: dsTextStyle(DsType.codeSm)
                .copyWith(color: colors.textPrimary),
          ),
        ],
        if (work.isNotEmpty) ...[
          const SizedBox(height: DsSpace.s4),
          Text(work.join(', '), style: secondary),
        ],
        if (model.stalledFor != null) ...[
          const SizedBox(height: DsSpace.s4),
          Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              DsIcon(
                DsIcons.stateWarning,
                size: DsIconSizeRole.sm,
                color: colors.warningFg,
                decorative: true,
              ),
              const SizedBox(width: DsSpace.s4),
              Flexible(
                child: Text(
                  l.liveBoxStall(model.stalledFor!.inMinutes),
                  style: dsTextStyle(DsType.bodySm)
                      .copyWith(color: colors.warningFg),
                ),
              ),
            ],
          ),
        ],
        if (!model.live) ...[
          const SizedBox(height: DsSpace.s4),
          Text(
            l.runningLivePaused,
            style: dsTextStyle(DsType.caption)
                .copyWith(color: colors.textSecondary),
          ),
        ],
      ],
    );
    final buttons = [
      DsButton(
        key: ValueKey('runCard.graph.${model.runId}'),
        label: l.runningGraph,
        expand: expand,
        onPressed: onOpenGraph,
      ),
      if (model.canStop && onStop != null)
        DsButton(
          key: ValueKey('runCard.stop.${model.runId}'),
          label: l.runningStop,
          expand: expand,
          onPressed: onStop,
        )
      else if (onOpen != null)
        DsButton(
          key: ValueKey('runCard.open.${model.runId}'),
          label: l.runningOpen,
          expand: expand,
          onPressed: onOpen,
        ),
    ];
    return SizedBox(
      width: expand ? double.infinity : DsLayout.listWidth,
      child: CustomPaint(
        painter: RunCardPainter(
          fill: colors.surfaceRaised,
          border: colors.borderSubtle,
          spine: stateColor,
        ),
        child: Padding(
          padding: EdgeInsets.fromLTRB(
            DsSpace.s12 + DsBorderWidth.spine,
            context.dsDensity.liveboxPad,
            DsSpace.s12,
            context.dsDensity.liveboxPad,
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            mainAxisSize: MainAxisSize.min,
            children: [
              Semantics(
                container: true,
                button: true,
                label: parts(texts, model).join(', '),
                onTapHint: l.runningCardOpenGraph(model.agentName),
                onTap: onOpenGraph,
                child: ExcludeSemantics(
                  child: GestureDetector(
                    key: ValueKey('runCard.body.${model.runId}'),
                    behavior: HitTestBehavior.opaque,
                    onTap: onOpenGraph,
                    child: info,
                  ),
                ),
              ),
              const SizedBox(height: DsSpace.s8),
              if (expand)
                for (final (i, button) in buttons.indexed) ...[
                  if (i > 0) const SizedBox(height: DsSpace.s6),
                  button,
                ]
              else
                Wrap(
                  alignment: WrapAlignment.spaceBetween,
                  spacing: DsSpace.s8,
                  runSpacing: DsSpace.s4,
                  children: buttons,
                ),
            ],
          ),
        ),
      ),
    );
  }
}

/// The `RunSummary` the server lists for the run document [run] (`listActiveRuns`,
/// `run.state_changed`): the same members, `step_count` as the number of stored steps.
///
/// The harness and the tests build the summary-only form of a card from a fixture with it.
api.RunSummary runSummaryOf(api.Run run) => api.RunSummary(
  (b) => b
    ..id = run.id
    ..threadId = run.threadId
    ..channelId = run.channelId
    ..agentId = run.agentId
    ..triggeredBy = run.triggeredBy
    ..state = run.state
    ..stateReason = run.stateReason
    ..version = run.version
    ..billingModeExpected = run.billingModeExpected
    ..billingModeUsed = run.billingModeUsed
    ..statusLine = run.statusLine
    ..summary = run.summary
    ..cost.replace(run.cost)
    ..stepCount = run.steps.length
    ..durationMs = run.durationMs
    ..queuedAt = run.queuedAt
    ..startedAt = run.startedAt
    ..finishedAt = run.finishedAt
    ..stallWarnedAt = run.stallWarnedAt,
);
