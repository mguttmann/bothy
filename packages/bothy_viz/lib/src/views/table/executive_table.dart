// SPDX-License-Identifier: Apache-2.0

/// The table view of the executive page (docs/spec/25-visualization.md 6.4, 7.4).
library;

import '../../marks/metric_tile_mark.dart';
import '../../marks/viz_texts.dart';
import '../../model/metric_tile_model.dart';
import '../../model/traffic_light_model.dart';
import 'viz_table.dart';

/// The tiles of [tiles] as rows of label, value and note (comparison or cost label), the
/// texts of the metric tile.
VizTableData tileTable(
  VizTexts texts,
  String caption,
  List<MetricTileModel> tiles,
) => VizTableData(
  caption: caption,
  columns: [
    texts.l10n.vizColMetric,
    texts.l10n.vizColValue,
    texts.l10n.vizColNote,
  ],
  rows: [
    for (final tile in tiles)
      [
        VizCell(MetricTileMark.label(texts, tile)),
        VizCell(MetricTileMark.value(texts, tile), sortBy: tile.value ?? -1),
        VizCell(
          [
            ?MetricTileMark.comparison(texts, tile),
            ?MetricTileMark.costLabelText(texts, tile),
          ].join(', '),
        ),
      ],
  ],
);

/// The traffic lights as rows of SLO, state and the label "own measurement".
VizTableData trafficLightTable(
  VizTexts texts,
  List<TrafficLightModel> lights,
) => VizTableData(
  caption: texts.l10n.vizTileSlo,
  columns: [
    texts.l10n.vizColSlo,
    texts.l10n.vizColState,
    texts.l10n.vizColNote,
  ],
  rows: [
    for (final light in lights)
      [
        VizCell(texts.slo(light)),
        VizCell(texts.sloState(light.status)),
        VizCell(texts.l10n.vizOwnMeasurement),
      ],
  ],
);
