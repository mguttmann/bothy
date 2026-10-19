// SPDX-License-Identifier: Apache-2.0

import 'package:bothy_viz_harness/perf_budget.dart';
import 'package:domain/domain.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  final config = AppConfig.defaults();

  test('the reference budget is viz.perf.frame_build_p95', () {
    expect(frameBuildBudget(config, 'reference'), (
      key: 'viz.perf.frame_build_p95',
      limit: config.perfFrameBuildP95,
    ));
  });

  test('the ci budget is viz.perf.frame_build_p95_ci', () {
    expect(frameBuildBudget(config, 'ci'), (
      key: 'viz.perf.frame_build_p95_ci',
      limit: config.perfFrameBuildP95Ci,
    ));
  });

  test('the ci budget is above the reference budget', () {
    expect(config.perfFrameBuildP95Ci, greaterThan(config.perfFrameBuildP95));
  });

  test(
    'an empty or unknown budget fails instead of measuring against a guess',
    () {
      expect(() => frameBuildBudget(config, ''), throwsArgumentError);
      expect(() => frameBuildBudget(config, 'fast'), throwsArgumentError);
    },
  );
}
