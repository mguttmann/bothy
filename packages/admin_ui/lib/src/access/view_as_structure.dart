// SPDX-License-Identifier: Apache-2.0

/// The structure a person sees during "View as person" in the console (docs/spec/01-product.md
/// 5.2 A4; 07 9.2; ADR 0028; brief M6-04 Design note 3).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../data/access_repository.dart';
import '../grants/grant_labels.dart';

/// The names of the spaces among [ownerTargets] (the owner's own `listSpaces`) that are not in
/// [snapshot], sorted without regard to case, for the "Not visible" line of [ViewAsStructure].
///
/// Only targets of [AdminTargetKind.space] count: a channel belongs to a space, and the
/// instance and hosts are no part of a person's sidebar. The client compares two server
/// answers and decides nothing itself (15 1 principle 1). A space with an empty name is
/// skipped, because the line could not name it.
///
/// Example:
/// ```dart
/// final hidden = notVisibleSpaces(await admin.targets(), snapshot); // ['Board', 'Sales']
/// ```
List<String> notVisibleSpaces(
  List<AdminTarget> ownerTargets,
  ViewAsResult snapshot,
) {
  final seen = {for (final s in snapshot.spaces) s.id};
  return [
    for (final t in ownerTargets)
      if (t.kind == AdminTargetKind.space &&
          !seen.contains(t.id) &&
          t.name.isNotEmpty)
        t.name,
  ]..sort((a, b) => a.toLowerCase().compareTo(b.toLowerCase()));
}

/// The sidebar of [name] as text: each space of [snapshot] with the person's level and
/// capabilities, its channels indented below it, then the channels whose space the snapshot
/// does not list; "Nothing visible." for an empty snapshot; the spaces in [notVisible]; and
/// always the note that the console shows structure and access only.
///
/// Nothing in it can be tapped and nothing opens a channel: the console has no content routes
/// (ADR 0028), so the view shows structure and access, never messages (brief M6-04 fixed
/// requirement 14). A null [snapshot] is a view the console did not start in this tab (it
/// knows the person and the end time from `getConsoleSession` only); it then shows the note to
/// start the view again and nothing else, because the server gives the structure only on
/// `viewAsUser`.
///
/// Example:
/// ```dart
/// ViewAsStructure(name: 'Sam', snapshot: snapshot, notVisible: const ['Board']);
/// ```
class ViewAsStructure extends StatelessWidget {
  /// Creates the structure.
  const ViewAsStructure({
    super.key,
    required this.name,
    required this.snapshot,
    required this.notVisible,
  });

  /// The display name of the person.
  final String name;

  /// The answer of `viewAsUser`, or null when the host has none.
  final ViewAsResult? snapshot;

  /// The names of the spaces the person does not see ([notVisibleSpaces]).
  final List<String> notVisible;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final secondary = dsTextStyle(DsType.bodySm)
        .copyWith(color: colors.textSecondary);
    final snapshot = this.snapshot;
    if (snapshot == null) {
      return Text(
        l.adminViewAsStructureLater,
        key: const ValueKey('viewAs.later'),
        style: dsTextStyle(DsType.body).copyWith(color: colors.textSecondary),
      );
    }
    final spaceIds = {for (final s in snapshot.spaces) s.id};
    final body = dsTextStyle(DsType.body).copyWith(color: colors.textPrimary);
    Widget row(
      String key,
      String label,
      ViewAsItem item, {
      double indent = 0,
    }) => Padding(
      key: ValueKey(key),
      padding: EdgeInsets.only(
        left: indent,
        top: DsSpace.s4,
        bottom: DsSpace.s4,
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Expanded(flex: 2, child: Text(label, style: body)),
          Expanded(
            flex: 3,
            child: Text(
              levelWithCapabilities(l, item.level, item.capabilities),
              style: secondary,
            ),
          ),
        ],
      ),
    );
    return Column(
      key: const ValueKey('viewAs.structure'),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Semantics(
          header: true,
          child: Text(
            l.adminViewAsSidebarOf(name),
            style: dsTextStyle(DsType.heading)
                .copyWith(color: colors.textPrimary),
          ),
        ),
        const SizedBox(height: DsSpace.s8),
        if (snapshot.spaces.isEmpty && snapshot.channels.isEmpty)
          Text(l.adminViewAsNothing, style: secondary),
        for (final s in snapshot.spaces) ...[
          row('viewAs.space.${s.id}', s.name, s),
          for (final c in snapshot.channels)
            if (c.spaceId == s.id)
              row('viewAs.channel.${c.id}', c.name, c, indent: DsSpace.s24),
        ],
        // A channel the person sees without its space (a grant on the channel only): the
        // snapshot names no space for it, so the row carries the channel's name alone.
        for (final c in snapshot.channels)
          if (!spaceIds.contains(c.spaceId))
            row('viewAs.channel.${c.id}', c.name, c),
        if (notVisible.isNotEmpty) ...[
          const SizedBox(height: DsSpace.s8),
          Text(
            l.adminViewAsNotVisible(name, notVisible.join(', ')),
            key: const ValueKey('viewAs.notVisible'),
            style: secondary,
          ),
        ],
        const SizedBox(height: DsSpace.s8),
        Text(
          l.adminViewAsStructureOnly,
          key: const ValueKey('viewAs.structureOnly'),
          style: secondary,
        ),
      ],
    );
  }
}
