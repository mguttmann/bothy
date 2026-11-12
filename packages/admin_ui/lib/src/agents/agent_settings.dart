// SPDX-License-Identifier: Apache-2.0

/// The tool class and the features of an adapter agent: their words, and the two dialogs that
/// change them (docs/spec/07-permissions.md 2.11, SR2; 09 2.2; rulings AUD-AA1/7 and
/// AUD-AA1/8).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/confirm_change.dart';
import '../data/agents_repository.dart';
import 'new_agent/new_agent_draft.dart' show AgentToolClass;
import 'new_agent/new_agent_texts.dart' show toolClassName;

/// What the runtime of an adapter agent with the class [c] may do (07 2.11) and, for `host`,
/// who may start it (SR2).
String adapterToolClassExplanation(AppLocalizations l, AgentToolClass c) =>
    switch (c) {
      AgentToolClass.chat => l.adminAgentClassChatExplain,
      AgentToolClass.scoped => l.adminAgentClassScopedExplain,
      AgentToolClass.host => l.adminAgentClassHostExplain,
    };

/// The name of the feature switch [key] of [agentFeatureKeys], or [key] itself for another
/// one.
String agentFeatureLabel(AppLocalizations l, String key) => switch (key) {
  'supports_followups' => l.adminAgentFeatureFollowups,
  'supports_components' => l.adminAgentFeatureComponents,
  'supports_voice' => l.adminAgentFeatureVoice,
  'supports_title' => l.adminAgentFeatureTitle,
  'supports_files_out' => l.adminAgentFeatureFilesOut,
  _ => key,
};

/// The features of [agentFeatureKeys] that [config] switches off (`false`; a missing key
/// allows, 09 2.2).
List<String> agentFeaturesOff(Map<String, Object?> config) => [
  for (final key in agentFeatureKeys)
    if (config[key] == false) key,
];

/// The dialog of "Change tool class" for the adapter [agent]: the three classes with what the
/// runtime may do in each, and a required reason. Pops the chosen class and the reason, null
/// for "Cancel"; the class the agent already has cannot be confirmed.
///
/// Whether the change is allowed is the server's answer (owner, step-up); the adapter starts
/// no run while it reports a higher class than the one set here (07 2.11).
///
/// Example:
/// ```dart
/// final answer = await ToolClassDialog.show(context, agent: agent);
/// if (answer case (final toolClass, final reason)) { ... }
/// ```
class ToolClassDialog extends StatefulWidget {
  /// Creates the dialog.
  const ToolClassDialog({super.key, required this.agent});

  /// The agent.
  final AgentRecord agent;

  /// Shows the dialog over [context]; returns (class, reason), or null for "Cancel".
  static Future<(AgentToolClass toolClass, String reason)?> show(
    BuildContext context, {
    required AgentRecord agent,
  }) => showDialog<(AgentToolClass, String)>(
    context: context,
    builder: (_) => ToolClassDialog(agent: agent),
  );

  @override
  State<ToolClassDialog> createState() => _ToolClassDialogState();
}

class _ToolClassDialogState extends State<ToolClassDialog> {
  final TextEditingController _reason = TextEditingController();
  late final AgentToolClass _current = AgentToolClass.of(
    widget.agent.toolClass,
  );
  late AgentToolClass _choice = _current;

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
    final reason = _reason.text.trim();
    return ConfirmChange(
      title: l.adminAgentClassTitle(widget.agent.displayName),
      body: l.adminAgentClassBody,
      confirmLabel: l.adminAgentClassChange,
      canConfirm: reason.isNotEmpty && _choice != _current,
      onConfirm: (_) => Navigator.of(context).pop((_choice, reason)),
      fields: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          RadioGroup<AgentToolClass>(
            groupValue: _choice,
            onChanged: (c) => setState(() => _choice = c ?? _choice),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                for (final c in AgentToolClass.values)
                  RadioListTile<AgentToolClass>(
                    key: ValueKey('toolClass.${c.wire}'),
                    contentPadding: EdgeInsets.zero,
                    value: c,
                    title: Text(toolClassName(l, c)),
                    subtitle: Text(adapterToolClassExplanation(l, c)),
                  ),
              ],
            ),
          ),
          // The server stores the reason with the change in the audit log, so it is required.
          TextField(
            key: const ValueKey('confirm.reason'),
            controller: _reason,
            decoration: InputDecoration(labelText: l.adminReasonRequired),
          ),
        ],
      ),
    );
  }
}

