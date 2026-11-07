// SPDX-License-Identifier: Apache-2.0

/// The sentences of "New agent" (docs/spec/01-product.md 6.20; docs/decisions.d/A1-08.md): the
/// step labels, one sentence for the cause of a failed step, the field errors of the server and
/// the explanation of each tool class.
library;

import 'package:l10n/l10n.dart';

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
String newAgentFailureText(
  AppLocalizations l,
  NewAgentStep step,
  NewAgentFailure failure,
) {
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
