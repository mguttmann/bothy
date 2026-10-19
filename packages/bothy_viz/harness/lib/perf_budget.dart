// SPDX-License-Identifier: Apache-2.0

/// The budget of the performance test of the live topology (docs/spec/25-visualization.md 7.3;
/// docs/DECISIONS.md D-M6-12-10): `reference` is `viz.perf.frame_build_p95` on the reference
/// hardware (`make perf`), `ci` is `viz.perf.frame_build_p95_ci` on a shared hosted CI runner.
/// `tool/perf_macos.sh` passes the choice as `--dart-define=BOTHY_PERF_BUDGET=...`.
library;

import 'package:domain/domain.dart';

/// The named config value and its limit for [budget] (`reference` or `ci`); any other value
/// throws an [ArgumentError], so a run never measures against a guessed budget.
({String key, Duration limit}) frameBuildBudget(
  AppConfig config,
  String budget,
) => switch (budget) {
  'reference' => (
    key: 'viz.perf.frame_build_p95',
    limit: config.perfFrameBuildP95,
  ),
  'ci' => (
    key: 'viz.perf.frame_build_p95_ci',
    limit: config.perfFrameBuildP95Ci,
  ),
  _ => throw ArgumentError.value(
    budget,
    'BOTHY_PERF_BUDGET',
    'is neither reference nor ci',
  ),
};
