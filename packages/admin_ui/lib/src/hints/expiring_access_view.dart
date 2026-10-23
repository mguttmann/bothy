// SPDX-License-Identifier: Apache-2.0

/// The list "Expiring soon" (docs/spec/07-permissions.md 9.4; brief M6-07 Design notes 3 and
/// 4): grants and group memberships that expire within `authz.expiry_reminder_lead`, and the
/// API tokens of agents and apps in the same window.
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/admin_action.dart';
import '../data/access_repository.dart';
import '../grants/grant_labels.dart' show lastDayLabel;
import '../shell/admin_routes.dart';
import 'access_list.dart';
import 'hint_texts.dart';

/// "Expiring soon": the scope filter (the instance or one space), the entries of
/// `listExpiringAccess` ordered by their end with "Load more", each with its kind as icon and
/// text, who and what expires, and its last day in the caller's time zone.
///
/// Actions: a grant has "Explain" (A3 of its space or channel), "Revoke" and "Extend": the
/// grant wizard with the old grant's subject, target, level and capabilities, an empty reason
/// and a new last day; confirming creates the new grant, then revokes the old one (07 9.4).
/// An agent token has "Renew token", which opens the agent's page with its tokens (M4-28: a new
/// token shown once, then revoking the old one); an app token has no screen in the
/// administration yet and says how to renew it (08 12: a new token, then revoking the old one).
/// A membership opens its group.
/// A refused list shows the translated reason (15 1 principle 1).
///
/// Example:
/// ```dart
/// ExpiringAccessView(repository: admin, onNavigate: (to) => go('/admin${to.path}'));
/// ```
class ExpiringAccessView extends AccessListView {
  /// Creates the list.
  const ExpiringAccessView({
    super.key,
    required super.repository,
    required super.onNavigate,
    super.timeZone,
    super.clock,
    super.onChanged,
    super.onViewAs,
  });

  @override
  State<ExpiringAccessView> createState() => _ExpiringAccessViewState();
}

class _ExpiringAccessViewState extends State<ExpiringAccessView>
    with
        AdminActionState<ExpiringAccessView>,
        AccessListState<ExpiringAccessView, ExpiringEntry> {
  @override
  Future<AccessListPage<ExpiringEntry>> fetch(String? cursor) async {
    final page = await widget.repository.expiringAccess(
      spaceId: spaceId,
      cursor: cursor,
    );
    return (
      items: page.items,
      labels: page.labels,
      nextCursor: page.nextCursor,
    );
  }

  /// The key of [entry]'s row: its kind and its ID.
  static String _keyOf(ExpiringEntry entry) => switch (entry.kind) {
    ExpiringKind.grant => 'expiring.grant.${entry.grant?.id}',
    ExpiringKind.membership => 'expiring.membership.${entry.membershipId}',
    ExpiringKind.token => 'expiring.token.${entry.tokenId}',
    ExpiringKind.unknown =>
      'expiring.unknown.${entry.expiresAt.toIso8601String()}',
  };

  List<Widget> _actions(String key, ExpiringEntry entry) {
    final l = AppLocalizations.of(context);
    final grant = entry.grant;
    final group = entry.groupId;
    final holder = entry.principalId;
    return switch (entry.kind) {
      ExpiringKind.grant when grant != null => grantActions(key, grant),
      ExpiringKind.membership when group != null => [
        TextButton(
          key: ValueKey('$key.group'),
          onPressed: () =>
              widget.onNavigate(AdminLocation(AdminSection.groups, id: group)),
          child: Text(l.expiringOpenGroup),
        ),
      ],
      ExpiringKind.token when holder != null && !isAppToken(entry, labels) => [
        TextButton(
          key: ValueKey('$key.renew'),
          onPressed: () =>
              widget.onNavigate(AdminLocation(AdminSection.agents, id: holder)),
          child: Text(l.expiringRenewToken),
        ),
      ],
      _ => const [],
    };
  }

  Widget _row(ExpiringEntry entry) {
    final l = AppLocalizations.of(context);
    final key = _keyOf(entry);
    return accessRow(
      key: key,
      glyph: expiringKindGlyph(entry.kind),
      kind: expiringKindLabel(l, entry.kind),
      lines: [expiringText(l, entry, labels)],
      detail: l.adminGrantsUntil(
        lastDayLabel(
          context,
          entry.expiresAt,
          now: widget.clock(),
          timeZone: widget.timeZone,
        ),
      ),
      note: entry.kind == ExpiringKind.token && isAppToken(entry, labels)
          ? l.expiringAppTokenNote
          : null,
      actions: _actions(key, entry),
    );
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    return Column(
      key: const ValueKey('expiring.view'),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        heading(l.expiringTitle, l.expiringIntro),
        const SizedBox(height: DsSpace.s12),
        scopeFilter('expiring'),
        const SizedBox(height: DsSpace.s12),
        noticeBanner(),
        ...listBody('expiring', empty: l.expiringEmpty, row: _row),
      ],
    );
  }
}
