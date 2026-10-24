// SPDX-License-Identifier: Apache-2.0

/// The REST reads of the agent interface (brief M4-21 Design notes 1, 4 and 5): the document
/// of a finished run when its line is expanded (`getRun`), the agent card (`getAgent`), the
/// agent sessions (`listAgentSessions`), the thread cost (`getThreadCosts`), the runs of the
/// thread (`listThreadRuns`), the project binding (`getChannelBehavior`), the commands of a
/// channel and the option suggestions. Values the person may not read are absent in the
/// server's answer and stay absent here (a refusal reads as null or empty).
library;

import 'package:api_client/api_client.dart' as wire;
import 'package:domain/domain.dart';
import 'package:sync_engine/sync_engine.dart';

import 'commands/autocomplete_client.dart';
import 'commands/command_loader.dart';

/// The agent of the session column (`getAgent`): configured mode, presence and the mode the
/// adapter reports.
typedef AgentInfo = ({
  BillingMode billingMode,
  AgentRuntimeState presence,
  BillingMode? reportedBillingMode,
});

/// One agent session of a thread (API `AgentSession`).
typedef AgentSessionView = ({
  String id,
  String? externalSessionRef,
  bool current,
  String? model,
  Money cost,
  DateTime createdAt,
  DateTime? endedAt,
});

/// The cost of a thread per backend policy (API `CostBreakdown.totals`).
typedef ThreadCost = ({BillingMode mode, Money money});

/// The reads; tests replace them.
abstract interface class AgentReads {
  /// The whole document of [run] (`getRun`) as JSON with its `version`, or null.
  Future<Map<String, Object?>?> run(RunId run);

  /// The agent [agent] (`getAgent`), or null when the reader may not read it.
  Future<AgentInfo?> agent(PrincipalId agent);

  /// The current and earlier sessions of [thread], newest first.
  Future<List<AgentSessionView>> sessions(ThreadId thread);

  /// The cost totals of [thread], or null when the reader may not read them.
  Future<List<ThreadCost>?> threadCosts(ThreadId thread);

  /// The number of runs of [thread] (`listThreadRuns`, first page) and whether more exist.
  Future<({int count, bool more})?> runCount(ThreadId thread);

  /// The name of the project binding of [channel] (`workspace_ref` and branch), or null.
  Future<String?> project(ChannelId channel);

  /// The commands of a channel (`listChannelCommands`).
  ChannelCommandsFetch get commands;

  /// The option suggestions (`autocompleteCommand`).
  AutocompleteFetch get autocomplete;
}

/// [AgentReads] through the REST client of [engine].
final class EngineAgentReads implements AgentReads {
  /// Creates the reads.
  EngineAgentReads(this._engine);

  final SyncEngine _engine;

  wire.ApiClient get _api => _engine.api;

  @override
  Future<Map<String, Object?>?> run(RunId run) async {
    final doc = (await _api.getRunsApi().getRun(runId: run.value)).data;
    if (doc == null) return null;
    final json = _api.serializers.serializeWith(wire.Run.serializer, doc);
    return json is Map ? json.cast<String, Object?>() : null;
  }

  @override
  Future<AgentInfo?> agent(PrincipalId agent) async {
    try {
      final a = (await _api.getAgentsApi().getAgent(agentId: agent.value)).data;
      if (a == null) return null;
      return (
        billingMode: BillingMode.fromCode(a.billingMode.name),
        presence: AgentRuntimeState.fromCode(a.presence.state.name),
        reportedBillingMode: a.presence.reportedBillingMode == null
            ? null
            : BillingMode.fromCode(a.presence.reportedBillingMode!.name),
      );
    } on Object {
      return null;
    }
  }

  @override
  Future<List<AgentSessionView>> sessions(ThreadId thread) async {
    try {
      final list = (await _api.getThreadsApi().listAgentSessions(
        threadId: thread.value,
      )).data;
      final items = [
        for (final s in list?.items ?? const <wire.AgentSession>[])
          (
            id: s.id,
            externalSessionRef: s.externalSessionRef,
            current: s.isCurrent,
            model: s.model,
            cost: Money(
              amountMicros: s.cost.amountMicros,
              currency: s.cost.currency,
              isEstimate: s.cost.isEstimate,
            ),
            createdAt: s.createdAt,
            endedAt: s.endedAt,
          ),
      ]..sort((a, b) => b.createdAt.compareTo(a.createdAt));
      return items;
    } on Object {
      return const [];
    }
  }

  @override
  Future<List<ThreadCost>?> threadCosts(ThreadId thread) async {
    try {
      final costs = (await _api.getStatsApi().getThreadCosts(
        threadId: thread.value,
      )).data;
      if (costs == null) return null;
      return [
        for (final t in costs.totals)
          (
            mode: BillingMode.fromCode(t.billingMode.name),
            money: Money(
              amountMicros: t.amountMicros,
              currency: t.currency,
              isEstimate: t.isEstimate,
            ),
          ),
      ];
    } on Object {
      return null;
    }
  }

  @override
  Future<({int count, bool more})?> runCount(ThreadId thread) async {
    try {
      final page = (await _api.getRunsApi().listThreadRuns(
        threadId: thread.value,
      )).data;
      if (page == null) return null;
      return (count: page.items.length, more: page.nextCursor != null);
    } on Object {
      return null;
    }
  }

  @override
  Future<String?> project(ChannelId channel) async {
    try {
      final b = (await _api.getChannelsApi().getChannelBehavior(
        channelId: channel.value,
      )).data?.projectBinding;
      if (b == null) return null;
      final branch = b.defaultBranch;
      return branch == null ? b.workspaceRef : '${b.workspaceRef}, $branch';
    } on Object {
      return null;
    }
  }

  @override
  ChannelCommandsFetch get commands => channelCommandsFetch(_api);

  @override
  AutocompleteFetch get autocomplete => AutocompleteClient.api(_api);
}

/// Reads that answer nothing (no signed-in engine).
final class NoAgentReads implements AgentReads {
  /// Creates the reads.
  const NoAgentReads();

  @override
  Future<Map<String, Object?>?> run(RunId run) async => null;

  @override
  Future<AgentInfo?> agent(PrincipalId agent) async => null;

  @override
  Future<List<AgentSessionView>> sessions(ThreadId thread) async => const [];

  @override
  Future<List<ThreadCost>?> threadCosts(ThreadId thread) async => null;

  @override
  Future<({int count, bool more})?> runCount(ThreadId thread) async => null;

  @override
  Future<String?> project(ChannelId channel) async => null;

  @override
  ChannelCommandsFetch get commands =>
      (_) async => const [];

  @override
  AutocompleteFetch get autocomplete =>
      (_, _) async => const [];
}
