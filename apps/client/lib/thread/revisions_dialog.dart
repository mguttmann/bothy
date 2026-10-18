// SPDX-License-Identifier: Apache-2.0

/// The edit conflict (04 13.3: `412 version_conflict`): both versions side by side and the
/// person decides; no automatic merge (docs/DECISIONS.md D-M4-20-3).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

/// What the person chose in the conflict dialog.
enum ConflictChoice {
  /// Send the own text again on the current version.
  keepMine,

  /// Keep the server's text and drop the own edit.
  takeTheirs,
}

/// Shows [mine] (the local text) and [theirs] (the server's current revision); returns the
/// choice, or null when the person closes the dialog without one.
///
/// Example: `final choice = await showConflict(context, mine: text, theirs: current);`
Future<ConflictChoice?> showConflict(
  BuildContext context, {
  required String mine,
  required String theirs,
}) => showDialog<ConflictChoice>(
  context: context,
  builder: (context) => RevisionsDialog(mine: mine, theirs: theirs),
);

/// The dialog of [showConflict].
class RevisionsDialog extends StatelessWidget {
  /// Creates the dialog.
  const RevisionsDialog({super.key, required this.mine, required this.theirs});

  /// The local text of the refused edit.
  final String mine;

  /// The text of the server's current revision.
  final String theirs;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    Widget side(Key key, String heading, String text) => Expanded(
      child: Column(
        key: key,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(heading, style: dsTextStyle(DsType.label)),
          const SizedBox(height: DsSpace.s4),
          SelectableText(text, style: dsTextStyle(DsType.body)),
        ],
      ),
    );
    return AlertDialog(
      key: const ValueKey('conflict.dialog'),
      title: Text(l.messageConflictTitle),
      content: SizedBox(
        width: DsLayout.listWidth * 2,
        child: SingleChildScrollView(
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              side(
                const ValueKey('conflict.mine'),
                l.messageConflictMine,
                mine,
              ),
              const SizedBox(width: DsSpace.s16),
              side(
                const ValueKey('conflict.theirs'),
                l.messageConflictTheirs,
                theirs,
              ),
            ],
          ),
        ),
      ),
      actions: [
        DsButton(
          key: const ValueKey('conflict.takeTheirs'),
          label: l.messageConflictTakeTheirs,
          style: DsButtonStyle.secondary,
          onPressed: () => Navigator.of(context).pop(ConflictChoice.takeTheirs),
        ),
        DsButton(
          key: const ValueKey('conflict.keepMine'),
          label: l.messageConflictKeepMine,
          style: DsButtonStyle.primary,
          onPressed: () => Navigator.of(context).pop(ConflictChoice.keepMine),
        ),
      ],
    );
  }
}