/// The dialog of "Edit features" for the adapter [agent]: one switch per feature of
/// [agentFeatureKeys] (on allows, off stores `false`) and the default model. Pops the new
/// `operator_config`, null for "Cancel".
///
/// The popped object is the stored one with these members replaced: an allowed feature and an
/// empty model drop their key (a missing key allows, and without a model the adapter chooses,
/// 09 2.2), and a member this build does not know stays as it is.
///
/// Example:
/// ```dart
/// final config = await AgentFeaturesDialog.show(context, agent: agent);
/// if (config != null) await repository.setOperatorConfig(agent, config, purpose: purpose);
/// ```
class AgentFeaturesDialog extends StatefulWidget {
  /// Creates the dialog.
  const AgentFeaturesDialog({super.key, required this.agent});

  /// The agent.
  final AgentRecord agent;

  /// Shows the dialog over [context]; returns the new configuration, or null for "Cancel".
  static Future<Map<String, Object?>?> show(
    BuildContext context, {
    required AgentRecord agent,
  }) => showDialog<Map<String, Object?>>(
    context: context,
    builder: (_) => AgentFeaturesDialog(agent: agent),
  );

  @override
  State<AgentFeaturesDialog> createState() => _AgentFeaturesDialogState();
}

class _AgentFeaturesDialogState extends State<AgentFeaturesDialog> {
  late final TextEditingController _model = TextEditingController(
    text: switch (widget.agent.operatorConfig[agentDefaultModelKey]) {
      final String model => model,
      _ => '',
    },
  );
  late final Set<String> _off = agentFeaturesOff(widget.agent.operatorConfig)
      .toSet();

  @override
  void initState() {
    super.initState();
    _model.addListener(() => setState(() {}));
  }

  @override
  void dispose() {
    _model.dispose();
    super.dispose();
  }

  Map<String, Object?> _config(String model) => {
    for (final MapEntry(:key, :value) in widget.agent.operatorConfig.entries)
      if (key != agentDefaultModelKey && !agentFeatureKeys.contains(key))
        key: value,
    if (model.isNotEmpty) agentDefaultModelKey: model,
    for (final key in agentFeatureKeys)
      if (_off.contains(key)) key: false,
  };

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final model = _model.text.trim();
    final modelOk = model.isEmpty || agentDefaultModelPattern.hasMatch(model);
    return ConfirmChange(
      title: l.adminAgentFeaturesTitle(widget.agent.displayName),
      body: l.adminAgentFeaturesExplain,
      confirmLabel: l.agentEditSave,
      canConfirm: modelOk,
      onConfirm: (_) => Navigator.of(context).pop(_config(model)),
      fields: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          for (final key in agentFeatureKeys)
            SwitchListTile(
              key: ValueKey('features.$key'),
              contentPadding: EdgeInsets.zero,
              value: !_off.contains(key),
              title: Text(agentFeatureLabel(l, key)),
              onChanged: (on) => setState(() {
                if (on) {
                  _off.remove(key);
                } else {
                  _off.add(key);
                }
              }),
            ),
          TextField(
            key: const ValueKey('features.defaultModel'),
            controller: _model,
            autocorrect: false,
            decoration: InputDecoration(
              labelText: l.adminAgentDefaultModel,
              errorText: modelOk ? null : l.adminAgentDefaultModelInvalid,
            ),
          ),
          // Below the field, not in it: a text field with a child is a leaf to VoiceOver on
          // macOS, which would never read the child.
          Padding(
            padding: const EdgeInsets.only(top: DsSpace.s4),
            child: Text(
              l.adminAgentDefaultModelHint,
              style: dsTextStyle(DsType.bodySm)
                  .copyWith(color: context.dsColors.textSecondary),
            ),
          ),
        ],
      ),
    );
  }
}
