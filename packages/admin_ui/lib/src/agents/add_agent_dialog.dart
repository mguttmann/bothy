// SPDX-License-Identifier: Apache-2.0

/// Creating the agent of an adapter: handle, name, description and the adapter kind
/// (docs/spec/11-extensions.md 13 step 1; 09 2.1; ruling AUD-AA1/5).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/confirm_change.dart';
import '../data/agents_repository.dart';

/// What [AddAgentDialog] asks for (`AgentCreate` without `hosted_by`).
final class AddAgentRequest {
  /// Creates the request.
  const AddAgentRequest({
    required this.handle,
    required this.displayName,
    required this.adapterKind,
    this.description = '',
  });

  /// The handle (`^[a-z0-9][a-z0-9._-]*$`).
  final String handle;

  /// The display name.
  final String displayName;

  /// One of [adapterKinds].
  final String adapterKind;

  /// The description, empty for none.
  final String description;
}

/// The handle pattern of `AgentCreate` (docs/api/openapi.yaml).
final RegExp _handlePattern = RegExp(r'^[a-z0-9][a-z0-9._-]*$');

/// The name of the adapter kind [kind] of [adapterKinds], or [kind] itself for another one.
String adapterKindLabel(AppLocalizations l, String kind) => switch (kind) {
  'code_runner' => l.adminAddAgentKindRunner,
  'hermes' => l.adminAddAgentKindHermes,
  'custom' => l.adminAddAgentKindCustom,
  _ => kind,
};

/// The confirmation of "Add adapter agent" with its fields ([ConfirmChange]): it says what is
/// created (an agent with the backend Organization and the tool class Chat) and that the token
/// comes next, on the agent's page. Pops an [AddAgentRequest] for "Create agent", null for
/// "Cancel".
///
/// The dialog offers the adapter kinds of [adapterKinds]; a built-in agent needs a hosting
/// runner and is created by "New agent". What the server refuses (a taken handle, a caller
/// who is no owner) is its answer to the request, shown by the list.
///
/// Example:
/// ```dart
/// final request = await AddAgentDialog.show(context);
/// if (request != null) await repository.createAgent(handle: request.handle, ...);
/// ```
class AddAgentDialog extends StatefulWidget {
  /// Creates the dialog.
  const AddAgentDialog({super.key});

  /// Shows the dialog over [context].
  static Future<AddAgentRequest?> show(BuildContext context) =>
      showDialog<AddAgentRequest>(
        context: context,
        builder: (_) => const AddAgentDialog(),
      );

  @override
  State<AddAgentDialog> createState() => _AddAgentDialogState();
}

class _AddAgentDialogState extends State<AddAgentDialog> {
  final TextEditingController _name = TextEditingController();
  final TextEditingController _handle = TextEditingController();
  final TextEditingController _description = TextEditingController();
  String? _kind;

  @override
  void initState() {
    super.initState();
    _name.addListener(_changed);
    _handle.addListener(_changed);
  }

  @override
  void dispose() {
    _name.dispose();
    _handle.dispose();
    _description.dispose();
    super.dispose();
  }

  void _changed() => setState(() {});

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final name = _name.text.trim();
    final handle = _handle.text.trim();
    final kind = _kind;
    final handleOk = _handlePattern.hasMatch(handle);
    return ConfirmChange(
      title: l.adminAgentsAdd,
      body: l.adminAddAgentBody,
      confirmLabel: l.adminAddAgentSubmit,
      canConfirm: name.isNotEmpty && handleOk && kind != null,
      onConfirm: (_) => Navigator.of(context).pop(
        AddAgentRequest(
          handle: handle,
          displayName: name,
          adapterKind: kind!,
          description: _description.text.trim(),
        ),
      ),
      fields: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          TextField(
            key: const ValueKey('addAgent.name'),
            controller: _name,
            autofocus: true,
            decoration: InputDecoration(labelText: l.newAgentName),
          ),
          TextField(
            key: const ValueKey('addAgent.handle'),
            controller: _handle,
            autocorrect: false,
            decoration: InputDecoration(
              labelText: l.newAgentHandle,
              errorText: handle.isEmpty || handleOk
                  ? null
                  : l.newAgentHandleInvalid,
            ),
          ),
          // The hint stands below the field, not in it: a text field with a child is a leaf to
          // VoiceOver on macOS, which would never read the child.
          Padding(
            padding: const EdgeInsets.only(top: DsSpace.s4),
            child: Text(
              l.newAgentHandleHint,
              style: dsTextStyle(DsType.bodySm)
                  .copyWith(color: context.dsColors.textSecondary),
            ),
          ),
          DropdownButtonFormField<String>(
            key: const ValueKey('addAgent.kind'),
            initialValue: kind,
            decoration: InputDecoration(labelText: l.adminAddAgentKind),
            items: [
              for (final k in adapterKinds)
                DropdownMenuItem(
                  key: ValueKey('addAgent.kind.$k'),
                  value: k,
                  child: Text(adapterKindLabel(l, k)),
                ),
            ],
            onChanged: (v) => setState(() => _kind = v),
          ),
          TextField(
            key: const ValueKey('addAgent.description'),
            controller: _description,
            decoration: InputDecoration(labelText: l.newAgentDescription),
          ),
        ],
      ),
    );
  }
}
