// SPDX-License-Identifier: Apache-2.0

/// A space row of the sidebar tree (components.md section 7).
library;

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:vector_graphics/vector_graphics.dart';

import '../generated/design_tokens.g.dart';
import '../generated/ds_icon_assets.g.dart';
import '../icons/ds_icon.dart';
import '../theme/ds_theme.dart';
import 'ds_common.dart';
import 'view_models.dart';

/// The sidebar color of a category (`category.<name>.on-sidebar`).
Color dsCategoryOnSidebar(BuildContext context, DsCategory category) {
  final c = context.dsColors;
  return switch (category) {
    DsCategory.spruce => c.categorySpruceOnSidebar,
    DsCategory.loch => c.categoryLochOnSidebar,
    DsCategory.heather => c.categoryHeatherOnSidebar,
    DsCategory.rowan => c.categoryRowanOnSidebar,
    DsCategory.amber => c.categoryAmberOnSidebar,
    DsCategory.gorse => c.categoryGorseOnSidebar,
    DsCategory.moss => c.categoryMossOnSidebar,
    DsCategory.granite => c.categoryGraniteOnSidebar,
  };
}

/// A space in the sidebar: an expandable tree node.
///
/// The arrow (`action.expand`, caret down, when expanded; `action.collapse`, caret right, when
/// collapsed), the space icon
/// of `space-icons` in `category.<name>.on-sidebar` on a lightly tinted tile of the same color,
/// the name in `type.section` and `color.text.on-sidebar`, and on the right a dot labeled
/// "Unread" when the space is collapsed and has unread items. A tap or Enter toggles; arrow
/// left collapses and arrow right expands. [onToggle] receives the requested expanded state;
/// the row shows [DsSpaceRowData.expanded] until the caller passes the new value (the state is
/// synchronized through `sidebar.update`). Screen readers get the expanded or collapsed state.
///
/// ```dart
/// DsSpaceRow(space, onToggle: (expanded) => setExpanded(space.id, expanded))
/// ```
class DsSpaceRow extends StatelessWidget {
  /// The alpha of the icon tile in the category color; the token set has no tint role for a
  /// tile on the sidebar (components.md 7 "a lightly tinted tile of the same color").
  static const double _tileTint = 0.16;

  /// Creates the row of [data].
  const DsSpaceRow(
    this.data, {
    super.key,
    required this.onToggle,
    this.focusNode,
  });

  /// The space.
  final DsSpaceRowData data;

  /// Called with the requested expanded state.
  final ValueChanged<bool> onToggle;

  /// The focus node of the row, so the tree of the sidebar can move the focus to it (arrow
  /// left on a channel goes to its space, D-SB-8); null makes its own.
  final FocusNode? focusNode;

  @override
  Widget build(BuildContext context) {
    final l = dsL10n(context);
    final c = context.dsColors;
    final tint = dsCategoryOnSidebar(context, data.category);
    final asset =
        DsIconAssets.space(data.icon) ??
        DsIconAssets.space(DsIcons.spaceIcons.first)!;
    final iconEdge = DsIconSizeRole.sm.size;
    return Focus(
      canRequestFocus: false,
      skipTraversal: true,
      includeSemantics: false,
      onKeyEvent: (node, event) {
        if (event is! KeyDownEvent) return KeyEventResult.ignored;
        if (event.logicalKey == LogicalKeyboardKey.arrowLeft) {
          if (data.expanded) onToggle(false);
          return KeyEventResult.handled;
        }
        if (event.logicalKey == LogicalKeyboardKey.arrowRight) {
          if (!data.expanded) onToggle(true);
          return KeyEventResult.handled;
        }
        return KeyEventResult.ignored;
      },
      child: DsTappable(
        focusNode: focusNode,
        onTap: () => onToggle(!data.expanded),
        label: data.name,
        hint: data.expanded
            ? l.spaceRowCollapse(data.name)
            : l.spaceRowExpand(data.name),
        expanded: data.expanded,
        button: false,
        onSidebar: true,
        hoverColor: c.surfaceSidebarHover,
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: DsSpace.s4),
          child: Row(
            children: [
              DsIcon(
                data.expanded ? DsIcons.actionExpand : DsIcons.actionCollapse,
                size: DsIconSizeRole.sm,
                color: c.textOnSidebarSecondary,
                decorative: true,
              ),
              const SizedBox(width: DsSpace.s6),
              ExcludeSemantics(
                child: Container(
                  padding: const EdgeInsets.all(DsSpace.s2),
                  decoration: BoxDecoration(
                    color: tint.withValues(alpha: _tileTint),
                    borderRadius: BorderRadius.circular(DsRadius.sm),
                  ),
                  child: VectorGraphic(
                    loader: AssetBytesLoader(asset, packageName: dsIconPackage),
                    width: iconEdge,
                    height: iconEdge,
                    colorFilter: ColorFilter.mode(tint, BlendMode.srcIn),
                    placeholderBuilder: (_) =>
                        DsIconPlaceholder(size: iconEdge),
                  ),
                ),
              ),
              const SizedBox(width: DsSpace.s8),
              Expanded(
                child: ExcludeSemantics(
                  child: Text(
                    dsIsolate(data.name),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: dsTextStyle(DsType.section)
                        .copyWith(color: c.textOnSidebar),
                  ),
                ),
              ),
              if (!data.expanded && data.hasUnread)
                Semantics(
                  container: true,
                  label: l.spaceRowUnread,
                  child: Container(
                    width: DsSpace.s8,
                    height: DsSpace.s8,
                    decoration: BoxDecoration(
                      color: c.textOnSidebar,
                      shape: BoxShape.circle,
                    ),
                  ),
                ),
            ],
          ),
        ),
      ),
    );
  }
}
