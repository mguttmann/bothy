// SPDX-License-Identifier: Apache-2.0

/// Mirror tables of runs and principals (15 6.1: `runs`, `agents`, `principals`).
library;

import 'package:drift/drift.dart';

/// 03 `runs`, only the summary of `run.state_changed`; the run document stays in memory (M4-16).
@DataClassName('RunRow')
@TableIndex(name: 'runs_thread', columns: {#threadId})
class Runs extends Table {
  @override
  String get tableName => 'runs';

  /// 03 `runs.id`.
  TextColumn get id => text()();

  /// 03 `runs.thread_id`.
  TextColumn get threadId => text()();

  /// API `RunSummary.channel_id`.
  TextColumn get channelId => text()();

  /// 03 `runs.agent_id`.
  TextColumn get agentId => text()();

  /// 03 `runs.triggered_by`.
  TextColumn get triggeredBy => text()();

  /// 03 `runs.state`.
  TextColumn get state => text()();

  /// 03 `runs.state_reason`.
  TextColumn get stateReason => text().nullable()();

  /// 03 `runs.version`.
  IntColumn get version => integer()();

  /// 03 `runs.billing_mode_expected`.
  TextColumn get billingModeExpected => text()();

  /// 03 `runs.billing_mode_used`.
  TextColumn get billingModeUsed => text().nullable()();

  /// 03 `runs.status_line`.
  TextColumn get statusLine => text().nullable()();

  /// 03 `runs.summary`.
  TextColumn get summary => text().nullable()();

  /// 03 `runs.cost_micros`.
  IntColumn get costMicros => integer()();

  /// 03 `runs.currency`.
  TextColumn get currency => text()();

  /// 03 `runs.cost_is_estimate`.
  BoolColumn get costIsEstimate => boolean()();

  /// API `RunSummary.step_count`.
  IntColumn get stepCount => integer()();

  /// 03 `runs.duration_ms`.
  IntColumn get durationMs => integer().nullable()();

  /// 03 `runs.queued_at`.
  DateTimeColumn get queuedAt => dateTime()();

  /// 03 `runs.started_at`.
  DateTimeColumn get startedAt => dateTime().nullable()();

  /// 03 `runs.finished_at`.
  DateTimeColumn get finishedAt => dateTime().nullable()();

  @override
  Set<Column<Object>> get primaryKey => {id};
}

/// The card of an agent (15 6.1 `agents`): display, mode badge and reported presence; no
/// operator configuration, credential holder or heartbeat details.
@DataClassName('AgentRow')
class Agents extends Table {
  @override
  String get tableName => 'agents';

  /// 03 `agents.principal_id`.
  TextColumn get principalId => text()();

  /// 03 `principals.handle`.
  TextColumn get handle => text()();

  /// 03 `principals.display_name`.
  TextColumn get displayName => text()();

  /// 03 `principals.avatar_file_id`.
  TextColumn get avatarFileId => text().nullable()();

  /// 03 `agents.description`.
  TextColumn get description => text()();

  /// 03 `agents.billing_mode`.
  TextColumn get billingMode => text()();

  /// 03 `agents.tool_class`.
  TextColumn get toolClass => text()();

  /// 03 `agents.reported_runtime_state`.
  TextColumn get reportedRuntimeState => text()();

  /// 03 `agents.reported_billing_mode`.
  TextColumn get reportedBillingMode => text().nullable()();

  /// 03 `agents.version`.
  IntColumn get version => integer()();

  @override
  Set<Column<Object>> get primaryKey => {principalId};
}

/// 03 `principals` for display (API `PrincipalRef`).
@DataClassName('PrincipalRow')
class Principals extends Table {
  @override
  String get tableName => 'principals';

  /// 03 `principals.id`.
  TextColumn get id => text()();

  /// 03 `principals.kind`.
  TextColumn get kind => text()();

  /// 03 `principals.handle`.
  TextColumn get handle => text()();

  /// 03 `principals.display_name`.
  TextColumn get displayName => text()();

  /// 03 `principals.avatar_file_id`.
  TextColumn get avatarFileId => text().nullable()();

  @override
  Set<Column<Object>> get primaryKey => {id};
}
