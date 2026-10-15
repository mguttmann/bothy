// SPDX-License-Identifier: Apache-2.0

/// Interface texts of the marks, all from `l10n` (docs/spec/25-visualisierung.md 7.4).
library;

import 'package:api_client/api_client.dart' as api;
import 'package:built_value/serializer.dart';
import 'package:domain/domain.dart';
import 'package:flutter/widgets.dart';
import 'package:l10n/l10n.dart';

import '../model/costs.dart';
import '../model/run_card_model.dart';
import '../model/traffic_light_model.dart';
import '../slo/slo_catalog.dart';

/// The texts of the marks for the locale of a [BuildContext].
///
/// ```dart
/// final texts = VizTexts.of(context);
/// Text(texts.metric(api.StatsMetric.toolCalls)); // "Tool calls"
/// ```
final class VizTexts {
  /// Creates the texts for [l10n] and [locale].
  VizTexts(this.l10n, Locale locale) : format = L10nFormat(locale);

  /// The texts of the ambient `Localizations` (the app registers `AppLocalizations.delegate`).
  factory VizTexts.of(BuildContext context) =>
      VizTexts(AppLocalizations.of(context), Localizations.localeOf(context));

  /// The generated texts.
  final AppLocalizations l10n;

  /// Number and money formatting of the locale.
  final L10nFormat format;

  /// The label of [metric] (25 3.2); an unknown metric shows its wire code.
  String metric(api.StatsMetric metric) {
    final known = <api.StatsMetric, String>{
      api.StatsMetric.messagesCreated: l10n.vizMetricMessagesCreated,
      api.StatsMetric.threadsCreated: l10n.vizMetricThreadsCreated,
      api.StatsMetric.runsStarted: l10n.vizMetricRunsStarted,
      api.StatsMetric.runsFinished: l10n.vizMetricRunsFinished,
      api.StatsMetric.toolCalls: l10n.vizMetricToolCalls,
      api.StatsMetric.approvalCards: l10n.vizMetricApprovalCards,
      api.StatsMetric.cost: l10n.vizMetricCost,
      api.StatsMetric.invocationsDenied: l10n.vizMetricInvocationsDenied,
      api.StatsMetric.billingModeSwitches: l10n.vizMetricBillingModeSwitches,
      api.StatsMetric.catchups: l10n.vizMetricCatchups,
      api.StatsMetric.resyncs: l10n.vizMetricResyncs,
      api.StatsMetric.slowConsumerCloses: l10n.vizMetricSlowConsumerCloses,
    }[metric];
    if (known != null) return known;
    final code = wireMetric(metric);
    return sloDisplayName(l10n, code) ?? l10n.vizMetricUnknown(code);
  }

  /// The label of a metric name of a query response (`messages`, `runs` and so on), which
  /// the schema leaves open: the name itself.
  String queryMetric(String name) => l10n.vizMetricUnknown(name);

  /// The text of a run [state].
  String runState(RunCardState state) => switch (state) {
    RunCardState.queued => l10n.liveBoxStateQueued,
    RunCardState.running => l10n.liveBoxStateRunning,
    RunCardState.waitingInput => l10n.vizRunStateWaitingInput,
    RunCardState.unknown => l10n.vizRunStateUnknown,
  };

  /// The text of a final or current run state of the timeline.
  String timelineState(api.RunState state) =>
      <api.RunState, String>{
        api.RunState.queued: l10n.liveBoxStateQueued,
        api.RunState.running: l10n.liveBoxStateRunning,
        api.RunState.waitingInput: l10n.vizRunStateWaitingInput,
        api.RunState.completed: l10n.liveBoxStateCompleted,
        api.RunState.failed: l10n.liveBoxStateFailed,
        api.RunState.cancelled: l10n.liveBoxStateCancelled,
      }[state] ??
      l10n.vizRunStateUnknown;

  /// The tool chip text of a step.
  String step(RunStepKind kind, String name) => switch (kind) {
    RunStepKind.toolCall => l10n.vizStepToolCall(name),
    RunStepKind.status => l10n.vizStepStatus(name),
    RunStepKind.subagent => l10n.vizStepSubagent(name),
    RunStepKind.workflow => l10n.vizStepWorkflow(name),
    RunStepKind.backgroundJob => l10n.vizStepBackgroundJob(name),
    RunStepKind.unknown => name,
  };

  /// The badge text of [mode].
  String badge(api.BillingMode mode) => mode == api.BillingMode.organization
      ? l10n.vizBadgeOrganization
      : l10n.vizBadgePersonal;

  /// [cost] as money with its label: "0.62 USD, Estimate", "0.05 USD, information only".
  String cost(VizCost cost) {
    final amount = format.money(cost.amount, cost.currency);
    return switch (cost.label) {
      CostLabel.estimate => '$amount, ${l10n.vizCostEstimate}',
      CostLabel.informationOnly => '$amount, ${l10n.vizCostInformationOnly}',
      CostLabel.finalAmount => amount,
    };
  }

  /// The text of a traffic light [status].
  String sloState(TrafficLightStatus status) => switch (status) {
    TrafficLightStatus.onTarget => l10n.vizSloOnTarget,
    TrafficLightStatus.budgetTight => l10n.vizSloBudgetTight,
    TrafficLightStatus.targetMissed => l10n.vizSloTargetMissed,
    TrafficLightStatus.notEnoughData => l10n.vizSloNotEnoughData,
  };

  /// The display name of an SLO by its metric, else its `service/name`.
  String slo(TrafficLightModel light) =>
      sloDisplayName(l10n, wireMetric(light.metric)) ?? light.slo;
}

/// The wire value of [metric], for example `tool_calls`.
String wireMetric(api.StatsMetric metric) =>
    api.standardSerializers.serialize(
          metric,
          specifiedType: const FullType(api.StatsMetric),
        )!
        as String;
