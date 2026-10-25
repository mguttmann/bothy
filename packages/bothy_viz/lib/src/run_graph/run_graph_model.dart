// SPDX-License-Identifier: Apache-2.0

/// The run graph: the pure derivation of a tree from the run document
/// (docs/spec/09-agents.md 5.6; design note run-visualization D-RV-1, D-RV-2, D-RV-5, D-RV-6).
library;

import 'dart:collection';

import 'package:api_client/api_client.dart' as api;
import 'package:built_collection/built_collection.dart';
import 'package:built_value/json_object.dart';
import 'package:domain/domain.dart' show CostLabel;

import '../model/costs.dart';

/// What a row of the run graph stands for.
enum RunGraphNodeKind {
  /// The run itself, the root (09 5.6 rule 1).
  run,

  /// A step `tool_call`.
  toolCall,

  /// A step `subagent`, or a tool call merged with its single subagent (rule 4).
  subagent,

  /// A step `workflow`, or a tool call merged with its single workflow.
  workflow,

  /// A step `background_job`, or a tool call merged with its single job.
  backgroundJob,

  /// The summary step `steps_summarized` of 09 5.3: the row "n steps summarized".
  summary,

  /// A step kind of a newer server; the row shows the name only.
  unknown,
}

/// The state of a row: the run states of the root (09 5.1) and the step states of the
/// children, colored with the run state roles of the live box (components.md 2).
enum RunGraphState {
  /// Run `queued`.
  queued,

  /// Step `pending`.
  pending,

  /// Run or step `running`.
  running,

  /// Run `waiting_input`.
  waitingInput,

  /// Run or step `completed`.
  completed,

  /// Run or step `failed`.
  failed,

  /// Run or step `cancelled`.
  cancelled,

  /// A state of a newer server.
  unknown;

  /// Whether the row still works: queued, pending, running or waiting for input.
  bool get isOpen =>
      this == queued ||
      this == pending ||
      this == running ||
      this == waitingInput;
}

/// A step of kind `status` drawn as a marker on the row of its parent (09 5.6 rule 2).
final class RunGraphMarker {
  /// Creates a marker.
  const RunGraphMarker({
    required this.id,
    required this.name,
    required this.title,
    required this.state,
  });

  /// The step ID.
  final String id;

  /// The step `name`, for example `compacting`.
  final String name;

  /// The step `title`, possibly empty.
  final String title;

  /// The step state.
  final RunGraphState state;
}

/// One node of the run graph: the root or a step, with its children in `ordinal` order.
final class RunGraphNode {
  RunGraphNode._({
    required this.id,
    required this.kind,
    required this.name,
    required this.title,
    required this.state,
    required this.ordinal,
    required this.start,
    required this.end,
    required this.duration,
    required this.children,
    required this.markers,
    this.subtitle,
    this.tokens,
    this.cost,
    this.stalledSince,
    this.summarized,
  });

  /// The run ID for the root, else the step ID (of the task for a merged node).
  final String id;

  /// What the row stands for.
  final RunGraphNodeKind kind;

  /// The agent name for the root, else the step `name`.
  final String name;

  /// The step `title`, possibly empty; empty for the root.
  final String title;

  /// The tool name of a merged tool call (rule 4), else null.
  final String? subtitle;

  /// The state as color, icon and word.
  final RunGraphState state;

  /// The step `ordinal`; 0 for the root.
  final int ordinal;

  /// Start of the bar: `started_at` (the root: else `queued_at`).
  final DateTime start;

  /// End of the bar: `finished_at`, or null while the node runs (the bar ends at now).
  final DateTime? end;

  /// The time from [start] to [end] or to the derivation's now; never negative.
  final Duration duration;

  /// The child nodes in `ordinal` order.
  final List<RunGraphNode> children;

  /// The status steps of this row (rule 2).
  final List<RunGraphMarker> markers;

  /// Tokens from `detail.usage.tokens`, where the adapter reported them (rule 6).
  final int? tokens;

  /// The cost: the run cost for the root (organization only), a reported `usage` cost for a
  /// child (organization only, always an estimate); never a share of the run cost (D-RV-5).
  final VizCost? cost;

  /// `stall_warned_at` of the run; only on the root.
  final DateTime? stalledSince;

  /// The `count` of a summary row ([RunGraphNodeKind.summary]).
  final int? summarized;

  /// The newest child `tool_call` with `status = running`, counted after the merge rule.
  String? get currentTool {
    for (final c in children.reversed) {
      if (c.kind == RunGraphNodeKind.toolCall &&
          c.state == RunGraphState.running) {
        return c.name;
      }
    }
    return null;
  }

  /// Whether the node still works ([RunGraphState.isOpen]).
  bool get isRunning => state.isOpen;
}

