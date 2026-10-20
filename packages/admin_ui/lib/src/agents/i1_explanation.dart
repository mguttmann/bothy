// SPDX-License-Identifier: Apache-2.0

/// Why "Personal" is not possible right now: the explanation of an I1 rejection
/// (`409 i1_violation` with `conflicts`, or the violations of a grant preview) with the
/// principals besides the holder that may trigger the agent (docs/spec/10-backend-policy.md
/// 5.4; brief M4-28 fixed requirement 14).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../data/access_repository.dart';
import '../grants/grant_labels.dart';

/// The name of the person or agent [id] when the caller may read it (it is in one of the
/// directory lists of [names]), else null: 10 5.4 lets the server send IDs the caller may not
/// read, and the screen never shows a name it did not get.
String? readableName(AccessNames names, String id) =>
    names.people.where((p) => p.id == id).firstOrNull?.displayName ??
    names.agents.where((a) => a.id == id).firstOrNull?.displayName;

/// The I1 rejection of [conflicts]: per agent the holder and the other principals that may
/// trigger it, by name where [names] has one; the rest only counted ("2 more you cannot
/// see"). The way back to personal is no guided flow (10 11), so the panel ends with what the
/// owner can do instead. Entries with another code than `i1_violation` are left out.
///
/// Example: `I1Explanation(conflicts: failure.conflicts, names: names)`.
class I1Explanation extends StatelessWidget {
  /// Creates the explanation.
  const I1Explanation({
    super.key,
    required this.conflicts,
    required this.names,
  });

  /// The entries of `Problem.conflicts` or of the preview's violations.
  final List<InvariantFinding> conflicts;

  /// The names the caller may read.
  final AccessNames names;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final t = dsToneColors(context, DsTone.danger);
    final style = dsTextStyle(DsType.bodySm).copyWith(color: t.fg);
    final i1 = [
      for (final c in conflicts)
        if (c.code == 'i1_violation') c,
    ];
    return Container(
      key: const ValueKey('i1.explanation'),
      padding: const EdgeInsets.all(DsSpace.s8),
      decoration: BoxDecoration(
        color: t.subtle,
        border: Border.all(color: t.border, width: DsBorderWidth.hairline),
        borderRadius: BorderRadius.circular(DsRadius.md),
      ),
      child: Semantics(
        liveRegion: true,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(
              l.adminI1Title,
              style: style.copyWith(fontWeight: FontWeight.w600),
            ),
            for (final c in i1) _agentLine(l, c, style),
            Text(l.adminI1Way, style: style),
          ],
        ),
      ),
    );
  }

  Widget _agentLine(AppLocalizations l, InvariantFinding c, TextStyle style) {
    final agentId = c.agentId ?? '';
    final agent = readableName(names, agentId) ?? agentId;
    final others = [
      for (final p in c.principalIds)
        if (p != c.holderId) p,
    ];
    final shown = [for (final p in others) ?readableName(names, p)];
    final hidden = others.length - shown.length;
    final list = [...shown, if (hidden > 0) l.adminI1Hidden(hidden)].join(', ');
    final holder = c.holderId == null ? null : readableName(names, c.holderId!);
    return Padding(
      key: ValueKey('i1.agent.$agentId'),
      padding: const EdgeInsets.only(top: DsSpace.s4),
      child: Text(
        holder == null
            ? l.adminI1AgentNoHolder(agent, list)
            : l.adminI1Agent(agent, holder, list),
        style: style,
      ),
    );
  }
}
