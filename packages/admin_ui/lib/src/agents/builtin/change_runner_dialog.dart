// SPDX-License-Identifier: Apache-2.0

/// "Change runner" of a built-in agent (docs/spec/26-builtin-agents.md 3.1 items 3 and 4, 8.4;
/// docs/spec/10-backend-policy.md 10.3; brief A1-09 requirement 14).
///
/// The dialog offers the runners that report `hosts_agents` with the highest class each loads,
/// says that the agent's connections restart, and moves the agent with `agent.host.set`
/// (`If-Match`, step-up). The server's refusals stay in the dialog as sentences with their way
/// out: `host_tool_class_ceiling` (choose another runner or lower the class first),
/// `hosted_holder_mismatch` ("Switch billing" on the agent card, or another runner) and
/// `host_not_capable` (choose another runner).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../new_agent/new_agent_port.dart';
import '../new_agent/new_agent_texts.dart';

/// What "Change runner" ended with.
sealed class ChangeRunnerOutcome {
  const ChangeRunnerOutcome();
}

/// The agent moved to [runner].
final class RunnerChanged extends ChangeRunnerOutcome {
  /// Creates the outcome.
  const RunnerChanged(this.runner);

  /// The new hosting runner.
  final HostingRunner runner;
}

/// The person chose "Switch billing" after `hosted_holder_mismatch`.
final class SwitchBillingFirst extends ChangeRunnerOutcome {
  /// Creates the outcome.
  const SwitchBillingFirst();
}

/// The dialog "Change runner" of [agent] with the choices [runners] (the current runner left
/// out); [canSwitchBilling] offers "Switch billing" after `hosted_holder_mismatch`.
///
/// Example:
/// ```dart
/// final outcome = await ChangeRunnerDialog.show(context, port: port, agent: agent,
///     runners: offer.runners, canSwitchBilling: true);
/// ```
class ChangeRunnerDialog extends StatefulWidget {
  /// Creates the dialog.
  const ChangeRunnerDialog({
    super.key,
    required this.port,
    required this.agent,
    required this.runners,
    this.canSwitchBilling = false,
  });

  /// The server side.
  final NewAgentPort port;

  /// The agent to move.
  final NewAgentState agent;

  /// The runners it may move to.
  final List<HostingRunner> runners;

  /// Whether "Switch billing" is offered.
  final bool canSwitchBilling;

  /// Shows the dialog; null when it was cancelled. [fullScreen] is the phone layout.
  static Future<ChangeRunnerOutcome?> show(
    BuildContext context, {
    required NewAgentPort port,
    required NewAgentState agent,
    required List<HostingRunner> runners,
    bool canSwitchBilling = false,
    bool fullScreen = false,
  }) => showDialog<ChangeRunnerOutcome>(
    context: context,
    useSafeArea: !fullScreen,
    builder: (_) => ChangeRunnerDialog(
      port: port,
      agent: agent,
      runners: runners,
      canSwitchBilling: canSwitchBilling,
    ),
  );

  @override
  State<ChangeRunnerDialog> createState() => _ChangeRunnerDialogState();
}

class _ChangeRunnerDialogState extends State<ChangeRunnerDialog> {
  HostingRunner? _choice;
  String? _error;
  bool _holder = false;
  bool _busy = false;

  @override
  void initState() {
    super.initState();
    if (widget.runners.length == 1) _choice = widget.runners.single;
  }

  Future<void> _move(AppLocalizations l) async {
    final runner = _choice;
    if (runner == null) return;
    setState(() {
      _busy = true;
      _error = null;
      _holder = false;
    });
    try {
      // The agent's version for `If-Match` changes with every write; read it fresh.
      final fresh = await widget.port.agent(widget.agent.id);
      await widget.port.setHost(fresh, runner.id, name: widget.agent.name);
      if (mounted) Navigator.of(context).pop(RunnerChanged(runner));
    } on Object catch (e) {
      if (!mounted) return;
      final f = e is NewAgentFailure ? e : const NewAgentFailure.offline();
      setState(() {
        _holder = f.reason == 'hosted_holder_mismatch';
        _error = _sentence(l, f, runner);
      });
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  String _sentence(AppLocalizations l, NewAgentFailure f, HostingRunner to) {
    switch (f.kind) {
      case NewAgentFailureKind.stepUpCancelled:
        return l.newAgentErrStepUp;
      case NewAgentFailureKind.offline:
        return l.newAgentErrOffline;
      case NewAgentFailureKind.problem:
        if (f.reason == 'host_tool_class_ceiling') {
          return l.agentHostErrCeiling(to.name, toolClassName(l, to.ceiling));
        }
        if (f.reason == 'hosted_holder_mismatch') return l.agentHostErrHolder;
        if (f.errors.any((e) => e.code == 'host_not_capable')) {
          return l.agentHostErrNotCapable;
        }
        if (f.code == 'version_conflict') return l.newAgentErrVersion;
        return problemText(l, f.code ?? '', reason: f.reason);
    }
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final error = _error;
    return AlertDialog(
      key: const ValueKey('agentHost.dialog'),
      title: Text(l.agentHostTitle(widget.agent.name)),
      content: SizedBox(
        width: 520,
        child: SingleChildScrollView(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(l.agentHostBody, style: dsTextStyle(DsType.body)),
              const SizedBox(height: DsSpace.s8),
              if (widget.runners.isEmpty)
                Text(
                  l.agentHostNone,
                  key: const ValueKey('agentHost.none'),
                  style: dsTextStyle(DsType.bodySm)
                      .copyWith(color: colors.textSecondary),
                ),
              RadioGroup<String>(
                groupValue: _choice?.id,
                onChanged: (id) => _busy
                    ? null
                    : setState(() {
                        _choice = widget.runners.firstWhere((r) => r.id == id);
                        _error = null;
                        _holder = false;
                      }),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    for (final r in widget.runners)
                      RadioListTile<String>(
                        key: ValueKey('agentHost.runner.${r.id}'),
                        contentPadding: EdgeInsets.zero,
                        value: r.id,
                        title: Text(
                          l.agentHostChoice(
                            r.name,
                            toolClassName(l, r.ceiling),
                          ),
                        ),
                        subtitle: widget.agent.toolClass.isAbove(r.ceiling)
                            ? Text(
                                l.agentHostErrCeiling(
                                  r.name,
                                  toolClassName(l, r.ceiling),
                                ),
                                style: dsTextStyle(DsType.bodySm)
                                    .copyWith(color: colors.warningFg),
                              )
                            : null,
                      ),
                  ],
                ),
              ),
              if (error != null)
                Padding(
                  padding: const EdgeInsets.only(top: DsSpace.s8),
                  child: Semantics(
                    liveRegion: true,
                    child: Text(
                      error,
                      key: const ValueKey('agentHost.error'),
                      style: dsTextStyle(DsType.body)
                          .copyWith(color: colors.dangerFg),
                    ),
                  ),
                ),
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
          key: const ValueKey('agentHost.cancel'),
          onPressed: _busy ? null : () => Navigator.of(context).pop(),
          child: Text(l.newAgentCancel),
        ),
        if (_holder && widget.canSwitchBilling)
          OutlinedButton(
            key: const ValueKey('agentHost.switchBilling'),
            onPressed: () =>
                Navigator.of(context).pop(const SwitchBillingFirst()),
            child: Text(l.agentHostSwitchBilling),
          ),
        FilledButton(
          key: const ValueKey('agentHost.confirm'),
          onPressed: _busy || _choice == null ? null : () => _move(l),
          child: Text(l.agentHostConfirm),
        ),
      ],
    );
  }
}
