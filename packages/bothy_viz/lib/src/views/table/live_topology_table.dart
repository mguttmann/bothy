// SPDX-License-Identifier: Apache-2.0

/// The table view of the live topology (docs/spec/25-visualisierung.md 6.1, 7.4).
library;

import 'package:api_client/api_client.dart' as api;

import '../../marks/viz_texts.dart';
import '../../model/run_card_model.dart';
import '../../model/series_model.dart';
import '../view_texts.dart';
import 'viz_table.dart';

/// The runs of [lanes] and the shown [rates] as tables: one row per run card with the texts
/// of the card, one row per rate. The column "Cost" exists only when a card shows a cost.
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
          for (final card in lane.cards)
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
