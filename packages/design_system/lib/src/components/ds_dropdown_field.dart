// SPDX-License-Identifier: Apache-2.0

import 'package:flutter/material.dart';

/// A text field with a list to choose from, named for screen readers and without a node
/// below the field (01 8, WCAG 4.1.2; brief M4-32).
///
/// Flutter's `DropdownMenu` puts its arrow button as a node below the text field and gives
/// the field no name of its own. macOS presents a text field as a leaf, so VoiceOver never
/// reaches that button, and a field without a name is read as "text field" only. This field
/// shows [label] as the field's label, which is its name, and keeps the arrow a button for the
/// keyboard (Tab, then Enter opens the list) that screen readers do not see; they open the
/// list with the field's expand action. Use it instead of `DropdownMenu`.
///
/// [entries] are the choices; [initialSelection] the chosen value; [onSelected] gets the new
/// value; [enabled], [enableFilter], [requestFocusOnTap], [width] and [menuHeight] are passed
/// to `DropdownMenu`.
///
/// ```dart
/// DsDropdownField<String>(
///   key: const ValueKey('settings.timeZone'),
///   label: l.settingsTimeZone,
///   entries: [for (final z in zones) DropdownMenuEntry(value: z, label: z)],
///   initialSelection: zone,
///   onSelected: change,
/// )
/// ```
class DsDropdownField<T> extends StatelessWidget {
  /// Creates the field named [label] with the choices [entries].
  const DsDropdownField({
    super.key,
    required this.label,
    required this.entries,
    required this.onSelected,
    this.initialSelection,
    this.enabled = true,
    this.enableFilter = false,
    this.requestFocusOnTap = true,
    this.width,
    this.menuHeight,
  });

  /// The name of the field, shown as its label.
  final String label;

  /// The choices.
  final List<DropdownMenuEntry<T>> entries;

  /// Called with the chosen value.
  final ValueChanged<T?> onSelected;

  /// The value chosen at the start, or null.
  final T? initialSelection;

  /// Whether the field can be used.
  final bool enabled;

  /// Whether typing filters the list.
  final bool enableFilter;

  /// Whether a tap on the field focuses it for typing.
  final bool requestFocusOnTap;

  /// The width of the field, or null for the width of the widest entry.
  final double? width;

  /// The highest the open list grows, or null.
  final double? menuHeight;

  @override
  Widget build(BuildContext context) => DropdownMenu<T>(
    width: width,
    enabled: enabled,
    enableFilter: enableFilter,
    requestFocusOnTap: requestFocusOnTap,
    initialSelection: initialSelection,
    menuHeight: menuHeight,
    dropdownMenuEntries: entries,
    onSelected: onSelected,
    decorationBuilder: (context, menu) => InputDecoration(
      label: Text(label),
      suffixIcon: ExcludeSemantics(
        child: IconButton(
          onPressed: enabled
              ? () => menu.isOpen ? menu.close() : menu.open()
              : null,
          icon: Icon(menu.isOpen ? Icons.arrow_drop_up : Icons.arrow_drop_down),
        ),
      ),
    ),
  );
}
