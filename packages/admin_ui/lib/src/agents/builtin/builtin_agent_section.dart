// SPDX-License-Identifier: Apache-2.0

/// The built-in agent on its agent card (docs/spec/01-product.md 6.20 item 6;
/// docs/spec/26-builtin-agents.md 3.1 item 4, 4.1, 8.3, 8.4; brief A1-09 requirements 12 to
/// 16; docs/decisions.d/A1-09.md).
///
/// The section shows the hosting runner and the current definition version, or "Not set up",
/// and the actions of an owner: "Edit" (the preview of "New agent" with the current
/// definition), "Versions" (show and restore), "Change runner", "Disable", and for an agent
/// without a definition "Continue setup". Every write goes through the port of the signed-in
/// session, so step-up, `If-Match` and the server's refusals apply; the section decides no
/// permission and shows the server's answer as a sentence.
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../../common/confirm_change.dart';
import '../new_agent/new_agent_dialog.dart';
import '../new_agent/new_agent_draft.dart';
import '../new_agent/new_agent_flow.dart' show NewAgentResult;
import '../new_agent/new_agent_port.dart';
import 'agent_versions_dialog.dart';
import 'change_runner_dialog.dart';

/// The section of the built-in agent [agentId] over [host]'s port.
///
/// [onChanged] runs after every successful change (the agent card reloads), [onSwitchBilling]
/// opens the billing switch of the card for the action of `hosted_holder_mismatch` (null hides
/// that action, as on the phone), [timeZone] and [clock] format the times of the versions.
///
/// Example:
/// ```dart
/// BuiltinAgentSection(host: newAgentHost, agentId: agent.id,
///     onChanged: reload, onSwitchBilling: () => openSwitch(agent));
/// ```
class BuiltinAgentSection extends StatefulWidget {
  /// Creates the section.
  const BuiltinAgentSection({
    super.key,
    required this.host,
    required this.agentId,
    this.onChanged,
    this.onSwitchBilling,
    this.timeZone = 'UTC',
    this.clock = DateTime.now,
    this.showTitle = true,
  });

  /// The port, the key maker, the announcement interval and the layout of the dialogs.
  final NewAgentHost host;

  /// The agent.
  final String agentId;

  /// Runs after a successful change.
  final VoidCallback? onChanged;

  /// Opens the billing switch of the agent card, or null for no such action.
  final VoidCallback? onSwitchBilling;

  /// The time zone of the signed-in person.
  final String timeZone;

  /// The current time.
  final DateTime Function() clock;

  /// Whether the heading "Built-in agent" shows (the phone list names the agent instead).
  final bool showTitle;

  @override
  State<BuiltinAgentSection> createState() => _BuiltinAgentSectionState();
}

class _BuiltinAgentSectionState extends State<BuiltinAgentSection> {
  NewAgentState? _agent;
  StoredDefinition? _stored;
  NewAgentOffer _offer = const NewAgentOffer(templates: [], runners: []);
  HostingRunner? _runner;
  NewAgentContext? _context;
  NewAgentFailure? _loadFailure;
  String? _notice;
  bool _busy = false;

  NewAgentPort get _port => widget.host.port;

  @override
  void initState() {
    super.initState();
    unawaited(_load());
  }

  @override
  void didUpdateWidget(BuiltinAgentSection old) {
    super.didUpdateWidget(old);
    if (old.agentId != widget.agentId) {
      _notice = null;
      unawaited(_load());
    }
  }

  Future<void> _load() async {
    try {
      final agent = await _port.agent(widget.agentId);
      final stored = await _port.definition(widget.agentId);
      NewAgentOffer? offer;
      try {
        offer = await _port.offer();
      } on Object {
        // Without the offer the runners are unknown; the server checks every move anyway.
        offer = null;
      }
      final runner = await _runnerOf(agent, offer);
      if (!mounted) return;
      setState(() {
        _agent = agent;
        _stored = stored;
        _offer = NewAgentOffer(
          templates: offer?.templates ?? const [],
          runners: [
            ...?offer?.runners,
            if (runner != null &&
                !(offer?.runners.any((r) => r.id == runner.id) ?? false))
              runner,
          ],
        );
        _runner = runner;
        _loadFailure = null;
      });
    } on Object catch (e) {
      if (!mounted) return;
      setState(
        () => _loadFailure = e is NewAgentFailure
            ? e
            : const NewAgentFailure.offline(),
      );
    }
    try {
      final c = await _port.context();
      if (mounted) setState(() => _context = c);
    } on Object {
      // Without the context the versions name their author by ID.
    }
  }

