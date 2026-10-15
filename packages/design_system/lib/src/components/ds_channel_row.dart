// SPDX-License-Identifier: Apache-2.0

/// A channel row of the sidebar tree (components.md section 7).
library;

import 'package:flutter/material.dart';

import '../generated/design_tokens.g.dart';
import '../icons/ds_icon.dart';
import '../theme/ds_theme.dart';
import 'ds_common.dart';
import 'view_models.dart';

/// A channel in the sidebar, one line in `density.row-channel`.
///
/// The icon follows the kind (`channel.text`, `channel.forum`, `channel.dm`), the name in
/// `type.body-sm`. States of the table in components.md 7: read
/// (`color.text.on-sidebar-secondary`), unread (`color.text.on-sidebar`, bold, bar on the
/// left), mentions (counter on `color.attention.bg`), active run (dot
/// `category.spruce.on-sidebar` that pulses with `motion.duration.pulse`, still in reduced
/// motion), muted (`color.text.on-sidebar-muted` and the `action.mute` icon), selected
/// (`color.surface.sidebar-selected`), hover (`color.surface.sidebar-hover`) and focus (ring
/// `color.focus.ring-on-sidebar`). The label reads "{Name}, {n} unread, {m} mentions, run
/// active"; Enter or a tap calls [onOpen].
///
/// ```dart
/// DsChannelRow(channel, selected: channel.id == current, onOpen: () => open(channel.id))
/// ```
class DsChannelRow extends StatefulWidget {
  /// Creates the row of [data].
  const DsChannelRow(
    this.data, {
    super.key,
    required this.onOpen,
    this.selected = false,
  });

  /// The channel.
  final DsChannelRowData data;

  /// Opens the channel.
  final VoidCallback onOpen;

  /// Whether the channel is the open one.
  final bool selected;

  @override
  State<DsChannelRow> createState() => DsChannelRowState();
}

/// State of [DsChannelRow]; exposes the resolved state colors to tests.
class DsChannelRowState extends State<DsChannelRow> {
  /// The name color of the current state.
  @visibleForTesting
  Color get nameColor {
    final c = context.dsColors;
    if (widget.data.muted) return c.textOnSidebarMuted;
    if (widget.data.unreadCount > 0) return c.textOnSidebar;
    return c.textOnSidebarSecondary;
  }

  /// Whether the unread bar on the left shows.
  @visibleForTesting
  bool get unreadBar => !widget.data.muted && widget.data.unreadCount > 0;

  /// The row background of the selected state, or null.
  @visibleForTesting
  Color? get background =>
      widget.selected ? context.dsColors.surfaceSidebarSelected : null;

  @override
  Widget build(BuildContext context) {
    final l = dsL10n(context);
    final c = context.dsColors;
    final d = widget.data;
    final glyph = switch (d.kind) {
      DsChannelKind.forum => DsIcons.channelForum,
      DsChannelKind.dm => DsIcons.channelDm,
      DsChannelKind.text || DsChannelKind.unknown => DsIcons.channelText,
    };
    final label = [
      d.name,
      if (d.unreadCount > 0) l.threadRowUnread(d.unreadCount),
      if (d.mentionCount > 0) l.threadRowMentions(d.mentionCount),
      if (d.runActive) l.channelRowRunActive,
      if (d.muted) l.channelRowMuted,
    ].join(', ');
    final color = nameColor;
    return DsTappable(
      onTap: widget.onOpen,
      label: label,
      selected: widget.selected,
      button: false,
      onSidebar: true,
      hoverColor: c.surfaceSidebarHover,
      excludeChildSemantics: true,
      child: Container(
        constraints: BoxConstraints(minHeight: context.dsDensity.rowChannel),
        decoration: BoxDecoration(
          color: background,
          borderRadius: BorderRadius.circular(DsRadius.sm),
          border: unreadBar
              ? Border(
                  left: BorderSide(
                    color: c.textOnSidebar,
                    width: DsBorderWidth.spine,
                  ),
                )
              : null,
        ),
        padding: const EdgeInsets.symmetric(horizontal: DsSpace.s8),
        child: Row(
          children: [
            DsIcon(
              glyph,
              size: DsIconSizeRole.sm,
              color: color,
              decorative: true,
            ),
            const SizedBox(width: DsSpace.s8),
            Expanded(
              child: Text(
                dsIsolate(d.name),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: dsTextStyle(DsType.bodySm).copyWith(
                  color: color,
                  fontWeight: unreadBar ? FontWeight.w700 : null,
                ),
              ),
            ),
            if (d.runActive) ...[
              const SizedBox(width: DsSpace.s6),
              _RunDot(color: c.categorySpruceOnSidebar),
            ],
            if (d.mentionCount > 0) ...[
              const SizedBox(width: DsSpace.s6),
              DsBadge(
                text: dsFormat(context).count(d.mentionCount),
                background: c.attentionBg,
                foreground: c.attentionOnBg,
              ),
            ],
            if (d.muted) ...[
              const SizedBox(width: DsSpace.s6),
              DsIcon(
                DsIcons.actionMute,
                size: DsIconSizeRole.sm,
                color: color,
                decorative: true,
              ),
            ],
          ],
        ),
      ),
    );
  }
}

/// The dot of an active run; pulses once per `motion.duration.pulse`, still when the motion
/// context is reduced (a zero pulse).
class _RunDot extends StatefulWidget {
  const _RunDot({required this.color});

  final Color color;

  @override
  State<_RunDot> createState() => _RunDotState();
}

class _RunDotState extends State<_RunDot> with SingleTickerProviderStateMixin {
  AnimationController? _pulse;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    final pulse = context.dsMotion.durationPulse;
    if (pulse <= Duration.zero) {
      _pulse?.dispose();
      _pulse = null;
    } else {
      (_pulse ??= AnimationController(vsync: this, duration: pulse))
        ..duration = pulse
        ..repeat(reverse: true);
    }
  }

  @override
  void dispose() {
    _pulse?.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    Widget dot = Container(
      width: DsSpace.s8,
      height: DsSpace.s8,
      decoration: BoxDecoration(color: widget.color, shape: BoxShape.circle),
    );
    final pulse = _pulse;
    if (pulse != null) {
      dot = FadeTransition(
        opacity: Tween<double>(begin: 1, end: 0.4).animate(pulse),
        child: dot,
      );
    }
    return dot;
  }
}
