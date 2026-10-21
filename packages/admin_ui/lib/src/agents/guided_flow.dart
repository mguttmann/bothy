// SPDX-License-Identifier: Apache-2.0

/// The guided flow "grant a person access in a personal space": the explanation of an I1
/// conflict with the offer to switch the agent to organization, the reason of that switch and
/// the result afterwards (docs/spec/10-backend-policy.md 11, BR-07; 05 rule 1.6; brief M4-28
/// Design note 2 and fixed requirements 10 and 15).
///
/// The switch and the change that met I1 travel in ONE request with `resolve_i1`
/// ([I1Resolution]); the server runs both in one transaction, so both happen or none. The
/// screens built here never send the switch on its own.
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/confirm_change.dart';
import '../common/problem_banner.dart';
import '../data/access_repository.dart';
import '../data/admin_repository.dart';
import '../data/agents_repository.dart';
import '../grants/grant_labels.dart' show AccessNames;
import 'agent_labels.dart';
import 'i1_explanation.dart';

/// The entries of [conflicts] with the code `i1_violation` and an agent: the ones a
/// `resolve_i1` can settle (10 5.4).
List<InvariantFinding> _i1(List<InvariantFinding> conflicts) => [
  for (final c in conflicts)
    if (c.code == 'i1_violation' && c.agentId != null) c,
];

/// The agent IDs of the I1 entries of [conflicts], each once and in the server's order: the
/// list `switch_to_organization` of `resolve_i1` (10 11 step 4).
///
/// Example: `I1Resolution(guidedAgentIds(failure.conflicts), reason: reason)`.
List<String> guidedAgentIds(List<InvariantFinding> conflicts) => [
  ...{for (final c in _i1(conflicts)) c.agentId!},
];

/// The names of the agents of [conflicts] joined with ", ", from [names] where the caller may
/// read them, else the ID (as `I1Explanation` names an agent).
String guidedAgentNames(AccessNames names, List<InvariantFinding> conflicts) =>
    [for (final id in guidedAgentIds(conflicts)) readableName(names, id) ?? id]
        .join(', ');

/// Step 2 of 10 11 for the I1 entries of [conflicts]: per agent "This grant (change) allows
/// [person] to trigger the agent ... it uses the personal subscription of ... it must first be
/// switched to Organization", the sentence of an adapter that does not report an organization
/// key yet when `organization_ready` is false, and one button that takes the guided option.
///
/// [grant] chooses the words of the grant wizard ("... and grant") over those of adding a
/// group member or an owner ("... and continue"). [onTake] runs on the button; null disables
/// it (for example while a change runs). The widget decides nothing: the conflicts are the
/// server's answer (15 1 principle 1).
///
/// Example:
/// ```dart
/// GuidedSwitch(
///   conflicts: failure.conflicts,
///   person: 'Sam',
///   names: names,
///   grant: false,
///   onTake: () => _guided(failure.conflicts),
/// )
/// ```
class GuidedSwitch extends StatelessWidget {
  /// Creates the explanation with its offer.
  const GuidedSwitch({
    super.key,
    required this.conflicts,
    required this.person,
    required this.names,
    this.grant = true,
    this.onTake,
  });

  /// The entries of `Problem.conflicts` or of the preview's violations; entries with another
  /// code than `i1_violation` are left out.
  final List<InvariantFinding> conflicts;

  /// The name of the person (or group) the change lets trigger the agent.
  final String person;

  /// The names the caller may read.
  final AccessNames names;

  /// Whether the change is a grant (wizard) rather than a member or an owner.
  final bool grant;

