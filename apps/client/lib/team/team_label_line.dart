// SPDX-License-Identifier: Apache-2.0

/// The line under a message of a team channel (09 3.9.3, 3.9.4): "from @handle" under a
/// member's mirrored reply, "to @handle" under an owner's message that mentions members (it
/// reaches only them) and "to all members" under an owner's message without such a mention
/// (owner ruling of 2026-11-22).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import 'team_model.dart';

/// The text of [label].
String teamLabelText(AppLocalizations l, TeamMessageLabel label) =>
    switch (label) {
      FromMember(:final handles) => l.teamFromMember(
        [for (final h in handles) '@$h'].join(', '),
      ),
      ToMembers(:final handles) => l.teamToMembers(
        [for (final h in handles) '@$h'].join(', '),
      ),
      ToAllMembers() => l.teamToAllMembers,
    };

/// One caption line with [label], beside the content like the content itself.
class TeamLabelLine extends StatelessWidget {
  /// Creates the line.
  const TeamLabelLine({super.key, required this.label});

  /// The label.
  final TeamMessageLabel label;

  @override
  Widget build(BuildContext context) {
    final colors = context.dsColors;
    return Padding(
      padding: EdgeInsets.only(
        left: context.dsDensity.messageGutter + DsSpace.s8,
        right: DsSpace.s8,
      ),
      child: Row(
        children: [
          DsIcon(
            label is FromMember ? DsIcons.agent : DsIcons.group,
            size: DsIconSizeRole.sm,
            color: colors.textSecondary,
            decorative: true,
          ),
          const SizedBox(width: DsSpace.s4),
          Flexible(
            child: Text(
              teamLabelText(AppLocalizations.of(context), label),
              style: dsTextStyle(DsType.caption)
                  .copyWith(color: colors.textSecondary),
            ),
          ),
        ],
      ),
    );
  }
}
