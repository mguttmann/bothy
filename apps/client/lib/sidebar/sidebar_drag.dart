// SPDX-License-Identifier: Apache-2.0

/// Dragging a channel or forum post row into a new place among its siblings (docs/spec/
/// 01-product.md 6.16; brief M4-51; D-M4-51-6): the row is the drag source, every sibling row a
/// drop target with a line that shows where the row lands. The spaces keep the reorderable list of
/// M4-19 (`SpaceTree`), whose gap is their drop indicator.
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/semantics.dart';
import 'package:l10n/l10n.dart';

import 'sidebar_arrange.dart';

/// What a dragged row carries: its [level], its [parent] (the space of a channel, the forum of a
/// post), its [id] and its [index] among the rows on screen. A row accepts a drop only from its
/// own level and parent (D-M4-51-6).
@immutable
final class SidebarDragData {
  /// Creates the data.
  const SidebarDragData({
    required this.level,
    required this.parent,
    required this.id,
    required this.index,
  });

  /// The level of the row.
  final SidebarLevel level;

  /// The ID of the parent.
  final String parent;

  /// The ID of the row.
  final String id;

  /// The index of the row among the rows of its parent on screen.
  final int index;
}

/// Shows the note that [level] is arranged by activity and where to switch it off (D-M4-51-6),
/// replacing a note that still shows.
void showArrangedHint(BuildContext context, SidebarLevel level) {
  final l = AppLocalizations.of(context);
  final messenger = ScaffoldMessenger.maybeOf(context);
  if (messenger == null) return;
  messenger
    ..hideCurrentSnackBar()
    ..showSnackBar(
      SnackBar(
        key: ValueKey('sidebar.arrangedHint.${level.name}'),
        content: Text(arrangedHintText(l, level)),
      ),
    );
}

/// The text of the note of [level].
String arrangedHintText(AppLocalizations l, SidebarLevel level) =>
    switch (level) {
      SidebarLevel.spaces => l.sidebarSpacesArrangedHint,
      SidebarLevel.channels => l.sidebarChannelsArrangedHint,
      SidebarLevel.chats => l.sidebarChatsArrangedHint,
    };

/// Wraps a row that is never dragged itself so a drag attempt at an automatic [level] shows the
/// note (a long press on [touch], a mouse drag on the desktop).
class SidebarArrangedRow extends StatelessWidget {
  /// Creates the wrapper.
  const SidebarArrangedRow({
    super.key,
    required this.level,
    required this.touch,
    required this.child,
  });

  /// The automatic level.
  final SidebarLevel level;

  /// Whether the screen is a touch screen.
  final bool touch;

  /// The row.
  final Widget child;

  @override
  Widget build(BuildContext context) => GestureDetector(
    behavior: HitTestBehavior.translucent,
    onLongPress: touch ? () => showArrangedHint(context, level) : null,
    onPanStart: touch ? null : (_) => showArrangedHint(context, level),
    child: child,
  );
}

/// A sibling row that can be dragged ([enabled]) and that takes a drop of a sibling.
///
/// On the desktop the row lifts as the mouse moves with the button down; on a touch screen
/// ([touch]) after a long press, so a short touch opens the row and a swipe scrolls. A sibling
/// dragged over the row shows a line above or below it, by the half the pointer is in; a drop
/// calls [onDrop] with the dragged row's ID and its new index among the rows on screen without
/// it (the input of `moveAmong`). When not [enabled] because the level is automatic
/// ([arranged]), a drag attempt shows the note instead.
class SidebarDragRow extends StatefulWidget {
  /// Creates the row.
  const SidebarDragRow({
    super.key,
    required this.data,
    required this.label,
    required this.enabled,
    required this.touch,
    required this.onDrop,
    required this.child,
    this.arranged = false,
  });

  /// The row's own data.
  final SidebarDragData data;

  /// The text of the lifted row (its name or title).
  final String label;

  /// Whether the row can be dragged and takes drops.
  final bool enabled;

  /// Whether the screen is a touch screen.
  final bool touch;

  /// Whether the level is arranged by activity (a drag attempt shows the note).
  final bool arranged;

  /// Called with the ID of the dropped row and its new index.
  final void Function(String id, int to) onDrop;

  /// The row.
  final Widget child;

  @override
  State<SidebarDragRow> createState() => _SidebarDragRowState();
}

/// Where the line of a drop shows.
enum _Edge { above, below }

class _SidebarDragRowState extends State<SidebarDragRow> {
  _Edge? _edge;

  bool _accepts(SidebarDragData other) =>
      other.level == widget.data.level &&
      other.parent == widget.data.parent &&
      other.id != widget.data.id;

