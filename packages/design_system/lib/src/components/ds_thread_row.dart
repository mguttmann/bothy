// SPDX-License-Identifier: Apache-2.0

/// A row of the forum post list (components.md section 6).
library;

import 'package:flutter/material.dart';

import '../generated/design_tokens.g.dart';
import '../icons/ds_icon.dart';
import '../theme/ds_theme.dart';
import 'ds_common.dart';
import 'view_models.dart';

/// The chip colors of a category (`category.<name>.chip-*`).
({Color bg, Color fg}) dsCategoryChip(
  BuildContext context,
  DsCategory category,
) {
  final c = context.dsColors;
  return switch (category) {
    DsCategory.spruce => (
      bg: c.categorySpruceChipBg,
      fg: c.categorySpruceChipFg,
    ),
    DsCategory.loch => (bg: c.categoryLochChipBg, fg: c.categoryLochChipFg),
    DsCategory.heather => (
      bg: c.categoryHeatherChipBg,
      fg: c.categoryHeatherChipFg,
    ),
    DsCategory.rowan => (bg: c.categoryRowanChipBg, fg: c.categoryRowanChipFg),
    DsCategory.amber => (bg: c.categoryAmberChipBg, fg: c.categoryAmberChipFg),
    DsCategory.gorse => (bg: c.categoryGorseChipBg, fg: c.categoryGorseChipFg),
    DsCategory.moss => (bg: c.categoryMossChipBg, fg: c.categoryMossChipFg),
    DsCategory.granite => (
      bg: c.categoryGraniteChipBg,
      fg: c.categoryGraniteChipFg,
    ),
  };
}

/// A forum post in the dense post list, two lines in `density.row-post`.
///
/// Left the state icon: the run icon of an active run, else the mention icon in
/// `color.attention.fg`, else nothing. The title in `type.body-sm`, bold when unread, muted when
/// done; the time on the right in `type.caption`. The second line: "Waiting for you", the
/// operator, the mention counter, the tags and an error reason. States rest, hover, selected
/// ([selected]) and focus. The label reads "{Title}, {State}, {Time}, {n} unread"; Enter opens.
///
/// ```dart
/// DsThreadRow(row, selected: row.id == current, onOpen: () => open(row.id))
/// ```
class DsThreadRow extends StatefulWidget {
  /// Creates the row.
  const DsThreadRow(
    this.data, {
    super.key,
    required this.onOpen,
    this.selected = false,
  });

  /// The post.
  final DsThreadRowData data;

  /// Opens the post.
  final VoidCallback onOpen;

  /// Whether the post is the open one.
  final bool selected;

  @override
  State<DsThreadRow> createState() => DsThreadRowState();
}

/// State of [DsThreadRow].
class DsThreadRowState extends State<DsThreadRow> {
  /// The row background of the selected state, or null.
  @visibleForTesting
  Color? get background =>
      widget.selected ? context.dsColors.surfaceSelected : null;

  @override
  Widget build(BuildContext context) {
    final l = dsL10n(context);
    final colors = context.dsColors;
    final d = widget.data;
    final unread = d.unreadCount > 0;
    final run = d.runState;
    final stateWord = switch (run) {
      final DsRunState s? when s.isActive => dsRunStateWord(
        l,
        DsRunData(id: d.id, agentName: '', state: s),
      ),
      _ => d.done ? l.threadRowStateDone : l.threadRowStateOpen,
    };
    Widget? lead;
    if (run != null && run.isActive) {
      lead = DsIcon(
        dsRunStateGlyph(run),
        size: DsIconSizeRole.sm,
        color: dsRunStateColor(context, run),
        decorative: true,
      );
    } else if (d.mentionCount > 0) {
      lead = DsIcon(
        DsIcons.runWaiting,
        size: DsIconSizeRole.sm,
        color: colors.attentionFg,
        decorative: true,
      );
    }
    final titleColor = d.done ? colors.textMuted : colors.textPrimary;
    final caption = dsTextStyle(DsType.caption)
        .copyWith(color: colors.textSecondary);
    final second = <Widget>[
      if (d.waitingForMe)
        DsBadge(
          text: l.threadRowWaitingForYou,
          glyph: DsIcons.runWaiting,
          background: colors.attentionBg,
          foreground: colors.attentionOnBg,
        ),
      if (d.operatorName != null)
        Text(l.threadRowOperator(dsIsolate(d.operatorName!)), style: caption),
      if (d.mentionCount > 0)
        DsBadge(
          text: dsFormat(context).count(d.mentionCount),
          background: colors.attentionBg,
          foreground: colors.attentionOnBg,
          semanticsLabel: l.threadRowMentions(d.mentionCount),
        ),
      for (final tag in d.tags)
        Builder(
          builder: (context) {
            final chip = dsCategoryChip(context, tag.category);
            return DsBadge(
              text: dsIsolate(tag.name),
              background: chip.bg,
              foreground: chip.fg,
              semanticsLabel: tag.name,
            );
          },
        ),
      if (d.errorText != null)
        Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            DsIcon(
              DsIcons.stateError,
              size: DsIconSizeRole.sm,
              color: colors.dangerFg,
              decorative: true,
            ),
            const SizedBox(width: DsSpace.s4),
            Text(d.errorText!, style: caption.copyWith(color: colors.dangerFg)),
          ],
        ),
    ];
    return DsTappable(
      onTap: widget.onOpen,
      label: l.threadRowLabel(
        d.title,
        stateWord,
        d.timeLabel,
        l.threadRowUnread(d.unreadCount),
      ),
      selected: widget.selected,
      button: false,
      radius: DsRadius.sm,
      child: Container(
        constraints: BoxConstraints(minHeight: context.dsDensity.rowPost),
        color: background,
        padding: const EdgeInsets.symmetric(
          horizontal: DsSpace.s8,
          vertical: DsSpace.s4,
        ),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            SizedBox(
              width: DsIconSize.sizeSm,
              child: lead == null
                  ? null
                  : Padding(
                      padding: const EdgeInsets.only(top: DsSpace.s2),
                      child: lead,
                    ),
            ),
            const SizedBox(width: DsSpace.s8),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                mainAxisSize: MainAxisSize.min,
                children: [
                  Row(
                    children: [
                      Expanded(
                        child: ExcludeSemantics(
                          child: Text(
                            dsIsolate(d.title),
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: dsTextStyle(DsType.bodySm).copyWith(
                              color: titleColor,
                              fontWeight: unread ? FontWeight.w700 : null,
                            ),
                          ),
                        ),
                      ),
                      const SizedBox(width: DsSpace.s8),
                      ExcludeSemantics(
                        child: Text(d.timeLabel, style: caption),
                      ),
                    ],
                  ),
                  if (second.isNotEmpty)
                    Padding(
                      padding: const EdgeInsets.only(top: DsSpace.s2),
                      child: Wrap(
                        spacing: DsSpace.s6,
                        runSpacing: DsSpace.s2,
                        crossAxisAlignment: WrapCrossAlignment.center,
                        children: second,
                      ),
                    ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}