  /// Takes the guided option, or null for a disabled button.
  final VoidCallback? onTake;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final t = dsToneColors(context, DsTone.warning);
    final style = dsTextStyle(DsType.bodySm).copyWith(color: t.fg);
    final agents = guidedAgentNames(names, conflicts);
    return Container(
      key: const ValueKey('guided.explain'),
      padding: const EdgeInsets.all(DsSpace.s8),
      decoration: BoxDecoration(
        color: t.subtle,
        border: Border.all(color: t.border, width: DsBorderWidth.hairline),
        borderRadius: BorderRadius.circular(DsRadius.md),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          for (final c in _i1(conflicts)) ..._explain(l, c, style),
          const SizedBox(height: DsSpace.s8),
          Align(
            alignment: Alignment.centerRight,
            child: FilledButton(
              key: const ValueKey('guided.offer'),
              onPressed: onTake,
              child: Text(
                grant
                    ? l.adminGuidedGrant(agents)
                    : l.adminGuidedContinue(agents),
              ),
            ),
          ),
        ],
      ),
    );
  }

  List<Widget> _explain(
    AppLocalizations l,
    InvariantFinding c,
    TextStyle style,
  ) {
    final id = c.agentId!;
    final agent = readableName(names, id) ?? id;
    final holderId = c.holderId ?? '';
    final holder = readableName(names, holderId) ?? holderId;
    return [
      Text(
        grant
            ? l.adminGuidedExplainGrant(person, agent, holder)
            : l.adminGuidedExplainChange(person, agent, holder),
        key: ValueKey('guided.explain.$id'),
        style: style,
      ),
      // 10 11 step 2: without an organization key the adapter's runs fail until it is
      // switched; the owner learns this before taking the option.
      if (c.organizationReady == false)
        Text(
          l.adminGuidedNotReady(agent),
          key: ValueKey('guided.notReady.$id'),
          style: style.copyWith(fontWeight: FontWeight.w600),
        ),
    ];
  }
}

/// The reason step of the guided flow (10 11 step 3).
abstract final class GuidedFlow {
  /// Asks the reason of switching [agents] (their names, joined) to organization: a
  /// [ConfirmChange] titled "Switch to Organization" that says what the switch does, with a
  /// required reason (key `confirm.reason`; "Confirm" stays disabled while it is empty). The
  /// server stores the reason in the audit entry of every switched agent. Returns the trimmed
  /// reason, or null for "Cancel". The step-up follows from the interceptor when the caller
  /// sends the request.
  ///
  /// Example: `final reason = await GuidedFlow.askReason(context, agents: 'Support');`
  static Future<String?> askReason(
    BuildContext context, {
    required String agents,
  }) => showDialog<String>(
    context: context,
    builder: (_) => _ReasonDialog(agents: agents),
  );
}

/// The [ConfirmChange] of [GuidedFlow.askReason] with its own required reason field.
class _ReasonDialog extends StatefulWidget {
  const _ReasonDialog({required this.agents});

  final String agents;

  @override
  State<_ReasonDialog> createState() => _ReasonDialogState();
}

class _ReasonDialogState extends State<_ReasonDialog> {
  final TextEditingController _reason = TextEditingController();

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
    // ConfirmChange's own reason field is optional; the switch needs one (10 9.1), so the
    // dialog brings the field under the same key and pops its text itself.
    return ConfirmChange(
      title: l.adminGuidedTitle,
      body: l.adminSwitchToOrgBody(widget.agents),
      confirmLabel: l.adminSwitchConfirm,
      canConfirm: reason.isNotEmpty,
      onConfirm: (_) => Navigator.of(context).pop(reason),
      fields: TextField(
        key: const ValueKey('confirm.reason'),
        controller: _reason,
        decoration: InputDecoration(labelText: l.adminReason),
      ),
    );
  }
}

/// Step 5 of 10 11: per switched agent "... now uses Organization" with its badge, the mode its
/// adapter reports, and while the server says the reported mode does not match
/// (`billing_mode_matches = false`, at once after the switch; the warning code
/// `billing_mode_mismatch` follows only after `billing.mismatch_alert_after`, 10 8) which mode
/// it reports and how to switch it (10 10). Every warning code the server reports is shown
/// with its text. The comparison is the server's; the dialog never compares the modes itself.
class GuidedResult extends StatefulWidget {
  /// Creates the result for [agentIds].
  const GuidedResult({
    super.key,
    required this.repository,
    required this.agentIds,
  });

  /// The administration's calls; `agentRecord` reads each agent anew.
  final AdminRepository repository;

  /// The agents the request switched.
  final List<String> agentIds;

  /// Shows the result over [context] until "Close".
  ///
  /// Example: `await GuidedResult.show(context, repository: admin, agentIds: ['support']);`
  static Future<void> show(
    BuildContext context, {
    required AdminRepository repository,
    required List<String> agentIds,
  }) => showDialog<void>(
    context: context,
    builder: (_) => GuidedResult(repository: repository, agentIds: agentIds),
  );

