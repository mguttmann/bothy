// SPDX-License-Identifier: Apache-2.0

/// The words of the agent administration of A5: the mode names of 10 13, the texts of the
/// server's warning codes of 10 8 and the badge of a mode (docs/spec/01-product.md 5.2 A5;
/// brief M4-28 fixed requirements 11 and 12).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/widgets.dart';
import 'package:l10n/l10n.dart';

import '../data/agents_repository.dart';

/// The name of [mode] ("Personal", "Organization"; 10 13), or the raw [code] of a value this
/// build does not know.
String agentModeLabel(
  AppLocalizations l,
  AgentBillingMode mode, {
  String code = '',
}) => switch (mode) {
  AgentBillingMode.personal => l.billingPersonal,
  AgentBillingMode.organization => l.billingOrganization,
  AgentBillingMode.unknown => l.billingUnknown(code),
};

/// The text of the server's warning [code] of [agent] (10 8). The screen maps the code only;
/// it never compares the modes itself (15 1 principle 1). An unknown code shows as it is.
String agentWarningText(AppLocalizations l, AgentRecord agent, String code) =>
    switch (code) {
      'billing_mode_mismatch' => l.adminAgentWarnMismatch(
        agentModeLabel(
          l,
          agent.runtime.reportedMode ?? AgentBillingMode.unknown,
        ),
        agentModeLabel(l, agent.mode),
      ),
      'billing_mode_unsupported' => l.adminAgentWarnUnsupported,
      'billing_evidence_unknown' => l.adminAgentWarnEvidenceUnknown,
      'tool_class_mismatch' => l.adminAgentWarnToolClass,
      _ => l.adminAgentWarnUnknown(code),
    };

/// The consequence of the warning [code] (10 8: a mismatch or an unprovable mode makes runs
/// fail until the adapter is switched; an unknown login type makes every run fail), or null
/// for a code without one.
String? agentWarningConsequence(AppLocalizations l, String code) =>
    switch (code) {
      'billing_mode_mismatch' ||
      'billing_mode_unsupported' => l.adminAgentWarnFailUntilSwitched,
      'billing_evidence_unknown' => l.adminAgentWarnFailAll,
      _ => null,
    };

/// The badge "Personal" or "Organization" with icon and text of the configured [mode]
/// (10 13, components.md 10). [holderName] names the holder of a personal agent.
///
/// The reported mode is deliberately not passed to `DsBillingBadge`: the badge would compare
/// it itself, and the warnings of A5 come from the server only (brief M4-28 Design note 1).
///
/// Example: `AgentModeBadge(mode: AgentBillingMode.personal, holderName: 'Alex')`.
class AgentModeBadge extends StatelessWidget {
  /// Creates the badge.
  const AgentModeBadge({super.key, required this.mode, this.holderName});

  /// The configured mode.
  final AgentBillingMode mode;

  /// The name of the holder with personal, or null.
  final String? holderName;

  @override
  Widget build(BuildContext context) => DsBillingBadge(
    mode: switch (mode) {
      AgentBillingMode.personal => DsBillingMode.personal,
      AgentBillingMode.organization => DsBillingMode.organization,
      AgentBillingMode.unknown => DsBillingMode.unknown,
    },
    holderName: holderName,
    code: mode.wire,
  );
}
