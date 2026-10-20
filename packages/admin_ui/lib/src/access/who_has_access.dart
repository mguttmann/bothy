// SPDX-License-Identifier: Apache-2.0

/// "Who has access?" of a space or channel (docs/spec/01-product.md 5.2 A3; 07 9.1; brief
/// M4-27 Design note 2 and fixed requirement 16).
library;

import 'package:api_client/api_client.dart' show ApiProblem;
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/admin_action.dart';
import '../common/problem_banner.dart';
import '../data/access_repository.dart';
import '../data/admin_repository.dart';
import '../grants/grant_labels.dart';
import '../grants/grant_wizard.dart';
import '../grants/revoke_grant_dialog.dart';
import '../self/self_feedback.dart';
import 'source_chain.dart';
import 'view_as_person.dart';

/// Every principal with access to [target] as the server explains it (`explainSpaceAccess`,
/// `explainChannelAccess`): the effective level, the capabilities and the [SourceChain]. An
/// entry only via an agent shows the level "No level" and hint H1; a disabled principal is
/// struck through. Actions: "Create access grant" on the target, "Revoke" on a direct grant,
/// and "View as person" ([ViewAsPerson]).
///
/// Example:
/// ```dart
/// WhoHasAccess(repository: admin, target: channel, timeZone: 'Europe/Berlin');
/// ```
class WhoHasAccess extends StatefulWidget {
  /// Creates the view.
  const WhoHasAccess({
    super.key,
    required this.repository,
    required this.target,
    this.timeZone = 'UTC',
    this.clock = DateTime.now,
    this.onChanged,
  });

  /// The administration's calls.
  final AdminRepository repository;

  /// The space or channel.
  final AdminTarget target;

  /// The time zone of the signed-in person.
  final String timeZone;

  /// The current time.
  final DateTime Function() clock;

  /// Runs after a change.
  final VoidCallback? onChanged;

  @override
  State<WhoHasAccess> createState() => _WhoHasAccessState();
}

class _WhoHasAccessState extends State<WhoHasAccess>
    with AdminActionState<WhoHasAccess> {
  AccessNames? _names;
  List<AccessEntryInfo>? _entries;
  ApiProblem? _problem;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    final names = _names ?? await AccessNames.load(widget.repository);
    try {
      final explained = await widget.repository.explain(widget.target);
      if (!mounted) return;
      setState(() {
        _names = names;
        _entries = explained.entries;
        _problem = null;
      });
    } on Object catch (e) {
      if (mounted) {
        setState(() {
          _names = names;
          _problem = loadProblem(e);
        });
      }
    }
  }

  Future<void> _create() async {
    final grant = await GrantWizard.show(
      context,
      repository: widget.repository,
      target: widget.target,
      timeZone: widget.timeZone,
      clock: widget.clock,
    );
    if (grant == null || !mounted) return;
    widget.onChanged?.call();
    await _load();
  }

  Future<void> _revoke(AccessEntryInfo entry, String grantId) async {
    final l = AppLocalizations.of(context);
    final name = entry.principal.displayName;
    final reason = await RevokeGrantDialog.show(context, name: name);
    if (reason == null || !mounted) return;
    final ok = await runChange('revoke.$grantId', () async {
      await widget.repository.revokeGrant(
        grantId,
        reason: reason,
        purpose: l.stepUpPurposeRevokeGrant(name),
      );
      return SelfNotice(l.adminGrantRevoked(name));
    });
    if (!ok || !mounted) return;
    widget.onChanged?.call();
    await _load();
  }

  Future<void> _viewAs(AdminPerson person) => showDialog<void>(
    context: context,
    builder: (dialog) => Dialog(
      backgroundColor: dialog.dsColors.surfaceRaised,
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 720),
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(DsSpace.s16),
          child: ViewAsPerson(
            repository: widget.repository,
            userId: person.id,
            name: person.displayName,
            timeZone: widget.timeZone,
            clock: widget.clock,
            onExit: () => Navigator.of(dialog).pop(),
          ),
        ),
      ),
    ),
  );

  Widget _entry(AccessEntryInfo e, AccessNames names) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final id = e.principal.id;
    return Padding(
      key: ValueKey('access.entry.$id'),
      padding: const EdgeInsets.symmetric(vertical: DsSpace.s8),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Expanded(
            flex: 2,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  e.principal.displayName,
                  key: ValueKey('access.name.$id'),
                  style: dsTextStyle(DsType.bodyStrong).copyWith(
                    color: e.inactive
                        ? colors.textSecondary
                        : colors.textPrimary,
                    decoration: e.inactive
                        ? TextDecoration.lineThrough
                        : TextDecoration.none,
                  ),
                ),
                if (e.inactive)
                  Text(
                    l.adminStatusDisabled,
                    style: dsTextStyle(DsType.bodySm)
                        .copyWith(color: colors.textSecondary),
                  ),
              ],
            ),
          ),
          Expanded(
            flex: 5,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  levelWithCapabilities(l, e.level, e.capabilities),
                  style: dsTextStyle(DsType.body)
                      .copyWith(color: colors.textPrimary),
                ),
                SourceChain(
                  sources: e.sources,
                  names: names,
                  timeZone: widget.timeZone,
                  clock: widget.clock,
                  trailing: (s) {
                    final grant = s.grantId;
                    if (s.kind != AccessSourceKind.direct ||
                        grant == null ||
                        denied.contains('revoke.$grant')) {
                      return null;
                    }
                    return TextButton(
                      key: ValueKey('access.revoke.$grant'),
                      onPressed: busy ? null : () => _revoke(e, grant),
                      child: Text(l.adminRevoke),
                    );
                  },
                ),
              ],
            ),
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
    final entries = _entries;
    final people = [
      for (final p in names?.people ?? const <AdminPerson>[])
        if (p.status == AdminPersonStatus.active) p,
    ];
    return Column(
      key: const ValueKey('access'),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Semantics(
          header: true,
          child: Text(
            l.adminAccessTitle(targetLabel(l, widget.target)),
            style: dsTextStyle(DsType.title)
                .copyWith(color: colors.textPrimary),
          ),
        ),
        const SizedBox(height: DsSpace.s8),
        noticeBanner(),
        if (_problem case final problem?)
          ProblemBanner(problem: problem, onRetry: _load)
        else if (names == null || entries == null)
          const Center(child: CircularProgressIndicator())
        else ...[
          Wrap(
            spacing: DsSpace.s8,
            crossAxisAlignment: WrapCrossAlignment.center,
            children: [
              FilledButton(
                key: const ValueKey('access.create'),
                onPressed: busy ? null : _create,
                child: Text(l.adminGrantsCreate),
              ),
              if (people.isNotEmpty)
                DropdownButton<AdminPerson>(
                  key: const ValueKey('access.viewAs'),
                  hint: Text(l.adminAccessViewAs),
                  items: [
                    for (final p in people)
                      DropdownMenuItem(
                        key: ValueKey('access.viewAs.${p.id}'),
                        value: p,
                        child: Text(p.displayName),
                      ),
                  ],
                  onChanged: (p) {
                    if (p != null) _viewAs(p);
                  },
                ),
            ],
          ),
          const SizedBox(height: DsSpace.s8),
          if (entries.isEmpty)
            Text(
              l.adminAccessEmpty,
              style: dsTextStyle(DsType.body)
                  .copyWith(color: colors.textSecondary),
            ),
          for (final e in entries) ...[
            _entry(e, names),
            Divider(height: 1, color: colors.borderSubtle),
          ],
        ],
      ],
    );
  }
}
