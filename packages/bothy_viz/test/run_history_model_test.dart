// SPDX-License-Identifier: Apache-2.0

import 'package:api_client/api_client.dart' as api;
import 'package:bothy_viz/bothy_viz.dart';
import 'package:domain/domain.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support/views.dart';

void main() {
  late api.RunHistoryPage page;
  const names = VizNames(
    agents: {'0192f4a0-7b1c-7000-8a00-000000000202': 'dev-runner'},
  );

  setUpAll(() async {
    page = await memoryFixtures.runHistoryPage('normal');
  });

  test('a row per entry in the order of the server with every figure', () {
    final model = RunHistoryModel.fromEntries(
      page.items.toList(),
      names: names,
    );
    expect(model.rows.map((r) => r.runId), [for (final e in page.items) e.id]);
    final first = model.rows.first;
    expect(first.label.project, 'backend');
    expect(first.label.chat, 'Payment integration');
    expect(first.label.agent, 'dev-runner');
    expect(first.state, api.RunState.completed);
    expect(first.model, 'model-a');
    expect(first.duration, const Duration(milliseconds: 2352000));
    expect(first.startedAt, DateTime.utc(2026, 10, 24, 9));
    expect(first.finishedAt, DateTime.utc(2026, 10, 24, 9, 39, 12));
    expect(first.tokens.input, 120400);
    expect(first.tokens.output, 8200);
    expect(first.tokens.cacheRead, 940000);
    expect(first.tokens.cacheWrite, 30500);
    expect(first.subagents, 3);
    expect(first.cost.label, CostLabel.estimate);
    expect(first.cost.micros, 880000);
  });

  test(
    'personal costs are information only and missing figures stay absent',
    () {
      final model = RunHistoryModel.fromEntries(
        page.items.toList(),
        names: names,
      );
      final failed = model.rows[1];
      expect(failed.cost.label, CostLabel.informationOnly);
      expect(failed.tokens.cacheRead, isNull);
      expect(failed.tokens.cacheWrite, isNull);
      expect(failed.subagents, isNull);
      final cancelled = model.rows[2];
      expect(cancelled.label.named, isFalse);
      expect(cancelled.startedAt, isNull);
      expect(cancelled.duration, isNull);
      expect(cancelled.model, isNull);
    },
  );

  test('cache writes of five minutes and one hour count together', () {
    final entry = page.items.first.rebuild(
      (b) => b.metrics
        ..tokensCacheWrite = 100
        ..tokensCacheWrite1h = 20,
    );
    final model = RunHistoryModel.fromEntries([entry], names: names);
    expect(model.rows.single.tokens.cacheWrite, 120);
  });

  test('without duration_ms the duration is the time from start to end', () {
    final entry = page.items.first.rebuild((b) => b..durationMs = null);
    final model = RunHistoryModel.fromEntries([entry], names: names);
    expect(
      model.rows.single.duration,
      const Duration(minutes: 39, seconds: 12),
    );
  });
}
