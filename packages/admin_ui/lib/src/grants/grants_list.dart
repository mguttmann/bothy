// SPDX-License-Identifier: Apache-2.0

/// The grants of one target with "Create access grant", "Who has access?" and "Revoke"
/// (docs/spec/01-product.md 5.2 A2, A3; 07 9.5; brief M4-27 Scope).
library;

import 'package:api_client/api_client.dart' show ApiProblem;
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../access/who_has_access.dart';
import '../common/admin_action.dart';
import '../common/problem_banner.dart';
import '../data/access_repository.dart';
import '../data/admin_repository.dart';
import '../self/self_feedback.dart';
import 'grant_labels.dart';
import 'grant_wizard.dart';
import 'revoke_grant_dialog.dart';

/// The grants screen: a picker of the target (the instance, or a space or channel from the
/// server's lists), the valid grants on it (`listGrants`) with subject, level, capabilities,
/// validity and reason, and "Revoke" per grant ([RevokeGrantDialog]: takes effect immediately,
/// required reason). "Create access grant" opens the [GrantWizard] on the target; "Who has
/// access?" shows [WhoHasAccess] for a space or channel.
///
/// [targetKey] is the open target ([AdminTarget.key]), or null before one is chosen;
/// [onTarget] opens another target (the shell maps it to `/grants/<key>`). A list the server
/// refuses shows its reason (15 1 principle 1).
class GrantsList extends StatefulWidget {
  /// Creates the screen.
  const GrantsList({
    super.key,
    required this.repository,
    required this.onTarget,
    this.targetKey,
    this.timeZone = 'UTC',
    this.clock = DateTime.now,
    this.onChanged,
  });

  /// The administration's calls.
  final AdminRepository repository;

  /// The open target, or null.
  final String? targetKey;

  /// Opens the target with this key.
  final ValueChanged<String> onTarget;

  /// The time zone of the signed-in person.
  final String timeZone;

  /// The current time.
  final DateTime Function() clock;

  /// Runs after a change.
  final VoidCallback? onChanged;

  @override
  State<GrantsList> createState() => _GrantsListState();
}

