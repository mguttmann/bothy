// SPDX-License-Identifier: Apache-2.0

/// The grants on a host with the state the server's preview gives them (docs/spec/01-product.md
/// 5.2 A8; 07 2.12, SR10, SR12, 9.3; brief M4-37 Design note 4 and fixed requirement 18).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/problem_banner.dart';
import '../data/access_repository.dart';
import '../data/admin_repository.dart';
import '../data/hosts_repository.dart';
import '../grants/grant_labels.dart';
import '../grants/grant_wizard.dart';
import '../grants/preview_panel.dart' show NotEffectiveBadge;
import 'host_labels.dart';

/// The valid grants on [host] (`listGrants` with target `host`), each with its subject and the
/// server's preview of the same grant (`previewGrant`, a dry run that stores nothing): the
/// preview's `text` and a "Not effective" badge for a hint `terminal_not_effective`. The client
/// tells no cause apart (07 9.3: `PreviewHint` has no reason member; that would be permission
/// logic). "Grant terminal" opens the [GrantWizard] on the host; [onChanged] runs after a new
/// grant.
class HostGrantsSection extends StatefulWidget {
  /// Creates the section.
  const HostGrantsSection({
    super.key,
    required this.repository,
    required this.host,
    required this.people,
    this.timeZone = 'UTC',
    this.clock = DateTime.now,
    this.onChanged,
  });

  /// The administration's calls.
  final AdminRepository repository;

  /// The host.
  final AdminHost host;

  /// The people (for the subjects' names).
  final List<AdminPerson> people;

  /// The time zone of the signed-in person.
  final String timeZone;

  /// The current time.
  final DateTime Function() clock;

  /// Runs after a new grant.
  final VoidCallback? onChanged;

  @override
  State<HostGrantsSection> createState() => _HostGrantsSectionState();
}

/// A grant and the server's preview of it (null when the preview failed).
typedef _Row = (AdminGrant, GrantPreviewResult?);

class _HostGrantsSectionState extends State<HostGrantsSection> {
  List<_Row>? _rows;
  Object? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void didUpdateWidget(HostGrantsSection old) {
    super.didUpdateWidget(old);
    if (old.host.version != widget.host.version) _load();
  }

  Future<void> _load() async {
    final r = widget.repository;
    final target = widget.host.target;
    try {
      final grants = await r.grantsOn(target);
      final rows = <_Row>[];
      for (final g in grants) {
        GrantPreviewResult? preview;
        try {
          preview = await r.previewGrant(
            GrantRequest(
              subjectKind: g.subjectKind,
              subjectId: g.subjectId,
              target: target,
              level: AccessLevel.none,
              capabilities: g.capabilities.toSet(),
              expiresAt: g.expiresAt,
              reason: g.reason,
            ),
          );
        } on Object {
          preview = null;
        }
        rows.add((g, preview));
      }
      if (mounted) {
        setState(() {
          _rows = rows;
          _error = null;
        });
      }
    } on Object catch (e) {
      if (mounted) {
        setState(() {
          _rows = null;
          _error = e;
        });
      }
    }
  }

  Future<void> _add() async {
    final created = await GrantWizard.show(
      context,
      repository: widget.repository,
      target: widget.host.target,
      timeZone: widget.timeZone,
      clock: widget.clock,
    );
    if (created == null || !mounted) return;
    widget.onChanged?.call();
    await _load();
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final rows = _rows;
    final error = _error;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        if (error != null)
          ProblemBanner(problem: loadProblem(error), onRetry: _load)
        else if (rows == null)
          const Align(
            alignment: Alignment.centerLeft,
            child: CircularProgressIndicator(),
          )
        else ...[
          if (rows.isEmpty)
            Text(
              l.hostGrantsNone,
              style: dsTextStyle(DsType.bodySm)
                  .copyWith(color: colors.textSecondary),
            ),
          for (final (g, preview) in rows)
            Padding(
              key: ValueKey('hostGrant.${g.id}'),
              padding: const EdgeInsets.only(bottom: DsSpace.s8),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Wrap(
                    spacing: DsSpace.s8,
                    crossAxisAlignment: WrapCrossAlignment.center,
                    children: [
                      Text(
                        g.subjectKind == AdminSubjectKind.user
                            ? personName(widget.people, g.subjectId)
                            : g.subjectId,
                        style: dsTextStyle(DsType.body)
                            .copyWith(color: colors.textPrimary),
                      ),
                      Text(
                        [for (final c in g.capabilities) capabilityLabel(l, c)]
                            .join(', '),
                        style: dsTextStyle(DsType.bodySm)
                            .copyWith(color: colors.textSecondary),
                      ),
                      if (preview != null &&
                          preview.hints.any(
                            (h) => h.code == 'terminal_not_effective',
                          ))
                        NotEffectiveBadge(
                          key: ValueKey('hostGrant.${g.id}.notEffective'),
                        ),
                    ],
                  ),
                  if (preview != null)
                    Text(
                      preview.text,
                      locale: Locale(preview.textLocale),
                      style: dsTextStyle(DsType.bodySm)
                          .copyWith(color: colors.textSecondary),
                    ),
                ],
              ),
            ),
          if (widget.host.state != HostLifecycle.retired)
            Align(
              alignment: Alignment.centerLeft,
              child: TextButton(
                key: const ValueKey('hostGrants.add'),
                onPressed: _add,
                child: Text(l.hostGrantsAdd),
              ),
            ),
        ],
      ],
    );
  }
}
