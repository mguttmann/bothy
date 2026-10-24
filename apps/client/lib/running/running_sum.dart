// SPDX-License-Identifier: Apache-2.0

/// The footer of "Running now": the sum of the organization's active runs as an estimate and
/// the note that personal runs are information only (docs/spec/01-product.md 5.1 D10, 6.7).
library;

import 'package:domain/domain.dart';

/// The cost lines of the shown runs.
///
/// [organizationMicros] holds the summed costs of the runs that the organization pays, per ISO
/// 4217 currency in millionths (a sum across currencies would be wrong); [hasOrganization] and
/// [hasPersonal] say which of the two footer lines apply.
final class RunningSum {
  /// Creates the sum.
  const RunningSum({
    required this.organizationMicros,
    required this.hasOrganization,
    required this.hasPersonal,
  });

  /// The organization costs per currency, in millionths of the currency unit, sorted by code.
  final Map<String, int> organizationMicros;

  /// Whether at least one shown run runs in Organization mode.
  final bool hasOrganization;

  /// Whether at least one shown run runs in Personal mode (or a mode this version does not
  /// know, which the client never claims the organization pays, 01 6.7).
  final bool hasPersonal;
}

/// The billing mode a cost of [run] is shown under: the mode the adapter reported, else the
/// mode the server expected at the last delivery (09 5).
BillingMode costModeOf(RunSummary run) =>
    run.billingModeUsed ?? run.billingModeExpected;

/// Sums the costs of [runs] for the footer of D10.
///
/// Only runs in Organization mode count into the sum; the sum is always labeled "Estimate",
/// because an active run is never reconciled (01 6.7). Personal runs and runs of an unknown mode
/// only switch on the note "Personal runs: information only" (`costLabel` of `domain`).
///
/// Example: two organization runs of 0.30 and 0.50 USD give `{'USD': 800000}`.
RunningSum sumRunningCosts(Iterable<RunSummary> runs) {
  final sums = <String, int>{};
  var organization = false;
  var personal = false;
  for (final run in runs) {
    final label = costLabel(mode: costModeOf(run), isEstimate: true);
    if (label == CostLabel.informationOnly) {
      personal = true;
      continue;
    }
    organization = true;
    sums.update(
      run.cost.currency,
      (v) => v + run.cost.amountMicros,
      ifAbsent: () => run.cost.amountMicros,
    );
  }
  final keys = sums.keys.toList()..sort();
  return RunningSum(
    organizationMicros: {for (final k in keys) k: sums[k]!},
    hasOrganization: organization,
    hasPersonal: personal,
  );
}