class _GrantsListState extends State<GrantsList>
    with AdminActionState<GrantsList> {
  AccessNames? _names;
  List<AdminGrant>? _grants;
  ApiProblem? _problem;
  bool _showAccess = false;
  int _accessVersion = 0;

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void didUpdateWidget(GrantsList old) {
    super.didUpdateWidget(old);
    if (old.targetKey != widget.targetKey) {
      _showAccess = false;
      _grants = null;
      _loadGrants();
    }
  }

  AdminTarget? get _target {
    final key = widget.targetKey;
    if (key == null) return null;
    if (key == AdminTarget.instance.key) return AdminTarget.instance;
    return _names?.targets.where((t) => t.key == key).firstOrNull;
  }

  Future<void> _load() async {
    final names = await AccessNames.load(widget.repository);
    if (!mounted) return;
    setState(() => _names = names);
    await _loadGrants();
  }

  Future<void> _loadGrants() async {
    final target = _target;
    if (target == null) return;
    try {
      final grants = await widget.repository.grantsOn(target);
      if (!mounted) return;
      setState(() {
        _grants = grants;
        _problem = null;
      });
    } on Object catch (e) {
      if (mounted) setState(() => _problem = loadProblem(e));
    }
  }

  String _subjectName(AdminGrant g) => g.subjectKind == AdminSubjectKind.group
      ? _names!.group(g.subjectId)
      : _names!.principal(g.subjectId);

  Future<void> _create(AdminTarget target) async {
    final grant = await GrantWizard.show(
      context,
      repository: widget.repository,
      target: target,
      timeZone: widget.timeZone,
      clock: widget.clock,
    );
    if (grant == null || !mounted) return;
    widget.onChanged?.call();
    setState(() => _accessVersion++);
    await _loadGrants();
  }

  Future<void> _revoke(AdminGrant grant) async {
    final l = AppLocalizations.of(context);
    final name = _subjectName(grant);
    final reason = await RevokeGrantDialog.show(context, name: name);
    if (reason == null || !mounted) return;
    final ok = await runChange('revoke.${grant.id}', () async {
      await widget.repository.revokeGrant(
        grant.id,
        reason: reason,
        purpose: l.stepUpPurposeRevokeGrant(name),
      );
      return SelfNotice(l.adminGrantRevoked(name));
    });
    if (!ok || !mounted) return;
    widget.onChanged?.call();
    setState(() => _accessVersion++);
    await _loadGrants();
  }

  Widget _row(AdminGrant g) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final secondary = dsTextStyle(DsType.bodySm)
        .copyWith(color: colors.textSecondary);
    final level = g.level;
    final expires = g.expiresAt;
    return Padding(
      key: ValueKey('grant.${g.id}'),
      padding: const EdgeInsets.symmetric(vertical: DsSpace.s8),
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  _subjectName(g),
                  style: dsTextStyle(DsType.bodyStrong)
                      .copyWith(color: colors.textPrimary),
                ),
                Text(
                  [
                    if (level != null)
                      levelWithCapabilities(l, level, g.capabilities)
                    else
                      g.capabilities
                          .map((c) => capabilityLabel(l, c))
                          .join(', '),
                    if (expires != null)
                      l.adminGrantsUntil(
                        lastDayLabel(
                          context,
                          expires,
                          now: widget.clock(),
                          timeZone: widget.timeZone,
                        ),
                      ),
                  ].join(', '),
                  style: secondary,
                ),
                Text(l.adminGrantsReason(g.reason), style: secondary),
              ],
            ),
          ),
          if (!denied.contains('revoke.${g.id}'))
            TextButton(
              key: ValueKey('grant.${g.id}.revoke'),
              onPressed: busy ? null : () => _revoke(g),
              child: Text(l.adminRevoke),
            ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final names = _names;
    final target = _target;
    final grants = _grants;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Semantics(
          header: true,
          child: Text(
            l.adminGrantsTitle,
            style: dsTextStyle(DsType.title)
                .copyWith(color: colors.textPrimary),
          ),
        ),
        const SizedBox(height: DsSpace.s12),
        if (names == null)
          const Center(child: CircularProgressIndicator())
        else ...[
          DropdownButton<String>(
            key: const ValueKey('grants.target'),
            isExpanded: true,
            value: target?.key,
            hint: Text(l.adminGrantsPickTarget),
            items: [
              for (final t in [AdminTarget.instance, ...names.targets])
                DropdownMenuItem(
                  key: ValueKey('grants.target.${t.key}'),
                  value: t.key,
                  child: Text(targetLabel(l, t)),
                ),
            ],
            onChanged: (key) {
              if (key != null) widget.onTarget(key);
            },
          ),
          const SizedBox(height: DsSpace.s8),
          if (target != null)
            Wrap(
              spacing: DsSpace.s8,
              children: [
                FilledButton(
                  key: const ValueKey('grants.create'),
                  onPressed: busy ? null : () => _create(target),
                  child: Text(l.adminGrantsCreate),
                ),
                if (target.kind != AdminTargetKind.instance)
                  OutlinedButton(
                    key: const ValueKey('grants.access'),
                    onPressed: () => setState(() => _showAccess = !_showAccess),
                    child: Text(l.adminGrantsWhoHasAccess),
                  ),
              ],
            ),
          const SizedBox(height: DsSpace.s12),
          noticeBanner(),
          if (_problem case final problem?)
            ProblemBanner(problem: problem, onRetry: _loadGrants)
          else if (grants != null && grants.isEmpty)
            Text(
              l.adminGrantsEmpty,
              style: dsTextStyle(DsType.body)
                  .copyWith(color: colors.textSecondary),
            )
          else if (grants != null)
            for (final g in grants) ...[
              _row(g),
              Divider(height: 1, color: colors.borderSubtle),
            ],
          if (_showAccess && target != null) ...[
            const SizedBox(height: DsSpace.s24),
            WhoHasAccess(
              key: ValueKey('access.${target.key}.$_accessVersion'),
              repository: widget.repository,
              target: target,
              timeZone: widget.timeZone,
              clock: widget.clock,
              onChanged: () {
                widget.onChanged?.call();
                _loadGrants();
              },
            ),
          ],
        ],
      ],
    );
  }
}
