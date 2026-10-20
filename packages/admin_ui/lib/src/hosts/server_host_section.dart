// SPDX-License-Identifier: Apache-2.0

/// The server host question and its answers (docs/spec/01-product.md 5.2 A8; 24 2.3; brief
/// M4-37 Design note 1 and fixed requirement 14).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../data/hosts_repository.dart';

/// The mandatory question of the confirmation, "Does the server run on this host?", with no
/// answer preselected: the owner answers it (24 2.3 makes the server host owner only, and a
/// guessed default would declare it silently).
///
/// Example: `ServerHostChoice(value: null, onChanged: (v) => setState(() => answer = v))`.
class ServerHostChoice extends StatelessWidget {
  /// Creates the question.
  const ServerHostChoice({
    super.key,
    required this.value,
    required this.onChanged,
  });

  /// The answer, or null before one is given.
  final bool? value;

  /// Gives the answer.
  final ValueChanged<bool> onChanged;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        Text(
          l.hostServerHostQuestion,
          style: dsTextStyle(DsType.bodyStrong)
              .copyWith(color: context.dsColors.textPrimary),
        ),
        Wrap(
          spacing: DsSpace.s8,
          children: [
            for (final (answer, key, label) in [
              (true, 'serverHost.yes', l.hostYes),
              (false, 'serverHost.no', l.hostNo),
            ])
              ChoiceChip(
                key: ValueKey(key),
                label: Text(label),
                selected: value == answer,
                onSelected: (_) => onChanged(answer),
              ),
          ],
        ),
      ],
    );
  }
}

/// The server host of a confirmed [host]: the owner's declaration and the report of the host
/// service, side by side as the server returns them (24 2.3: a difference is shown, never
/// resolved by the client).
class ServerHostSection extends StatelessWidget {
  /// Creates the section.
  const ServerHostSection({super.key, required this.host});

  /// The host.
  final AdminHost host;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    String yesNo(bool v) => v ? l.hostYes : l.hostNo;
    final reported = host.reportedIsServerHost;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        Text(
          l.hostServerHostDeclared(yesNo(host.declaredServerHost)),
          key: const ValueKey('serverHost.declared'),
          style: dsTextStyle(DsType.body).copyWith(color: colors.textPrimary),
        ),
        Text(
          l.hostServerHostReported(
            reported == null ? l.hostNotReported : yesNo(reported),
          ),
          key: const ValueKey('serverHost.reported'),
          style: dsTextStyle(DsType.bodySm)
              .copyWith(color: colors.textSecondary),
        ),
      ],
    );
  }
}
