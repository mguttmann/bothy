// SPDX-License-Identifier: Apache-2.0

/// Moving a thread (01 6.7, M1-62): the dialog shows only channels with the same effective
/// operator, the thread ID stays.
library;

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

/// The target and reason of a move.
final class MoveChoice {
  /// Creates the choice.
  const MoveChoice(this.target, this.reason);

  /// The target channel.
  final ChannelId target;

  /// The reason (`ThreadMove.reason`).
  final String reason;
}

/// The channels of [channels] a thread with the effective operator [operator] may move to: not
/// [from], no direct message, the same effective operator (an absent one equals an absent
/// one). The server checks again (`operator_mismatch`).
///
/// Example: `moveTargets(visible, from: thread.channelId, operator: robin)`.
List<Channel> moveTargets(
  Iterable<Channel> channels, {
  required ChannelId from,
  required PrincipalId? operator,
}) => [
  for (final c in channels)
    if (c.id != from &&
        c.kind != ChannelKind.dm &&
        c.archivedAt == null &&
        c.operator.effectiveAgentId == operator)
      c,
];

/// Shows the dialog with [targets]; returns the choice, or null when the person cancels.
Future<MoveChoice?> showMoveThread(
  BuildContext context,
  List<Channel> targets,
) => showDialog<MoveChoice>(
  context: context,
  builder: (context) => MoveThreadDialog(targets: targets),
);

/// The dialog of [showMoveThread].
class MoveThreadDialog extends StatefulWidget {
  /// Creates the dialog.
  const MoveThreadDialog({super.key, required this.targets});

  /// The valid targets.
  final List<Channel> targets;

  @override
  State<MoveThreadDialog> createState() => _MoveThreadDialogState();
}

class _MoveThreadDialogState extends State<MoveThreadDialog> {
  final TextEditingController _reason = TextEditingController();
  ChannelId? _target;

  @override
  void initState() {
    super.initState();
    _reason.addListener(() => setState(() {}));
  }

  @override
  void dispose() {
    _reason.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final target = _target;
    final reason = _reason.text.trim();
    return AlertDialog(
      key: const ValueKey('move.dialog'),
      title: Text(l.threadMove),
      content: SizedBox(
        width: DsLayout.listWidth * 1.5,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            if (widget.targets.isEmpty)
              Text(key: const ValueKey('move.empty'), l.threadMoveNoTarget)
            else
              ConstrainedBox(
                constraints: const BoxConstraints(
                  maxHeight: DsLayout.listWidth,
                ),
                child: ListView(
                  shrinkWrap: true,
                  children: [
                    for (final c in widget.targets)
                      ListTile(
                        key: ValueKey('move.channel.${c.id.value}'),
                        title: Text(dsIsolate(c.name)),
                        selected: c.id == target,
                        onTap: () => setState(() => _target = c.id),
                      ),
                  ],
                ),
              ),
            const SizedBox(height: DsSpace.s12),
            TextField(
              key: const ValueKey('move.reason'),
              controller: _reason,
              decoration: InputDecoration(labelText: l.threadMoveReason),
            ),
          ],
        ),
      ),
      actions: [
        DsButton(
          key: const ValueKey('move.cancel'),
          label: l.threadCancel,
          style: DsButtonStyle.ghost,
          onPressed: () => Navigator.of(context).pop(),
        ),
        DsButton(
          key: const ValueKey('move.confirm'),
          label: l.threadMove,
          style: DsButtonStyle.primary,
          onPressed: target == null || reason.isEmpty
              ? null
              : () => Navigator.of(context).pop(MoveChoice(target, reason)),
        ),
      ],
    );
  }
}
