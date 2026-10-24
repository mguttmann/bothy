// SPDX-License-Identifier: Apache-2.0

/// The header of a thread or of the main history of a channel (brief M4-20 Design note 5;
/// 01 5.1 D1 and D2, 6.1, 6.7): the title with its "automatic" mark, the operator or "No agent
/// answers here", Done or Reopen, Move, Pin, Column and the menu of M4-33. On the phone
/// ([ThreadHeader.compact], 01 5.3 M3) the actions sit in one overflow menu next to the title.
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

/// What the header shows; built by the history column from the stored thread and channel.
final class ThreadHeaderData {
  /// Creates the data.
  const ThreadHeaderData({
    required this.title,
    required this.automaticTitle,
    required this.operatorName,
    required this.mainHistory,
    required this.done,
    this.canEditTitle = false,
    this.canSetDone = false,
    this.canMove = false,
  });

  /// The thread title, or the channel name of a main history.
  final String title;

  /// Whether the server set the title (`title_source = auto`).
  final bool automaticTitle;

  /// The effective operator's name, or null when no agent answers here.
  final String? operatorName;

  /// Whether this is the main history of a text channel (no Done, title or move).
  final bool mainHistory;

  /// Whether the thread is done (`state = closed`).
  final bool done;

  /// Whether the title may be edited (05 `thread.title.update`).
  final bool canEditTitle;

  /// Whether Done and Reopen are offered (05 `thread.close`, `thread.reopen`: write).
  final bool canSetDone;

  /// Whether Move is offered (05 `thread.move`: manage).
  final bool canMove;
}

/// The header bar.
class ThreadHeader extends StatelessWidget {
  /// Creates the header.
  const ThreadHeader({
    super.key,
    required this.data,
    required this.onEditTitle,
    required this.onDone,
    required this.onMove,
    required this.onPin,
    required this.onColumn,
    this.menu,
    this.compact = false,
  });

  /// What it shows.
  final ThreadHeaderData data;

  /// Opens the title editor.
  final VoidCallback onEditTitle;

  /// Done, or Reopen for a done thread.
  final VoidCallback onDone;

  /// Opens the move dialog.
  final VoidCallback onMove;

  /// Shows the pinned messages in the session column.
  final VoidCallback onPin;

  /// Shows or hides the session column.
  final VoidCallback onColumn;

  /// The button that opens the menu of the thread or of the channel (M4-33), or null.
  final Widget? menu;

