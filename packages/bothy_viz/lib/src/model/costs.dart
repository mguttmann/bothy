// SPDX-License-Identifier: Apache-2.0

/// A cost figure with its label (docs/spec/25-visualisierung.md 4 rule 2, 6.1).
library;

import 'package:api_client/api_client.dart' as api;
import 'package:domain/domain.dart';

/// One cost figure the server delivered, in micro units of [currency].
///
/// A view model only holds a [VizCost] when every member of it was present in the response;
/// a missing member means "not shown" (no permission), never zero (25 4 rule 5).
final class VizCost {
  /// Creates a cost figure.
  const VizCost({
    required this.micros,
    required this.currency,
    required this.mode,
    required this.label,
  });

  /// Builds the figure of [mode]: `organization` is an estimate while [isEstimate], `personal`
  /// is always "information only" ([costLabel] of `domain`).
  factory VizCost.of({
    required int micros,
    required String currency,
    required api.BillingMode mode,
    required bool isEstimate,
  }) => VizCost(
    micros: micros,
    currency: currency,
    mode: mode,
    label: costLabel(mode: domainMode(mode), isEstimate: isEstimate),
  );

  /// Amount in millionths of the currency unit.
  final int micros;

  /// ISO 4217 code.
  final String currency;

  /// Billing mode the cost belongs to.
  final api.BillingMode mode;

  /// "Estimate", final amount or "information only".
  final CostLabel label;

  /// The amount in currency units.
  double get amount => micros / 1000000;

  @override
  bool operator ==(Object other) =>
      other is VizCost &&
      other.micros == micros &&
      other.currency == currency &&
      other.mode == mode &&
      other.label == label;

  @override
  int get hashCode => Object.hash(micros, currency, mode, label);
}

/// The billing mode of `domain` for a generated [api.BillingMode]; an unknown value maps to
/// [BillingMode.unknown], which [costLabel] treats as information only.
BillingMode domainMode(api.BillingMode mode) => switch (mode) {
  api.BillingMode.organization => BillingMode.organization,
  api.BillingMode.personal => BillingMode.personal,
  _ => BillingMode.unknown,
};
