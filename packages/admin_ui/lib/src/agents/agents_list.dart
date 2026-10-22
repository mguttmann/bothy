// SPDX-License-Identifier: Apache-2.0

/// The agents of screen A5 with "Switch all agents" (docs/spec/01-product.md 5.2 A5; 10 9.1,
/// 13; brief M4-28 fixed requirement 11, Design note 3).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/problem_banner.dart';
import '../data/admin_repository.dart';
import '../data/agents_repository.dart';
import '../hosts/host_labels.dart' show personName;
import '../self/self_feedback.dart';
import 'agent_labels.dart';
import 'switch_all_dialog.dart';

/// Every agent the server lists (`listAgents`) with the badge "Personal" or "Organization"
/// and its holder (10 13), "Disabled" for an inactive agent and the number of warnings the
/// server reports (10 8); a row opens the agent ([onOpen] receives its ID).
///
/// "Switch all agents" opens the [SwitchAllDialog]; after a switch the list loads again, shows
/// how many agents changed and [onChanged] runs. The warnings are the server's codes only: the
/// list never compares the configured with the reported mode (15 1 principle 1). A list the
/// server refuses shows its reason and nothing else.
///
/// Example: `AgentsList(repository: repository, onOpen: (id) => open(id))`.
class AgentsList extends StatefulWidget {
  /// Creates the list.
  const AgentsList({
    super.key,
    required this.repository,
    required this.onOpen,
    this.onChanged,
  });

  /// The administration's calls.
  final AdminRepository repository;

  /// Opens an agent.
  final ValueChanged<String> onOpen;

  /// Runs after a change.
  final VoidCallback? onChanged;

  @override
  State<AgentsList> createState() => _AgentsListState();
}

class _AgentsListState extends State<AgentsList> {
  List<AgentRecord>? _agents;
  List<AdminPerson> _people = const [];
  Object? _error;
  SelfNotice? _notice;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final agents = await widget.repository.agentRecords();
      List<AdminPerson> people;
      try {
        people = await allPeople(widget.repository);
      } on Object {
        // Without the directory the holder shows by its ID.
        people = const [];
      }
      if (mounted) {
        setState(() {
          _agents = agents;
          _people = people;
          _error = null;
        });
      }
    } on Object catch (e) {
      if (mounted) {
        setState(() {
          _agents = null;
          _error = e;
        });
      }
    }
  }

  Future<void> _switchAll(List<AgentRecord> agents) async {
    final l = AppLocalizations.of(context);
    final result = await SwitchAllDialog.show(
      context,
      repository: widget.repository,
      agents: agents,
    );
    if (result == null || !mounted) return;
    setState(
      () => _notice = SelfNotice(
        l.adminSwitchAllDone(result.changedAgentIds.length),
      ),
    );
    await _load();
    widget.onChanged?.call();
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final agents = _agents;
    final error = _error;
    final Widget content;
    if (error != null) {
      content = ProblemBanner(problem: loadProblem(error), onRetry: _load);
    } else if (agents == null) {
      content = const Center(child: CircularProgressIndicator());
    } else {
      content = Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          Row(
            children: [
              Expanded(
                child: Semantics(
                  header: true,
                  child: Text(
                    l.adminAgentsTitle,
                    style: dsTextStyle(DsType.title)
                        .copyWith(color: colors.textPrimary),
                  ),
                ),
              ),
              OutlinedButton(
                key: const ValueKey('agents.switchAll'),
                onPressed: () => _switchAll(agents),
                child: Text(l.adminAgentsSwitchAll),
              ),
            ],
          ),
          const SizedBox(height: DsSpace.s12),
          if (_notice case final n?) selfNoticeBanner(n),
          if (agents.isEmpty)
            Text(
              l.adminAgentsEmpty,
              style: dsTextStyle(DsType.body)
                  .copyWith(color: colors.textSecondary),
            ),
          for (final a in agents) _row(l, colors, a),
        ],
      );
    }
    return KeyedSubtree(key: const ValueKey('agents'), child: content);
  }

  Widget _row(AppLocalizations l, DsColors colors, AgentRecord a) {
    final secondary = dsTextStyle(DsType.bodySm)
        .copyWith(color: colors.textSecondary);
    final holder = a.mode == AgentBillingMode.personal && a.holderId != null
        ? personName(_people, a.holderId!)
        : null;
    final warnings = a.runtime.warnings.length;
    return ListTile(
      key: ValueKey('agent.${a.id}'),
      contentPadding: EdgeInsets.zero,
      title: Text(a.displayName),
      subtitle: Padding(
        padding: const EdgeInsets.only(top: DsSpace.s4),
        child: Wrap(
          spacing: DsSpace.s8,
          runSpacing: DsSpace.s4,
          crossAxisAlignment: WrapCrossAlignment.center,
          children: [
            AgentModeBadge(mode: a.mode, holderName: holder),
            if (holder != null)
              Text(l.adminAgentHolder(holder), style: secondary),
            Text(a.handle, style: secondary),
            if (!a.active) Text(l.adminAgentDisabled, style: secondary),
            if (warnings > 0)
              Text(
                l.adminAgentWarningCount(warnings),
                key: ValueKey('agent.${a.id}.warnings'),
                style: dsTextStyle(DsType.bodySm)
                    .copyWith(color: colors.warningFg),
              ),
          ],
        ),
      ),
      onTap: () => widget.onOpen(a.id),
    );
  }
}