/// A row of the trace view and of its list form.
sealed class RunGraphRow {
  const RunGraphRow(this.depth);

  /// The indentation level: 0 for the root.
  final int depth;
}

/// A row that shows [node].
final class RunGraphNodeRow extends RunGraphRow {
  /// Creates the row.
  const RunGraphNodeRow({
    required this.node,
    required int depth,
    required this.position,
    required this.siblings,
    required this.hasChildren,
    required this.expanded,
  }) : super(depth);

  /// The node.
  final RunGraphNode node;

  /// The position among the siblings, from 1 (tree semantics).
  final int position;

  /// The number of siblings including this one.
  final int siblings;

  /// Whether the node has children the person can show or hide.
  final bool hasChildren;

  /// Whether its children are shown.
  final bool expanded;
}

/// The row "n earlier steps" in place of the oldest finished children of [parentId] (09 5.6
/// rule 7); expanding it adds [parentId] to the `expanded` set of [RunGraphModel.rows].
final class RunGraphEarlierRow extends RunGraphRow {
  /// Creates the row.
  const RunGraphEarlierRow({
    required this.parentId,
    required this.count,
    required int depth,
  }) : super(depth);

  /// The node whose children are hidden.
  final String parentId;

  /// The number of hidden children.
  final int count;
}

/// The run graph of one run document (09 5.6): the root with its tree, the time axis and the
/// numbers the control room card shows.
///
/// One pure function of the document and the time: [RunGraphModel.fromRun] runs in time
/// linear in the number of steps (a map by ID, one pass for the children, one for the merge
/// rule), so the app derives it again on every applied patch.
///
/// ```dart
/// final graph = RunGraphModel.fromRun(run, clock.now(), agentName: 'dev-runner');
/// final rows = graph.rows(visibleSteps: config.liveboxVisibleSteps);
/// ```
final class RunGraphModel {
  RunGraphModel._({
    required this.root,
    required this.axisStart,
    required this.axisEnd,
    required this.nodeCount,
    required this.currentTool,
    required this.openSubagents,
    required this.openWorkflows,
    required this.openBackgroundJobs,
  });

