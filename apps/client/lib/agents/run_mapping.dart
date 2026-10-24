// SPDX-License-Identifier: Apache-2.0

/// The pure mapping of a run to `DsRunData` (brief M4-21 Design note 1; 09 5.4; 01 6.2): the
/// open box from the run document of the `RunStore`, the collapsed line from the run summary
/// of `run.state_changed`. Nothing here reads a store or the clock; the caller passes the
/// time, the names and `ui.livebox_visible_steps` is applied by `DsLiveBox` itself.
library;

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:sync_engine/sync_engine.dart';

/// The run states of 09 5.1 that keep the live box open (`queued`, `running`,
/// `waiting_input`).
bool isActiveRun(RunState state) =>
    state == RunState.queued ||
    state == RunState.running ||
    state == RunState.waitingInput;

/// The reasons after which the person may send the triggering message again (09 5.5: no
/// automatic retry after `interrupted` and `stale`).
const Set<String> sendAgainReasons = {'interrupted', 'stale'};

/// Whether [run] ended with a reason of [sendAgainReasons].
bool offersSendAgain(RunSummary run) =>
    run.state == RunState.failed && sendAgainReasons.contains(run.stateReason);

/// [money] as `DsCost` with its suffix rule (components.md 10, 01 6.7): an estimate in
/// `organization` mode reads "Estimate", every `personal` cost "for information only".
DsCost costOf(Money money, BillingMode mode) => DsCost(
  DsMoney(money.amountMicros / 1000000, money.currency),
  mode: DsBillingMode.fromCode(mode.code),
  isEstimate: money.isEstimate,
);

/// The run time of a run: from [startedAt] (or [queuedAt] before the start) to [finishedAt],
/// or to [now] while it runs; never negative.
Duration runElapsed({
  required DateTime now,
  DateTime? queuedAt,
  DateTime? startedAt,
  DateTime? finishedAt,
  int? durationMs,
}) {
  if (durationMs != null) return Duration(milliseconds: durationMs);
  final from = startedAt ?? queuedAt;
  if (from == null) return Duration.zero;
  final to = finishedAt ?? now;
  final d = to.difference(from);
  return d.isNegative ? Duration.zero : d;
}

/// The open live box of [doc] (09 5.4): state with its reason, run time counted locally from
/// `started_at` to [now], context share, tokens, cost with its suffix, the backend badge of
/// `billing_mode_used` (falling back to `billing_mode_expected`), the status line, the last
/// text, the steps with subagents nested by depth, the stall notice while `stall_warned_at`
/// is set (09 5.5; the time since then) and the open question of `waiting_input`.
///
/// [agentName] is the display name of the run's agent, [cancelledByName] the person of
/// `user_cancelled`, [waitingForName] the person a question waits for (null: the reader),
/// [holderName] the credential holder of a `personal` run.
///
/// Example: `runFromDocument(doc, agentName: 'Robin', now: clock.now())`.
DsRunData runFromDocument(
  RunDocument doc, {
  required String agentName,
  required DateTime now,
  String? cancelledByName,
  String? waitingForName,
  String? holderName,
}) {
  final m = doc.metrics;
  final tokensIn = m.tokensIn;
  final tokensOut = m.tokensOut;
  final mode = doc.billingModeUsed ?? doc.billingModeExpected;
  final cost = doc.cost;
  final stall = doc.stallWarnedAt;
  final steps = <DsStepData>[];
  void walk(RunStepNode node, int depth) {
    final s = node.step;
    final started = s.startedAt;
    steps.add(
      DsStepData(
        id: s.id,
        kind: DsStepKind.fromCode(s.kind.code),
        name: s.name ?? s.kind.code,
        title: s.title,
        status: DsStepStatus.fromCode(s.status.code),
        duration: started == null
            ? null
            : runElapsed(
                now: now,
                startedAt: started,
                finishedAt: s.finishedAt,
              ),
        depth: depth,
      ),
    );
    for (final child in node.children) {
      walk(child, depth + 1);
    }
  }

  for (final node in doc.stepsInOrder) {
    walk(node, 0);
  }
  final state = DsRunState.fromCode(doc.state.code);
  return DsRunData(
    id: doc.id?.value ?? '',
    agentName: agentName,
    state: state,
    stateReason: doc.stateReason,
    cancelledByName: cancelledByName,
    waitingForName: waitingForName,
    elapsed: runElapsed(
      now: now,
      queuedAt: doc.queuedAt,
      startedAt: doc.startedAt,
      finishedAt: doc.finishedAt,
      durationMs: doc.durationMs,
    ),
    contextPercent: m.contextPct,
    tokens: tokensIn == null && tokensOut == null
        ? null
        : (tokensIn ?? 0) + (tokensOut ?? 0),
    cost: cost == null || mode == null ? null : costOf(cost, mode),
    billingMode: mode == null ? null : DsBillingMode.fromCode(mode.code),
    billingHolderName: mode == BillingMode.personal ? holderName : null,
    statusLine: doc.statusLine,
    lastTextExcerpt: doc.lastTextExcerpt,
    steps: steps,
    toolCount: steps.where((s) => s.kind == DsStepKind.toolCall).length,
    subagentCount: steps.where((s) => s.kind == DsStepKind.subagent).length,
    // 09 5.5: the notice shows while the server marks the stall; the box counts from then.
    stalledFor: stall == null || !state.isActive
        ? null
        : runElapsed(now: now, startedAt: stall),
    // One ID per waiting period: the announcer takes a change from null as a new question,
    // and patches during the wait keep the same ID, so they are never announced (01 8).
    questionId: state == DsRunState.waitingInput
        ? 'question.${doc.id?.value ?? ''}'
        : null,
  );
}

/// The collapsed line of an ended run from its summary (`run.state_changed`, 04 11.3 item 6):
/// state, duration, steps and cost; [steps] are the step counts per kind when the document
/// was loaded once, else the summary's `step_count` counts as tools.
DsRunData runFromSummary(
  RunSummary run, {
  required String agentName,
  required DateTime now,
  ({int tools, int subagents})? steps,
  String? cancelledByName,
}) {
  final mode = run.billingModeUsed ?? run.billingModeExpected;
  return DsRunData(
    id: run.id.value,
    agentName: agentName,
    state: DsRunState.fromCode(run.state.code),
    stateReason: run.stateReason,
    cancelledByName: cancelledByName,
    elapsed: runElapsed(
      now: now,
      queuedAt: run.queuedAt,
      startedAt: run.startedAt,
      finishedAt: run.finishedAt,
      durationMs: run.durationMs,
    ),
    cost: costOf(run.cost, mode),
    billingMode: DsBillingMode.fromCode(mode.code),
    statusLine: run.statusLine,
    lastTextExcerpt: run.summary,
    toolCount: steps?.tools ?? run.stepCount,
    subagentCount: steps?.subagents ?? 0,
  );
}

/// The step counts per kind of [doc] for the collapsed line.
({int tools, int subagents}) stepCounts(RunDocument doc) {
  var tools = 0;
  var subagents = 0;
  void walk(RunStepNode n) {
    if (n.step.kind == StepKind.toolCall) tools++;
    if (n.step.kind == StepKind.subagent) subagents++;
    n.children.forEach(walk);
  }

  doc.stepsInOrder.forEach(walk);
  return (tools: tools, subagents: subagents);
}
