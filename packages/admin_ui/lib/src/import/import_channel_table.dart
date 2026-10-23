// SPDX-License-Identifier: Apache-2.0

/// The mapped channels of an import source with their lead actions (docs/spec/
/// 22-import-migration.md 12.1 and 13; brief M8-08 Design note 3).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import 'import_controller.dart';
import 'import_labels.dart';
import 'import_repository.dart';

/// One row per mapped channel whose target is not `skip`: the Discord ID, the Bothy channel,
/// the phase (not imported, mirror, Bothy leads with the operator check), the read-only state
/// and the planned operator.
///
/// A row offers lead actions only while the channel is imported and [actionable] (the source
/// is neither `completed` nor `failed`): "Bothy leads" on a mirror, "Discord leads" and, when
/// the operator is not the planned one, "Set operator" while Bothy leads. The actions run in
/// the caller; [busy] turns them off.
///
/// Keys: `import.channels`, `import.channel.<discord id>`, `.name`, `.phase`, `.readOnly`,
/// `.planned`, `.leadBothy`, `.leadDiscord`, `.setOperator`.
class ImportChannelTable extends StatelessWidget {
  /// Creates the table.
  const ImportChannelTable({
    super.key,
    required this.rows,
    required this.actionable,
    required this.onLeadBothy,
    required this.onLeadDiscord,
    required this.onSetOperator,
    this.busy = false,
  });

  /// The rows in the mapping's order.
  final List<ImportChannelRow> rows;

  /// Whether the source still takes lead changes.
  final bool actionable;

  /// Hands the lead of a row to Bothy.
  final ValueChanged<ImportChannelRow> onLeadBothy;

  /// Hands the lead of a row back to Discord.
  final ValueChanged<ImportChannelRow> onLeadDiscord;

  /// Opens the operator step of a row led by Bothy.
  final ValueChanged<ImportChannelRow> onSetOperator;

  /// Whether a change runs.
  final bool busy;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final secondary = dsTextStyle(DsType.bodySm)
        .copyWith(color: colors.textSecondary);
    return Column(
      key: const ValueKey('import.channels'),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Semantics(
          header: true,
          child: Text(
            l.importChannelMapping,
            style: dsTextStyle(DsType.heading)
                .copyWith(color: colors.textPrimary),
          ),
        ),
        const SizedBox(height: DsSpace.s8),
        if (rows.isEmpty) Text(l.importNoChannels, style: secondary),
        for (final r in rows) _row(context, l, r, secondary),
      ],
    );
  }

  Widget _row(
    BuildContext context,
    AppLocalizations l,
    ImportChannelRow r,
    TextStyle secondary,
  ) {
    final colors = context.dsColors;
    final id = r.mapped.discordId;
    final key = 'import.channel.$id';
    final phase = r.phase;
    final channel = r.channel;
    final planned = r.plannedAgent?.displayName ?? r.mapped.plannedAgentHandle;
    final canAct = actionable && r.imported && channel != null && !busy;
    return Container(
      key: ValueKey(key),
      margin: const EdgeInsets.only(bottom: DsSpace.s8),
      padding: const EdgeInsets.all(DsSpace.s12),
      decoration: BoxDecoration(
        border: Border.all(
          color: colors.borderSubtle,
          width: DsBorderWidth.hairline,
        ),
        borderRadius: BorderRadius.circular(DsRadius.md),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(
            r.bothyName ?? l.importChannelUnknownName,
            key: ValueKey('$key.name'),
            style: dsTextStyle(DsType.body).copyWith(color: colors.textPrimary),
          ),
          Text(l.importChannelDiscord(id), style: secondary),
          const SizedBox(height: DsSpace.s4),
          Text(
            importPhaseLabel(l, phase),
            key: ValueKey('$key.phase'),
            style: dsTextStyle(DsType.bodySm)
                .copyWith(color: colors.textPrimary),
          ),
          if (channel != null)
            Text(
              channel.readOnly ? l.importReadOnly : l.importWritable,
              key: ValueKey('$key.readOnly'),
              style: secondary,
            ),
          Text(
            planned == null ? l.importPlannedNone : l.importPlanned(planned),
            key: ValueKey('$key.planned'),
            style: secondary,
          ),
          if (r.imported && r.channelError != null)
            Text(l.importChannelUnreadable, style: secondary),
          if (actionable && r.imported) ...[
            const SizedBox(height: DsSpace.s8),
            Wrap(
              spacing: DsSpace.s8,
              runSpacing: DsSpace.s8,
              children: [
                if (r.mapped.lead == ImportLead.discord)
                  FilledButton(
                    key: ValueKey('$key.leadBothy'),
                    onPressed: canAct ? () => onLeadBothy(r) : null,
                    child: Text(l.importLeadBothyAction),
                  )
                else ...[
                  if (phase != ImportChannelPhase.bothyOperatorOk)
                    FilledButton(
                      key: ValueKey('$key.setOperator'),
                      onPressed: canAct ? () => onSetOperator(r) : null,
                      child: Text(l.importSetOperatorAction),
                    ),
                  OutlinedButton(
                    key: ValueKey('$key.leadDiscord'),
                    onPressed: canAct ? () => onLeadDiscord(r) : null,
                    child: Text(l.importLeadDiscordAction),
                  ),
                ],
              ],
            ),
          ],
        ],
      ),
    );
  }
}
