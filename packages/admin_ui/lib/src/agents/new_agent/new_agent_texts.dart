// SPDX-License-Identifier: Apache-2.0

/// The sentences of "New agent" (docs/spec/01-product.md 6.20; docs/decisions.d/A1-08.md,
/// A1-09.md): the step labels, one sentence for the cause of a failed step, the field errors
/// of the server, the explanation of each tool class and the changed parts of an edit.
library;

import 'package:api_client/api_client.dart' show ApiClient;
import 'package:l10n/l10n.dart';

import 'agent_edit_changes.dart';
import 'new_agent_draft.dart';
import 'new_agent_flow.dart';
import 'new_agent_port.dart';

/// The name of [c] in the person's language.
String toolClassName(AppLocalizations l, AgentToolClass c) => switch (c) {
  AgentToolClass.chat => l.newAgentClassChat,
  AgentToolClass.scoped => l.newAgentClassScoped,
  AgentToolClass.host => l.newAgentClassHost,
};

/// What [c] does and does not load (26 4.4 items 3 and 6, 5.7).
String toolClassExplanation(AppLocalizations l, AgentToolClass c) =>
    switch (c) {
      AgentToolClass.chat => l.newAgentClassChatExplain,
      AgentToolClass.scoped => l.newAgentClassScopedExplain,
      AgentToolClass.host => l.newAgentClassHostExplain,
    };

/// The label of [step] in [flow].
String newAgentStepLabel(
  AppLocalizations l,
  NewAgentFlow flow,
  NewAgentStep step,
) {
  final d = flow.draft;
  return switch (step) {
    NewAgentStep.createAgent => l.newAgentStepCreate(flow.runner?.name ?? ''),
    NewAgentStep.setClass => l.newAgentStepClass(
      toolClassName(l, flow.targetClass),
    ),
    NewAgentStep.storeDefinition => l.newAgentStepDefinition,
    NewAgentStep.setBilling => l.newAgentStepBilling(flow.billing),
    NewAgentStep.createChannel => l.newAgentStepChannel(d?.channelName ?? ''),
    NewAgentStep.setOperator => l.newAgentStepOperator,
  };
}

/// One sentence for the cause of [failure] at [step] (01 6.20 item 4; brief A1-08 Design note
/// 3: `host_tool_class_ceiling` after `agent.create` points to "Close" and "Change runner").
/// In [NewAgentMode.edit] a `version_conflict` says that another device changed the agent
/// and points to "Reload" (brief A1-09 requirement 12).
String newAgentFailureText(
  AppLocalizations l,
  NewAgentStep step,
  NewAgentFailure failure, {
  NewAgentMode mode = NewAgentMode.create,
}) {
  switch (failure.kind) {
    case NewAgentFailureKind.stepUpCancelled:
      return l.newAgentErrStepUp;
    case NewAgentFailureKind.offline:
      return l.newAgentErrOffline;
    case NewAgentFailureKind.problem:
      final code = failure.code ?? '';
      final reason = failure.reason;
      if (failure.errors.any((e) => e.code == 'tool_class_too_low')) {
        return l.newAgentErrClassTooLow;
      }
      if (code == 'validation_failed') return l.newAgentErrFields;
      return switch ((reason, step)) {
        ('host_tool_class_ceiling', _) => l.newAgentErrCeiling,
        ('definition_needs_host', _) => l.newAgentErrNeedsHost,
        ('hosted_holder_mismatch', _) => l.newAgentErrHolder,
        (_, NewAgentStep.createAgent) when code == 'conflict' =>
          l.newAgentErrHandleTaken,
        (_, NewAgentStep.createChannel) when code == 'conflict' =>
          l.newAgentErrChannelTaken,
        _ when code == 'version_conflict' && mode == NewAgentMode.edit =>
          l.agentEditErrVersion,
        _ when code == 'version_conflict' => l.newAgentErrVersion,
        _ => problemText(l, code, reason: reason),
      };
  }
}

