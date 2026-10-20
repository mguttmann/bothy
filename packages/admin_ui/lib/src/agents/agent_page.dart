// SPDX-License-Identifier: Apache-2.0

/// The page of one agent of A5: adapter, presence, configured and reported mode, the server's
/// warnings, holder, operated targets, the backend switch and the tokens
/// (docs/spec/01-product.md 5.2 A5; 10 8, 9, 11, 13; brief M4-28 Design note 1, fixed
/// requirements 11 to 14).
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/admin_action.dart';
import '../common/problem_banner.dart';
import '../data/access_repository.dart';
import '../data/admin_repository.dart';
import '../data/agents_repository.dart';
import '../data/self_repository.dart';
import '../grants/grant_labels.dart';
import '../hosts/host_labels.dart';
import '../self/self_feedback.dart';
import 'agent_labels.dart';
import 'agent_tokens.dart';
import 'holder_dialog.dart';
import 'i1_explanation.dart';
import 'switch_mode_dialog.dart';

/// The agent [agentId] with every field of A5.
///
/// The page loads `getAgent`, the targets the agent operates, the people (holder names and
/// the holder picker) and the names the caller may read (for [I1Explanation]). The badge
/// shows the configured mode; the reported mode of the heartbeat stands next to it, and the
/// warnings are exactly the server's codes of 10 8 mapped to their texts: the page never
/// compares the two modes itself (15 1 principle 1). [presence] is the live presence (the
/// desktop app maps the engine's `EphemeralStore` to it; the console passes null and the
/// state comes from `getAgent`, Design note 1): its latest value replaces the shown state and,
/// when it carries one, the shown reported mode.
///
/// "Switch backend" ([SwitchModeDialog]) and, for a personal agent, "Change holder"
/// ([HolderDialog]) ask a reason and go through the step-up interceptor; a `409
/// i1_violation` shows [I1Explanation] and changes nothing, a `403` hides both controls.
/// After a successful change the page reloads the agent and runs [onChanged]; [onBack]
/// returns to the list.
///
/// Example:
/// ```dart
/// AgentPage(repository: admin, agentId: 'support', onBack: () => go(AdminLocation.agents));
/// ```
class AgentPage extends StatefulWidget {
  /// Creates the page.
  const AgentPage({
    super.key,
    required this.repository,
    required this.agentId,
    this.presence,
    this.timeZone = 'UTC',
    this.clock = DateTime.now,
    this.onBack,
    this.onChanged,
  });

  /// The administration's calls.
  final AdminRepository repository;

  /// The agent.
  final String agentId;

  /// The live presence of the agent, or null to show the presence of `getAgent`.
  final Stream<AgentLiveState>? presence;

  /// The time zone of the signed-in person.
  final String timeZone;

  /// The current time.
  final DateTime Function() clock;

  /// Returns to the list, or null for no button.
  final VoidCallback? onBack;

  /// Runs after a successful change.
  final VoidCallback? onChanged;

  @override
  State<AgentPage> createState() => _AgentPageState();
}

/// The key under which a refused change hides the switch and the holder change: both are
/// owner actions, so a `403` of one means the other is refused too.
const String _backend = 'backend';

