// SPDX-License-Identifier: Apache-2.0

/// The head of the forum overview D15 (components.md 6; 01 D15): forum name with the kind
/// sentence, "New chat" or "New post", the search in the forum, "Pin" and the filter chips
/// Open, Waiting for me, Tags and Sort.
library;

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import 'forum_model.dart';

/// The head of the overview of [forumName].
///
/// Below the name the kind sentence explains the forum (01 6.8 item 11, [projectBound] picks
/// the project one and the icon `channel.project`). "New chat" in a project forum, else "New
/// post", is the primary button ([onNewPost]); [onSearch] opens the search with the filter of
/// this forum; [onPin] pins or unpins the overview as a column ([pinned]; null hides the
/// button); the chips Open and Waiting for me choose the view ([onView]); Tags opens a
/// multi-select of [tags] ([onTags]); Sort chooses last activity or creation ([onSort]), shown
/// as [sort], the sort in effect.
class PostListHeader extends StatelessWidget {
  /// Creates the head.
  const PostListHeader({
    super.key,
    required this.forumName,
    required this.filter,
    required this.sort,
    required this.tags,
    required this.onNewPost,
    required this.onView,
    required this.onTags,
    required this.onSort,
    this.projectBound = false,
    this.pinned = false,
    this.onPin,
    this.onSearch,
  });

  /// The forum's name (a foreign name).
  final String forumName;

  /// Whether the forum has a project binding (`Channel.project_bound`).
  final bool projectBound;

  /// Whether the overview is pinned as a column on this device.
  final bool pinned;

  /// Pins or unpins the overview; null hides "Pin".
  final VoidCallback? onPin;

  /// Opens the search in this forum; null hides it.
  final VoidCallback? onSearch;

  /// The filters.
  final PostListFilter filter;

  /// The sort in effect.
  final ForumSort sort;

  /// The tags of the forum.
  final List<ForumTag> tags;

  /// Opens the dialog of a new post.
  final VoidCallback onNewPost;

  /// Chooses the view chip.
  final ValueChanged<PostView> onView;

  /// Sets the selected tags.
  final ValueChanged<Set<ForumTagId>> onTags;

