// SPDX-License-Identifier: Apache-2.0

/// The page of one agent of A5: adapter, presence, configured and reported mode, the server's
/// warnings, holder, operated targets, the backend switch and the tokens
/// (docs/spec/01-product.md 5.2 A5; 10 8, 9, 11, 13; brief M4-28 Design note 1, fixed
/// requirements 11 to 14), for a built-in agent its section with "Edit", "Versions",
/// "Change runner", "Disable" and "Continue setup" (01 6.20 item 6; brief A1-09), and for the
/// agent of an adapter its tool class, its features and "Disable" and "Enable" (07 2.11; 09
/// 2.1, 2.2; rulings AUD-AA1/6 to AUD-AA1/8).
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/admin_action.dart';
import '../common/confirm_change.dart';
import '../common/problem_banner.dart';
import '../data/access_repository.dart';
import '../data/admin_repository.dart';
import '../data/agents_repository.dart';
import '../data/self_repository.dart';
import '../grants/grant_labels.dart';
import '../hosts/host_labels.dart';
import '../self/self_feedback.dart';
import 'agent_labels.dart';
import 'agent_settings.dart';
import 'agent_tokens.dart';
import 'builtin/builtin_agent_section.dart';
import 'holder_dialog.dart';
import 'i1_explanation.dart';
import 'new_agent/new_agent_dialog.dart' show NewAgentHost;
import 'new_agent/new_agent_draft.dart' show AgentToolClass;
import 'new_agent/new_agent_port.dart' show builtinAdapterKind;
import 'new_agent/new_agent_texts.dart' show toolClassName;
import 'switch_mode_dialog.dart';

