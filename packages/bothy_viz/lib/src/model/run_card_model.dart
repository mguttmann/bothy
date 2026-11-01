// SPDX-License-Identifier: Apache-2.0

/// The run card and the lane of the live topology (docs/spec/25-visualization.md 6.1).
library;

import 'package:api_client/api_client.dart' as api;
import 'package:flutter/foundation.dart';

import 'costs.dart';
import 'names.dart';
import 'task_tree_model.dart';

/// State of an active run as the card shows it, with the run state roles of the live box
/// (components.md 2).
enum RunCardState {
  /// `queued`.
  queued,

  /// `running`.
  running,

  /// `waiting_input`.
  waitingInput,

  /// A state of a newer server; the card shows "Unknown state".
  unknown,
}

/// Kind of the current step, shown as the tool chip (components.md 3).
enum RunStepKind {
  /// `tool_call`: the name is the tool name.
  toolCall,

  /// `status`.
  status,

  /// `subagent`.
  subagent,

  /// `workflow`.
  workflow,

  /// `background_job`.
  backgroundJob,

  /// A kind of a newer server; the chip shows the name only.
  unknown,
}

/// One active run of a live frame as a card: agent, state, tool chip, badge, approval note
/// and cost; never content (25 1 rule 3).
final class RunCardModel {
  /// Creates a card model; [RunCardModel.fromRun] is the normal way.
  const RunCardModel({
    required this.runId,
    required this.threadId,
    required this.agentId,
    required this.agentName,
    required this.state,
    required this.billingMode,
    required this.approvalPending,
    this.stepKind,
    this.stepName,
    this.cost,
    this.tasks = const [],
    this.tasksTruncated = false,
  });

  /// Derives the card from [run]; [names] gives the agent name, [at] (the frame time) the
  /// runtime of the running nodes of its agent tree.
  ///
  /// The cost appears only when `cost_micros` and `currency` are both present; in mode
  /// `organization` `cost_is_estimate` gives the "Estimate" label, in mode `personal` it carries
  /// "information only" (25 6.1, FIX-cost-display); a missing member stays missing, never zero
  /// (25 4 rule 5).
  factory RunCardModel.fromRun(
    api.StatsLiveRun run,
    VizNames names, {
    DateTime? at,
  }) {
    final micros = run.costMicros;
    final currency = run.currency;
    final showCost = micros != null && currency != null;
    return RunCardModel(
      runId: run.runId,
      threadId: run.threadId,
      agentId: run.agentId,
      agentName: names.agent(run.agentId),
      state: switch (run.state) {
        api.StatsLiveRunStateEnum.queued => RunCardState.queued,
        api.StatsLiveRunStateEnum.running => RunCardState.running,
        api.StatsLiveRunStateEnum.waitingInput => RunCardState.waitingInput,
        _ => RunCardState.unknown,
      },
      billingMode: run.billingMode,
      approvalPending: run.approvalPending,
      stepKind: run.step == null ? null : _kind(run.step!.kind),
      stepName: run.step?.name,
      cost: showCost
          ? VizCost.of(
              micros: micros,
              currency: currency,
              mode: run.billingMode,
              isEstimate: run.costIsEstimate ?? false,
            )
          : null,
      tasks: TaskNodeModel.tree(
        run.tasks ?? const <api.StatsLiveTask>[],
        at ?? run.startedAt ?? run.queuedAt,
      ),
      tasksTruncated: run.tasksTruncated ?? false,
    );
  }

  static RunStepKind _kind(api.StatsLiveStepKindEnum kind) => switch (kind) {
    api.StatsLiveStepKindEnum.toolCall => RunStepKind.toolCall,
    api.StatsLiveStepKindEnum.status => RunStepKind.status,
    api.StatsLiveStepKindEnum.subagent => RunStepKind.subagent,
    api.StatsLiveStepKindEnum.workflow => RunStepKind.workflow,
    api.StatsLiveStepKindEnum.backgroundJob => RunStepKind.backgroundJob,
    _ => RunStepKind.unknown,
  };

  /// Run ID.
  final String runId;

  /// Thread of the run; a tap on the card reports it (the apps open the deep link).
  final String threadId;

  /// Agent ID.
  final String agentId;

  /// Display name of the agent.
  final String agentName;

  /// State of the run.
  final RunCardState state;

  /// Mode of the badge: the reported mode of the run, else the configured mode (5.2).
  final api.BillingMode billingMode;

