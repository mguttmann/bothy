// SPDX-License-Identifier: Apache-2.0

import 'package:api_client/api_client.dart' as api;
import 'package:bothy_viz/bothy_viz.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support/fixtures.dart';

void main() {
  group('LiveTopologyModel', () {
    test('groups the runs of the normal frame into lanes by space and channel, in frame order', () async {
      final model = LiveTopologyModel.fromFrame(
        await fixtures.liveFrame('normal'),
        names: names,
      );
      expect(
        [
          for (final l in model.lanes)
            '${l.spaceName} ${l.channelName} ${l.cards.length}',
        ],
        [
          'Engineering #backend 1',
          'Engineering #release 1',
          'Support #tickets 2',
        ],
      );
      expect(model.runsTruncated, isFalse);
      final first = model.lanes.first.cards.single;
      expect(first.agentName, 'dev-runner');
      expect(first.state, RunCardState.running);
      expect(first.stepKind, RunStepKind.toolCall);
      expect(first.stepName, 'Bash');
      expect(first.cost?.micros, 620000);
      expect(first.cost?.label, CostLabel.estimate);
      expect(model.lanes[1].cards.single.approvalPending, isTrue);
    });

    test('runs_truncated becomes the "more runs than shown" marker', () async {
      final model = LiveTopologyModel.fromFrame(
        await fixtures.liveFrame('truncated'),
      );
      expect(model.runsTruncated, isTrue);
    });

    test(
      'a frame without cost members gives cards without cost, never zero',
      () async {
        final model = LiveTopologyModel.fromFrame(
          await fixtures.liveFrame('no_cost_permission'),
        );
        expect(model.cards, isNotEmpty);
        expect(model.cards.map((c) => c.cost), everyElement(isNull));
        final cost = model.rates.where((r) => r.metric == api.StatsMetric.cost);
        expect(cost, isEmpty, reason: 'no cost rate without the permission');
        expect(
          model.rates.where((r) => r.lastMinuteSum == 0),
          isEmpty,
          reason: 'a missing sum stays missing',
        );
      },
    );

    test('a run in mode personal shows its cost as "information only" (FIX-cost-display)', () async {
      final model = LiveTopologyModel.fromFrame(
        await fixtures.liveFrame('normal'),
      );
      final personal = model.cards.where(
        (c) => c.billingMode == api.BillingMode.personal,
      );
      expect(personal, isNotEmpty);
      for (final card in personal) {
        expect(card.cost, isNotNull);
        expect(card.cost!.label, CostLabel.informationOnly);
        expect(card.cost!.amount, greaterThan(0));
      }
    });
  });

  group('ExecutiveModel', () {
    test('a cost tile of mode personal carries "information only"', () async {
      final model = ExecutiveModel.from(
        costs: await fixtures.costSeries('normal'),
      );
      final byMode = {for (final t in model.costTiles!) t.costMode: t};
      expect(
        byMode[api.BillingMode.personal]!.costLabel,
        CostLabel.informationOnly,
      );
      expect(
        byMode[api.BillingMode.organization]!.costLabel,
        isNot(CostLabel.informationOnly),
      );
      expect(byMode[api.BillingMode.personal]!.value, greaterThan(0));
    });

    test(
      'without a cost response the cost tiles are not shown, not zero',
      () async {
        final model = ExecutiveModel.from(
          usage: await fixtures.series('normal', query: 'usage'),
        );
        expect(model.costTiles, isNull);
        expect(
          model.tiles.map((t) => t.queryMetric),
          containsAll(['messages', 'runs']),
        );
      },
    );

    test(
      'traffic light states of the normal report map to role, icon and text',
      () async {
        final model = ExecutiveModel.from(
          slo: await fixtures.sloReport('normal'),
        );
        final byState = {for (final l in model.trafficLights) l.status: l};
        expect(byState.keys, unorderedEquals(TrafficLightStatus.values));
        expect(
          byState[TrafficLightStatus.onTarget]!.role,
          VizStateRole.success,
        );
        expect(
          byState[TrafficLightStatus.budgetTight]!.role,
          VizStateRole.warning,
        );
        expect(
          byState[TrafficLightStatus.targetMissed]!.role,
          VizStateRole.danger,
        );
        expect(
          byState[TrafficLightStatus.notEnoughData]!.role,
          VizStateRole.neutral,
        );
        expect(
          byState[TrafficLightStatus.onTarget]!.glyph,
          DsIcons.triggerDone,
        );
        expect(
          byState[TrafficLightStatus.targetMissed]!.glyph,
          DsIcons.stateError,
        );
        expect(
          model.trafficLights.first.slo,
          'bothy-api/requests-availability',
        );
      },
    );
  });

  group('BackendModel', () {
    test('costs per mode are null without the permission', () async {
      final model = BackendModel.fromReport(
        await fixtures.backendReport('no_cost_permission'),
      );
      expect(model.rows, isNotEmpty);
      expect(model.rows.map((r) => r.costs), everyElement(isNull));
    });

    test('the live frame supplies presence and the reported mode', () async {
      final model = BackendModel.fromReport(
        await fixtures.backendReport('normal'),
        live: await fixtures.liveFrame('normal'),
        names: names,
      );
      final dev = model.rows.firstWhere((r) => r.agentName == 'dev-runner');
      expect(dev.presence, api.AgentRuntimeState.degraded);
      expect(dev.costs, isNotEmpty);
    });
  });

  group('DeliveryModel', () {
    test(
      'carries the SLIs, gateway rates and connections of an instance frame',
      () async {
        final model = DeliveryModel.fromFrame(
          await fixtures.liveFrame('normal'),
        );
        expect(model.slis, hasLength(6));
        expect(model.slis.firstWhere((s) => s.total == 0).ratio, isNull);
        expect(model.gatewayRates.map((r) => r.metric), [
          api.StatsMetric.catchups,
          api.StatsMetric.resyncs,
          api.StatsMetric.slowConsumerCloses,
        ]);
        expect(model.connections?.user, 12);
      },
    );

    test(
      'a space frame has no SLIs and no connections instead of zeros',
      () async {
        final model = DeliveryModel.fromFrame(
          await fixtures.liveFrame('offline'),
        );
        expect(model.slis, isEmpty);
        expect(model.connections, isNull);
      },
    );
  });

  group('TimelineModel', () {
    test(
      'runs become bars per agent and channel, buckets series per metric',
      () async {
        final timeline = await fixtures.timeline('normal');
        final model = TimelineModel.fromTimeline(timeline, names: names);
        expect(model.bars.rows, isNotEmpty);
        expect(
          model.bars.rows.expand((r) => r.segments),
          hasLength(timeline.runs.length),
        );
        expect(model.bars.rows.first.label, contains(', '));
        expect(model.series.map((s) => s.colorIndex), [
          for (var i = 0; i < model.series.length; i++) i,
        ]);
        final running = timeline.runs.where((r) => r.finishedAt == null);
        for (final run in running) {
          final bar = model.bars.rows
              .expand((r) => r.segments)
              .firstWhere((s) => s.start == (run.startedAt ?? run.queuedAt));
          expect(bar.end, timeline.to);
        }
      },
    );

    test('runs_truncated is kept', () async {
      expect(
        TimelineModel.fromTimeline(await fixtures.timeline('truncated'))
            .runsTruncated,
        isTrue,
      );
    });
  });

  test(
    'series of a grouped query keep metric and group with consecutive colors',
    () async {
      final series = SeriesModel.fromStatsSeries(
        await fixtures.series('normal', query: 'usage'),
      );
      expect(series, isNotEmpty);
      expect(
        series.map((s) => (s.queryMetric, s.groupKey)).toSet(),
        hasLength(series.length),
      );
      expect(series.last.colorIndex, series.length - 1);
    },
  );
}
