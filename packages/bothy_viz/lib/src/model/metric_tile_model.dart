// SPDX-License-Identifier: Apache-2.0

/// The metric tile of the executive page (components.md 11).
library;

import 'package:api_client/api_client.dart' as api;
import 'package:domain/domain.dart';

import 'costs.dart';

/// How the tile formats its value.
enum TileUnit {
  /// A count.
  count,

  /// A share in percent (value 0 to 100).
  percent,

  /// Money in micro units of the tile's currency.
  money,

  /// A duration in milliseconds, for example a percentile of the run duration.
  milliseconds,
}

/// One large number with its label, an optional comparison and a cost label.
///
/// [value] `null` means the server did not deliver it (no permission): the tile says "not
/// shown", never zero (25 4 rule 5).
final class MetricTileModel {
  /// Creates a tile; exactly one of [metric], [queryMetric] and [costMode] names it.
  const MetricTileModel({
    required this.value,
    required this.unit,
    this.metric,
    this.queryMetric,
    this.costMode,
    this.previous,
    this.currency,
    this.costLabel,
  });

  /// A cost tile of [cost]'s mode: `personal` carries "information only", `organization`
  /// "Estimate" until reconciled.
  factory MetricTileModel.cost(VizCost cost) => MetricTileModel(
    value: cost.micros,
    unit: TileUnit.money,
    costMode: cost.mode,
    currency: cost.currency,
    costLabel: cost.label,
  );

  /// Metric of the own measurement.
  final api.StatsMetric? metric;

  /// Metric name of a query response.
  final String? queryMetric;

  /// Billing mode of a cost tile.
  final api.BillingMode? costMode;

  /// Value; `null` when not shown.
  final num? value;

  /// Value of the previous period for the comparison text; `null` without comparison.
  final num? previous;

  /// How [value] is formatted.
  final TileUnit unit;

  /// Currency of a money tile.
  final String? currency;

  /// Label of a cost tile.
  final CostLabel? costLabel;
}
