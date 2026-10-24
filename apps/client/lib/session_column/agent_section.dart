// SPDX-License-Identifier: Apache-2.0

/// The agent part of the session column (01 5.1 D1; components.md 10; 01 6.7; brief M4-21
/// Design note 5): the operator agent of the thread (or the agent of its latest run) with its
/// presence, the configured backend policy with the warning when the reported mode differs,
/// model, agent session, project, context, thread cost, runs and earlier sessions.
///
/// Every value comes from a server answer or the stored run; a value the person may not read
/// is absent in that answer and its row is left out.
library;

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';

import '../agents/agent_reads.dart';
import '../agents/followups.dart';
import '../agents/providers.dart';
import '../agents/run_mapping.dart';
import '../sidebar/providers.dart';
import '../thread/providers.dart';

/// The agent part of the session column of [threadId]; empty while the thread has neither an
/// operator nor a run.
class AgentSection extends ConsumerWidget {
  /// Creates the section.
  const AgentSection({super.key, required this.threadId});

  /// The open thread.
  final ThreadId threadId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final thread = ref.watch(storedThreadProvider(threadId.value)).value;
    if (thread == null) return const SizedBox.shrink();
    final channel = ref
        .watch(storedChannelOfRouteProvider(thread.channelId.value))
        .value;
    final runs =
        ref.watch(threadRunsProvider(threadId.value)).value ?? const [];
    final latest = runs.isEmpty ? null : runs.last;
    final agentId =
        thread.operatorAgentId ??
        channel?.operator.effectiveAgentId ??
        latest?.agentId;
    if (agentId == null) return const SizedBox.shrink();
    final principals = ref.watch(principalsByIdProvider).value ?? const {};
    final name = principals[agentId]?.displayName ?? agentId.value;
    final info = ref.watch(agentInfoProvider(agentId.value)).value;
    final live = ref.watch(agentPresenceProvider(agentId.value)).value;
    final presence = live?.state ?? info?.presence;
    final reported = live?.reportedBillingMode ?? info?.reportedBillingMode;
    final sessions =
        ref.watch(agentSessionsProvider(threadId.value)).value ?? const [];
    AgentSessionView? current;
    for (final s in sessions) {
      if (s.current) current = s;
    }
    final earlier = [
      for (final s in sessions)
        if (!s.current) s,
    ];
    final active = newestActiveRun(runs);
    final doc = active == null
        ? null
        : ref.watch(runDocumentProvider(active.id.value, threadId.value)).value;
    final model = doc?.model ?? current?.model;
    final context_ = doc?.metrics.contextPct;
    final project = ref
        .watch(channelProjectProvider(thread.channelId.value))
        .value;
    final costs = ref.watch(threadCostsProvider(threadId.value)).value;
    final count = ref.watch(threadRunCountProvider(threadId.value)).value;
    return AgentSectionView(
      data: (
        name: name,
        presence: presence,
        billingMode: info?.billingMode,
        reportedBillingMode: reported,
        model: model,
        session: current == null
            ? null
            : current.externalSessionRef ?? current.id,
        project: project,
        contextPercent: context_,
        costs: costs,
        runs: count,
        earlier: [for (final s in earlier) s.externalSessionRef ?? s.id],
      ),
    );
  }
}

/// What the agent part shows; a null value leaves its row out.
typedef AgentSectionData = ({
  String name,
  AgentRuntimeState? presence,
  BillingMode? billingMode,
  BillingMode? reportedBillingMode,
  String? model,
  String? session,
  String? project,
  double? contextPercent,
  List<ThreadCost>? costs,
  ({int count, bool more})? runs,
  List<String> earlier,
});

/// The agent part drawn from [data] (no provider; the goldens use it directly).
class AgentSectionView extends StatelessWidget {
  /// Creates the view.
  const AgentSectionView({super.key, required this.data});

  /// The values.
  final AgentSectionData data;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final d = data;
    final name = d.name;
    final presence = d.presence;
    final mode = d.billingMode;
    final reported = d.reportedBillingMode;
    final model = d.model;
    final project = d.project;
    final context_ = d.contextPercent;
    final costs = d.costs;
    final count = d.runs;
    final earlier = d.earlier;
    final format = L10nFormat(Localizations.localeOf(context));
    final caption = dsTextStyle(DsType.caption)
        .copyWith(color: colors.textSecondary);
    final value = dsTextStyle(DsType.bodySm)
        .copyWith(color: colors.textPrimary);
    Widget row(String key, String label, Widget child) => Padding(
      key: ValueKey('session.agent.$key'),
      padding: const EdgeInsets.symmetric(vertical: DsSpace.s2),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Expanded(child: Text(label, style: caption)),
          const SizedBox(width: DsSpace.s8),
          Flexible(
            flex: 2,
            child: Align(
              alignment: AlignmentDirectional.centerEnd,
              child: child,
            ),
          ),
        ],
      ),
    );
    Text text(String s) => Text(
      s,
      style: value,
      overflow: TextOverflow.ellipsis,
      textAlign: TextAlign.end,
    );
    return Column(
      key: const ValueKey('session.agent'),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Row(
          children: [
            DsAvatar(name: name, origin: DsMessageOrigin.agent),
            const SizedBox(width: DsSpace.s8),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    dsIsolate(name),
                    style: dsTextStyle(DsType.label)
                        .copyWith(color: colors.textPrimary),
                  ),
                  if (presence != null)
                    Text(
                      _presenceWord(l, presence),
                      key: const ValueKey('session.agent.presence'),
                      style: caption,
                    ),
                ],
              ),
            ),
          ],
        ),
        const SizedBox(height: DsSpace.s8),
        if (mode != null)
          row(
            'backend',
            l.sessionAgentBackend,
            DsBillingBadge(
              mode: DsBillingMode.fromCode(mode.code),
              reportedMode: reported == null
                  ? null
                  : DsBillingMode.fromCode(reported.code),
            ),
          ),
        if (model != null) row('model', l.sessionAgentModel, text(model)),
        if (d.session case final session?)
          row('session', l.sessionAgentSession, text(session)),
        if (project != null)
          row('project', l.sessionAgentProject, text(project)),
        if (context_ != null)
          row('context', l.sessionAgentContext, text(format.percent(context_))),
        if (costs != null)
          for (final c in costs)
            row(
              'cost.${c.mode.code}',
              l.sessionAgentThreadCost,
              DsCostText(costOf(c.money, c.mode)),
            ),
        if (count != null)
          row(
            'runs',
            l.sessionAgentRuns,
            text(
              count.more
                  ? l.sessionAgentRunsMore(count.count)
                  : format.count(count.count),
            ),
          ),
        if (earlier.isNotEmpty) ...[
          const SizedBox(height: DsSpace.s6),
          Semantics(
            header: true,
            child: Text(
              l.sessionAgentEarlier(earlier.length),
              key: const ValueKey('session.agent.earlier'),
              style: dsTextStyle(DsType.label)
                  .copyWith(color: colors.textPrimary),
            ),
          ),
          for (final s in earlier)
            Padding(
              padding: const EdgeInsets.symmetric(vertical: DsSpace.s2),
              child: Text(s, style: caption, overflow: TextOverflow.ellipsis),
            ),
        ],
      ],
    );
  }
}

String _presenceWord(AppLocalizations l, AgentRuntimeState state) =>
    switch (state) {
      AgentRuntimeState.online => l.adminAgentStateOnline,
      AgentRuntimeState.degraded => l.adminAgentStateDegraded,
      AgentRuntimeState.offline => l.adminAgentStateOffline,
      AgentRuntimeState.unknown => l.adminAgentStateUnknown,
    };
