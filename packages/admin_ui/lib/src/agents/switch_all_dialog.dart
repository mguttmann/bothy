// SPDX-License-Identifier: Apache-2.0

/// Switching the backend of several agents in one step (10 9.1; brief M4-28 Design note 3).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/problem_banner.dart';
import '../data/access_repository.dart';
import '../data/admin_repository.dart';
import '../data/agents_repository.dart';
import '../data/self_repository.dart';
import '../grants/grant_labels.dart';
import 'i1_explanation.dart';

/// The dialog "Switch several agents" (`setAllAgentsBillingMode`, one transaction).
///
/// "All to Organization" needs only a reason: the server takes every personal agent and
/// clears their holders (10 9.1), so no list is sent. "All to Personal" needs a holder and an
/// explicit, editable list of agents; it is proposed from the newest `agent.billing_mode.set_all`
/// to organization in the audit ([AgentsRepository.lastGlobalSwitch], which reads the audit
/// with step-up), never derived by the client. One agent that would break I1 changes nothing
/// (BR-08): the dialog stays open with the server's conflicts ([I1Explanation]).
///
/// The dialog sends the change itself; the step-up interceptor asks when the server answers
/// `401 step_up_required`. It pops the [SwitchAllResult] after a switch and null on "Cancel".
///
/// Example:
/// ```dart
/// final result = await SwitchAllDialog.show(context, repository: repo, agents: agents);
/// ```
class SwitchAllDialog extends StatefulWidget {
  /// Creates the dialog over [agents].
  const SwitchAllDialog({
    super.key,
    required this.repository,
    required this.agents,
  });

  /// The administration's calls.
  final AdminRepository repository;

  /// Every agent of the list.
  final List<AgentRecord> agents;

  /// Shows the dialog; returns the server's result after a switch, or null.
  static Future<SwitchAllResult?> show(
    BuildContext context, {
    required AdminRepository repository,
    required List<AgentRecord> agents,
  }) => showDialog<SwitchAllResult>(
    context: context,
    builder: (_) => SwitchAllDialog(repository: repository, agents: agents),
  );

  @override
  State<SwitchAllDialog> createState() => _SwitchAllDialogState();
}

class _SwitchAllDialogState extends State<SwitchAllDialog> {
  final TextEditingController _reason = TextEditingController();
  AgentBillingMode? _mode;
  bool _busy = false;

  // Personal only: the people, the proposal and the person's choice.
  bool _loadingPersonal = false;
  bool _personalLoaded = false;
  List<AdminPerson> _people = const [];
  bool _proposed = false;
  String? _holderId;
  final Set<String> _checked = {};

  // The last failure: its text and, for `409 i1_violation`, the explanation.
  String? _failure;
  List<InvariantFinding> _conflicts = const [];
  AccessNames? _names;

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

  void _pick(AgentBillingMode mode) {
    setState(() {
      _mode = mode;
      _failure = null;
      _conflicts = const [];
    });
    if (mode == AgentBillingMode.personal &&
        !_personalLoaded &&
        !_loadingPersonal) {
      _loadPersonal();
    }
  }

  /// Loads the active people (the holder candidates) and the proposal of the audit once.
  Future<void> _loadPersonal() async {
    final l = AppLocalizations.of(context);
    setState(() => _loadingPersonal = true);
    List<AdminPerson> people;
    try {
      people = await allPeople(
        widget.repository,
        status: AdminPersonStatus.active,
      );
    } on Object {
      people = const [];
    }
    SwitchAllProposal? proposal;
    String? failure;
    try {
      proposal = await widget.repository.lastGlobalSwitch(
        purpose: l.stepUpPurposeSwitchAll,
      );
    } on Object catch (e) {
      // Without the audit there is no proposal; the person checks the agents.
      failure = adminFailure(l, e)?.text;
    }
    if (!mounted) return;
    final known = {for (final a in widget.agents) a.id};
    setState(() {
      _loadingPersonal = false;
      _personalLoaded = true;
      _people = people;
      _failure = failure;
      if (proposal != null) {
        _proposed = true;
        _checked
          ..clear()
          ..addAll(proposal.agentIds.where(known.contains));
        if (proposal.holderId case final h? when people.any((p) => p.id == h)) {
          _holderId = h;
        }
      }
    });
  }

  /// The names the caller may read, plus the agents of this dialog: the caller read them
  /// through `listAgents`, so their names are readable too (10 5.4).
  Future<AccessNames> _loadNames() async {
    final names = await AccessNames.load(widget.repository);
    final known = {for (final a in names.agents) a.id};
    return AccessNames(
      people: names.people,
      groups: names.groups,
      agents: [
        ...names.agents,
        for (final a in widget.agents)
          if (!known.contains(a.id))
            AdminAgent(id: a.id, handle: a.handle, displayName: a.displayName),
      ],
      targets: names.targets,
      hosts: names.hosts,
    );
  }

  bool get _canSubmit {
    if (_busy || _reason.text.trim().isEmpty) return false;
    return switch (_mode) {
      AgentBillingMode.organization => true,
      AgentBillingMode.personal => _holderId != null && _checked.isNotEmpty,
      _ => false,
    };
  }