  /// The hosting runner of [agent]: from the offer, else its card; a runner that reports no
  /// `hosts_agents` now keeps the ceiling `host`, so the server decides.
  Future<HostingRunner?> _runnerOf(
    NewAgentState agent,
    NewAgentOffer? offer,
  ) async {
    final id = agent.hostedBy;
    if (id == null) return null;
    final known = offer?.runners.where((r) => r.id == id).firstOrNull;
    if (known != null) return known;
    var name = id;
    try {
      name = (await _port.agent(id)).name;
    } on Object {
      // The ID stands for the name.
    }
    return HostingRunner(id: id, name: name, ceiling: AgentToolClass.host);
  }

  Future<void> _after(String notice) async {
    if (!mounted) return;
    setState(() => _notice = notice);
    widget.onChanged?.call();
    await _load();
  }

  Future<void> _openEditor(ExistingAgent existing) async {
    final l = AppLocalizations.of(context);
    final setup = existing.stored == null && existing.content == null;
    final NewAgentResult? result = await widget.host.openExisting(
      context,
      _offer,
      existing,
    );
    if (!mounted) return;
    if (result == null) {
      // A closed dialog may have run some steps; the card shows what exists (26 8.3).
      await _load();
      return;
    }
    final stored = await _port.definition(widget.agentId);
    await _after(
      setup
          ? l.builtinAgentSetUp(existing.agent.name)
          : l.builtinAgentSaved(existing.agent.name, stored?.version ?? 0),
    );
  }

  Future<void> _edit() async {
    final agent = _agent;
    final runner = _runner;
    if (agent == null || runner == null) return;
    await _openEditor(
      ExistingAgent(agent: agent, runner: runner, stored: _stored),
    );
  }

  Future<void> _versions() async {
    final agent = _agent;
    final runner = _runner;
    if (agent == null || runner == null) return;
    final l = AppLocalizations.of(context);
    final restored = await AgentVersionsDialog.show(
      context,
      port: _port,
      agent: agent,
      ownerNames: _context?.ownerNames ?? const {},
      timeZone: widget.timeZone,
      clock: widget.clock,
      onShow: (version) => widget.host.openExisting(
        context,
        _offer,
        ExistingAgent(
          agent: agent,
          runner: runner,
          stored: _stored,
          content: version.json,
          shownVersion: version.version,
        ),
      ),
      onRestore: (version) async {
        // The restore is edited against what the agent has now.
        final current = await _port.definition(agent.id);
        final fresh = await _port.agent(agent.id);
        if (!mounted) return false;
        final result = await widget.host.openExisting(
          context,
          _offer,
          ExistingAgent(
            agent: fresh,
            runner: runner,
            stored: current,
            content: version.json,
            restoredVersion: version.version,
          ),
        );
        return result != null;
      },
    );
    if (!mounted || !(restored ?? false)) return;
    final stored = await _port.definition(agent.id);
    await _after(l.builtinAgentSaved(agent.name, stored?.version ?? 0));
  }

  Future<void> _changeRunner() async {
    final agent = _agent;
    if (agent == null) return;
    final l = AppLocalizations.of(context);
    final outcome = await ChangeRunnerDialog.show(
      context,
      port: _port,
      agent: agent,
      runners: [
        for (final r in _offer.runners)
          if (r.id != agent.hostedBy) r,
      ],
      canSwitchBilling: widget.onSwitchBilling != null,
      fullScreen: widget.host.fullScreen,
    );
    if (!mounted || outcome == null) return;
    switch (outcome) {
      case RunnerChanged(:final runner):
        await _after(l.builtinAgentMoved(agent.name, runner.name));
      case SwitchBillingFirst():
        widget.onSwitchBilling?.call();
    }
  }