  /// Derives the graph of [run] at [now]; [agentName] labels the root (the agent ID without
  /// one).
  ///
  /// Rules of 09 5.6: the run is the root (1); every step except kind `status` is a node, a
  /// `status` step is a marker on the row of its parent and the summary step
  /// `steps_summarized` a row of its own (2); a node hangs under its parent, under the root
  /// while the parent is unknown (3); a tool call with exactly one child of a task kind is
  /// drawn as that task (4); children in `ordinal` order, bars from `started_at` to
  /// `finished_at` or [now], the axis from the run's `started_at` (else `queued_at`) (5); a
  /// child's tokens and cost only from `detail.usage` (6). A parent cycle, which the server
  /// never stores, puts the first node of the cycle under the root, so no step is lost.
  factory RunGraphModel.fromRun(
    api.Run run,
    DateTime now, {
    String? agentName,
  }) {
    final organization = run.billingModeUsed == api.BillingMode.organization;
    final steps = run.steps.values.toList()..sort(_byOrdinal);
    final byId = <String, api.RunStep>{for (final s in steps) s.id: s};

    // One pass: children per parent ID (null for the root), in ordinal order because the
    // steps are sorted; a parent the client has not received counts as the root (rule 3).
    final childrenOf = <String?, List<api.RunStep>>{};
    for (final s in steps) {
      final parent = s.parentStepId;
      final key = parent != null && byId.containsKey(parent) ? parent : null;
      (childrenOf[key] ??= []).add(s);
    }

    final axisStart = run.startedAt ?? run.queuedAt;
    final axisEnd = run.finishedAt ?? now;
    var count = 0;
    final visited = HashSet<String>();
    String? newestTool;
    var newestToolOrdinal = -1;
    var subagents = 0;
    var workflows = 0;
    var jobs = 0;

    RunGraphNode build(api.RunStep s, List<api.RunStep> ownChildren) {
      visited.add(s.id);
      var source = s;
      String? subtitle;
      var kids = ownChildren;
      // Rule 4: a tool call with exactly one task child is drawn as that task.
      if (s.kind == api.RunStepKindEnum.toolCall) {
        final tasks = [
          for (final c in ownChildren)
            if (_isTask(c.kind)) c,
        ];
        if (tasks.length == 1) {
          final task = tasks.single;
          visited.add(task.id);
          source = task;
          subtitle = s.name;
          kids = [
            for (final c in ownChildren)
              if (c.id != task.id) c,
            ...?childrenOf[task.id],
          ]..sort(_byOrdinal);
        }
      }
      final markers = <RunGraphMarker>[];
      final children = <RunGraphNode>[];
      for (final c in kids) {
        if (visited.contains(c.id)) continue;
        if (c.kind == api.RunStepKindEnum.status && !_isSummary(c)) {
          visited.add(c.id);
          markers.add(_marker(c));
          continue;
        }
        children.add(build(c, childrenOf[c.id] ?? const []));
      }
      count++;
      final kind = _isSummary(source)
          ? RunGraphNodeKind.summary
          : _kindOf(source.kind);
      final state = _stepState(source.status);
      if (state.isOpen) {
        switch (kind) {
          case RunGraphNodeKind.subagent:
            subagents++;
          case RunGraphNodeKind.workflow:
            workflows++;
          case RunGraphNodeKind.backgroundJob:
            jobs++;
          case RunGraphNodeKind.toolCall when state == RunGraphState.running:
            if (source.ordinal > newestToolOrdinal) {
              newestToolOrdinal = source.ordinal;
              newestTool = source.name;
            }
          default:
        }
      }
      final usage = _usage(source.detail);
      final micros = usage?['cost_micros'];
      final currency = usage?['currency'];
      final tokens = usage?['tokens'];
      final start = source.startedAt;
      final end = source.finishedAt;
      return RunGraphNode._(
        id: source.id,
        kind: kind,
        name: source.name,
        title: source.title,
        subtitle: subtitle,
        state: state,
        ordinal: s.ordinal,
        start: start,
        end: end,
        duration: _between(start, end ?? now),
        children: List.unmodifiable(children),
        markers: List.unmodifiable(markers),
        tokens: tokens is int ? tokens : null,
        cost: organization && micros is int && currency is String
            ? VizCost(
                micros: micros,
                currency: currency,
                mode: api.BillingMode.organization,
                label: CostLabel.estimate,
              )
            : null,
        summarized: _summarizedCount(source),
      );
    }

    final rootMarkers = <RunGraphMarker>[];
    final rootChildren = <RunGraphNode>[];
    void attach(api.RunStep s) {
      if (s.kind == api.RunStepKindEnum.status && !_isSummary(s)) {
        visited.add(s.id);
        rootMarkers.add(_marker(s));
      } else {
        rootChildren.add(build(s, childrenOf[s.id] ?? const []));
      }
    }

    for (final s in childrenOf[null] ?? const <api.RunStep>[]) {
      if (!visited.contains(s.id)) attach(s);
    }
    // Steps in a parent cycle are reached from no root: the first of each cycle joins the
    // root, which draws the rest of the cycle below it.
    for (final s in steps) {
      if (!visited.contains(s.id)) attach(s);
    }
    rootChildren.sort((a, b) => a.ordinal.compareTo(b.ordinal));

    final mode = run.billingModeUsed ?? run.billingModeExpected;
    final cost = run.cost;
    final root = RunGraphNode._(
      id: run.id,
      kind: RunGraphNodeKind.run,
      name: agentName ?? run.agentId,
      title: '',
      state: _runState(run.state),
      ordinal: 0,
      start: axisStart,
      end: run.finishedAt,
      duration: run.durationMs != null
          ? Duration(milliseconds: run.durationMs!)
          : _between(axisStart, axisEnd),
      children: List.unmodifiable(rootChildren),
      markers: List.unmodifiable(rootMarkers),
      cost: mode == api.BillingMode.organization
          ? VizCost.of(
              micros: cost.amountMicros,
              currency: cost.currency,
              mode: mode,
              isEstimate: cost.isEstimate,
            )
          : null,
      stalledSince: run.stallWarnedAt,
    );
    return RunGraphModel._(
      root: root,
      axisStart: axisStart,
      axisEnd: axisEnd.isBefore(axisStart) ? axisStart : axisEnd,
      nodeCount: count,
      currentTool: newestTool,
      openSubagents: subagents,
      openWorkflows: workflows,
      openBackgroundJobs: jobs,
    );
  }

  /// The root: the run.
  final RunGraphNode root;

  /// Start of the time axis: the run's `started_at`, else `queued_at`.
  final DateTime axisStart;

  /// End of the time axis: the run's `finished_at`, else the derivation's now.
  final DateTime axisEnd;

  /// The number of nodes below the root (summary rows included, markers not).
  final int nodeCount;

  /// The newest running tool call of the whole run (09 5.4 "Current tool"), for the card.
  final String? currentTool;

  /// Open subagents (pending or running).
  final int openSubagents;

  /// Open workflows.
  final int openWorkflows;

  /// Open background jobs.
  final int openBackgroundJobs;

