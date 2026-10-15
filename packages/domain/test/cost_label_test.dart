// SPDX-License-Identifier: Apache-2.0

import 'package:domain/domain.dart';
import 'package:test/test.dart';

void main() {
  test('organization: estimate until reconciled, then final', () {
    expect(
      costLabel(mode: BillingMode.organization, isEstimate: true),
      CostLabel.estimate,
    );
    expect(
      costLabel(mode: BillingMode.organization, isEstimate: false),
      CostLabel.finalAmount,
    );
  });

  test('personal and unknown modes: information only', () {
    expect(
      costLabel(mode: BillingMode.personal, isEstimate: true),
      CostLabel.informationOnly,
    );
    expect(
      costLabel(mode: BillingMode.personal, isEstimate: false),
      CostLabel.informationOnly,
    );
    expect(
      costLabel(mode: BillingMode.unknown, isEstimate: false),
      CostLabel.informationOnly,
    );
  });
}