  Future<void> _disable() async {
    final agent = _agent;
    if (agent == null) return;
    final l = AppLocalizations.of(context);
    final reason = await ConfirmChange.show(
      context,
      title: l.builtinAgentDisableTitle(agent.name),
      body: l.builtinAgentDisableBody(agent.name),
      confirmLabel: l.builtinAgentDisable,
      destructive: true,
      withReason: true,
    );
    if (reason == null || !mounted) return;
    setState(() => _busy = true);
    try {
      await _port.disable(
        agent.id,
        reason: reason.isEmpty ? l.adminDefaultReason : reason,
        name: agent.name,
      );
      await _after(l.builtinAgentDisabled(agent.name));
    } on Object catch (e) {
      if (mounted) {
        setState(
          () => _notice = e is NewAgentFailure ? _failureText(l, e) : null,
        );
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _continueSetup() async {
    final agent = _agent;
    final runner = _runner;
    if (agent == null || runner == null) return;
    await _openEditor(ExistingAgent(agent: agent, runner: runner));
  }

  static String _failureText(AppLocalizations l, NewAgentFailure f) =>
      switch (f.kind) {
        NewAgentFailureKind.stepUpCancelled => l.newAgentErrStepUp,
        NewAgentFailureKind.offline => l.newAgentErrOffline,
        NewAgentFailureKind.problem => problemText(
          l,
          f.code ?? '',
          reason: f.reason,
        ),
      };

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final agent = _agent;
    final failure = _loadFailure;
    final body = dsTextStyle(DsType.body).copyWith(color: colors.textPrimary);
    final secondary = dsTextStyle(DsType.bodySm)
        .copyWith(color: colors.textSecondary);
    final notice = _notice;
    final stored = _stored;
    final children = <Widget>[
      if (widget.showTitle)
        Padding(
          padding: const EdgeInsets.only(top: DsSpace.s16, bottom: DsSpace.s4),
          child: Semantics(
            header: true,
            child: Text(
              l.builtinAgentTitle,
              style: dsTextStyle(DsType.title)
                  .copyWith(color: colors.textPrimary),
            ),
          ),
        ),
      if (notice != null)
        Padding(
          padding: const EdgeInsets.only(bottom: DsSpace.s8),
          child: DsBanner(
            key: const ValueKey('builtinAgent.notice'),
            tone: DsTone.info,
            text: notice,
            liveRegion: true,
          ),
        ),
      if (failure != null)
        DsBanner(
          key: const ValueKey('builtinAgent.error'),
          tone: DsTone.danger,
          text: _failureText(l, failure),
        )
      else if (agent == null)
        const Center(child: CircularProgressIndicator())
      else ...[
        if (_runner case final r?)
          Text(
            l.builtinAgentHost(r.name),
            key: const ValueKey('builtinAgent.host'),
            style: body,
          ),
        if (stored != null)
          Text(
            l.builtinAgentVersion(stored.version),
            key: const ValueKey('builtinAgent.version'),
            style: body,
          )
        else ...[
          Text(
            l.newAgentNotSetUp,
            key: const ValueKey('builtinAgent.notSetUp'),
            style: dsTextStyle(DsType.bodyStrong)
                .copyWith(color: colors.warningFg),
          ),
          Text(l.builtinAgentNotSetUpHint, style: secondary),
        ],
        if (!agent.active)
          Text(
            l.builtinAgentDisabledHint,
            key: const ValueKey('builtinAgent.disabled'),
            style: secondary,
          )
        else ...[
          const SizedBox(height: DsSpace.s8),
          Wrap(
            spacing: DsSpace.s8,
            runSpacing: DsSpace.s8,
            children: [
              if (stored == null)
                FilledButton(
                  key: const ValueKey('builtinAgent.continueSetup'),
                  onPressed: _busy ? null : _continueSetup,
                  child: Text(l.agentEditContinue),
                )
              else ...[
                OutlinedButton(
                  key: const ValueKey('builtinAgent.edit'),
                  onPressed: _busy ? null : _edit,
                  child: Text(l.builtinAgentEdit),
                ),
                OutlinedButton(
                  key: const ValueKey('builtinAgent.versions'),
                  onPressed: _busy ? null : _versions,
                  child: Text(l.builtinAgentVersions),
                ),
              ],
              OutlinedButton(
                key: const ValueKey('builtinAgent.changeRunner'),
                onPressed: _busy ? null : _changeRunner,
                child: Text(l.builtinAgentChangeRunner),
              ),
              OutlinedButton(
                key: const ValueKey('builtinAgent.disable'),
                onPressed: _busy ? null : _disable,
                child: Text(l.builtinAgentDisable),
              ),
            ],
          ),
        ],
      ],
    ];
    return Column(
      key: const ValueKey('builtinAgent'),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: children,
    );
  }
}
