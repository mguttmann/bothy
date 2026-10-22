// SPDX-License-Identifier: Apache-2.0

/// The owners: list, add and revoke (docs/spec/07-permissions.md 2.7, SR8; 08 4; 05
/// `owner.add`, `owner.remove` with step-up; 10 11: the guided switch when adding meets I1).
library;

import 'package:api_client/api_client.dart' show ApiProblem;
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../agents/guided_flow.dart';
import '../common/admin_action.dart';
import '../common/confirm_change.dart';
import '../common/problem_banner.dart';
import '../data/access_repository.dart' show I1Resolution, InvariantFinding;
import '../data/admin_repository.dart';
import '../data/self_repository.dart' show SelfActionFailed;
import '../grants/grant_labels.dart' show AccessNames;
import '../self/device_tile.dart' show activityText;
import '../self/self_feedback.dart';

/// The owners of the instance with "Revoke" each and "Add owner" (any active person who is not
/// an owner yet, from every page of `listUsers`).
///
/// The screen never decides which owner may go: revoking the last one is sent, and the
/// server's `409 last_owner` is shown with its text (07 SR8). A `403` of the list shows the
/// server's reason instead.
///
/// When adding an owner meets I1 (`409 i1_violation` with `conflicts`, 10 5.4), the section
/// offers the guided option of 10 11 ([GuidedSwitch]): after a reason it repeats the same call
/// once with `resolve_i1`, so the server switches the agents and adds the owner in one
/// transaction (BR-07), and shows [GuidedResult].
class OwnersSection extends StatefulWidget {
  /// Creates the section.
  const OwnersSection({
    super.key,
    required this.repository,
    this.timeZone = 'UTC',
    this.clock = DateTime.now,
    this.onChanged,
  });

  /// The administration's calls.
  final AdminRepository repository;

  /// The time zone of the signed-in person.
  final String timeZone;

  /// The current time.
  final DateTime Function() clock;

  /// Runs after a change.
  final VoidCallback? onChanged;

  @override
  State<OwnersSection> createState() => _OwnersSectionState();
}

