// SPDX-License-Identifier: Apache-2.0

/// The menu of a space in the sidebar (brief M4-33 Scope): its notification level and "Mute
/// until" (13 3.1).
library;

import 'package:domain/domain.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';

import '../notifications_settings/providers.dart';
import 'level_menu.dart';

/// The level menu of [space] around [child].
class SpaceMenu extends ConsumerWidget {
  /// Creates the menu of [space].
  const SpaceMenu({super.key, required this.space, required this.child});

  /// The space.
  final Space space;

  /// The row the menu belongs to.
  final Widget child;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    // Watched, so the menu reads current rows when it opens.
    ref.watch(levelContextProvider);
    return TargetMenuRegion(
      label: AppLocalizations.of(context).notifyMenuOf(space.name),
      menuChildren: (context) =>
          levelMenuItems(context, ref, spaceMenuTarget(ref, space)),
      child: child,
    );
  }
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