class _AgentPageState extends State<AgentPage>
    with AdminActionState<AgentPage> {
  AgentRecord? _agent;
  Object? _error;
  List<OperatedTarget>? _targets;
  Object? _targetsError;
  List<AdminPerson> _people = const [];
  AccessNames _names = AccessNames();
  AgentLiveState? _live;
  StreamSubscription<AgentLiveState>? _presence;

  /// The conflicts of the last `409 i1_violation`, shown until the next change.
  List<InvariantFinding>? _conflicts;

  @override
  void initState() {
    super.initState();
    _listen();
    _load();
  }

  @override
  void didUpdateWidget(AgentPage old) {
    super.didUpdateWidget(old);
    if (old.presence != widget.presence) _listen();
    if (old.agentId != widget.agentId) {
      _live = null;
      _conflicts = null;
      _load();
    }
  }

  @override
  void dispose() {
    _presence?.cancel();
    super.dispose();
  }

  void _listen() {
    _presence?.cancel();
    _live = null;
    _presence = widget.presence?.listen((live) {
      if (mounted) setState(() => _live = live);
    });
  }

  Future<void> _load() async {
    final r = widget.repository;
    try {
      final agent = await r.agentRecord(widget.agentId);
      List<OperatedTarget>? targets;
      Object? targetsError;
      try {
        targets = await r.operatedTargets(widget.agentId);
      } on Object catch (e) {
        targetsError = e;
      }
      List<AdminPerson> people;
      try {
        people = await allPeople(r);
      } on Object {
        people = const [];
      }
      final names = await AccessNames.load(r);
      if (!mounted) return;
      setState(() {
        _agent = agent;
        _targets = targets;
        _targetsError = targetsError;
        _people = people;
        _names = names;
        _error = null;
      });
    } on Object catch (e) {
      if (mounted) {
        setState(() {
          _agent = null;
          _error = e;
        });
      }
    }
  }

  /// Runs one change of the backend; a `409 i1_violation` keeps its conflicts for
  /// [I1Explanation] (10 5.4), and a success reloads the agent.
  Future<void> _run(Future<SelfNotice> Function() change) async {
    setState(() => _conflicts = null);
    final ok = await runChange(_backend, () async {
      try {
        return await change();
      } on SelfActionFailed catch (e) {
        if (e.problem?.code == 'i1_violation' && mounted) {
          setState(() => _conflicts = e.conflicts);
        }
        rethrow;
      }
    });
    if (!ok || !mounted) return;
    widget.onChanged?.call();
    await _load();
  }

  Future<void> _switch(AgentRecord a) async {
    final l = AppLocalizations.of(context);
    final choice = await SwitchModeDialog.show(
      context,
      agent: a,
      people: _people,
    );
    if (choice == null || !mounted) return;
    await _run(() async {
      await widget.repository.setMode(
        a,
        choice.mode,
        holderId: choice.holderId,
        reason: choice.reason,
        purpose: l.stepUpPurposeSwitchAgent(a.displayName),
      );
      return SelfNotice(
        l.adminSwitched(a.displayName, agentModeLabel(l, choice.mode)),
      );
    });
  }

  Future<void> _changeHolder(AgentRecord a) async {
    final l = AppLocalizations.of(context);
    final answer = await HolderDialog.show(context, agent: a, people: _people);
    if (answer == null || !mounted) return;
    final (holderId, reason) = answer;
    await _run(() async {
      await widget.repository.setHolder(
        a,
        holderId,
        reason: reason,
        purpose: l.stepUpPurposeSetHolder(a.displayName),
      );
      return SelfNotice(
        l.adminHolderChanged(personName(_people, holderId), a.displayName),
      );
    });
  }

  String _stateLabel(AppLocalizations l, AgentLiveStateKind state) =>
      switch (state) {
        AgentLiveStateKind.online => l.adminAgentStateOnline,
        AgentLiveStateKind.degraded => l.adminAgentStateDegraded,
        AgentLiveStateKind.offline => l.adminAgentStateOffline,
        AgentLiveStateKind.unknown => l.adminAgentStateUnknown,
      };

  /// The last heartbeat [at] as "today 10:58" in the person's time zone (UTC when the zone is
  /// unknown), like the connection of a host.
  String _when(DateTime at) {
    final format = L10nFormat(Localizations.localeOf(context));
    String in_(String zone) =>
        '${format.relativeDay(at, now: widget.clock(), timeZone: zone)} '
        '${format.time(at, timeZone: zone)}';
    try {
      return in_(widget.timeZone);
    } on ArgumentError {
      return in_('UTC');
    }
  }

  Widget _field(String label, Widget value) {
    final colors = context.dsColors;
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: DsSpace.s8),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 160,
            child: Text(
              label,
              style: dsTextStyle(DsType.label)
                  .copyWith(color: colors.textSecondary),
            ),
          ),
          Expanded(child: value),
        ],
      ),
    );
  }

  Widget _heading(String text) => Padding(
    padding: const EdgeInsets.only(top: DsSpace.s16, bottom: DsSpace.s4),
    child: Semantics(
      header: true,
      child: Text(
        text,
        style: dsTextStyle(DsType.title)
            .copyWith(color: context.dsColors.textPrimary),
      ),
    ),
  );

  /// One warning of the server: its text and, where 10 8 names one, its consequence.
  Widget _warning(AppLocalizations l, AgentRecord a, String code) {
    final t = dsToneColors(context, DsTone.warning);
    final style = dsTextStyle(DsType.bodySm).copyWith(color: t.fg);
    final consequence = agentWarningConsequence(l, code);
    return Container(
      key: ValueKey('agentPage.warning.$code'),
      margin: const EdgeInsets.only(bottom: DsSpace.s4),
      padding: const EdgeInsets.all(DsSpace.s8),
      decoration: BoxDecoration(
        color: t.subtle,
        border: Border.all(color: t.border, width: DsBorderWidth.hairline),
        borderRadius: BorderRadius.circular(DsRadius.md),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(
            agentWarningText(l, a, code),
            style: style.copyWith(fontWeight: FontWeight.w600),
          ),
          if (consequence != null) Text(consequence, style: style),
        ],
      ),
    );
  }

  Widget _targetsSection(AppLocalizations l, TextStyle body) {
    final error = _targetsError;
    if (error != null) return ProblemBanner(problem: loadProblem(error));
    final targets = _targets ?? const [];
    if (targets.isEmpty) {
      return Text(
        l.adminAgentTargetsNone,
        key: const ValueKey('agentPage.targetsNone'),
        style: body,
      );
    }
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        for (final t in targets)
          Text(
            t.byDefault
                ? l.adminAgentTargetDefault(targetLabel(l, t.target))
                : targetLabel(l, t.target),
            key: ValueKey('agentPage.target.${t.target.id}'),
            style: body,
          ),
      ],
    );
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final a = _agent;
    final error = _error;
    final body = dsTextStyle(DsType.body).copyWith(color: colors.textPrimary);
    final secondary = dsTextStyle(DsType.bodySm)
        .copyWith(color: colors.textSecondary);
    return Column(
      key: const ValueKey('agentPage'),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        if (widget.onBack case final back?)
          Align(
            alignment: Alignment.centerLeft,
            child: TextButton(
              key: const ValueKey('agentPage.back'),
              onPressed: back,
              child: Text(l.adminAgentBack),
            ),
          ),
        if (error != null)
          ProblemBanner(problem: loadProblem(error), onRetry: _load)
        else if (a == null)
          const Center(child: CircularProgressIndicator())
        else
          ..._details(l, a, body, secondary),
      ],
    );
  }

  List<Widget> _details(
    AppLocalizations l,
    AgentRecord a,
    TextStyle body,
    TextStyle secondary,
  ) {
    final colors = context.dsColors;
    final r = a.runtime;
    final live = _live;
    // Design note 1: the live presence replaces the state and, when it carries one, the
    // reported mode; the warnings below stay the server's.
    final state = live?.state ?? r.state;
    final reported = live?.reportedMode ?? r.reportedMode;
    final holderId = a.holderId;
    final holderName = holderId == null ? null : personName(_people, holderId);
    final personal = a.mode == AgentBillingMode.personal;
    final evidence = r.evidence;
    final supported = r.modesSupported;
    final conflicts = _conflicts;
    final heartbeat = r.lastHeartbeatAt;
    final version = r.runtimeVersion;
    return [
      Row(
        children: [
          Expanded(
            child: Semantics(
              header: true,
              child: Text(
                a.displayName,
                style: dsTextStyle(DsType.title)
                    .copyWith(color: colors.textPrimary),
              ),
            ),
          ),
          AgentModeBadge(
            key: const ValueKey('agentPage.badge'),
            mode: a.mode,
            holderName: holderName,
          ),
        ],
      ),
      Text(a.handle, key: const ValueKey('agentPage.handle'), style: secondary),
      Text(
        _stateLabel(l, state),
        key: const ValueKey('agentPage.state'),
        style: secondary,
      ),
      Text(l.adminAgentAdapter(a.adapterKind), style: body),
      if (version != null)
        Text(l.adminAgentRuntimeVersion(version), style: body),
      if (heartbeat != null)
        Text(
          l.adminAgentLastHeartbeat(_when(heartbeat)),
          key: const ValueKey('agentPage.heartbeat'),
          style: body,
        ),
      const SizedBox(height: DsSpace.s8),
      noticeBanner(),
      if (conflicts != null && conflicts.isNotEmpty)
        Padding(
          padding: const EdgeInsets.only(bottom: DsSpace.s8),
          child: I1Explanation(conflicts: conflicts, names: _names),
        ),
      _field(
        l.adminAgentConfigured,
        Text(
          agentModeLabel(l, a.mode, code: a.mode.wire),
          key: const ValueKey('agentPage.configured'),
          style: body,
        ),
      ),
      _field(
        l.adminAgentReported,
        Text(
          // 10 8: an adapter that follows the configured mode per run reports none.
          reported == null
              ? l.adminAgentReportedPerRun
              : agentModeLabel(l, reported),
          key: const ValueKey('agentPage.reported'),
          style: body,
        ),
      ),
      if (personal && holderName != null)
        Text(
          l.adminAgentHolder(holderName),
          key: const ValueKey('agentPage.holderName'),
          style: body,
        ),
      Text(
        supported == null
            ? l.adminAgentSupportedNone
            : l.adminAgentSupported(
                [for (final m in supported) agentModeLabel(l, m)].join(', '),
              ),
        key: const ValueKey('agentPage.supported'),
        style: body,
      ),
      if (evidence != null)
        Text(
          l.adminAgentEvidence(evidence.method ?? evidence.keyType),
          key: const ValueKey('agentPage.evidence'),
          style: body,
        ),
      if (r.warnings.isNotEmpty) const SizedBox(height: DsSpace.s8),
      // Exactly one entry per code the server reports (10 8; 15 1 principle 1).
      for (final code in r.warnings) _warning(l, a, code),
      // 10 9.2, 11 step 5: the mismatch disappears only when the runtime reports the
      // configured mode, so the page says how to switch the adapter.
      if (r.warnings.contains('billing_mode_mismatch'))
        Text(
          l.adminAgentSwitchHint(agentModeLabel(l, a.mode, code: a.mode.wire)),
          key: const ValueKey('agentPage.switchHint'),
          style: body,
        ),
      _heading(l.adminAgentTargets),
      _targetsSection(l, body),
      _heading(l.adminAgentBackend),
      Text(l.adminAgentSwitchExplain, style: secondary),
      if (!denied.contains(_backend))
        Wrap(
          spacing: DsSpace.s8,
          children: [
            OutlinedButton(
              key: const ValueKey('agentPage.switch'),
              onPressed: busy ? null : () => _switch(a),
              child: Text(l.adminAgentSwitch),
            ),
            if (personal)
              OutlinedButton(
                key: const ValueKey('agentPage.holder'),
                onPressed: busy ? null : () => _changeHolder(a),
                child: Text(l.adminAgentHolderChange),
              ),
          ],
        ),
      const SizedBox(height: DsSpace.s16),
      AgentTokens(
        key: ValueKey('agentPage.tokens.${a.id}'),
        repository: widget.repository,
        agent: a,
        timeZone: widget.timeZone,
        clock: widget.clock,
      ),
    ];
  }
}