class _OwnersSectionState extends State<OwnersSection>
    with AdminActionState<OwnersSection> {
  List<AdminOwner>? _owners;
  ApiProblem? _problem;
  _GuidedOwner? _guided;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final owners = await widget.repository.owners();
      if (mounted) {
        setState(() {
          _owners = owners;
          _problem = null;
        });
      }
    } on Object catch (e) {
      if (mounted) setState(() => _problem = loadProblem(e));
    }
  }

  Future<void> _revoke(AdminOwner owner) async {
    final l = AppLocalizations.of(context);
    final reason = await ConfirmChange.show(
      context,
      title: l.adminOwnersRevokeTitle(owner.displayName),
      body: l.adminOwnersRevokeBody(owner.displayName),
      confirmLabel: l.adminOwnersRevoke,
      destructive: true,
      withReason: true,
    );
    if (reason == null || !mounted) return;
    final ok = await runChange('${owner.userId}.revoke', () async {
      await widget.repository.revokeOwner(
        owner.userId,
        reason: reason.isEmpty ? l.adminDefaultReason : reason,
        purpose: l.stepUpPurposeRevokeOwner(owner.displayName),
      );
      return SelfNotice(l.adminOwnersRevoked(owner.displayName));
    });
    if (ok) widget.onChanged?.call();
    if (mounted) await _load();
  }

  Future<void> _add() async {
    final l = AppLocalizations.of(context);
    final owners = {for (final o in _owners ?? const <AdminOwner>[]) o.userId};
    final List<AdminPerson> active;
    try {
      // Every active person, not only the page the list shows.
      active = await allPeople(
        widget.repository,
        status: AdminPersonStatus.active,
      );
    } on Object catch (e) {
      final failure = adminFailure(l, e);
      if (mounted && failure != null) {
        setState(() => notice = SelfNotice(failure.text, tone: DsTone.danger));
      }
      return;
    }
    if (!mounted) return;
    final candidates = [
      for (final p in active)
        if (!owners.contains(p.id)) p,
    ];
    final picked = await showDialog<(AdminPerson, String)>(
      context: context,
      builder: (_) => _AddOwnerDialog(candidates: candidates),
    );
    if (picked == null || !mounted) return;
    setState(() => _guided = null);
    final (person, typed) = picked;
    final reason = typed.isEmpty ? l.adminDefaultReason : typed;
    var conflicts = const <InvariantFinding>[];
    final ok = await runChange('add', () async {
      try {
        await widget.repository.addOwner(
          person.id,
          reason: reason,
          purpose: l.stepUpPurposeAddOwner(person.displayName),
        );
      } on SelfActionFailed catch (e) {
        if (e.problem?.code == 'i1_violation') conflicts = e.conflicts;
        rethrow;
      }
      return SelfNotice(l.adminOwnersAdded(person.displayName));
    });
    if (ok) widget.onChanged?.call();
    if (mounted) await _load();
    if (guidedAgentIds(conflicts).isEmpty || !mounted) return;
    final names = await AccessNames.load(widget.repository);
    if (!mounted) return;
    // The guided option explains the conflict itself (10 11 step 2) and replaces the plain
    // rejection.
    setState(() {
      notice = null;
      _guided = _GuidedOwner(person, reason, conflicts, names);
    });
  }

  /// The guided option for [g]: the reason of the switch, then the same `addOwner` once more
  /// with `resolve_i1` (05 rule 1.6); never a switch on its own.
  Future<void> _takeGuided(_GuidedOwner g) async {
    final l = AppLocalizations.of(context);
    final ids = guidedAgentIds(g.conflicts);
    final switchReason = await GuidedFlow.askReason(
      context,
      agents: guidedAgentNames(g.names, g.conflicts),
    );
    if (switchReason == null || !mounted) return;
    setState(() => _guided = null);
    final ok = await runChange('add', () async {
      await widget.repository.addOwner(
        g.person.id,
        reason: g.reason,
        purpose: l.stepUpPurposeAddOwner(g.person.displayName),
        resolveI1: I1Resolution(ids, reason: switchReason),
      );
      return SelfNotice(l.adminOwnersAdded(g.person.displayName));
    });
    if (ok) widget.onChanged?.call();
    if (!mounted) return;
    await _load();
    if (ok && mounted) {
      await GuidedResult.show(
        context,
        repository: widget.repository,
        agentIds: ids,
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final owners = _owners;
    final now = widget.clock();
    return Column(
      key: const ValueKey('owners'),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Row(
          children: [
            Expanded(
              child: Semantics(
                header: true,
                child: Text(
                  l.adminOwnersTitle,
                  style: dsTextStyle(DsType.heading)
                      .copyWith(color: colors.textPrimary),
                ),
              ),
            ),
            if (owners != null && !denied.contains('add'))
              OutlinedButton(
                key: const ValueKey('owners.add'),
                onPressed: busy ? null : _add,
                child: Text(l.adminOwnersAdd),
              ),
          ],
        ),
        const SizedBox(height: DsSpace.s8),
        noticeBanner(),
        if (_guided case final g?)
          GuidedSwitch(
            conflicts: g.conflicts,
            person: g.person.displayName,
            names: g.names,
            grant: false,
            onTake: busy ? null : () => _takeGuided(g),
          ),
        if (_problem case final problem?)
          ProblemBanner(problem: problem, onRetry: _load)
        else if (owners != null)
          for (final owner in owners) ...[
            Padding(
              key: ValueKey('owner.${owner.userId}'),
              padding: const EdgeInsets.symmetric(vertical: DsSpace.s8),
              child: Row(
                children: [
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          owner.displayName,
                          style: dsTextStyle(DsType.bodyStrong)
                              .copyWith(color: colors.textPrimary),
                        ),
                        Text(
                          l.adminOwnerSince(
                            activityText(
                              context,
                              owner.grantedAt,
                              now: now,
                              timeZone: widget.timeZone,
                            ),
                            owner.reason,
                          ),
                          style: dsTextStyle(DsType.bodySm)
                              .copyWith(color: colors.textSecondary),
                        ),
                      ],
                    ),
                  ),
                  if (!denied.contains('${owner.userId}.revoke'))
                    OutlinedButton(
                      key: ValueKey('owner.${owner.userId}.revoke'),
                      onPressed: busy ? null : () => _revoke(owner),
                      child: Text(l.adminOwnersRevoke),
                    ),
                ],
              ),
            ),
            Divider(height: 1, color: colors.borderSubtle),
          ],
      ],
    );
  }
}

/// An "Add owner" the server rejected with I1, waiting for the guided option.
final class _GuidedOwner {
  const _GuidedOwner(this.person, this.reason, this.conflicts, this.names);

  final AdminPerson person;
  final String reason;
  final List<InvariantFinding> conflicts;
  final AccessNames names;
}

/// Picks the new owner and the reason; pops `(person, reason)` or null.
class _AddOwnerDialog extends StatefulWidget {
  const _AddOwnerDialog({required this.candidates});

  final List<AdminPerson> candidates;

  @override
  State<_AddOwnerDialog> createState() => _AddOwnerDialogState();
}

class _AddOwnerDialogState extends State<_AddOwnerDialog> {
  AdminPerson? _picked;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    return ConfirmChange(
      title: l.adminOwnersAddTitle,
      body: l.adminOwnersAddBody,
      confirmLabel: l.adminOwnersAdd,
      withReason: true,
      canConfirm: _picked != null,
      onConfirm: (reason) => Navigator.of(context).pop((_picked!, reason)),
      fields: DropdownButton<AdminPerson>(
        key: const ValueKey('owners.pick'),
        isExpanded: true,
        value: _picked,
        hint: Text(l.adminPickPerson),
        items: [
          for (final p in widget.candidates)
            DropdownMenuItem(
              key: ValueKey('owners.pick.${p.id}'),
              value: p,
              child: Text(p.displayName),
            ),
        ],
        onChanged: (p) => setState(() => _picked = p),
      ),
    );
  }
}
