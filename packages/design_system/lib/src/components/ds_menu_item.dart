// SPDX-License-Identifier: Apache-2.0

import 'package:flutter/material.dart';

/// An entry of a menu that a screen reader announces as a button and, where the menu offers a
/// choice, as selected (01 8, WCAG 4.1.2; brief M4-32).
///
/// Flutter's `MenuItemButton` marks itself as a button only on the web, so VoiceOver and
/// Narrator read a desktop menu entry as plain text with an action; the check mark of a chosen
/// entry is an icon that is not read at all. [DsMenuItem] wraps `MenuItemButton` and merges a
/// button role and the [selected] state into the entry's node. Use it for every entry of a
/// `MenuAnchor`, a submenu or a select instead of `MenuItemButton`.
///
/// [onPressed] null disables the entry; [leadingIcon], [trailingIcon] and [closeOnActivate]
/// are passed to `MenuItemButton`; [selected] null means the entry is not a choice.
///
/// ```dart
/// DsMenuItem(
///   key: const ValueKey('menu.level.all'),
///   selected: level == NotificationLevel.all,
///   onPressed: () => set(NotificationLevel.all),
///   child: Text(l.notifyLevelAll),
/// )
/// ```
class DsMenuItem extends StatelessWidget {
  /// Creates the entry with the label [child].
  const DsMenuItem({
    super.key,
    required this.child,
    required this.onPressed,
    this.leadingIcon,
    this.trailingIcon,
    this.selected,
    this.closeOnActivate = true,
  });

  /// The label of the entry.
  final Widget child;

  /// Called when the entry is chosen; null disables it.
  final VoidCallback? onPressed;

  /// The icon before the label, or null.
  final Widget? leadingIcon;

  /// The icon after the label, or null.
  final Widget? trailingIcon;

  /// Whether the entry is the chosen one of a choice, or null when it is not a choice.
  final bool? selected;

  /// Whether choosing the entry closes the menu.
  final bool closeOnActivate;

  @override
  Widget build(BuildContext context) => MergeSemantics(
    child: Semantics(
      button: true,
      selected: selected,
      child: MenuItemButton(
        onPressed: onPressed,
        leadingIcon: leadingIcon,
        trailingIcon: trailingIcon,
        closeOnActivate: closeOnActivate,
        child: child,
      ),
    ),
  );
}
