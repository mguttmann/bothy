// SPDX-License-Identifier: Apache-2.0

/// Step 3 of the grant wizard: level, capabilities, validity and reason
/// (docs/spec/01-product.md 5.2 A2; 07 2.2, 2.3, 2.5, 9.5; brief M4-27 Design note 1).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../data/access_repository.dart';
import 'grant_labels.dart';

/// The capabilities the wizard offers; `terminal` belongs to host targets (M4-37).
const List<AccessCapability> wizardCapabilities = [
  AccessCapability.approveActions,
  AccessCapability.upload,
  AccessCapability.export,
  AccessCapability.manage,
];

/// The level (`read`, `write`, `trigger`), the capabilities, the optional last day
/// (`YYYY-MM-DD`) and the required reason. The text controllers belong to the wizard, which
/// previews the full request after every change.
class LevelStep extends StatelessWidget {
  /// Creates the step.
  const LevelStep({
    super.key,
    required this.level,
    required this.capabilities,
    required this.until,
    required this.untilValid,
    required this.reason,
    required this.onLevel,
    required this.onCapability,
  });

  /// The chosen level, or null.
  final AccessLevel? level;

  /// The chosen capabilities.
  final Set<AccessCapability> capabilities;

  /// The text of the last day.
  final TextEditingController until;

  /// Whether [until] is empty or a valid day.
  final bool untilValid;

  /// The text of the reason.
  final TextEditingController reason;

  /// Picks the level.
  final ValueChanged<AccessLevel> onLevel;

  /// Turns a capability on or off.
  final void Function(AccessCapability capability, bool on) onCapability;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final label = dsTextStyle(DsType.label)
        .copyWith(color: colors.textSecondary);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Wrap(
          spacing: DsSpace.s8,
          children: [
            for (final v in const [
              AccessLevel.read,
              AccessLevel.write,
              AccessLevel.trigger,
            ])
              ChoiceChip(
                key: ValueKey('level.${v.wire}'),
                label: Text(levelLabel(l, v)),
                selected: level == v,
                onSelected: (_) => onLevel(v),
              ),
          ],
        ),
        const SizedBox(height: DsSpace.s12),
        Text(l.adminWizardCapabilities, style: label),
        Wrap(
          spacing: DsSpace.s8,
          children: [
            for (final c in wizardCapabilities)
              FilterChip(
                key: ValueKey('capability.${c.wire}'),
                label: Text(capabilityLabel(l, c)),
                selected: capabilities.contains(c),
                onSelected: (on) => onCapability(c, on),
              ),
          ],
        ),
        TextField(
          key: const ValueKey('wizard.until'),
          controller: until,
          keyboardType: TextInputType.datetime,
          decoration: InputDecoration(
            labelText: l.adminWizardUntil,
            errorText: untilValid ? null : l.adminMemberUntilInvalid,
          ),
        ),
        TextField(
          key: const ValueKey('wizard.reason'),
          controller: reason,
          decoration: InputDecoration(labelText: l.adminReasonRequired),
        ),
      ],
    );
  }
}
