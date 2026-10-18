// SPDX-License-Identifier: Apache-2.0

/// The table view of the backend view (docs/spec/25-visualization.md 6.2, 7.4).
library;

import '../../marks/viz_texts.dart';
import '../../model/backend_model.dart';
import '../backend_texts.dart';
import 'viz_table.dart';

/// One row per agent with the texts of its panel; the column "Costs per mode" exists only
/// when a row shows costs ([costsVisible] is `false` in TV mode without
/// `viz.tv_show_costs`).
List<VizTableData> backendTables(
  VizTexts texts,
  BackendModel model, {
  required bool costsVisible,
  required String timeZone,
}) {
  final l10n = texts.l10n;
  final withCost =
      costsVisible && model.rows.any((r) => r.costs?.isNotEmpty ?? false);
  return [
    VizTableData(
      caption: l10n.vizViewBackend,
      columns: [
        l10n.vizColAgent,
        l10n.vizColPresence,
        l10n.vizColConfiguredMode,
        l10n.vizColReportedMode,
        l10n.vizColSupportedModes,
        l10n.vizColSwitches,
        l10n.vizColI2Denied,
        l10n.vizColI3Denied,
        if (withCost) l10n.vizColCosts,
        l10n.vizColRateLimit,
      ],
      rows: [
        for (final row in model.rows)
          [
            VizCell(row.agentName),
            VizCell(backendPresence(texts, row)),
            VizCell(texts.badge(row.billingMode)),
            VizCell(backendReported(texts, row)),
            VizCell(backendSupported(texts, row)),
            VizCell(texts.format.count(row.switches), sortBy: row.switches),
            VizCell(texts.format.count(row.i2Denied), sortBy: row.i2Denied),
            VizCell(
              texts.format.count(row.billingModeMismatch),
              sortBy: row.billingModeMismatch,
            ),
            if (withCost) VizCell(backendCosts(texts, row) ?? l10n.vizNone),
            VizCell(backendRateLimit(texts, row, timeZone)),
          ],
      ],
    ),
  ];
}
