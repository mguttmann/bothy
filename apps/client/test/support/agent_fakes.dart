// SPDX-License-Identifier: Apache-2.0

// Fakes of the agent interface (brief M4-21 Tests): run documents at given versions, a run
// source on a real `RunStore` paced by the frames of the test binding, and REST reads answered
// by the test.

import 'dart:async';

import 'package:client/agents/agent_reads.dart';
import 'package:client/agents/commands/autocomplete_client.dart';
import 'package:client/agents/commands/command_loader.dart';
import 'package:client/agents/providers.dart';
import 'package:client/platform/frame_ticker.dart';
import 'package:dio/dio.dart';
import 'package:domain/domain.dart';
import 'package:sync_engine/sync_engine.dart';

import 'structure.dart';

/// The ID of run [n] (as `run(n, ...)` of structure.dart).
RunId runId(int n) => RunId.parse(uuid(0x700 + n));

/// A run document of run [n] in thread 1 at [version] (API `Run`, `run_document.v1`).
Map<String, Object?> runDoc(
  int n, {
  int version = 1,
  String state = 'running',
  String? reason,
  String? statusLine,
  String? lastText,
  Map<String, Object?> steps = const {},
  String? stallWarnedAt,
  double? contextPct,
  String mode = 'organization',
  String? model,
  String? triggeringMessageId,
}) => {
  'id': runId(n).value,
  'thread_id': threadId(1).value,
  'channel_id': channelId(1).value,
  'agent_id': robin.value,
  'triggered_by': alex.value,
  'triggering_message_id': ?triggeringMessageId,
  'state': state,
  'state_reason': ?reason,
  'version': version,
  'billing_mode_expected': mode,
  'model': ?model,
  'status_line': ?statusLine,
  'last_text_excerpt': ?lastText,
  'metrics': {'context_pct': ?contextPct, 'tokens_in': 1200, 'tokens_out': 300},
  'cost': {'amount_micros': 620000, 'currency': 'USD', 'is_estimate': true},
  'steps': steps,
  'followups': <String, Object?>{},
  'queued_at': t0.toIso8601String(),
  'started_at': t0.toIso8601String(),
  'stall_warned_at': ?stallWarnedAt,
};

/// A step of a run document.
Map<String, Object?> step(
  String id,
  int ordinal, {
  String kind = 'tool_call',
  String name = 'search',
  String status = 'running',
  String? parent,
}) => {
  'id': id,
  'ordinal': ordinal,
  'kind': kind,
  'name': name,
  'title': 'Step $id',
  'status': status,
  'parent_step_id': ?parent,
  'started_at': t0.toIso8601String(),
};

/// Run documents fetched by the store, by run.
final class FakeRunFetcher implements RunDocumentFetcher {
  /// The document each fetch returns.
  final Map<RunId, Map<String, Object?>> docs = {};

  /// How many fetches ran.
  int fetches = 0;

  @override
  Future<Map<String, Object?>> fetchRun(RunId id) async {
    fetches++;
    final doc = docs[id];
    if (doc == null) throw const FormatException('unknown run');
    return Map.of(doc);
  }
}

/// A [RunSource] on a real [RunStore] with the frames of the test binding; [published]
/// counts the documents each watcher received (one per frame at most, 15 7.4).
final class StoreRunSource implements RunSource {
  /// Creates the source.
  StoreRunSource() : this._(FakeRunFetcher());

  StoreRunSource._(this.fetcher)
    : store = RunStore(fetch: fetcher, ticker: SchedulerFrameTicker());

  /// The fetcher of [store].
  final FakeRunFetcher fetcher;

  /// The store.
  final RunStore store;

  /// The documents published to the widgets, in order.
  final List<RunDocument?> published = [];

  @override
  Stream<RunDocument?> watch(RunId run, ThreadId thread) {
    store.focus(run);
    return store.watch(run).map((d) {
      published.add(d);
      return d;
    });
  }
}

/// REST reads of the agent interface answered by the test.
final class FakeAgentReads implements AgentReads {
  /// `getRun` answers by run.
  final Map<RunId, Map<String, Object?>> runs = {};

  /// `getAgent` answers by agent.
  final Map<PrincipalId, AgentInfo> agents = {};

  /// `listAgentSessions` answers by thread.
  final Map<ThreadId, List<AgentSessionView>> sessionsOf = {};

  /// `getThreadCosts` answers by thread.
  final Map<ThreadId, List<ThreadCost>> costs = {};

  /// `listThreadRuns` counts by thread.
  final Map<ThreadId, int> runCounts = {};

  /// `getChannelBehavior` project names by channel.
  final Map<ChannelId, String> projects = {};

  /// `listChannelCommands` answers by channel.
  final Map<ChannelId, List<Command>> channelCommands = {};

  /// The option suggestions: option name and typed value in, choices out.
  List<CommandChoice> Function(String option, String value) suggest = (_, _) =>
      const [];

  /// Every autocomplete request in order, with its cancel token.
  final List<(AutocompleteRequest, CancelToken)> autocompleteRequests = [];

  /// Answers of autocomplete requests wait for this, when set.
  Completer<void>? hold;

  @override
  Future<Map<String, Object?>?> run(RunId run) async => runs[run];

  @override
  Future<AgentInfo?> agent(PrincipalId agent) async => agents[agent];

  @override
  Future<List<AgentSessionView>> sessions(ThreadId thread) async =>
      sessionsOf[thread] ?? const [];

  @override
  Future<List<ThreadCost>?> threadCosts(ThreadId thread) async => costs[thread];

  @override
  Future<({int count, bool more})?> runCount(ThreadId thread) async {
    final n = runCounts[thread];
    return n == null ? null : (count: n, more: false);
  }

  @override
  Future<String?> project(ChannelId channel) async => projects[channel];

  @override
  ChannelCommandsFetch get commands =>
      (channel) async => channelCommands[channel] ?? const [];

  @override
  AutocompleteFetch get autocomplete => (request, cancel) async {
    autocompleteRequests.add((request, cancel));
    await hold?.future;
    return suggest(request.option, request.value);
  };
}

/// The ID of component [n] of a card.
ComponentId componentId(int n) => ComponentId.parse(uuid(0x800 + n));

/// Component [n] of the card [message]: a button [customId] with [label] bound to run 1.
MessageComponent button(
  int n,
  MessageId message, {
  required String customId,
  required String label,
  String style = 'primary',
  ComponentState state = ComponentState.active,
  Capability? capability,
  ComponentRestriction restrictedTo = ComponentRestriction.any,
  PrincipalId? usedBy,
  DateTime? usedAt,
  int version = 1,
}) => MessageComponent(
  id: componentId(n),
  messageId: message,
  customId: customId,
  type: ComponentType.button,
  definition: {'label': label, 'value': customId, 'style': style},
  position: n,
  singleUse: true,
  requiredCapability: capability,
  restrictedTo: restrictedTo,
  runId: runId(1),
  state: state,
  usedBy: usedBy,
  usedAt: usedAt,
  version: ResourceVersion(version),
);
