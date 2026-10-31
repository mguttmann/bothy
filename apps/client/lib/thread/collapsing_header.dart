// SPDX-License-Identifier: Apache-2.0

/// The phone's thread header that gives its height to the history while the person reads
/// older messages (brief M5-17; 01 5.3 M3, addendum of 2026-10-31).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';

/// Whether the header shows, decided from the scrolling of the history below it.
///
/// A drag towards older messages (the scroll offset of the reversed history grows) hides the
/// header, a drag back towards the newest message or reaching it shows it again. With a screen
/// reader or switch control (`MediaQuery.accessibleNavigation`) the header always shows: its
/// back button and menu must stay where the focus order expects them.
class HeaderCollapse extends ValueNotifier<bool> {
  /// Creates the state, shown.
  HeaderCollapse() : super(true);

  /// Hears a scroll of the history; returns false so the notification travels on.
  bool onScroll(UserScrollNotification n) {
    if (n.depth != 0) return false;
    switch (n.direction) {
      case ScrollDirection.reverse:
        value = false;
      case ScrollDirection.forward:
        value = true;
      case ScrollDirection.idle:
        break;
    }
    return false;
  }

  /// The newest message came into view: the header shows.
  void newestVisible({required bool visible}) {
    if (visible) value = true;
  }
}

/// [child] at its full height while [collapse] shows it, else at no height and out of the
/// semantics tree; the change animates with `motion.duration-move` (none with reduced motion).
class CollapsingHeader extends StatelessWidget {
  /// Creates the header.
  const CollapsingHeader({
    super.key,
    required this.collapse,
    required this.child,
  });

  /// The state.
  final HeaderCollapse collapse;

  /// The header.
  final Widget child;

  @override
  Widget build(BuildContext context) {
    final always = MediaQuery.accessibleNavigationOf(context);
    final motion = context.dsMotion;
    return ValueListenableBuilder<bool>(
      valueListenable: collapse,
      builder: (context, shown, _) {
        final show = always || shown;
        return ClipRect(
          child: AnimatedAlign(
            alignment: Alignment.bottomCenter,
            heightFactor: show ? 1 : 0,
            duration: motion.durationMove,
            curve: motion.easingStandard,
            child: ExcludeSemantics(
              excluding: !show,
              child: IgnorePointer(ignoring: !show, child: child),
            ),
          ),
        );
      },
    );
  }
}
