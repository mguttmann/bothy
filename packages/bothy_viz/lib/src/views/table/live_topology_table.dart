// SPDX-License-Identifier: Apache-2.0

/// The table view of the live topology (docs/spec/25-visualization.md 6.1, 7.4).
library;

import 'package:api_client/api_client.dart' as api;

import '../../marks/task_tree_mark.dart';
import '../../marks/viz_texts.dart';
import '../../model/run_card_model.dart';
import '../../model/series_model.dart';
import '../../model/task_tree_model.dart';
import '../view_texts.dart';
import 'viz_table.dart';

/// The runs of [lanes] and the shown [rates] as tables: one row per run card with the texts
/// of the card, followed by one row per node of its agent tree in tree order, its agent cell
/// indented by level (25 6.1); one row per rate. The column "Cost" exists only when a card
/// shows a cost.
List<VizTableData> liveTopologyTables(
  VizTexts texts, {
  required List<LaneModel> lanes,
  required List<RateModel> rates,
}) {
  final l10n = texts.l10n;
  final withCost = lanes.any((l) => l.cards.any((c) => c.cost != null));
  return [
    VizTableData(
      caption: l10n.vizViewLiveTopology,
      columns: [
        l10n.vizColSpace,
        l10n.vizColChannel,
        l10n.vizColAgent,
        l10n.vizColState,
        l10n.vizColStep,
        l10n.vizColMode,
        l10n.vizColApproval,
        if (withCost) l10n.vizColCost,
      ],
      rows: [
        for (final lane in lanes)
          for (final card in lane.cards) ...[
            [
              VizCell(lane.spaceName),
              VizCell(lane.channelName),
              VizCell(card.agentName),
              VizCell(texts.runState(card.state)),
              VizCell(
                card.stepKind != null && card.stepName != null
                    ? texts.step(card.stepKind!, card.stepName!)
                    : l10n.vizNone,
              ),
              VizCell(texts.badge(card.billingMode)),
              VizCell(
                card.approvalPending ? l10n.vizApprovalWaiting : l10n.vizNone,
              ),
              if (withCost)
                VizCell(
                  card.cost == null ? l10n.vizNone : texts.cost(card.cost!),
                  sortBy: card.cost?.micros ?? -1,
                ),
            ],
            for (final (node, level) in _tree(card.tasks, 1))
              [
                VizCell(lane.spaceName),
                VizCell(lane.channelName),
                VizCell(texts.taskName(node), level: level),
                VizCell(texts.taskState(node.state)),
                VizCell(
                  node.tool != null && !node.state.isFinished
                      ? l10n.vizStepToolCall(node.tool!)
                      : l10n.vizNone,
                ),
                VizCell(l10n.vizNone),
                VizCell(l10n.vizNone),
                if (withCost) VizCell(l10n.vizNone, sortBy: -1),
              ],
            if (card.tasksTruncated)
              [
                VizCell(lane.spaceName),
                VizCell(lane.channelName),
                VizCell(l10n.vizMoreTasksThanShown, level: 1),
                VizCell(l10n.vizNone),
                VizCell(l10n.vizNone),
                VizCell(l10n.vizNone),
                VizCell(l10n.vizNone),
                if (withCost) VizCell(l10n.vizNone, sortBy: -1),
              ],
          ],
      ],
    ),
    VizTableData(
      caption: l10n.vizRatesLastMinute,
      columns: [l10n.vizColMetric, l10n.vizColLastMinute],
      rows: [
        for (final rate in rates)
          [
            VizCell(vizRateLabel(texts, rate)),
            VizCell(
              vizRateValue(texts, rate),
              sortBy: rate.metric == api.StatsMetric.cost
                  ? rate.lastMinuteSum ?? rate.lastMinute
                  : rate.lastMinute,
            ),
          ],
      ],
    ),
  ];
}

/// The nodes of [nodes] and their descendants in tree order with their level.
Iterable<(TaskNodeModel, int)> _tree(
  List<TaskNodeModel> nodes,
  int level,
) sync* {
  for (final n in nodes) {
    yield (n, level);
    yield* _tree(n.children, level + 1);
  }
}
