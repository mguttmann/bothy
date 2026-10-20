// SPDX-License-Identifier: Apache-2.0

/// The chain of sources of one entry of "Who has access?" (docs/spec/07-permissions.md 9.1;
/// brief M4-27 Design note 2).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../data/access_repository.dart';
import '../grants/grant_labels.dart';

/// The sources of an entry as plain labels, in the server's order: owner, direct (with the
/// grant's expiry and reason), group (with the end of the membership), inherited (from the
/// space or the instance), participant, operator, via agent (hint H1, 07 6) and blocked by a
/// guard rule (with the rule and the agent). [trailing] adds a control after a source, for
/// example "Revoke" for a direct grant.
///
/// Example: `SourceChain(sources: entry.sources, names: names, timeZone: 'Europe/Berlin')`.
class SourceChain extends StatelessWidget {
  /// Creates the chain.
  const SourceChain({
    super.key,
    required this.sources,
    required this.names,
    this.timeZone = 'UTC',
    this.clock = DateTime.now,
    this.trailing,
  });

  /// The sources.
  final List<AccessSourceInfo> sources;

  /// The names of groups, agents and spaces.
  final AccessNames names;

  /// The time zone of the signed-in person.
  final String timeZone;

  /// The current time.
  final DateTime Function() clock;

  /// A control after a source, or null.
  final Widget? Function(AccessSourceInfo source)? trailing;

  String _label(AppLocalizations l, AccessSourceInfo s) => switch (s.kind) {
    AccessSourceKind.owner => l.adminOwnerBadge,
    AccessSourceKind.direct => l.adminSourceDirect,
    AccessSourceKind.group => l.adminSourceGroup(names.group(s.groupId ?? '')),
    AccessSourceKind.inherited =>
      s.fromTargetKind == AdminTargetKind.space && s.fromTargetId != null
          ? l.adminSourceInheritedSpace(names.target(s.fromTargetId!))
          : l.adminSourceInheritedInstance,
    AccessSourceKind.dmParticipant => l.adminSourceDm,
    AccessSourceKind.operator => l.adminSourceOperator,
    AccessSourceKind.viaAgent => l.adminSourceViaAgent(
      names.principal(s.agentId ?? ''),
    ),
    AccessSourceKind.guardBlocked => l.adminSourceGuardBlocked(
      (s.rule ?? '').toUpperCase(),
    ),
    AccessSourceKind.unknown => l.adminSourceUnknown,
  };

  List<String> _details(
    BuildContext context,
    AppLocalizations l,
    AccessSourceInfo s,
  ) {
    String day(DateTime end) =>
        lastDayLabel(context, end, now: clock(), timeZone: timeZone);
    final expires = s.expiresAt;
    final member = s.membershipExpiresAt;
    final reason = s.reason;
    final agent = s.agentId;
    return [
      if (s.kind == AccessSourceKind.guardBlocked && agent != null)
        l.adminSourceGuardAgent(names.principal(agent)),
      if (expires != null) l.adminGrantsUntil(day(expires)),
      if (member != null) l.adminSourceMemberUntil(day(member)),
      if (reason != null && reason.isNotEmpty) l.adminGrantsReason(reason),
    ];
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final main = dsTextStyle(DsType.bodySm).copyWith(color: colors.textPrimary);
    final secondary = dsTextStyle(DsType.bodySm)
        .copyWith(color: colors.textSecondary);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        for (final s in sources)
          Row(
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(_label(l, s), style: main),
                    for (final d in _details(context, l, s))
                      Text(d, style: secondary),
                  ],
                ),
              ),
              ?trailing?.call(s),
            ],
          ),
      ],
    );
  }
}
