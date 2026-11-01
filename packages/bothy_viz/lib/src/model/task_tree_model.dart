// SPDX-License-Identifier: Apache-2.0

/// The agent tree below a run card of the live topology (docs/spec/25-visualization.md 5.2 and
/// 6.1; docs/design/topology-agent-tree.md).
library;

import 'package:api_client/api_client.dart' as api;
import 'package:flutter/foundation.dart';

/// Kind of a task node.
enum TaskNodeKind {
  /// `subagent`: an agent of the Agent tool or of a workflow.
  subagent,

  /// `workflow`.
  workflow,

  /// `background_job`.
  backgroundJob,

  /// A kind of a newer server.
  unknown,
}

/// State of a task node.
enum TaskNodeState {
  /// `running`.
  running,

  /// `completed`.
  completed,

  /// `failed`.
  failed,

  /// `cancelled`.
  cancelled,

  /// A state of a newer server.
  unknown;

  /// Whether the node has ended; a finished node is drawn faded.
  bool get isFinished =>
      this == completed || this == failed || this == cancelled;
}

/// One node of the agent tree with its children in frame order.
@immutable
final class TaskNodeModel {
  /// Creates a node; [TaskNodeModel.tree] is the normal way.
  const TaskNodeModel({
    required this.id,
    required this.kind,
    required this.name,
    required this.state,
    required this.runtime,
    required this.position,
    this.label,
    this.tool,
    this.children = const [],
  });

  /// Builds the forest of [tasks] (a parent before its children, as the frame orders them);
  /// [at] is the time of the frame, against which a running node counts its runtime. A node
  /// whose parent is not an earlier node of the list hangs under the run.
  static List<TaskNodeModel> tree(
    Iterable<api.StatsLiveTask> tasks,
    DateTime at,
  ) {
    final children = <String, List<String>>{};
    final roots = <String>[];
    final byId = <String, api.StatsLiveTask>{};
    final positions = <String, int>{};
    final perKind = <TaskNodeKind, int>{};
    for (final t in tasks) {
      if (byId.containsKey(t.id)) continue;
      final parent = t.parentId;
      if (parent != null && byId.containsKey(parent)) {
        (children[parent] ??= []).add(t.id);
      } else {
        roots.add(t.id);
      }
      byId[t.id] = t;
      final kind = _kind(t.kind);
      positions[t.id] = perKind[kind] = (perKind[kind] ?? 0) + 1;
    }
    TaskNodeModel build(String id) {
      final t = byId[id]!;
      final end = t.finishedAt ?? at;
      final runtime = end.difference(t.startedAt);
      final label = t.label?.trim();
      return TaskNodeModel(
        id: id,
        kind: _kind(t.kind),
        name: t.name,
        state: _state(t.state),
        runtime: runtime.isNegative ? Duration.zero : runtime,
        position: positions[id]!,
        label: label == null || label.isEmpty ? null : label,
        tool: t.tool,
        children: List.unmodifiable([
          for (final c in children[id] ?? const <String>[]) build(c),
        ]),
      );
    }

    return List.unmodifiable([for (final r in roots) build(r)]);
  }

  static TaskNodeKind _kind(api.StatsLiveTaskKindEnum kind) => switch (kind) {
    api.StatsLiveTaskKindEnum.subagent => TaskNodeKind.subagent,
    api.StatsLiveTaskKindEnum.workflow => TaskNodeKind.workflow,
    api.StatsLiveTaskKindEnum.backgroundJob => TaskNodeKind.backgroundJob,
    _ => TaskNodeKind.unknown,
  };

  static TaskNodeState _state(api.StatsLiveTaskStateEnum state) =>
      switch (state) {
        api.StatsLiveTaskStateEnum.running => TaskNodeState.running,
        api.StatsLiveTaskStateEnum.completed => TaskNodeState.completed,
        api.StatsLiveTaskStateEnum.failed => TaskNodeState.failed,
        api.StatsLiveTaskStateEnum.cancelled => TaskNodeState.cancelled,
        _ => TaskNodeState.unknown,
      };

  /// The step ID.
  final String id;

  /// The kind.
  final TaskNodeKind kind;

  /// The step name, the adapter's type (`local_agent`, `workflow_agent`, ...).
  final String name;

  /// The state.
  final TaskNodeState state;

  /// From `started_at` to `finished_at`, or to the frame's `at` while it runs.
  final Duration runtime;

  /// The 1-based position among the nodes of the same kind of the run, for the name of a
  /// node without a label ("Subagent 2").
  final int position;

  /// The label; `null` when the server sent none (no `run.read`).
  final String? label;

  /// The current tool of a running node.
  final String? tool;

  /// The child nodes in frame order.
  final List<TaskNodeModel> children;

  /// This node and every node below it.
  int get count => children.fold(1, (n, c) => n + c.count);

  @override
  bool operator ==(Object other) =>
      other is TaskNodeModel &&
      other.id == id &&
      other.kind == kind &&
      other.name == name &&
      other.state == state &&
      other.runtime == runtime &&
      other.position == position &&
      other.label == label &&
      other.tool == tool &&
      listEquals(other.children, children);

  @override
  int get hashCode => Object.hash(
    id,
    kind,
    name,
    state,
    runtime,
    position,
    label,
    tool,
    Object.hashAll(children),
  );
}