  @override
  State<GuidedResult> createState() => _GuidedResultState();
}

class _GuidedResultState extends State<GuidedResult> {
  /// The record or the error of `agentRecord` per agent; absent while it loads.
  final Map<String, Object> _loaded = {};

  @override
  void initState() {
    super.initState();
    for (final id in widget.agentIds) {
      _load(id);
    }
  }

  Future<void> _load(String id) async {
    Object answer;
    try {
      answer = await widget.repository.agentRecord(id);
    } on Object catch (e) {
      answer = e;
    }
    if (mounted) setState(() => _loaded[id] = answer);
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final body = dsTextStyle(DsType.body).copyWith(color: colors.textPrimary);
    return AlertDialog(
      key: const ValueKey('guided.result'),
      backgroundColor: colors.surfaceRaised,
      title: Text(
        l.adminGuidedTitle,
        style: dsTextStyle(DsType.title).copyWith(color: colors.textPrimary),
      ),
      content: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: DsLayout.overlayWidth),
        child: SingleChildScrollView(
          child: Semantics(
            liveRegion: true,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                for (final id in widget.agentIds)
                  Padding(
                    key: ValueKey('guided.agent.$id'),
                    padding: const EdgeInsets.only(bottom: DsSpace.s12),
                    child: switch (_loaded[id]) {
                      null => const LinearProgressIndicator(),
                      final AgentRecord agent => _agent(l, agent, body),
                      // The switch is saved; only reading the agent anew failed.
                      final Object error => DsBanner(
                        tone: DsTone.warning,
                        text: adminFailure(l, error)?.text ?? '',
                      ),
                    },
                  ),
              ],
            ),
          ),
        ),
      ),
      actions: [
        FilledButton(
          key: const ValueKey('guided.close'),
          onPressed: () => Navigator.of(context).pop(),
          child: Text(l.adminGuidedDoneClose),
        ),
      ],
    );
  }

  Widget _agent(AppLocalizations l, AgentRecord agent, TextStyle body) {
    final reported = agent.runtime.reportedMode ?? AgentBillingMode.unknown;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        Row(
          children: [
            Expanded(
              child: Text(l.adminGuidedDone(agent.displayName), style: body),
            ),
            const SizedBox(width: DsSpace.s8),
            AgentModeBadge(mode: agent.mode),
          ],
        ),
        if (agent.runtime.reportedMode case final mode?)
          Wrap(
            key: ValueKey('guided.reported.${agent.id}'),
            spacing: DsSpace.s8,
            children: [
              Text(
                l.adminAgentReported,
                style: dsTextStyle(DsType.bodySm)
                    .copyWith(color: context.dsColors.textSecondary),
              ),
              Text(agentModeLabel(l, mode), style: dsTextStyle(DsType.bodySm)),
            ],
          ),
        for (final code in agent.runtime.warnings)
          Padding(
            key: ValueKey('guided.warning.${agent.id}.$code'),
            padding: const EdgeInsets.only(top: DsSpace.s4),
            child: DsBanner(
              tone: DsTone.warning,
              text: [
                agentWarningText(l, agent, code),
                ?agentWarningConsequence(l, code),
              ].join(' '),
            ),
          ),
        if (agent.runtime.modeMatches == false ||
            agent.runtime.warnings.contains('billing_mode_mismatch'))
          Padding(
            key: ValueKey('guided.stillReports.${agent.id}'),
            padding: const EdgeInsets.only(top: DsSpace.s8),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              mainAxisSize: MainAxisSize.min,
              children: [
                DsBanner(
                  tone: DsTone.warning,
                  text: l.adminGuidedStillReports(
                    agent.displayName,
                    agentModeLabel(l, reported),
                  ),
                ),
                const SizedBox(height: DsSpace.s4),
                Text(
                  l.adminAgentSwitchHint(agentModeLabel(l, agent.mode)),
                  style: dsTextStyle(DsType.bodySm)
                      .copyWith(color: context.dsColors.textSecondary),
                ),
              ],
            ),
          ),
      ],
    );
  }
}
