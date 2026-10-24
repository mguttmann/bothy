// SPDX-License-Identifier: Apache-2.0

/// Riverpod providers of the agent interface over the local database, the run store and the
/// REST reads (brief M4-21 Design notes 1 to 5; 15 4: one provider per query). Widgets never
/// query the database or the engine themselves (R-CL-03).
library;

import 'dart:async';

import 'package:domain/domain.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:sync_engine/sync_engine.dart';

import '../app/providers/session.dart';
import '../sidebar/providers.dart';
import 'agent_reads.dart';
import 'commands/command_suggestions.dart';

part 'providers.g.dart';

/// The REST reads of the agent interface; tests override it.
@riverpod
AgentReads agentReads(Ref ref) {
  final engine = ref.watch(syncEngineProvider);
  return engine == null ? const NoAgentReads() : EngineAgentReads(engine);
}

/// The run summaries of [threadId] from the local `runs` table, oldest first (the summary
/// of `run.state_changed`, 04 11.3).
@riverpod
Stream<List<RunSummary>> threadRuns(Ref ref, String threadId) {
  final db = ref.watch(localDatabaseProvider);
  return db == null
      ? Stream.value(const [])
      : db.runsDao.watchForThread(ThreadId.parse(threadId));
}

/// Where the live box reads the document of a run: the engine's run store, at most one state
/// per frame (04 11.3; 15 7.4).
abstract interface class RunSource {
  /// The document of [run] of [thread] while listened to; the run is focused (its thread
  /// joins the gateway focus, so patches arrive) until the listener cancels.
  Stream<RunDocument?> watch(RunId run, ThreadId thread);
}

/// [RunSource] on the engine: `EngineFocus.focusRun` and `RunStore.watch`.
final class EngineRunSource implements RunSource {
  /// Creates the source on [engine].
  const EngineRunSource(this.engine);

  /// The engine.
  final SyncEngine engine;

  @override
  Stream<RunDocument?> watch(RunId run, ThreadId thread) {
    StreamSubscription<RunDocument?>? sub;
    late final StreamController<RunDocument?> controller;
    controller = StreamController<RunDocument?>(
      onListen: () {
        engine.focus.focusRun(run, thread);
        sub = engine.runs
            .watch(run)
            .listen(controller.add, onError: controller.addError);
      },
      onCancel: () async {
        engine.focus.unfocusRun(run);
        await sub?.cancel();
      },
    );
    return controller.stream;
  }
}

/// The run source of the signed-in engine, or null; tests override it.
@riverpod
RunSource? runSource(Ref ref) {
  final engine = ref.watch(syncEngineProvider);
  return engine == null ? null : EngineRunSource(engine);
}

/// The document of [runId] in [threadId] while the live box shows it.
@riverpod
Stream<RunDocument?> runDocument(Ref ref, String runId, String threadId) {
  final source = ref.watch(runSourceProvider);
  return source == null
      ? Stream.value(null)
      : source.watch(RunId.parse(runId), ThreadId.parse(threadId));
}

/// The document of the ended run [runId], loaded once when its collapsed line is expanded
/// (`getRun`, 01 6.2; brief M4-21 Design note 1).
@riverpod
Future<RunDocument?> endedRunDocument(Ref ref, String runId) async {
  final json = await ref.watch(agentReadsProvider).run(RunId.parse(runId));
  if (json == null) return null;
  try {
    return RunDocument.fromJson(json);
  } on FormatException {
    return null;
  }
}

/// The components of the card [messageId] at its [version]: the echo `message.updated`
/// raises the version, which reads them again.
@riverpod
Future<List<MessageComponent>> messageComponents(
  Ref ref,
  String messageId,
  int version,
) async {
  final db = ref.watch(localDatabaseProvider);
  return db == null
      ? const []
      : db.messagesDao.componentsOf(MessageId.parse(messageId));
}

/// The registered commands that apply in [channelId] (`commandsForChannel`), from the local
/// `commands` table (15 6.1: suggestions work offline).
@riverpod
Stream<List<Command>> channelCommands(Ref ref, String channelId) {
  final db = ref.watch(localDatabaseProvider);
  if (db == null) return Stream.value(const []);
  final channel = ref.watch(storedChannelOfRouteProvider(channelId)).value;
  final id = ChannelId.parse(channelId);
  return db.commandsDao.watchAll().map(
    (all) => commandsForChannel(all, channel: id, spaceId: channel?.spaceId),
  );
}

/// The presence of [agentId] from the engine's ephemeral store (04 12 item 2).
@riverpod
Stream<AgentPresence?> agentPresence(Ref ref, String agentId) {
  final engine = ref.watch(syncEngineProvider);
  return engine == null
      ? Stream.value(null)
      : engine.ephemeral.watchPresence(PrincipalId.parse(agentId));
}

/// The agent [agentId] as `getAgent` answers it.
@riverpod
Future<AgentInfo?> agentInfo(Ref ref, String agentId) =>
    ref.watch(agentReadsProvider).agent(PrincipalId.parse(agentId));

/// The agent sessions of [threadId], newest first.
@riverpod
Future<List<AgentSessionView>> agentSessions(Ref ref, String threadId) =>
    ref.watch(agentReadsProvider).sessions(ThreadId.parse(threadId));

/// The cost totals of [threadId].
@riverpod
Future<List<ThreadCost>?> threadCosts(Ref ref, String threadId) =>
    ref.watch(agentReadsProvider).threadCosts(ThreadId.parse(threadId));

/// The number of runs of [threadId].
@riverpod
Future<({int count, bool more})?> threadRunCount(Ref ref, String threadId) =>
    ref.watch(agentReadsProvider).runCount(ThreadId.parse(threadId));

/// The project binding of [channelId].
@riverpod
Future<String?> channelProject(Ref ref, String channelId) =>
    ref.watch(agentReadsProvider).project(ChannelId.parse(channelId));

/// The runs whose box the person expanded or collapsed by hand, by run ID: true expanded,
/// false collapsed (local state; the keyboard binding is M4-24). It lasts while the app runs.
@Riverpod(keepAlive: true)
class RunExpansion extends _$RunExpansion {
  @override
  Map<String, bool> build() => const {};

  /// Sets the box of [runId] to [expanded].
  void set(String runId, {required bool expanded}) =>
      state = {...state, runId: expanded};
}