  Future<void> _submit() async {
    final l = AppLocalizations.of(context);
    final mode = _mode!;
    final personal = mode == AgentBillingMode.personal;
    setState(() {
      _busy = true;
      _failure = null;
      _conflicts = const [];
    });
    try {
      final result = await widget.repository.setAllModes(
        mode,
        holderId: personal ? _holderId : null,
        // In the order of the list, so the request reads like the dialog.
        agentIds: personal
            ? [
                for (final a in widget.agents)
                  if (_checked.contains(a.id)) a.id,
              ]
            : null,
        reason: _reason.text.trim(),
        purpose: l.stepUpPurposeSwitchAll,
      );
      if (mounted) Navigator.of(context).pop(result);
    } on SelfActionFailed catch (e) {
      if (e.problem?.code == 'i1_violation') {
        // BR-08: nothing changed; the server's conflicts say why.
        final names = _names ?? await _loadNames();
        if (!mounted) return;
        setState(() {
          _busy = false;
          _names = names;
          _conflicts = e.conflicts;
          _failure = problemText(l, 'i1_violation');
        });
        return;
      }
      _fail(l, e);
    } on Object catch (e) {
      _fail(l, e);
    }
  }

  void _fail(AppLocalizations l, Object error) {
    if (!mounted) return;
    setState(() {
      _busy = false;
      _failure = adminFailure(l, error)?.text;
    });
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final body = dsTextStyle(DsType.body).copyWith(color: colors.textSecondary);
    final mode = _mode;
    return AlertDialog(
      key: const ValueKey('switchAll.dialog'),
      backgroundColor: colors.surfaceRaised,
      title: Text(
        l.adminSwitchAllTitle,
        style: dsTextStyle(DsType.title).copyWith(color: colors.textPrimary),
      ),
      content: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: DsLayout.overlayWidth),
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Wrap(
                spacing: DsSpace.s8,
                children: [
                  for (final m in const [
                    AgentBillingMode.organization,
                    AgentBillingMode.personal,
                  ])
                    ChoiceChip(
                      key: ValueKey('switchAll.mode.${m.wire}'),
                      label: Text(
                        m == AgentBillingMode.personal
                            ? l.billingPersonal
                            : l.billingOrganization,
                      ),
                      selected: mode == m,
                      onSelected: _busy ? null : (_) => _pick(m),
                    ),
                ],
              ),
              if (mode == AgentBillingMode.organization) ...[
                const SizedBox(height: DsSpace.s12),
                Text(l.adminSwitchAllOrgBody, style: body),
              ],
              if (mode == AgentBillingMode.personal) ...[
                const SizedBox(height: DsSpace.s12),
                Text(l.adminSwitchAllPersonalBody, style: body),
                const SizedBox(height: DsSpace.s8),
                // While the proposal loads, the step-up dialog of the audit read lies above;
                // the list appears once the proposal (or its absence) is known.
                if (_personalLoaded) ..._personal(l, colors),
              ],
              if (mode != null) ...[
                const SizedBox(height: DsSpace.s12),
                TextField(
                  key: const ValueKey('switchAll.reason'),
                  controller: _reason,
                  enabled: !_busy,
                  decoration: InputDecoration(labelText: l.adminReason),
                ),
              ],
              if (_failure case final text?) ...[
                const SizedBox(height: DsSpace.s12),
                DsBanner(
                  key: const ValueKey('switchAll.failure'),
                  tone: DsTone.danger,
                  text: text,
                  liveRegion: true,
                ),
              ],
              if (_conflicts.isNotEmpty && _names != null) ...[
                const SizedBox(height: DsSpace.s8),
                I1Explanation(conflicts: _conflicts, names: _names!),
              ],
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
          key: const ValueKey('switchAll.cancel'),
          // Not while the switch runs: the list would miss a change the server made.
          onPressed: _busy ? null : () => Navigator.of(context).pop(),
          child: Text(l.selfCancel),
        ),
        FilledButton(
          key: const ValueKey('switchAll.submit'),
          onPressed: _canSubmit ? _submit : null,
          child: Text(l.adminSwitchConfirm),
        ),
      ],
    );
  }

  List<Widget> _personal(AppLocalizations l, DsColors colors) => [
    Text(
      _proposed ? l.adminSwitchAllProposed : l.adminSwitchAllNoProposal,
      key: const ValueKey('switchAll.proposal'),
      style: dsTextStyle(DsType.bodySm).copyWith(color: colors.textSecondary),
    ),
    DropdownButton<String>(
      key: const ValueKey('switchAll.holder'),
      isExpanded: true,
      value: _holderId,
      hint: Text(l.adminSwitchHolderPick),
      items: [
        for (final p in _people)
          DropdownMenuItem(
            key: ValueKey('switchAll.holder.${p.id}'),
            value: p.id,
            child: Text(p.displayName),
          ),
      ],
      onChanged: _busy ? null : (id) => setState(() => _holderId = id),
    ),
    for (final a in widget.agents)
      CheckboxListTile(
        key: ValueKey('switchAll.agent.${a.id}'),
        contentPadding: EdgeInsets.zero,
        controlAffinity: ListTileControlAffinity.leading,
        value: _checked.contains(a.id),
        title: Text(a.displayName),
        subtitle: Text(a.handle),
        onChanged: _busy
            ? null
            : (on) => setState(() {
                if (on ?? false) {
                  _checked.add(a.id);
                } else {
                  _checked.remove(a.id);
                }
              }),
      ),
  ];
}
