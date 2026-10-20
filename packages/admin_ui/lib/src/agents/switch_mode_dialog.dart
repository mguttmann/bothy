// SPDX-License-Identifier: Apache-2.0

/// Switching the backend of one agent: the target mode, the holder for Personal and the
/// required reason (docs/spec/10-backend-policy.md 9.1; brief M4-28 fixed requirement 13).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/confirm_change.dart';
import '../data/admin_repository.dart';
import '../data/agents_repository.dart';
import 'agent_labels.dart';

/// What the person chose in [SwitchModeDialog]: the target [mode], the [holderId] for
/// Personal (null for Organization, where the server clears the holder) and the [reason].
final class SwitchModeResult {
  /// Creates the result.
  const SwitchModeResult({
    required this.mode,
    this.holderId,
    required this.reason,
  });

  /// The target mode.
  final AgentBillingMode mode;

  /// The holder with [AgentBillingMode.personal], else null.
  final String? holderId;

  /// The reason the server stores in the audit log.
  final String reason;
}

/// The confirmation of "Switch backend" for [agent]: the target mode (the other one than the
/// current is preselected), for Personal a holder among the active [people], and a reason.
///
/// "Switch" stays disabled without a reason, and for Personal without a holder (10 9.1:
/// personal needs exactly one holder; organization is always possible). Whether the switch is
/// allowed is the server's answer (`409 i1_violation`), not this dialog's.
///
/// Example:
/// ```dart
/// final choice = await SwitchModeDialog.show(context, agent: agent, people: people);
/// if (choice != null) await repository.setMode(agent, choice.mode, ...);
/// ```
class SwitchModeDialog extends StatefulWidget {
  /// Creates the dialog.
  const SwitchModeDialog({
    super.key,
    required this.agent,
    required this.people,
  });

  /// The agent to switch.
  final AgentRecord agent;

  /// The people to pick a holder from; only active ones are offered.
  final List<AdminPerson> people;

  /// Shows the dialog over [context]; returns the choice, or null for "Cancel".
  static Future<SwitchModeResult?> show(
    BuildContext context, {
    required AgentRecord agent,
    required List<AdminPerson> people,
  }) => showDialog<SwitchModeResult>(
    context: context,
    builder: (_) => SwitchModeDialog(agent: agent, people: people),
  );

  @override
  State<SwitchModeDialog> createState() => _SwitchModeDialogState();
}

class _SwitchModeDialogState extends State<SwitchModeDialog> {
  final TextEditingController _reason = TextEditingController();
  late AgentBillingMode _mode =
      widget.agent.mode == AgentBillingMode.organization
      ? AgentBillingMode.personal
      : AgentBillingMode.organization;
  String? _holder;

  @override
  void initState() {
    super.initState();
    _reason.addListener(() => setState(() {}));
  }

  @override
  void dispose() {
    _reason.dispose();
    super.dispose();
  }

  bool get _personal => _mode == AgentBillingMode.personal;

  Widget _modeChip(AppLocalizations l, AgentBillingMode mode) => ChoiceChip(
    key: ValueKey('switch.mode.${mode.wire}'),
    label: Text(agentModeLabel(l, mode)),
    selected: _mode == mode,
    onSelected: (_) => setState(() => _mode = mode),
  );

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final name = widget.agent.displayName;
    final reason = _reason.text.trim();
    return ConfirmChange(
      title: l.adminSwitchTitle(name),
      body: _personal
          ? l.adminSwitchToPersonalBody(name)
          : l.adminSwitchToOrgBody(name),
      confirmLabel: l.adminSwitchConfirm,
      fields: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          Wrap(
            spacing: DsSpace.s8,
            children: [
              _modeChip(l, AgentBillingMode.organization),
              _modeChip(l, AgentBillingMode.personal),
            ],
          ),
          if (_personal) ...[
            const SizedBox(height: DsSpace.s8),
            DropdownButtonFormField<String>(
              key: const ValueKey('switch.holder'),
              initialValue: _holder,
              decoration: InputDecoration(labelText: l.adminSwitchHolderPick),
              items: [
                for (final p in widget.people)
                  if (p.status == AdminPersonStatus.active)
                    DropdownMenuItem(
                      key: ValueKey('switch.holder.${p.id}'),
                      value: p.id,
                      child: Text(p.displayName),
                    ),
              ],
              onChanged: (v) => setState(() => _holder = v),
            ),
          ],
          // The reason is required: the server stores it with the switch in the audit log
          // (10 9.1), so "Switch" waits for it.
          TextField(
            key: const ValueKey('confirm.reason'),
            controller: _reason,
            decoration: InputDecoration(labelText: l.adminReasonRequired),
          ),
        ],
      ),
      canConfirm: reason.isNotEmpty && (!_personal || _holder != null),
      onConfirm: (_) => Navigator.of(context).pop(
        SwitchModeResult(
          mode: _mode,
          // To organization sends no holder: the server clears it (10 9.1).
          holderId: _personal ? _holder : null,
          reason: reason,
        ),
      ),
    );
  }
}