  /// The phone form: Edit title, Done or Reopen, Move and the session ([onPin], a sheet on
  /// the phone) in one overflow menu `thread.more`; there is no session column to toggle.
  final bool compact;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final d = data;
    Widget action(String key, DsGlyph glyph, String label, VoidCallback tap) =>
        Tooltip(
          message: label,
          excludeFromSemantics: true,
          child: DsTappable(
            key: ValueKey(key),
            onTap: tap,
            label: label,
            excludeChildSemantics: true,
            child: Padding(
              padding: const EdgeInsets.symmetric(
                horizontal: DsSpace.s6,
                vertical: DsSpace.s4,
              ),
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  DsIcon(
                    glyph,
                    size: DsIconSizeRole.sm,
                    color: colors.textSecondary,
                    decorative: true,
                  ),
                  const SizedBox(width: DsSpace.s4),
                  Text(
                    label,
                    style: dsTextStyle(DsType.label)
                        .copyWith(color: colors.textSecondary),
                  ),
                ],
              ),
            ),
          ),
        );
    final operator = d.operatorName;
    if (compact) return _compact(context, l);
    return Container(
      key: const ValueKey('thread.header'),
      padding: const EdgeInsets.symmetric(
        horizontal: DsSpace.s16,
        vertical: DsSpace.s8,
      ),
      decoration: BoxDecoration(
        border: Border(
          bottom: BorderSide(
            color: colors.borderSubtle,
            width: DsBorderWidth.hairline,
          ),
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          Wrap(
            spacing: DsSpace.s8,
            runSpacing: DsSpace.s4,
            crossAxisAlignment: WrapCrossAlignment.center,
            children: [
              Semantics(
                header: true,
                child: Text(
                  dsIsolate(d.mainHistory ? '# ${d.title}' : d.title),
                  key: const ValueKey('thread.title'),
                  style: dsTextStyle(DsType.heading)
                      .copyWith(color: colors.textPrimary),
                ),
              ),
              if (d.automaticTitle && !d.mainHistory)
                DsBadge(
                  key: const ValueKey('thread.automatic'),
                  text: l.threadTitleAutomatic,
                  background: colors.surfaceSunken,
                  foreground: colors.textSecondary,
                ),
              if (d.canEditTitle && !d.mainHistory)
                action(
                  'thread.editTitle',
                  DsIcons.actionEdit,
                  l.threadEditTitle,
                  onEditTitle,
                ),
            ],
          ),
          const SizedBox(height: DsSpace.s4),
          Wrap(
            spacing: DsSpace.s8,
            runSpacing: DsSpace.s4,
            crossAxisAlignment: WrapCrossAlignment.center,
            children: [
              Text(
                operator == null
                    ? l.threadNoOperator
                    : l.threadOperator(dsIsolate(operator)),
                key: ValueKey(
                  operator == null ? 'thread.noOperator' : 'thread.operator',
                ),
                style: dsTextStyle(DsType.caption)
                    .copyWith(color: colors.textSecondary),
              ),
              if (!d.mainHistory && d.canSetDone)
                d.done
                    ? action(
                        'thread.reopen',
                        DsIcons.actionReopen,
                        l.threadReopen,
                        onDone,
                      )
                    : action(
                        'thread.done',
                        DsIcons.actionCloseThread,
                        l.threadDone,
                        onDone,
                      ),
              if (!d.mainHistory && d.canMove)
                action(
                  'thread.move',
                  DsIcons.actionShare,
                  l.threadMove,
                  onMove,
                ),
              action('thread.pin', DsIcons.actionPin, l.threadPin, onPin),
              action(
                'thread.column',
                DsIcons.actionTogglePanel,
                l.threadColumn,
                onColumn,
              ),
              ?menu,
            ],
          ),
        ],
      ),
    );
  }

  Widget _compact(BuildContext context, AppLocalizations l) {
    final colors = context.dsColors;
    final d = data;
    final operator = d.operatorName;
    final items = <(String, String, VoidCallback)>[
      if (d.canEditTitle && !d.mainHistory)
        ('thread.editTitle', l.threadEditTitle, onEditTitle),
      if (!d.mainHistory && d.canSetDone)
        d.done
            ? ('thread.reopen', l.threadReopen, onDone)
            : ('thread.done', l.threadDone, onDone),
      if (!d.mainHistory && d.canMove) ('thread.move', l.threadMove, onMove),
      ('thread.session', l.shellColumnSession, onPin),
    ];
    return Container(
      key: const ValueKey('thread.header'),
      padding: const EdgeInsets.fromLTRB(
        DsSpace.s16,
        DsSpace.s4,
        DsSpace.s4,
        DsSpace.s4,
      ),
      decoration: BoxDecoration(
        border: Border(
          bottom: BorderSide(
            color: colors.borderSubtle,
            width: DsBorderWidth.hairline,
          ),
        ),
      ),
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Semantics(
                  header: true,
                  child: Text(
                    dsIsolate(d.mainHistory ? '# ${d.title}' : d.title),
                    key: const ValueKey('thread.title'),
                    style: dsTextStyle(DsType.heading)
                        .copyWith(color: colors.textPrimary),
                  ),
                ),
                Text(
                  operator == null
                      ? l.threadNoOperator
                      : l.threadOperator(dsIsolate(operator)),
                  key: ValueKey(
                    operator == null ? 'thread.noOperator' : 'thread.operator',
                  ),
                  style: dsTextStyle(DsType.caption)
                      .copyWith(color: colors.textSecondary),
                ),
              ],
            ),
          ),
          ?menu,
          PopupMenuButton<VoidCallback>(
            key: const ValueKey('thread.more'),
            tooltip: l.phoneMoreActions,
            icon: DsIcon(
              DsIcons.actionMore,
              size: DsIconSizeRole.md,
              color: colors.textSecondary,
              decorative: true,
            ),
            onSelected: (action) => action(),
            itemBuilder: (context) => [
              for (final (key, label, action) in items)
                PopupMenuItem<VoidCallback>(
                  key: ValueKey(key),
                  value: action,
                  child: Text(label),
                ),
            ],
          ),
        ],
      ),
    );
  }
}
