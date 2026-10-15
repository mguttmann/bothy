// SPDX-License-Identifier: Apache-2.0

/// Run summaries (15 6.1 `runs`): the collapsed row from `run.state_changed`; the run document lives only in memory (M4-16).
library;

import '../coded.dart';
import '../ids.dart';
import '../seq.dart';
import 'principal.dart';

/// State of a run (03 `runs.state`, API `RunState`).
enum RunState implements Coded {
  /// Queued.
  queued('queued'),

  /// Running.
  running('running'),

  /// Waiting for input.
  waitingInput('waiting_input'),

  /// Completed.
  completed('completed'),

  /// Failed.
  failed('failed'),

  /// Cancelled.
  cancelled('cancelled'),

  /// A value of a newer server.
  unknown('');

  const RunState(this.code);

  @override
  final String code;

  /// Decodes [code]; a value this version does not know is [unknown].
  static RunState fromCode(String code) => decodeCode(values, code, unknown);
}

/// What started a run (03 `runs.trigger_kind`).
enum RunTriggerKind implements Coded {
  /// A message.
  message('message'),

  /// A command.
  command('command'),

  /// An interaction.
  interaction('interaction'),

  /// A value of a newer server.
  unknown('');

  const RunTriggerKind(this.code);

  @override
  final String code;

  /// Decodes [code]; a value this version does not know is [unknown].
  static RunTriggerKind fromCode(String code) =>
      decodeCode(values, code, unknown);
}

/// Kind of a run step (03 `run_steps.kind`).
enum StepKind implements Coded {
  /// Tool call.
  toolCall('tool_call'),

  /// Status.
  status('status'),

  /// Subagent.
  subagent('subagent'),

  /// Workflow.
  workflow('workflow'),

  /// Background job.
  backgroundJob('background_job'),

  /// A value of a newer server.
  unknown('');

  const StepKind(this.code);

  @override
  final String code;

  /// Decodes [code]; a value this version does not know is [unknown].
  static StepKind fromCode(String code) => decodeCode(values, code, unknown);
}

/// Status of a run step (03 `run_steps.status`).
enum StepStatus implements Coded {
  /// Pending.
  pending('pending'),

  /// Running.
  running('running'),

  /// Completed.
  completed('completed'),

  /// Failed.
  failed('failed'),

  /// Cancelled.
  cancelled('cancelled'),

  /// A value of a newer server.
  unknown('');

  const StepStatus(this.code);

  @override
  final String code;

  /// Decodes [code]; a value this version does not know is [unknown].
  static StepStatus fromCode(String code) => decodeCode(values, code, unknown);
}

/// An amount of money (API `Money`).
final class Money {
  /// Creates a [Money].
  const Money({
    required this.amountMicros,
    required this.currency,
    required this.isEstimate,
  });

  /// Amount in millionths of the currency unit.
  final int amountMicros;

  /// ISO 4217 code.
  final String currency;

  /// Whether the amount is an estimate (03 `runs.cost_is_estimate`).
  final bool isEstimate;

  /// Returns a copy with the given fields replaced; a nullable field cannot be cleared
  /// through it (build a new object for that).
  Money copyWith({int? amountMicros, String? currency, bool? isEstimate}) =>
      Money(
        amountMicros: amountMicros ?? this.amountMicros,
        currency: currency ?? this.currency,
        isEstimate: isEstimate ?? this.isEstimate,
      );

  @override
  bool operator ==(Object other) =>
      other is Money &&
      other.amountMicros == amountMicros &&
      other.currency == currency &&
      other.isEstimate == isEstimate;

  @override
  int get hashCode => Object.hash(amountMicros, currency, isEstimate);
}

