// SPDX-License-Identifier: Apache-2.0

/// Step 1 of the pairing wizard: a new host with name, optional assignee and reason
/// (docs/spec/01-product.md 5.2 A8; 24 3.2 item 1; brief M4-37 Design note 1 and fixed
/// requirement 12).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../data/admin_repository.dart';

/// The fields of a new host and "Create and get a code": [onCreate] receives the name, the
/// assignee (null for owners only, SR10) and the reason, and runs `createHost`, whose answer is
/// the first pairing code. The button waits for a name and a reason; [busy] turns it off while
/// the call runs.
///
/// Example: `CreateHostDialog(people: people, onCreate: (n, a, r) => create(n, a, r))`.
class CreateHostDialog extends StatefulWidget {
  /// Creates the form.
  const CreateHostDialog({
    super.key,
    required this.people,
    required this.onCreate,
    this.busy = false,
  });

  /// The people an assignee is chosen from.
  final List<AdminPerson> people;

  /// Creates the host.
  final void Function(String name, String? assigneeId, String reason) onCreate;

  /// Whether a call runs.
  final bool busy;

  @override
  State<CreateHostDialog> createState() => _CreateHostDialogState();
}

class _CreateHostDialogState extends State<CreateHostDialog> {
  final TextEditingController _name = TextEditingController();
  final TextEditingController _reason = TextEditingController();
  String? _assignee;

  @override
  void initState() {
    super.initState();
    _name.addListener(() => setState(() {}));
    _reason.addListener(() => setState(() {}));
  }

  @override
  void dispose() {
    _name.dispose();
    _reason.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final ready =
        !widget.busy &&
        _name.text.trim().isNotEmpty &&
        _reason.text.trim().isNotEmpty;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        TextField(
          key: const ValueKey('createHost.name'),
          controller: _name,
          decoration: InputDecoration(labelText: l.hostCreateName),
        ),
        DropdownButtonFormField<String?>(
          key: const ValueKey('createHost.assignee'),
          initialValue: _assignee,
          decoration: InputDecoration(labelText: l.hostCreateAssignee),
          items: [
            DropdownMenuItem(child: Text(l.hostOwnersOnly)),
            for (final p in widget.people)
              DropdownMenuItem(value: p.id, child: Text(p.displayName)),
          ],
          onChanged: (v) => setState(() => _assignee = v),
        ),
        TextField(
          key: const ValueKey('createHost.reason'),
          controller: _reason,
          decoration: InputDecoration(labelText: l.adminReasonRequired),
        ),
        const SizedBox(height: DsSpace.s12),
        Align(
          alignment: Alignment.centerRight,
          child: FilledButton(
            key: const ValueKey('pairing.create'),
            onPressed: ready
                ? () => widget.onCreate(
                    _name.text.trim(),
                    _assignee,
                    _reason.text.trim(),
                  )
                : null,
            child: Text(l.hostCreateSubmit),
          ),
        ),
      ],
    );
  }
}
