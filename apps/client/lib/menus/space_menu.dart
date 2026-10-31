// SPDX-License-Identifier: Apache-2.0

/// The menu of a space in the sidebar (brief M4-33 Scope): its notification level and "Mute
/// until" (13 3.1), headed by "New project in" and "Add forums for existing folders" for an
/// owner (01 6.9 item 1, M4-43; 01 6.10 item 1, M4-44).
library;

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';

import '../notifications_settings/providers.dart';
import '../sidebar/sidebar_drag.dart' show SidebarMoves, sidebarMoveMenuItems;
import 'level_menu.dart';

/// The level menu of [space] around [child].
class SpaceMenu extends StatelessWidget {
  /// Creates the menu of [space].
  const SpaceMenu({
    super.key,
    required this.space,
    required this.child,
    this.newProjectLabel,
    this.onNewProject,
    this.onAddFolderForums,
    this.moves,
  });

  /// "Move up" and "Move down" of the space in the sidebar (01 6.16, M4-51), or null for none.
  final SidebarMoves? moves;

  /// The space.
  final Space space;

  /// The label of the entry "New project in `space`", or null for none.
  final String? newProjectLabel;

  /// Opens the new project dialog of the space, or null for no entry.
  final VoidCallback? onNewProject;

  /// Opens "Add forums for existing folders" of the space, or null for no entry.
  final VoidCallback? onAddFolderForums;

  /// The row the menu belongs to.
  final Widget child;

  @override
  Widget build(BuildContext context) => TargetMenuRegion(
    label: AppLocalizations.of(context).notifyMenuOf(space.name),
    menuChildren: (context, ref) {
      final label = newProjectLabel;
      final start = onNewProject;
      final sync = onAddFolderForums;
      return [
        if (label != null && start != null)
          DsMenuItem(
            key: const ValueKey('menu.newProject'),
            onPressed: start,
            child: Text(dsIsolate(label)),
          ),
        if (sync != null)
          DsMenuItem(
            key: const ValueKey('menu.addFolderForums'),
            onPressed: sync,
            child: Text(AppLocalizations.of(context).spaceMenuAddFolderForums),
          ),
        ...levelMenuItems(context, ref, spaceMenuTarget(ref, space)),
        if (moves case final moves?) ...[
          const Divider(),
          ...sidebarMoveMenuItems(context, moves),
        ],
      ];
    },
    child: child,
  );
}

/// The level target of [space] from the current [LevelContext].
LevelMenuTarget spaceMenuTarget(WidgetRef ref, Space space) {
  final levels = ref.read(levelContextProvider);
  return LevelMenuTarget(
    kind: NotificationTargetKind.space,
    id: space.id.value,
    own: levels.own(NotificationTargetKind.space, space.id.value),
    inherited: levels.inherited(NotificationTargetKind.space),
    now: levels.now,
  );
}