  /// An approval card of the run waits for a decision ("Approval waiting").
  final bool approvalPending;

  /// Kind of the current step; `null` without a step.
  final RunStepKind? stepKind;

  /// Name of the current step (the tool name for `tool_call`); `null` without a step.
  final String? stepName;

  /// Cost so far; `null` when not shown.
  final VizCost? cost;

  /// The agent tree below the card: the nodes directly under the run, each with its children
  /// (25 6.1).
  final List<TaskNodeModel> tasks;

  /// The server left nodes of the tree out ("More agents and jobs than shown").
  final bool tasksTruncated;

  /// This card without its cost: TV mode without `viz.tv_show_costs` (25 6.6).
  RunCardModel withoutCost() => cost == null
      ? this
      : RunCardModel(
          runId: runId,
          threadId: threadId,
          agentId: agentId,
          agentName: agentName,
          state: state,
          billingMode: billingMode,
          approvalPending: approvalPending,
          stepKind: stepKind,
          stepName: stepName,
          tasks: tasks,
          tasksTruncated: tasksTruncated,
        );

  // Value equality lets the live topology reuse the widget of an unchanged card between two
  // frames instead of rebuilding it (25 7.3, performance).
  @override
  bool operator ==(Object other) =>
      other is RunCardModel &&
      other.runId == runId &&
      other.threadId == threadId &&
      other.agentId == agentId &&
      other.agentName == agentName &&
      other.state == state &&
      other.billingMode == billingMode &&
      other.approvalPending == approvalPending &&
      other.stepKind == stepKind &&
      other.stepName == stepName &&
      other.cost == cost &&
      other.tasksTruncated == tasksTruncated &&
      listEquals(other.tasks, tasks);

  @override
  int get hashCode => Object.hash(
    runId,
    threadId,
    agentId,
    agentName,
    state,
    billingMode,
    approvalPending,
    stepKind,
    stepName,
    cost,
    tasksTruncated,
    Object.hashAll(tasks),
  );
}

/// One lane of the live topology: the active runs of one channel of one space.
final class LaneModel {
  /// Creates a lane.
  const LaneModel({
    required this.spaceId,
    required this.spaceName,
    required this.channelId,
    required this.channelName,
    required this.cards,
  });

  /// Space ID.
  final String spaceId;

  /// Display name of the space.
  final String spaceName;

  /// Channel ID.
  final String channelId;

  /// Display name of the channel.
  final String channelName;

  /// The run cards in the order of the frame.
  final List<RunCardModel> cards;

  /// This lane with every card [RunCardModel.withoutCost].
  LaneModel withoutCosts() => LaneModel(
    spaceId: spaceId,
    spaceName: spaceName,
    channelId: channelId,
    channelName: channelName,
    cards: List.unmodifiable(cards.map((c) => c.withoutCost())),
  );

  @override
  bool operator ==(Object other) {
    if (other is! LaneModel ||
        other.spaceId != spaceId ||
        other.spaceName != spaceName ||
        other.channelId != channelId ||
        other.channelName != channelName ||
        other.cards.length != cards.length) {
      return false;
    }
    for (var i = 0; i < cards.length; i++) {
      if (other.cards[i] != cards[i]) return false;
    }
    return true;
  }

  @override
  int get hashCode => Object.hash(
    spaceId,
    channelId,
    spaceName,
    channelName,
    Object.hashAll(cards),
  );

  /// Groups [runs] into lanes by space and channel, in the order the frame gives: a lane
  /// appears where its first run appears, and its cards keep the frame order.
  static List<LaneModel> group(
    Iterable<api.StatsLiveRun> runs,
    VizNames names, {
    DateTime? at,
  }) {
    final lanes = <(String, String), List<RunCardModel>>{};
    final keys = <(String, String)>[];
    for (final run in runs) {
      final key = (run.spaceId, run.channelId);
      final cards = lanes.putIfAbsent(key, () {
        keys.add(key);
        return <RunCardModel>[];
      });
      cards.add(RunCardModel.fromRun(run, names, at: at));
    }
    return [
      for (final (space, channel) in keys)
        LaneModel(
          spaceId: space,
          spaceName: names.space(space),
          channelId: channel,
          channelName: names.channel(channel),
          cards: List.unmodifiable(lanes[(space, channel)]!),
        ),
    ];
  }
}
