// SPDX-License-Identifier: Apache-2.0

/// The built-in commands of 09 10.1 in the open thread (brief M4-21 Design note 4): each runs
/// its own operation through the outbox (`cancelRun`, `resetAgentSession`,
/// `bindAgentSession`, `setThreadTitle`, `closeThread`) or reads only (`/status`); none goes
/// through `invokeCommand`.
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';

import '../../app/providers/ports.dart';
import '../../app/providers/session.dart';
import '../../running/providers.dart' show stopRun;
import '../../sync/agent_ops.dart';
import '../../thread/providers.dart';
import '../../thread/thread_actions.dart';
import '../agent_reads.dart';
import '../followups.dart';
import '../providers.dart';
import '../run_mapping.dart';
import 'builtin_commands.dart';

/// Whether each built-in is allowed in [thread] by the server's answers: Stop like the live
/// box (an active run the person may stop), the session commands with the level `trigger`,
/// the title like the header's title action, Done for a writer while the thread is open.
BuiltinAvailability builtinAvailability({
  required Thread thread,
  required RunSummary? activeRun,
  required bool canStop,
  required bool triggers,
  required bool canEditTitle,
  required bool canWrite,
}) => (
  stop: activeRun != null && canStop,
  session: triggers,
  title: canEditTitle,
  close:
      canWrite &&
      thread.kind != ThreadKind.main &&
      thread.state != ThreadState.closed,
);

/// [BuiltinCommandRunner] of [thread] for the history column of [context].
final class ThreadCommandRunner implements BuiltinCommandRunner {
  /// Creates the runner.
  ThreadCommandRunner({
    required this.context,
    required this.ref,
    required this.thread,
  });

  /// The history column's context (dialogs).
  final BuildContext context;

  /// The history column's provider access.
  final WidgetRef ref;

  /// The open thread.
  final Thread thread;

  RunSummary? get _activeRun => newestActiveRun(
    ref.read(threadRunsProvider(thread.id.value)).value ?? const [],
  );

  @override
  Future<void> stop() async {
    final engine = ref.read(syncEngineProvider);
    final run = _activeRun;
    if (engine != null && run != null) await stopRun(engine, run);
  }

  @override
  Future<void> status() async {
    final reads = ref.read(agentReadsProvider);
    final sessions = await reads.sessions(thread.id);
    if (!context.mounted) return;
    final run = _activeRun;
    final principals = ref.read(principalsByIdProvider).value ?? const {};
    final config = ref.read(appConfigProvider);
    final now = ref.read(appPortsProvider).clock.now();
    AgentSessionView? current;
    for (final s in sessions) {
      if (s.current) current = s;
    }
    final l = AppLocalizations.of(context);
    await showDialog<void>(
      context: context,
      builder: (context) => AlertDialog(
        key: const ValueKey('command.status'),
        title: Text(l.commandStatusTitle),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            if (run == null)
              Text(l.commandStatusNoRun)
            else
              DsLiveBox.collapsed(
                runFromSummary(
                  run,
                  agentName:
                      principals[run.agentId]?.displayName ?? run.agentId.value,
                  now: now,
                ),
                announceMinInterval: config.liveAnnounceMinInterval,
              ),
            if (current != null) ...[
              const SizedBox(height: DsSpace.s8),
              Text(
                '${l.sessionAgentSession}: '
                '${current.externalSessionRef ?? current.id}',
              ),
              if (current.model != null)
                Text('${l.sessionAgentModel}: ${current.model}'),
            ],
          ],
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(),
            child: Text(l.threadCancel),
          ),
        ],
      ),
    );
  }

  @override
  Future<void> newSession() async {
    final engine = ref.read(syncEngineProvider);
    if (engine != null) await AgentOps.enqueueReset(engine, thread.id);
  }

  @override
  Future<void> resume() async {
    final engine = ref.read(syncEngineProvider);
    if (engine == null) return;
    final sessions = await ref.read(agentReadsProvider).sessions(thread.id);
    if (!context.mounted) return;
    String? current;
    for (final s in sessions) {
      if (s.current) current = s.id;
    }
    final choices = [
      for (final s in sessions)
        if (!s.current && s.externalSessionRef != null) s,
    ];
    final l = AppLocalizations.of(context);
    final chosen = await showDialog<AgentSessionView>(
      context: context,
      builder: (context) => SimpleDialog(
        key: const ValueKey('command.resume'),
        title: Text(l.commandResumeTitle),
        children: [
          if (choices.isEmpty)
            Padding(
              padding: const EdgeInsets.all(DsSpace.s16),
              child: Text(l.commandResumeEmpty),
            ),
          for (final s in choices)
            SimpleDialogOption(
              key: ValueKey('command.resume.${s.id}'),
              onPressed: () => Navigator.of(context).pop(s),
              child: Text(s.externalSessionRef!),
            ),
        ],
      ),
    );
    if (chosen == null) return;
    await AgentOps.enqueueBind(
      engine,
      thread.id,
      externalSessionRef: chosen.externalSessionRef!,
      expectedCurrent: current,
    );
  }

  @override
  Future<void> title(String text) async {
    final engine = ref.read(syncEngineProvider);
    if (engine == null || text.isEmpty) return;
    await ThreadActions(engine).setTitle(thread, text);
  }

  @override
  Future<void> close() async {
    final engine = ref.read(syncEngineProvider);
    if (engine != null) {
      await ThreadActions(engine).setDone(thread, reopen: false);
    }
  }
}

/// Runs [command] with [argument] on a [ThreadCommandRunner] of [thread].
void runThreadBuiltin(
  BuildContext context,
  WidgetRef ref,
  Thread thread,
  BuiltinCommand command,
  String argument,
) => unawaited(
  runBuiltin(
    ThreadCommandRunner(context: context, ref: ref, thread: thread),
    command,
    argument,
  ),
);
