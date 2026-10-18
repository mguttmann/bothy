// SPDX-License-Identifier: Apache-2.0

/// The reaction picker (01 3.3: reactions with any Unicode character): the reactions already
/// on the message, which toggle, and a field for any character of the system's emoji input.
library;

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

/// The chosen reaction and whether the reader removes it.
final class ReactionChoice {
  /// Creates the choice.
  const ReactionChoice(this.emoji, {required this.remove});

  /// The character.
  final String emoji;

  /// Whether the reader's reaction is removed (it was set).
  final bool remove;
}

/// Shows the picker with the [existing] reactions of the message; returns the choice, or null.
///
/// Example: `final choice = await pickReaction(context, reactions);`
Future<ReactionChoice?> pickReaction(
  BuildContext context,
  List<ReactionSummary> existing,
) => showDialog<ReactionChoice>(
  context: context,
  builder: (context) => ReactionPicker(existing: existing),
);

/// The dialog of [pickReaction].
class ReactionPicker extends StatefulWidget {
  /// Creates the picker.
  const ReactionPicker({super.key, required this.existing});

  /// The reactions on the message.
  final List<ReactionSummary> existing;

  @override
  State<ReactionPicker> createState() => _ReactionPickerState();
}

class _ReactionPickerState extends State<ReactionPicker> {
  final TextEditingController _input = TextEditingController();

  @override
  void dispose() {
    _input.dispose();
    super.dispose();
  }

  void _add() {
    final emoji = _input.text.trim();
    if (emoji.isEmpty) return;
    final mine = widget.existing.any((r) => r.emoji == emoji && r.me);
    Navigator.of(context).pop(ReactionChoice(emoji, remove: mine));
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    return AlertDialog(
      key: const ValueKey('reaction.dialog'),
      title: Text(l.messageReactionTitle),
      content: SizedBox(
        width: DsLayout.listWidth,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            if (widget.existing.isNotEmpty)
              Wrap(
                spacing: DsSpace.s6,
                runSpacing: DsSpace.s6,
                children: [
                  for (final r in widget.existing)
                    FilterChip(
                      key: ValueKey('reaction.chip.${r.emoji}'),
                      label: Text('${r.emoji} ${r.count}'),
                      selected: r.me,
                      onSelected: (_) =>
                          Navigator.of(context)
                              .pop(ReactionChoice(r.emoji, remove: r.me)),
                    ),
                ],
              ),
            const SizedBox(height: DsSpace.s12),
            TextField(
              key: const ValueKey('reaction.input'),
              controller: _input,
              autofocus: true,
              decoration: InputDecoration(labelText: l.messageReactionInput),
              onSubmitted: (_) => _add(),
            ),
          ],
        ),
      ),
      actions: [
        DsButton(
          key: const ValueKey('reaction.cancel'),
          label: l.threadCancel,
          style: DsButtonStyle.ghost,
          onPressed: () => Navigator.of(context).pop(),
        ),
        DsButton(
          key: const ValueKey('reaction.add'),
          label: l.messageReactionAdd,
          style: DsButtonStyle.primary,
          onPressed: _add,
        ),
      ],
    );
  }
}