/// The agent [agentId] with every field of A5.
///
/// The page loads `getAgent`, the targets the agent operates and the people (holder names and
/// the holder picker) side by side; the agents whose names [I1Explanation] shows besides the
/// people are read only when a `409 i1_violation` arrives, and an answer for an agent the page
/// no longer shows is dropped (ruling AUD-AA1/3). The badge
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
/// With [builtin] the card of a built-in agent (`adapter_kind = 'claude_agent'`) shows the
/// [BuiltinAgentSection]; its "Switch billing" after `hosted_holder_mismatch` opens the billing
/// switch of this card.
///
/// The agent of an adapter (every other `adapter_kind`) shows its tool class with "Change
/// tool class" ([ToolClassDialog]), its features with "Edit features"
/// ([AgentFeaturesDialog], `operator_config`) and its status with "Disable"; a disabled agent
/// of either kind has "Enable", which the server may refuse with `409 i1_violation` (10 5.1).
/// These are owner actions like the backend switch: each asks a reason or shows what it
/// changes, goes through the step-up, and a `403` hides them all.
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
    this.builtin,
  });

  /// The editing of a built-in agent (A1-09), or null for none.
  final NewAgentHost? builtin;

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

  /// Counts the loads: the answer of an older one (another agent, or a load that a change
  /// replaced) is dropped.
  int _loads = 0;

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
    final id = widget.agentId;
    final load = ++_loads;
    bool current() => mounted && load == _loads;
    // The three reads do not depend on each other. The targets and the people may fail alone:
    // the page then shows the problem of the targets, and the holder by its ID.
    final targetsRead = Future.sync(() => r.operatedTargets(id))
        .then<(List<OperatedTarget>?, Object?)>(
          (targets) => (targets, null),
          onError: (Object e) => (null, e),
        );
    final peopleRead = allPeople(r)
        .then((people) => people, onError: (Object _) => const <AdminPerson>[]);
    try {
      final agent = await r.agentRecord(id);
      final (targets, targetsError) = await targetsRead;
      final people = await peopleRead;
      if (!current()) return;
      setState(() {
        _agent = agent;
        _targets = targets;
        _targetsError = targetsError;
        _people = people;
        _error = null;
      });
    } on Object catch (e) {
      if (!current()) return;
      setState(() {
        _agent = null;
        _error = e;
      });
    }
  }

  /// The names [I1Explanation] shows: the people the page has, and the agents, read only now
  /// that the server answered `409 i1_violation`. Without the list an agent shows by its ID.
  Future<AccessNames> _conflictNames() async {
    try {
      return AccessNames(
        people: _people,
        agents: await widget.repository.agents(),
      );
    } on Object {
      return AccessNames(people: _people);
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
        if (e.problem?.code == 'i1_violation') {
          final names = await _conflictNames();
          if (mounted) {
            setState(() {
              _names = names;
              _conflicts = e.conflicts;
            });
          }
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

  Future<void> _changeToolClass(AgentRecord a) async {
    final l = AppLocalizations.of(context);
    final answer = await ToolClassDialog.show(context, agent: a);
    if (answer == null || !mounted) return;
    final (toolClass, reason) = answer;
    await _run(() async {
      await widget.repository.setToolClass(
        a,
        toolClass.wire,
        reason: reason,
        purpose: l.stepUpPurposeToolClass(a.displayName),
      );
      return SelfNotice(
        l.adminAgentClassChanged(a.displayName, toolClassName(l, toolClass)),
      );
    });
  }

  Future<void> _editFeatures(AgentRecord a) async {
    final l = AppLocalizations.of(context);
    final config = await AgentFeaturesDialog.show(context, agent: a);
    if (config == null || !mounted) return;
    await _run(() async {
      await widget.repository.setOperatorConfig(
        a,
        config,
        purpose: l.stepUpPurposeAgentFeatures(a.displayName),
      );
      return SelfNotice(l.adminAgentFeaturesSaved(a.displayName));
    });
  }

  /// "Disable" and "Enable": one confirmation that says what ends or comes back, with the
  /// reason the server stores in the audit log.
  Future<void> _setActive(AgentRecord a, {required bool active}) async {
    final l = AppLocalizations.of(context);
    final name = a.displayName;
    final reason = await ConfirmChange.show(
      context,
      title: active
          ? l.adminAgentEnableTitle(name)
          : l.builtinAgentDisableTitle(name),
      body: active
          ? l.adminAgentEnableBody(name)
          : l.adminAgentDisableBody(name),
      confirmLabel: active ? l.adminAgentEnable : l.builtinAgentDisable,
      destructive: !active,
      withReason: true,
    );
    if (reason == null || !mounted) return;
    await _run(() async {
      await widget.repository.setActive(
        a,
        active: active,
        reason: reason.isEmpty ? l.adminDefaultReason : reason,
        purpose: active
            ? l.stepUpPurposeEnableAgent(name)
            : l.stepUpPurposeDisableAgent(name),
      );
      return SelfNotice(
        active ? l.adminAgentEnabled(name) : l.builtinAgentDisabled(name),
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

  /// The tool class, the features and the status of [a]. [adapter] is false for a built-in
  /// agent, whose class follows its definition ("Edit"). While its own section shows
  /// ([builtinShown]), that section has "Disable" and says that the agent is disabled, so the
  /// status here appears only with "Enable", once the agent is disabled.
  List<Widget> _administration(
    AppLocalizations l,
    AgentRecord a,
    TextStyle body,
    TextStyle secondary, {
    required bool adapter,
    required bool builtinShown,
  }) {
    final allowed = !denied.contains(_backend);
    final toolClass = AgentToolClass.of(a.toolClass);
    final off = agentFeaturesOff(a.operatorConfig);
    final model = a.operatorConfig[agentDefaultModelKey];
    return [
      if (adapter) ...[
        _heading(l.newAgentClass),
        Text(
          toolClassName(l, toolClass),
          key: const ValueKey('agentPage.toolClass'),
          style: body,
        ),
        Text(adapterToolClassExplanation(l, toolClass), style: secondary),
        if (allowed)
          Align(
            alignment: AlignmentDirectional.centerStart,
            child: OutlinedButton(
              key: const ValueKey('agentPage.toolClassChange'),
              onPressed: busy ? null : () => _changeToolClass(a),
              child: Text(l.adminAgentClassChange),
            ),
          ),
        _heading(l.adminAgentFeatures),
        Text(
          off.isEmpty
              ? l.adminAgentFeaturesAll
              : l.adminAgentFeaturesOff(
                  [for (final key in off) agentFeatureLabel(l, key)].join(', '),
                ),
          key: const ValueKey('agentPage.features'),
          style: body,
        ),
        if (model is String && model.isNotEmpty)
          Text(
            l.adminAgentDefaultModelValue(model),
            key: const ValueKey('agentPage.defaultModel'),
            style: body,
          ),
        if (allowed)
          Align(
            alignment: AlignmentDirectional.centerStart,
            child: OutlinedButton(
              key: const ValueKey('agentPage.featuresEdit'),
              onPressed: busy ? null : () => _editFeatures(a),
              child: Text(l.adminAgentFeaturesEdit),
            ),
          ),
      ],
      if (!builtinShown || !a.active) ...[
        _heading(l.adminAgentStatus),
        // A built-in agent says that it is disabled in its own section.
        if (!builtinShown)
          Text(
            a.active ? l.adminAgentStatusActive : l.builtinAgentDisabledHint,
            key: const ValueKey('agentPage.status'),
            style: body,
          ),
        if (allowed && !a.active)
          Align(
            alignment: AlignmentDirectional.centerStart,
            child: OutlinedButton(
              key: const ValueKey('agentPage.enable'),
              onPressed: busy ? null : () => _setActive(a, active: true),
              child: Text(l.adminAgentEnable),
            ),
          )
        else if (allowed && !builtinShown)
          Align(
            alignment: AlignmentDirectional.centerStart,
            child: OutlinedButton(
              key: const ValueKey('agentPage.disable'),
              onPressed: busy ? null : () => _setActive(a, active: false),
              child: Text(l.builtinAgentDisable),
            ),
          ),
      ],
    ];
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
    final adapter = a.adapterKind != builtinAdapterKind;
    final builtinShown = !adapter && widget.builtin != null;
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
      // configured mode, so the page says how to switch the adapter. The server's comparison
      // `billing_mode_matches` counts at once; its warning code follows only after
      // `billing.mismatch_alert_after` (10 8).
      if (a.runtime.modeMatches == false ||
          a.runtime.warnings.contains('billing_mode_mismatch'))
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
      if (widget.builtin case final host?
          when a.adapterKind == builtinAdapterKind)
        BuiltinAgentSection(
          key: ValueKey('agentPage.builtin.${a.id}'),
          host: host,
          agentId: a.id,
          timeZone: widget.timeZone,
          clock: widget.clock,
          onChanged: () {
            widget.onChanged?.call();
            unawaited(_load());
          },
          onSwitchBilling: denied.contains(_backend) ? null : () => _switch(a),
        ),
      ..._administration(
        l,
        a,
        body,
        secondary,
        adapter: adapter,
        builtinShown: builtinShown,
      ),
      const SizedBox(height: DsSpace.s16),
      AgentTokens(
        // Disabling revokes the agent's tokens on the server, so the list is read again when
        // the status changes.
        key: ValueKey('agentPage.tokens.${a.id}.${a.active}'),
        repository: widget.repository,
        agent: a,
        timeZone: widget.timeZone,
        clock: widget.clock,
      ),
    ];
  }
}
