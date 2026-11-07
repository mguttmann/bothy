// SPDX-License-Identifier: Apache-2.0

/// Rows of runs, agents and principals to domain entities and back.
library;

import 'dart:convert';

import 'package:domain/domain.dart';
import 'package:drift/drift.dart';

import '../database.dart';
import 'codec.dart';

/// The insert companion of [run].
RunsCompanion runToCompanion(RunSummary run) => RunsCompanion.insert(
  id: run.id.value,
  threadId: run.threadId.value,
  channelId: run.channelId.value,
  agentId: run.agentId.value,
  triggeredBy: run.triggeredBy.value,
  state: run.state.code,
  stateReason: Value(run.stateReason),
  version: run.version.value,
  billingModeExpected: run.billingModeExpected.code,
  billingModeUsed: Value(run.billingModeUsed?.code),
  statusLine: Value(run.statusLine),
  summary: Value(run.summary),
  costMicros: run.cost.amountMicros,
  currency: run.cost.currency,
  costIsEstimate: run.cost.isEstimate,
  stepCount: run.stepCount,
  durationMs: Value(run.durationMs),
  queuedAt: run.queuedAt,
  startedAt: Value(run.startedAt),
  finishedAt: Value(run.finishedAt),
);

/// The run summary of [row].
RunSummary runFromRow(RunRow row) => RunSummary(
  id: RunId.parse(row.id),
  threadId: ThreadId.parse(row.threadId),
  channelId: ChannelId.parse(row.channelId),
  agentId: PrincipalId.parse(row.agentId),
  triggeredBy: PrincipalId.parse(row.triggeredBy),
  state: RunState.fromCode(row.state),
  stateReason: row.stateReason,
  version: ResourceVersion(row.version),
  billingModeExpected: BillingMode.fromCode(row.billingModeExpected),
  billingModeUsed: optional(row.billingModeUsed, BillingMode.fromCode),
  statusLine: row.statusLine,
  summary: row.summary,
  cost: Money(
    amountMicros: row.costMicros,
    currency: row.currency,
    isEstimate: row.costIsEstimate,
  ),
  stepCount: row.stepCount,
  durationMs: row.durationMs,
  queuedAt: row.queuedAt,
  startedAt: row.startedAt,
  finishedAt: row.finishedAt,
);

/// The insert companion of [wake].
RunWakesCompanion wakeToCompanion(RunWake wake) => RunWakesCompanion.insert(
  id: wake.id.value,
  agentId: wake.agentId.value,
  threadId: wake.threadId.value,
  sourceRunId: wake.sourceRunId.value,
  triggeredBy: wake.triggeredBy.value,
  kind: wake.kind.code,
  condition: jsonEncode(wake.condition),
  label: wake.label,
  reason: wake.reason.code,
  outage: Value(wake.outage),
  notBefore: wake.notBefore,
  deadline: wake.deadline,
  state: wake.state.code,
  stateReason: Value(wake.stateReason),
  resultText: wake.resultText,
  firedRunId: Value(wake.firedRunId?.value),
  firedIntoRunId: Value(wake.firedIntoRunId?.value),
  createdAt: wake.createdAt,
  endedAt: Value(wake.endedAt),
  version: wake.version.value,
);

/// The wake-up of [row].
RunWake wakeFromRow(RunWakeRow row) => RunWake(
  id: WakeId.parse(row.id),
  agentId: PrincipalId.parse(row.agentId),
  threadId: ThreadId.parse(row.threadId),
  sourceRunId: RunId.parse(row.sourceRunId),
  triggeredBy: PrincipalId.parse(row.triggeredBy),
  kind: RunWakeKind.fromCode(row.kind),
  reason: RunWakeReason.fromCode(row.reason),
  label: row.label,
  condition: switch (jsonDecode(row.condition)) {
    final Map<String, Object?> map => map,
    _ => const <String, Object?>{},
  },
  outage: row.outage,
  notBefore: row.notBefore,
  deadline: row.deadline,
  state: RunWakeState.fromCode(row.state),
  stateReason: row.stateReason,
  resultText: row.resultText,
  firedRunId: optional(row.firedRunId, RunId.parse),
  firedIntoRunId: optional(row.firedIntoRunId, RunId.parse),
  createdAt: row.createdAt,
  endedAt: row.endedAt,
  version: ResourceVersion(row.version),
);

/// The insert companion of [card].
AgentsCompanion agentToCompanion(AgentCard card) => AgentsCompanion.insert(
  principalId: card.id.value,
  handle: card.handle,
  displayName: card.displayName,
  avatarFileId: Value(card.avatarFileId?.value),
  description: card.description,
  billingMode: card.billingMode.code,
  toolClass: card.toolClass.code,
  reportedRuntimeState: card.presence.state.code,
  reportedBillingMode: Value(card.presence.reportedBillingMode?.code),
  version: card.version.value,
);

/// The agent card of [row].
AgentCard agentFromRow(AgentRow row) => AgentCard(
  id: PrincipalId.parse(row.principalId),
  handle: row.handle,
  displayName: row.displayName,
  avatarFileId: optional(row.avatarFileId, FileId.parse),
  description: row.description,
  billingMode: BillingMode.fromCode(row.billingMode),
  toolClass: ToolClass.fromCode(row.toolClass),
  presence: AgentPresence(
    state: AgentRuntimeState.fromCode(row.reportedRuntimeState),
    reportedBillingMode: optional(
      row.reportedBillingMode,
      BillingMode.fromCode,
    ),
  ),
  version: ResourceVersion(row.version),
);

/// The insert companion of [principal].
PrincipalsCompanion principalToCompanion(Principal principal) =>
    PrincipalsCompanion.insert(
      id: principal.id.value,
      kind: principal.kind.code,
      handle: principal.handle,
      displayName: principal.displayName,
      avatarFileId: Value(principal.avatarFileId?.value),
    );

/// The principal of [row].
Principal principalFromRow(PrincipalRow row) => Principal(
  id: PrincipalId.parse(row.id),
  kind: PrincipalKind.fromCode(row.kind),
  handle: row.handle,
  displayName: row.displayName,
  avatarFileId: optional(row.avatarFileId, FileId.parse),
);
