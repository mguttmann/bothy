// SPDX-License-Identifier: Apache-2.0

/// Words, icons and colors of the run graph rows: the run state roles of the live box
/// (components.md 2; docs/spec/09-agents.md 5.6 rule 6: state as color, icon and word).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/widgets.dart';
import 'package:l10n/l10n.dart';

import '../marks/viz_texts.dart';
import 'run_graph_model.dart';

/// The run state of the live box that colors [state]; a pending step looks queued.
DsRunState runGraphDsState(RunGraphState state) => switch (state) {
  RunGraphState.queued || RunGraphState.pending => DsRunState.queued,
  RunGraphState.running => DsRunState.running,
  RunGraphState.waitingInput => DsRunState.waitingInput,
  RunGraphState.completed => DsRunState.completed,
  RunGraphState.failed => DsRunState.failed,
  RunGraphState.cancelled => DsRunState.cancelled,
  RunGraphState.unknown => DsRunState.unknown,
};

/// The color of [state]: the run state role of the live box.
Color runGraphStateColor(BuildContext context, RunGraphState state) =>
    dsRunStateColor(context, runGraphDsState(state));

/// The icon of [state]; the state is never color alone (01 8 "Color").
DsGlyph runGraphStateGlyph(RunGraphState state) =>
    dsRunStateGlyph(runGraphDsState(state));

/// The icon of a node [kind].
DsGlyph runGraphKindGlyph(RunGraphNodeKind kind) => switch (kind) {
  RunGraphNodeKind.run => DsIcons.agent,
  RunGraphNodeKind.toolCall => DsIcons.stepToolCall,
  RunGraphNodeKind.subagent => DsIcons.stepSubagent,
  RunGraphNodeKind.workflow => DsIcons.stepWorkflow,
  RunGraphNodeKind.backgroundJob => DsIcons.stepBackgroundJob,
  RunGraphNodeKind.summary || RunGraphNodeKind.unknown => DsIcons.stepStatus,
};

/// The texts of the run graph and the control room card for one locale.
///
/// ```dart
/// final texts = RunGraphTexts.of(context);
/// texts.state(RunGraphState.running); // "Running"
/// ```
final class RunGraphTexts {
  /// Creates the texts on [viz].
  RunGraphTexts(this.viz);

  /// The texts of the ambient `Localizations`.
  factory RunGraphTexts.of(BuildContext context) =>
      RunGraphTexts(VizTexts.of(context));

  /// The shared texts of the marks (money, numbers).
  final VizTexts viz;

  /// The generated texts.
  AppLocalizations get l10n => viz.l10n;

  /// The state word of [state].
  String state(RunGraphState state) => switch (state) {
    RunGraphState.queued => l10n.liveBoxStateQueued,
    RunGraphState.pending => l10n.runGraphStatePending,
    RunGraphState.running => l10n.liveBoxStateRunning,
    RunGraphState.waitingInput => l10n.vizRunStateWaitingInput,
    RunGraphState.completed => l10n.liveBoxStateCompleted,
    RunGraphState.failed => l10n.liveBoxStateFailed,
    RunGraphState.cancelled => l10n.liveBoxStateCancelled,
    RunGraphState.unknown => l10n.vizRunStateUnknown,
  };

  /// The kind word of [kind]; null for the root and a summary row, which need none.
  String? kind(RunGraphNodeKind kind) => switch (kind) {
    RunGraphNodeKind.toolCall => l10n.toolChipKindToolCall,
    RunGraphNodeKind.subagent => l10n.toolChipKindSubagent,
    RunGraphNodeKind.workflow => l10n.toolChipKindWorkflow,
    RunGraphNodeKind.backgroundJob => l10n.toolChipKindBackgroundJob,
    RunGraphNodeKind.unknown => l10n.toolChipKindUnknown,
    RunGraphNodeKind.run || RunGraphNodeKind.summary => null,
  };

  /// The label of [node]: "Subagent tests", the agent for the root, "12 steps summarized".
  String label(RunGraphNode node) {
    if (node.kind == RunGraphNodeKind.summary) {
      return l10n.runGraphStepsSummarized(node.summarized ?? 0);
    }
    final word = kind(node.kind);
    return word == null ? node.name : l10n.runGraphRowLabel(word, node.name);
  }

  /// The facts of [node] in reading order after its label: title, the tool that started it,
  /// state, duration, current tool, markers, tokens, cost (09 5.6 rule 6) and, on a running
  /// root, the stall notice counted to [now] (09 5.5).
  List<String> facts(RunGraphNode node, {required DateTime now}) {
    final f = viz.format;
    final tool = node.currentTool;
    final stalled = node.stalledSince;
    return [
      if (node.title.isNotEmpty) node.title,
      if (node.subtitle != null) l10n.runGraphSubtitle(node.subtitle!),
      if (node.kind != RunGraphNodeKind.summary) state(node.state),
      if (node.kind != RunGraphNodeKind.summary) f.elapsed(node.duration),
      if (tool != null) l10n.vizStepToolCall(tool),
      for (final m in node.markers) m.title.isEmpty ? m.name : m.title,
      if (node.tokens != null) l10n.liveBoxTokens(f.count(node.tokens!)),
      if (node.cost != null) viz.cost(node.cost!),
      if (stalled != null && node.isRunning)
        // A mark ahead of the device clock (clock skew) reads zero, as on the card.
        l10n.liveBoxStall(
          now.isBefore(stalled) ? 0 : now.difference(stalled).inMinutes,
        ),
    ];
  }
}
