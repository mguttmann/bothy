// SPDX-License-Identifier: Apache-2.0

/// Interface texts of the marks, all from `l10n` (docs/spec/25-visualization.md 7.4).
library;

import 'package:api_client/api_client.dart' as api;
import 'package:built_value/serializer.dart';
import 'package:design_system/design_system.dart' show dsReasonText;
import 'package:domain/domain.dart';
import 'package:flutter/widgets.dart';
import 'package:l10n/l10n.dart';

import '../model/costs.dart';
import '../model/run_card_model.dart';
import '../model/run_label.dart';
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

  /// The label of [metric] with the code [dimension] of its closed list (25 3.2), for example
  /// "Tool calls, Failed": the one place that turns a dimension code into text, so no label of
  /// a rate, a series or a legend shows a raw code. A code of a newer server shows as
  /// `vizMetricUnknown` behind the name.
  String metricLabel(api.StatsMetric metric, String? dimension) {
    final name = this.metric(metric);
    if (dimension == null || dimension.isEmpty) return name;
    return l10n.vizMetricWithDimension(
      name,
      this.dimension(metric, dimension) ?? l10n.vizMetricUnknown(dimension),
    );
  }

  /// The text of the dimension [code] of [metric] (25 3.2), `null` for a code this app does not
  /// know: origins of messages, end states, approval decisions, refusals, billing modes, resync
  /// reasons and push providers.
  String? dimension(api.StatsMetric metric, String code) => switch (code) {
    'human' => l10n.vizDimHuman,
    'agent' => l10n.vizDimAgent,
    'app' => l10n.vizDimApp,
    'import' => l10n.vizDimImport,
    'completed' => l10n.vizDimCompleted,
    'failed' => l10n.vizDimFailed,
    'cancelled' => l10n.vizDimCancelled,
    'requested' => l10n.vizDimRequested,
    'approved' => l10n.vizDimApproved,
    'denied' => l10n.vizDimDenied,
    'expired' => l10n.vizDimExpired,
    'i2_denied' => l10n.vizDimI2Denied,
    'billing_mode_mismatch' => l10n.vizDimBillingModeMismatch,
    'personal' => l10n.vizBadgePersonal,
    'organization' => l10n.vizBadgeOrganization,
    'epoch_changed' => l10n.vizDimEpochChanged,
    'retention' => l10n.vizDimRetention,
    'too_far_behind' => l10n.vizDimTooFarBehind,
    'cursor_ahead' => l10n.vizDimCursorAhead,
    _ => pushProvider(code),
  };

  /// The product name of the push provider [code] ([vizPushProvider]).
  String? pushProvider(String code) => vizPushProvider(l10n, code);

  /// The label of a metric name of a query response (`getUsageStats`, `getRunStats`,
  /// `getErrorStats`; their operation descriptions list the names), with the same texts as the
  /// statistics page; a name of a newer server shows as `vizMetricUnknown`.
  String queryMetric(String name) {
    String runs(String code) => l10n.vizMetricWithDimension(
      l10n.statsColumnRuns,
      dimension(api.StatsMetric.runsFinished, code)!,
    );
    const messages = 'messages_';
    const reason = 'failed_reason_';
    const push = 'push_failure_rate_';
    return switch (name) {
      'messages' => l10n.statsColumnMessages,
      'threads_created' => l10n.statsColumnThreadsCreated,
      'threads_done' => l10n.statsColumnThreadsDone,
      'active_people' => l10n.statsColumnActivePeople,
      'storage_bytes' => l10n.statsColumnStorage,
      'runs' => l10n.statsColumnRuns,
      'completed' || 'failed' || 'cancelled' => runs(name),
      'duration_ms_p50' => l10n.statsColumnMedian,
      'duration_ms_p95' => l10n.statsColumnP95,
      'run_failure_rate' => l10n.statsErrorRunFailure,
      'invocation_denials' => l10n.statsErrorInvocationDenials,
      'webhook_failure_rate' => l10n.statsErrorWebhookFailure,
      'scan_errors' => l10n.statsErrorScanErrors,
      'scan_infected' => l10n.statsErrorScanInfected,
      _
          when name.startsWith(messages) &&
              dimension(
                    api.StatsMetric.messagesCreated,
                    name.substring(messages.length),
                  ) !=
                  null =>
        l10n.vizMetricWithDimension(
          l10n.statsColumnMessages,
          dimension(
            api.StatsMetric.messagesCreated,
            name.substring(messages.length),
          )!,
        ),
      _ when name.startsWith(reason) => l10n.vizMetricWithDimension(
        l10n.statsSectionFailures,
        dsReasonText(l10n, name.substring(reason.length)) ??
            name.substring(reason.length),
      ),
      _ when name.startsWith(push) => l10n.statsErrorPushFailure(
        pushProvider(name.substring(push.length)) ??
            name.substring(push.length),
      ),
      _ => l10n.vizMetricUnknown(name),
    };
  }

  /// The main text of a run label: project and chat, the project alone for a main thread, and
  /// the agent when the server named no chat.
  String runTitle(VizRunLabel label) {
    final project = label.project;
    if (project == null) return label.agent;
    final chat = label.chat;
    return chat == null ? project : l10n.vizRunTitle(project, chat);
  }

  /// The secondary text of a run label: agent and short ID, the short ID alone when the main
  /// text already is the agent.
  String runDetail(VizRunLabel label) => label.named
      ? l10n.vizRunDetail(label.agent, label.shortId)
      : label.shortId;

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

/// The product name of the push provider [code] (`bothy.push.provider`, the dimension of
/// `sli.push_acceptance`), `null` when unknown; the statistics page names the providers of
/// `push_failure_rate_<provider>` with it too.
String? vizPushProvider(AppLocalizations l10n, String code) => switch (code) {
  'apns' => l10n.vizDimApns,
  'fcm' => l10n.vizDimFcm,
  'unifiedpush' => l10n.vizDimUnifiedPush,
  'wns' => l10n.vizDimWns,
  _ => null,
};

/// The wire value of [metric], for example `tool_calls`.
String wireMetric(api.StatsMetric metric) =>
    api.standardSerializers.serialize(
          metric,
          specifiedType: const FullType(api.StatsMetric),
        )!
        as String;
