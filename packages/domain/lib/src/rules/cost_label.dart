// SPDX-License-Identifier: Apache-2.0

/// Label of a cost figure (01 6.7 "Costs").
library;

import '../entities/principal.dart';

/// How the client labels a cost figure.
enum CostLabel {
  /// Organization mode before reconciliation: "Estimate".
  estimate,

  /// Organization mode after reconciliation: the final amount, no label.
  finalAmount,

  /// Personal mode: "Information only".
  informationOnly,
}

/// The label of a cost in billing [mode]; [isEstimate] is `cost_is_estimate` of the run
/// (03 `runs`).
///
/// A mode this version does not know is labelled as information only, because the client
/// cannot claim that the organization pays.
CostLabel costLabel({required BillingMode mode, required bool isEstimate}) =>
    switch (mode) {
      BillingMode.organization =>
        isEstimate ? CostLabel.estimate : CostLabel.finalAmount,
      BillingMode.personal || BillingMode.unknown => CostLabel.informationOnly,
    };
