// SPDX-License-Identifier: Apache-2.0

/// The list "Permission hints" (docs/spec/07-permissions.md 6, 9.5; brief M6-07 Design note 4):
/// H1, grants to disabled principals, `trigger` grants that SR2 or SR3 blocks, and agents in
/// mode `personal` with their trigger set, as the server computes them.
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/admin_action.dart';
import '../data/access_repository.dart';
import '../shell/admin_routes.dart';
import 'access_list.dart';
import 'hint_texts.dart';

/// "Permission hints": the filters (the instance or one space, one kind or all), the hints of
/// `listPermissionHints` with "Load more", and per hint its kind as icon and text and its
/// sentence from the templates of `l10n` in the caller's language, filled with the names of the
/// page ("Sam can learn content of Marketing/Intern through agent Texter.").
///
/// Actions: "Explain" opens "Who has access?" of the hint's channel (H1) or of the grant's space
/// or channel; a grant has "Revoke" and, when it has an end, "Extend"; an agent in mode
/// `personal` opens its page, where its mode and holder change (M4-28). A hint is not a block
/// (07 6): the screen only lists what the server answers, and a refused list (`403`, `404`)
/// shows the translated reason and keeps the filters (15 1 principle 1).
///
/// Example:
/// ```dart
/// PermissionHintsView(repository: admin, onNavigate: (to) => go('/admin${to.path}'));
/// ```
class PermissionHintsView extends AccessListView {
  /// Creates the list.
  const PermissionHintsView({
    super.key,
    required super.repository,
    required super.onNavigate,
    super.timeZone,
    super.clock,
    super.onChanged,
    super.onViewAs,
  });

  @override
  State<PermissionHintsView> createState() => _PermissionHintsViewState();
}

class _PermissionHintsViewState extends State<PermissionHintsView>
    with
        AdminActionState<PermissionHintsView>,
        AccessListState<PermissionHintsView, AccessHint> {
  AccessHintKind? _kind;

  @override
  Future<AccessListPage<AccessHint>> fetch(String? cursor) async {
    final page = await widget.repository.permissionHints(
      spaceId: spaceId,
      kind: _kind,
      cursor: cursor,
    );
    return (
      items: page.items,
      labels: page.labels,
      nextCursor: page.nextCursor,
    );
  }

  /// The key of [hint]'s row: its kind and the IDs that make it unique (07 9.5).
  static String _keyOf(AccessHint hint) => switch (hint.kind) {
    AccessHintKind.h1ReadHorizon =>
      'hint.${hint.kind.wire}.${hint.agentId}.${hint.personId}.${hint.channelId}',
    AccessHintKind.agentPersonal => 'hint.${hint.kind.wire}.${hint.agentId}',
    _ => 'hint.${hint.kind.wire}.${hint.grant?.id ?? hint.agentId}',
  };

  List<Widget> _actions(String key, AccessHint hint) {
    final l = AppLocalizations.of(context);
    final grant = hint.grant;
    if (grant != null) return grantActions(key, grant);
    return switch (hint.kind) {
      AccessHintKind.h1ReadHorizon when hint.channelId != null => [
        ?explainAction(
          key,
          labels.target(AdminTargetKind.channel, hint.channelId),
        ),
      ],
      AccessHintKind.agentPersonal when hint.agentId != null => [
        TextButton(
          key: ValueKey('$key.agent'),
          onPressed: () => widget.onNavigate(
            AdminLocation(AdminSection.agents, id: hint.agentId),
          ),
          child: Text(l.hintsOpenAgent),
        ),
      ],
      _ => const [],
    };
  }

  Widget _row(AccessHint hint) {
    final l = AppLocalizations.of(context);
    final key = _keyOf(hint);
    return accessRow(
      key: key,
      glyph: hintKindGlyph(hint.kind),
      kind: hintKindLabel(l, hint.kind),
      lines: hintLines(l, hint, labels),
      actions: _actions(key, hint),
    );
  }

  Widget _kindFilter() {
    final l = AppLocalizations.of(context);
    return filterField(
      l.hintsKindLabel,
      DropdownButton<AccessHintKind?>(
        key: const ValueKey('hints.kind'),
        isExpanded: true,
        value: _kind,
        items: [
          DropdownMenuItem<AccessHintKind?>(
            key: const ValueKey('hints.kind.all'),
            child: Text(l.hintsKindAll),
          ),
          for (final k in AccessHintKind.listed)
            DropdownMenuItem<AccessHintKind?>(
              key: ValueKey('hints.kind.${k.wire}'),
              value: k,
              child: Text(hintKindLabel(l, k)),
            ),
        ],
        onChanged: (k) {
          if (k == _kind) return;
          _kind = k;
          reload();
        },
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    return Column(
      key: const ValueKey('hints.view'),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        heading(l.hintsTitle, l.hintsIntro),
        const SizedBox(height: DsSpace.s12),
        scopeFilter('hints'),
        const SizedBox(height: DsSpace.s8),
        _kindFilter(),
        const SizedBox(height: DsSpace.s12),
        noticeBanner(),
        ...listBody('hints', empty: l.hintsEmpty, row: _row),
      ],
    );
  }
}