  _Edge _edgeAt(Offset global) {
    final box = context.findRenderObject() as RenderBox?;
    if (box == null || !box.hasSize) return _Edge.below;
    final local = box.globalToLocal(global);
    return local.dy < box.size.height / 2 ? _Edge.above : _Edge.below;
  }

  void _setEdge(_Edge? edge) {
    if (edge != _edge && mounted) setState(() => _edge = edge);
  }

  @override
  Widget build(BuildContext context) {
    final w = widget;
    if (!w.enabled) {
      return w.arranged
          ? SidebarArrangedRow(
              level: w.data.level,
              touch: w.touch,
              child: w.child,
            )
          : w.child;
    }
    final target = DragTarget<SidebarDragData>(
      onWillAcceptWithDetails: (d) => _accepts(d.data),
      onMove: (d) {
        if (_accepts(d.data)) _setEdge(_edgeAt(d.offset));
      },
      onLeave: (_) => _setEdge(null),
      onAcceptWithDetails: (d) {
        final edge = _edgeAt(d.offset);
        _setEdge(null);
        final insertAt = w.data.index + (edge == _Edge.below ? 1 : 0);
        final to = insertAt > d.data.index ? insertAt - 1 : insertAt;
        w.onDrop(d.data.id, to);
      },
      builder: (context, _, _) => Stack(
        children: [
          w.child,
          if (_edge case final edge?)
            Positioned(
              key: ValueKey('sidebar.dropLine.${w.data.id}.${edge.name}'),
              left: DsSpace.s16,
              right: DsSpace.s4,
              top: edge == _Edge.above ? 0 : null,
              bottom: edge == _Edge.below ? 0 : null,
              child: IgnorePointer(
                child: Container(
                  height: 2,
                  color: context.dsColors.focusRingOnSidebar,
                ),
              ),
            ),
        ],
      ),
    );
    final feedback = Material(
      color: context.dsColors.surfaceSidebarSelected,
      borderRadius: BorderRadius.circular(DsRadius.sm),
      child: Padding(
        padding: const EdgeInsets.symmetric(
          horizontal: DsSpace.s12,
          vertical: DsSpace.s8,
        ),
        child: Text(
          dsIsolate(w.label),
          style: dsTextStyle(DsType.bodySm)
              .copyWith(color: context.dsColors.textOnSidebar),
        ),
      ),
    );
    return w.touch
        ? LongPressDraggable<SidebarDragData>(
            data: w.data,
            feedback: feedback,
            dragAnchorStrategy: pointerDragAnchorStrategy,
            child: target,
          )
        : Draggable<SidebarDragData>(
            data: w.data,
            feedback: feedback,
            dragAnchorStrategy: pointerDragAnchorStrategy,
            child: target,
          );
  }
}

/// The keyboard and screen reader moves of one sidebar row (D-M4-51-7): one place [up] or
/// [down] (null at that end), or none while its level is [arranged] by activity.
@immutable
final class SidebarMoves {
  /// Creates the moves.
  const SidebarMoves({this.up, this.down, this.arranged = false});

  /// Moves the row one place up; null at the top or when [arranged].
  final VoidCallback? up;

  /// Moves the row one place down; null at the bottom or when [arranged].
  final VoidCallback? down;

  /// Whether the level is arranged by activity.
  final bool arranged;

  /// The screen reader actions "Move up" and "Move down" of the row, those that apply.
  Map<CustomSemanticsAction, VoidCallback> semanticsActions(
    AppLocalizations l,
  ) => {
    CustomSemanticsAction(label: l.sidebarMoveUp): ?up,
    CustomSemanticsAction(label: l.sidebarMoveDown): ?down,
  };
}

/// The menu entries of [moves]: "Move up" and "Move down", disabled at the ends; at an
/// automatic level both disabled below the entry "Arranged by activity" (D-M4-51-7).
List<Widget> sidebarMoveMenuItems(BuildContext context, SidebarMoves moves) {
  final l = AppLocalizations.of(context);
  return [
    if (moves.arranged)
      DsMenuItem(
        key: const ValueKey('menu.arrangedByActivity'),
        onPressed: null,
        child: Text(l.sidebarArrangedByActivity),
      ),
    DsMenuItem(
      key: const ValueKey('menu.moveUp'),
      onPressed: moves.arranged ? null : moves.up,
      child: Text(l.sidebarMoveUp),
    ),
    DsMenuItem(
      key: const ValueKey('menu.moveDown'),
      onPressed: moves.arranged ? null : moves.down,
      child: Text(l.sidebarMoveDown),
    ),
  ];
}