  /// The rows of the trace view in tree order (09 5.6 rule 7).
  ///
  /// A node with more than [visibleSteps] (`ui.livebox_visible_steps`) finished children
  /// shows the newest [visibleSteps] of them and one [RunGraphEarlierRow] in place of the
  /// older ones, unless its ID is in [expanded]. A child that runs or has a running
  /// descendant is never hidden, so the running path stays visible. A node whose ID is in
  /// [collapsed] (closed by the person) shows no children.
  List<RunGraphRow> rows({
    required int visibleSteps,
    Set<String> expanded = const {},
    Set<String> collapsed = const {},
  }) {
    final out = <RunGraphRow>[];
    final busy = HashMap<RunGraphNode, bool>.identity();
    bool runningPath(RunGraphNode n) =>
        busy[n] ??= n.isRunning || n.children.any(runningPath);

    void walk(RunGraphNode node, int depth, int position, int siblings) {
      final open = !collapsed.contains(node.id);
      out.add(
        RunGraphNodeRow(
          node: node,
          depth: depth,
          position: position,
          siblings: siblings,
          hasChildren: node.children.isNotEmpty,
          expanded: open,
        ),
      );
      if (!open || node.children.isEmpty) return;
      final kids = node.children;
      final hidden = HashSet<RunGraphNode>.identity();
      if (!expanded.contains(node.id)) {
        final finished = [
          for (final c in kids)
            if (!runningPath(c)) c,
        ];
        if (finished.length > visibleSteps) {
          hidden.addAll(finished.take(finished.length - visibleSteps));
        }
      }
      var earlierShown = false;
      for (var i = 0; i < kids.length; i++) {
        final c = kids[i];
        if (hidden.contains(c)) {
          if (!earlierShown) {
            earlierShown = true;
            out.add(
              RunGraphEarlierRow(
                parentId: node.id,
                count: hidden.length,
                depth: depth + 1,
              ),
            );
          }
          continue;
        }
        walk(c, depth + 1, i + 1, kids.length);
      }
    }

    walk(root, 0, 1, 1);
    return out;
  }
}

int _byOrdinal(api.RunStep a, api.RunStep b) {
  final c = a.ordinal.compareTo(b.ordinal);
  return c != 0 ? c : a.id.compareTo(b.id);
}

bool _isTask(api.RunStepKindEnum kind) =>
    kind == api.RunStepKindEnum.subagent ||
    kind == api.RunStepKindEnum.workflow ||
    kind == api.RunStepKindEnum.backgroundJob;

/// The summary step of 09 5.3: kind `status`, name `steps_summarized`.
bool _isSummary(api.RunStep s) =>
    s.kind == api.RunStepKindEnum.status && s.name == 'steps_summarized';

RunGraphMarker _marker(api.RunStep s) => RunGraphMarker(
  id: s.id,
  name: s.name,
  title: s.title,
  state: _stepState(s.status),
);

RunGraphNodeKind _kindOf(api.RunStepKindEnum kind) =>
    <api.RunStepKindEnum, RunGraphNodeKind>{
      api.RunStepKindEnum.toolCall: RunGraphNodeKind.toolCall,
      api.RunStepKindEnum.subagent: RunGraphNodeKind.subagent,
      api.RunStepKindEnum.workflow: RunGraphNodeKind.workflow,
      api.RunStepKindEnum.backgroundJob: RunGraphNodeKind.backgroundJob,
    }[kind] ??
    RunGraphNodeKind.unknown;

RunGraphState _stepState(api.RunStepStatusEnum status) =>
    <api.RunStepStatusEnum, RunGraphState>{
      api.RunStepStatusEnum.pending: RunGraphState.pending,
      api.RunStepStatusEnum.running: RunGraphState.running,
      api.RunStepStatusEnum.completed: RunGraphState.completed,
      api.RunStepStatusEnum.failed: RunGraphState.failed,
      api.RunStepStatusEnum.cancelled: RunGraphState.cancelled,
    }[status] ??
    RunGraphState.unknown;

RunGraphState _runState(api.RunState state) =>
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

/// The object `detail.usage` (09 5.6, `run_step_detail.v1`) as plain values, or null.
Map<String, Object?>? _usage(Object? detail) {
  final usage = _member(detail, 'usage');
  return usage is Map ? usage.cast<String, Object?>() : null;
}

int? _summarizedCount(api.RunStep s) {
  if (!_isSummary(s)) return null;
  final summarized = _member(s.detail, 'summarized');
  final count = summarized is Map ? summarized['count'] : null;
  return count is int ? count : 0;
}

/// The plain value of [name] in a `detail` map of the generated type.
Object? _member(Object? detail, String name) {
  final value = switch (detail) {
    BuiltMap<String, JsonObject?>() => detail[name],
    Map() => detail[name],
    _ => null,
  };
  return value is JsonObject ? value.value : value;
}