/// The sentence of a field error [code] of the server (26 4.3, 7.4; 06 5); [pointer] picks
/// the handle rule for a `pattern_mismatch` on the handle.
String newAgentFieldErrorText(
  AppLocalizations l,
  String code, {
  String? pointer,
}) => pointer == PreviewField.handle && code == 'pattern_mismatch'
    ? l.newAgentHandleInvalid
    : l.newAgentFieldError(_errorKey(code), code);

String _errorKey(String code) => switch (code) {
  'too_many_items' => 'too_many',
  'pattern_mismatch' => 'invalid_name',
  final c => c,
};

/// The sentence why `personal` is not offered (10 10.3 item 3).
String personalOfferText(AppLocalizations l, PersonalOffer offer) =>
    l.newAgentPersonalNot(offer.name);

/// The sentence of an interview fallback [reason] (26 6.1 item 4).
String interviewFallbackText(AppLocalizations l, String reason) =>
    l.newAgentInterviewFallback(reason);

/// The line of one changed part [change] of an edit (brief A1-09 Design note 2).
///
/// Example: `agentEditChangeText(l, const DefinitionChange(DefinitionChangeKind.instructions))`
/// is "Instructions changed".
String agentEditChangeText(AppLocalizations l, DefinitionChange change) {
  final kind = change.kind;
  String className(String wire) => toolClassName(l, AgentToolClass.of(wire));
  return switch (kind) {
    DefinitionChangeKind.toolClass => l.agentEditChange(
      'toolClass',
      className(change.name),
      className(change.from),
    ),
    DefinitionChangeKind.setting => l.agentEditChange(
      'setting',
      switch (change.name) {
        'model' => l.newAgentModel,
        'allowed_tools' => l.newAgentAllowedTools,
        'max_cost_usd_per_run' => l.newAgentCostCap,
        _ => l.agentEditWish,
      },
      '',
    ),
    _ => l.agentEditChange(kind.name, change.name, ''),
  };
}

/// The step-up purpose of a write of [change] for the agent [name] in the person's language.
String agentChangePurpose(
  AppLocalizations l,
  AgentChangeKind change,
  String name,
) => switch (change) {
  AgentChangeKind.create => l.stepUpPurposeNewAgent(name),
  AgentChangeKind.setup => l.stepUpPurposeSetUpAgent(name),
  AgentChangeKind.edit => l.stepUpPurposeEditAgent(name),
  AgentChangeKind.host => l.stepUpPurposeMoveAgent(name),
  AgentChangeKind.disable => l.stepUpPurposeDisableAgent(name),
};

/// The audit reason of a write of [change] for the agent [name]; "Disable" sends the reason
/// the person typed instead.
String agentChangeReason(
  AppLocalizations l,
  AgentChangeKind change,
  String name,
) => switch (change) {
  AgentChangeKind.create => l.newAgentAuditReason(name),
  AgentChangeKind.setup => l.agentSetupAuditReason(name),
  AgentChangeKind.host => l.agentMoveAuditReason(name),
  AgentChangeKind.edit ||
  AgentChangeKind.disable => l.agentEditAuditReason(name),
};

/// The [ApiNewAgentPort] over [api] with the purposes and reasons of every
/// [AgentChangeKind] in the person's language; the desktop, the phones and the console use it,
/// so the step-up prompt says what is changed.
///
/// Example: `final port = localizedAgentPort(session.api, AppLocalizations.of(context));`
ApiNewAgentPort localizedAgentPort(ApiClient api, AppLocalizations l) =>
    ApiNewAgentPort(
      api,
      purpose: l.stepUpPurposeNewAgent,
      reason: l.newAgentAuditReason,
      purposeOf: (change, name) => agentChangePurpose(l, change, name),
      reasonOf: (change, name) => agentChangeReason(l, change, name),
    );