/// Summary of a run (API `RunSummary`).
final class RunSummary {
  /// Creates a [RunSummary].
  const RunSummary({
    required this.id,
    required this.threadId,
    required this.channelId,
    required this.agentId,
    required this.triggeredBy,
    required this.state,
    required this.version,
    required this.billingModeExpected,
    required this.cost,
    required this.stepCount,
    required this.queuedAt,
    this.stateReason,
    this.billingModeUsed,
    this.statusLine,
    this.summary,
    this.durationMs,
    this.startedAt,
    this.finishedAt,
  });

  /// ID.
  final RunId id;

  /// Thread.
  final ThreadId threadId;

  /// Channel.
  final ChannelId channelId;

  /// Agent.
  final PrincipalId agentId;

  /// Person who triggered it.
  final PrincipalId triggeredBy;

  /// State.
  final RunState state;

  /// Reason code of the state.
  final String? stateReason;

  /// Version (03 `version`; 04 9 item 3).
  final ResourceVersion version;

  /// Configured mode at the last delivery.
  final BillingMode billingModeExpected;

  /// Mode the adapter reported.
  final BillingMode? billingModeUsed;

  /// Current status line.
  final String? statusLine;

  /// Collapsed line after the end.
  final String? summary;

  /// Cost.
  final Money cost;

  /// Number of steps.
  final int stepCount;

  /// Duration in milliseconds.
  final int? durationMs;

  /// Queue time.
  final DateTime queuedAt;

  /// Start time.
  final DateTime? startedAt;

  /// End time.
  final DateTime? finishedAt;

  /// Returns a copy with the given fields replaced; a nullable field cannot be cleared
  /// through it (build a new object for that).
  RunSummary copyWith({
    ThreadId? threadId,
    ChannelId? channelId,
    PrincipalId? agentId,
    PrincipalId? triggeredBy,
    RunState? state,
    String? stateReason,
    ResourceVersion? version,
    BillingMode? billingModeExpected,
    BillingMode? billingModeUsed,
    String? statusLine,
    String? summary,
    Money? cost,
    int? stepCount,
    int? durationMs,
    DateTime? queuedAt,
    DateTime? startedAt,
    DateTime? finishedAt,
  }) => RunSummary(
    id: id,
    threadId: threadId ?? this.threadId,
    channelId: channelId ?? this.channelId,
    agentId: agentId ?? this.agentId,
    triggeredBy: triggeredBy ?? this.triggeredBy,
    state: state ?? this.state,
    stateReason: stateReason ?? this.stateReason,
    version: version ?? this.version,
    billingModeExpected: billingModeExpected ?? this.billingModeExpected,
    billingModeUsed: billingModeUsed ?? this.billingModeUsed,
    statusLine: statusLine ?? this.statusLine,
    summary: summary ?? this.summary,
    cost: cost ?? this.cost,
    stepCount: stepCount ?? this.stepCount,
    durationMs: durationMs ?? this.durationMs,
    queuedAt: queuedAt ?? this.queuedAt,
    startedAt: startedAt ?? this.startedAt,
    finishedAt: finishedAt ?? this.finishedAt,
  );

  @override
  bool operator ==(Object other) =>
      other is RunSummary &&
      other.id == id &&
      other.threadId == threadId &&
      other.channelId == channelId &&
      other.agentId == agentId &&
      other.triggeredBy == triggeredBy &&
      other.state == state &&
      other.stateReason == stateReason &&
      other.version == version &&
      other.billingModeExpected == billingModeExpected &&
      other.billingModeUsed == billingModeUsed &&
      other.statusLine == statusLine &&
      other.summary == summary &&
      other.cost == cost &&
      other.stepCount == stepCount &&
      other.durationMs == durationMs &&
      other.queuedAt == queuedAt &&
      other.startedAt == startedAt &&
      other.finishedAt == finishedAt;

  @override
  int get hashCode => Object.hash(
    id,
    threadId,
    channelId,
    agentId,
    triggeredBy,
    state,
    stateReason,
    version,
    billingModeExpected,
    billingModeUsed,
    statusLine,
    summary,
    cost,
    stepCount,
    durationMs,
    queuedAt,
    startedAt,
    finishedAt,
  );
}
