// SPDX-License-Identifier: Apache-2.0

/// Step 2 of the grant wizard: where (docs/spec/01-product.md 5.2 A2; 07 2.4; brief M4-27
/// Design note 1).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../data/access_repository.dart';
import 'grant_labels.dart';

/// The target of a grant: the instance or a space or channel from the server's lists, which
/// hold only targets the caller sees. Whether the caller may grant on the instance is the
/// server's decision, which the preview shows (07 2.6, 9.3).
class TargetStep extends StatelessWidget {
  /// Creates the step.
  const TargetStep({
    super.key,
    required this.targets,
    required this.selected,
    required this.onSelect,
  });

  /// The spaces and channels the caller sees.
  final List<AdminTarget> targets;

  /// The chosen target, or null.
  final AdminTarget? selected;

  /// Picks a target.
  final ValueChanged<AdminTarget> onSelect;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        for (final t in [AdminTarget.instance, ...targets])
          ListTile(
            key: ValueKey('target.${t.key}'),
            title: Text(targetLabel(l, t)),
            contentPadding: EdgeInsets.only(
              left: t.kind == AdminTargetKind.channel
                  ? DsSpace.s24
                  : DsSpace.s8,
            ),
            selected: t == selected,
            selectedColor: colors.textPrimary,
            selectedTileColor: colors.accentSubtle,
            onTap: () => onSelect(t),
          ),
      ],
    );
  }
}
