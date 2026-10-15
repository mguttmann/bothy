// SPDX-License-Identifier: Apache-2.0

/// One step of the live box (components.md section 3).
library;

import 'package:flutter/material.dart';

import '../generated/design_tokens.g.dart';
import '../icons/ds_icon.dart';
import '../theme/ds_theme.dart';
import 'ds_common.dart';
import 'view_models.dart';

/// A step of a run (`run_steps`): the `step.*` icon of its kind in the color of its status,
/// the name in `type.code-sm`, the title in `color.text.secondary` and the suffix of the
/// status (components.md section 3).
///
/// Click, Enter or Space opens [DsStepData.detail] in a disclosure below the chip. The label
/// reads "<Kind> <Name>, <Status>, <Duration>". A running step's icon spins in full motion.
///
/// ```dart
/// DsToolChip(step)
/// ```
class DsToolChip extends StatefulWidget {
  /// Creates the chip of [step].
  const DsToolChip(this.step, {super.key});

  /// The step.
  final DsStepData step;

  @override
  State<DsToolChip> createState() => _DsToolChipState();
}

class _DsToolChipState extends State<DsToolChip> {
  bool _open = false;

  @override
  Widget build(BuildContext context) {
    final l = dsL10n(context);
    final f = dsFormat(context);
    final colors = context.dsColors;
    final step = widget.step;
    final duration = step.duration == null ? null : f.durationShort(step.duration!);
    final (Color color, String suffix, String status) = switch (step.status) {
      DsStepStatus.pending => (colors.runQueued, l.toolChipWaiting, l.toolChipWaiting),
      DsStepStatus.running => (
        context.dsAccent.runRunning,
        l.toolChipRunning(duration ?? ''),
        l.toolChipRunning(duration ?? ''),
      ),
      DsStepStatus.completed => (
        colors.runCompleted,
        duration ?? '',
        duration == null ? l.toolChipCompleted : '${l.toolChipCompleted}, $duration',
      ),
      DsStepStatus.failed => (colors.runFailed, l.toolChipFailed, l.toolChipFailed),
      DsStepStatus.cancelled => (
        colors.runCancelled,
        l.toolChipCancelled,
        l.toolChipCancelled,
      ),
      DsStepStatus.unknown => (
        colors.textSecondary,
        l.toolChipUnknownStatus(''),
        l.toolChipUnknownStatus(''),
      ),
    };
    final (glyph, kind) = switch (step.kind) {
      DsStepKind.toolCall => (DsIcons.stepToolCall, l.toolChipKindToolCall),
      DsStepKind.status => (DsIcons.stepStatus, l.toolChipKindStatus),
      DsStepKind.subagent => (DsIcons.stepSubagent, l.toolChipKindSubagent),
      DsStepKind.workflow => (DsIcons.stepWorkflow, l.toolChipKindWorkflow),
      DsStepKind.backgroundJob => (
        DsIcons.stepBackgroundJob,
        l.toolChipKindBackgroundJob,
      ),
      DsStepKind.unknown => (DsIcons.stepStatus, l.toolChipKindUnknown),
    };
    // A running step spins with the run icon; the kind icon stays for the other states.
    final icon = step.status == DsStepStatus.running ? DsIcons.runRunning : glyph;
    final hasDetail = step.detail != null && step.detail!.isNotEmpty;
    final chip = Container(
      padding: const EdgeInsets.symmetric(horizontal: DsSpace.s8, vertical: DsSpace.s4),
      decoration: BoxDecoration(
        color: colors.surfaceSunken,
        borderRadius: BorderRadius.circular(DsRadius.sm),
      ),
      child: Row(
        children: [
          DsIcon(icon, size: DsIconSizeRole.sm, color: color, decorative: true),
          const SizedBox(width: DsSpace.s8),
          Text(
            step.name,
            style: dsTextStyle(DsType.codeSm).copyWith(color: colors.textPrimary),
          ),
          if (step.title != null) ...[
            const SizedBox(width: DsSpace.s8),
            Expanded(
              child: Text(
                step.title!,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: dsTextStyle(DsType.bodySm).copyWith(color: colors.textSecondary),
              ),
            ),
          ] else
            const Spacer(),
          const SizedBox(width: DsSpace.s8),
          Text(
            suffix,
            style: dsTextStyle(DsType.codeSm).copyWith(color: colors.textSecondary),
          ),
        ],
      ),
    );
    return Padding(
      padding: EdgeInsets.only(left: DsSpace.s16 * step.depth),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          DsTappable(
            onTap: hasDetail ? () => setState(() => _open = !_open) : null,
            button: hasDetail,
            label: l.toolChipLabel(kind, step.name, status),
            hint: hasDetail ? l.toolChipDetails : null,
            expanded: hasDetail ? _open : null,
            excludeChildSemantics: true,
            child: chip,
          ),
          if (_open && hasDetail)
            Container(
              margin: const EdgeInsets.only(top: DsSpace.s4),
              padding: const EdgeInsets.all(DsSpace.s8),
              decoration: BoxDecoration(
                color: colors.codeBg,
                borderRadius: BorderRadius.circular(DsRadius.sm),
              ),
              child: SingleChildScrollView(
                scrollDirection: Axis.horizontal,
                child: Text(
                  step.detail!,
                  softWrap: false,
                  style: dsTextStyle(DsType.codeSm).copyWith(color: colors.codeFg),
                ),
              ),
            ),
        ],
      ),
    );
  }
}