  /// Sets the sort.
  final ValueChanged<ForumSort> onSort;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final c = context.dsColors;
    final sentence = dsChannelKindSentence(
      l,
      DsChannelKind.forum,
      projectBound: projectBound,
    );
    return Padding(
      padding: const EdgeInsets.fromLTRB(
        DsSpace.s12,
        DsSpace.s12,
        DsSpace.s12,
        DsSpace.s8,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          // The name and "New post" share a line; with large text or a narrow column the
          // button moves to the next line instead of overflowing (01 8, text scaling).
          LayoutBuilder(
            builder: (context, constraints) => Wrap(
              alignment: WrapAlignment.spaceBetween,
              crossAxisAlignment: WrapCrossAlignment.center,
              spacing: DsSpace.s8,
              runSpacing: DsSpace.s6,
              children: [
                ConstrainedBox(
                  constraints: BoxConstraints(maxWidth: constraints.maxWidth),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      DsIcon(
                        dsChannelGlyph(
                          DsChannelKind.forum,
                          projectBound: projectBound,
                        ),
                        size: DsIconSizeRole.md,
                        color: c.textSecondary,
                        decorative: true,
                      ),
                      const SizedBox(width: DsSpace.s6),
                      Flexible(
                        child: Semantics(
                          header: true,
                          child: Text(
                            dsIsolate(forumName),
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: dsTextStyle(DsType.heading)
                                .copyWith(color: c.textPrimary),
                          ),
                        ),
                      ),
                    ],
                  ),
                ),
                ConstrainedBox(
                  constraints: BoxConstraints(maxWidth: constraints.maxWidth),
                  child: Wrap(
                    spacing: DsSpace.s6,
                    runSpacing: DsSpace.s6,
                    children: [
                      DsButton(
                        key: const ValueKey('postList.newPost'),
                        label: projectBound
                            ? l.postListNewChat
                            : l.postListNewPost,
                        style: DsButtonStyle.primary,
                        onPressed: onNewPost,
                      ),
                      if (onSearch case final search?)
                        DsButton(
                          key: const ValueKey('postList.search'),
                          label: l.overviewSearch(dsIsolate(forumName)),
                          style: DsButtonStyle.secondary,
                          onPressed: search,
                        ),
                      if (onPin case final pin?)
                        DsButton(
                          key: const ValueKey('postList.pin'),
                          label: pinned ? l.overviewUnpin : l.overviewPin,
                          style: DsButtonStyle.secondary,
                          onPressed: pin,
                        ),
                    ],
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(height: DsSpace.s4),
          // The kind sentence as the subtitle (01 6.8 item 11).
          Text(
            sentence,
            key: const ValueKey('postList.kind'),
            style: dsTextStyle(DsType.bodySm).copyWith(color: c.textSecondary),
          ),
          const SizedBox(height: DsSpace.s8),
          Wrap(
            spacing: DsSpace.s6,
            runSpacing: DsSpace.s6,
            children: [
              FilterChipButton(
                key: const ValueKey('postList.filter.open'),
                label: l.postListFilterOpen,
                selected: filter.view == PostView.open,
                onTap: () => onView(PostView.open),
              ),
              FilterChipButton(
                key: const ValueKey('postList.filter.waiting'),
                label: l.postListFilterWaitingForMe,
                selected: filter.view == PostView.waitingForMe,
                onTap: () => onView(PostView.waitingForMe),
              ),
              MenuAnchor(
                menuChildren: [
                  for (final t in tags)
                    CheckboxMenuButton(
                      key: ValueKey('postList.tag.${t.id.value}'),
                      value: filter.tags.contains(t.id),
                      closeOnActivate: false,
                      onChanged: (on) => onTags(
                        on == true
                            ? {...filter.tags, t.id}
                            : ({...filter.tags}..remove(t.id)),
                      ),
                      child: Text(dsIsolate(t.name)),
                    ),
                ],
                builder: (context, menu, _) => FilterChipButton(
                  key: const ValueKey('postList.filter.tags'),
                  label: filter.tags.isEmpty
                      ? l.postListFilterTags
                      : l.postListFilterTagsCount(filter.tags.length),
                  selected: filter.tags.isNotEmpty,
                  glyph: DsIcons.tag,
                  onTap: tags.isEmpty
                      ? null
                      : () => menu.isOpen ? menu.close() : menu.open(),
                ),
              ),
              MenuAnchor(
                menuChildren: [
                  for (final (value, text) in [
                    (ForumSort.lastActivity, l.postListSortLastActivity),
                    (ForumSort.created, l.postListSortCreated),
                  ])
                    DsMenuItem(
                      key: ValueKey('postList.sort.${value.code}'),
                      selected: value == sort,
                      onPressed: () => onSort(value),
                      child: Text(text),
                    ),
                ],
                builder: (context, menu, _) => FilterChipButton(
                  key: const ValueKey('postList.filter.sort'),
                  label: sort == ForumSort.created
                      ? l.postListSortCreated
                      : l.postListSortLastActivity,
                  selected: false,
                  glyph: DsIcons.actionFilter,
                  onTap: () => menu.isOpen ? menu.close() : menu.open(),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

/// A chip of the post list head: a pill with [label], filled in the accent when [selected].
class FilterChipButton extends StatelessWidget {
  /// Creates the chip.
  const FilterChipButton({
    super.key,
    required this.label,
    required this.selected,
    required this.onTap,
    this.glyph,
  });

  /// The text.
  final String label;

  /// Whether the chip is on.
  final bool selected;

  /// Called on activation; null disables the chip.
  final VoidCallback? onTap;

  /// An icon before the text, or null.
  final DsGlyph? glyph;

  @override
  Widget build(BuildContext context) {
    final c = context.dsColors;
    final a = context.dsAccent;
    final fg = selected ? a.accentFg : c.textPrimary;
    return DsTappable(
      onTap: onTap,
      label: label,
      selected: selected,
      radius: DsRadius.pill,
      excludeChildSemantics: true,
      child: Container(
        constraints: BoxConstraints(minHeight: context.dsDensity.targetMin),
        padding: const EdgeInsets.symmetric(horizontal: DsSpace.s12),
        decoration: BoxDecoration(
          color: selected ? a.accentSubtle : null,
          borderRadius: BorderRadius.circular(DsRadius.pill),
          border: Border.all(
            color: selected ? a.accentFg : c.borderControl,
            width: DsBorderWidth.hairline,
          ),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            if (glyph != null) ...[
              DsIcon(
                glyph!,
                size: DsIconSizeRole.sm,
                color: fg,
                decorative: true,
              ),
              const SizedBox(width: DsSpace.s4),
            ],
            Flexible(
              child: Text(
                label,
                style: dsTextStyle(DsType.label).copyWith(color: fg),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
