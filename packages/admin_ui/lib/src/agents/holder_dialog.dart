// SPDX-License-Identifier: Apache-2.0

/// Changing the credential holder of a personal agent (docs/spec/10-backend-policy.md 9.1:
/// `agent.credential_holder.set`; brief M4-28 Scope).
library;

import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/confirm_change.dart';
import '../data/admin_repository.dart';
import '../data/agents_repository.dart';

/// The confirmation of "Change holder" for the personal [agent]: a new holder among the
/// active [people] (the current holder is not offered) and a required reason.
///
/// Whether the new holder keeps I1 is the server's answer (`409 i1_violation`), not this
/// dialog's.
///
/// Example:
/// ```dart
/// final answer = await HolderDialog.show(context, agent: agent, people: people);
/// if (answer case (final holderId, final reason)) { ... }
/// ```
class HolderDialog extends StatefulWidget {
  /// Creates the dialog.
  const HolderDialog({super.key, required this.agent, required this.people});

  /// The personal agent.
  final AgentRecord agent;

  /// The people to pick the new holder from; only active ones are offered.
  final List<AdminPerson> people;

  /// Shows the dialog over [context]; returns (new holder ID, reason), or null for "Cancel".
  static Future<(String holderId, String reason)?> show(
    BuildContext context, {
    required AgentRecord agent,
    required List<AdminPerson> people,
  }) => showDialog<(String, String)>(
    context: context,
    builder: (_) => HolderDialog(agent: agent, people: people),
  );

  @override
  State<HolderDialog> createState() => _HolderDialogState();
}

class _HolderDialogState extends State<HolderDialog> {
  final TextEditingController _reason = TextEditingController();
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

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final name = widget.agent.displayName;
    final reason = _reason.text.trim();
    final holder = _holder;
    return ConfirmChange(
      title: l.adminHolderTitle(name),
      body: l.adminHolderBody(name),
      confirmLabel: l.adminAgentHolderChange,
      fields: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          DropdownButtonFormField<String>(
            key: const ValueKey('holder.person'),
            initialValue: holder,
            decoration: InputDecoration(labelText: l.adminSwitchHolderPick),
            items: [
              for (final p in widget.people)
                if (p.status == AdminPersonStatus.active &&
                    p.id != widget.agent.holderId)
                  DropdownMenuItem(
                    key: ValueKey('holder.person.${p.id}'),
                    value: p.id,
                    child: Text(p.displayName),
                  ),
            ],
            onChanged: (v) => setState(() => _holder = v),
          ),
          // The server stores the reason with the change in the audit log, so it is required.
          TextField(
            key: const ValueKey('confirm.reason'),
            controller: _reason,
            decoration: InputDecoration(labelText: l.adminReasonRequired),
          ),
        ],
      ),
      canConfirm: reason.isNotEmpty && holder != null,
      onConfirm: (_) => Navigator.of(context).pop((holder!, reason)),
    );
  }
}
