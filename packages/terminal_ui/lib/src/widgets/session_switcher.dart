// SPDX-License-Identifier: Apache-2.0

/// The session switcher of the phone terminal (screen M8 of docs/spec/01-product.md: "shell 1 v
/// ... [Detach]"; brief M5-12 Design note 1): the selected session with a menu of the other own
/// sessions on the host instead of the desktop tabs, and "Detach" and "End".
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:terminal_ui/src/widgets/session_tabs.dart' show SessionTabData;

/// The row of the switcher and the two actions; a null callback disables its button.
///
/// ```dart
/// SessionSwitcher(tabs: const [SessionTabData(label: 'shell 1')], selected: 0,
///     onSelect: select, onDetach: controller.detach, onEnd: controller.end)
/// ```
class SessionSwitcher extends StatelessWidget {
  /// Creates the row.
  const SessionSwitcher({
    super.key,
    required this.tabs,
    required this.selected,
    this.onSelect,
    this.onDetach,
    this.onEnd,
  });

  /// The sessions, in order.
  final List<SessionTabData> tabs;

  /// The index of the selected session.
  final int selected;

  /// Selects another session.
  final ValueChanged<int>? onSelect;

  /// Detaches the selected session; null disables "Detach".
  final VoidCallback? onDetach;

  /// Ends the selected session; null disables "End".
  final VoidCallback? onEnd;

  @override
  Widget build(BuildContext context) {
    final l = dsL10n(context);
    final c = context.dsColors;
    final label = tabs.isEmpty
        ? ''
        : tabs[selected.clamp(0, tabs.length - 1)].label;
    final select = onSelect;
    final canSelect = select != null && tabs.length > 1;
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: DsSpace.s8),
      child: Wrap(
        spacing: DsSpace.s8,
        runSpacing: DsSpace.s4,
        crossAxisAlignment: WrapCrossAlignment.center,
        children: [
          // One node with the switcher's name and the menu button's tap (only its visible text is
          // excluded): with one session it says it is disabled, with more a screen reader opens
          // the menu.
          Semantics(
            container: true,
            label: l.terminalSessionSwitcher(label),
            button: true,
            enabled: canSelect,
            child: PopupMenuButton<int>(
              key: const ValueKey('terminal.switcher'),
              enabled: canSelect,
              onSelected: select,
              itemBuilder: (context) => [
                for (var i = 0; i < tabs.length; i++)
                  CheckedPopupMenuItem<int>(
                    key: ValueKey('terminal.switcher.$i'),
                    value: i,
                    checked: i == selected,
                    child: Text(tabs[i].label),
                  ),
              ],
              child: ConstrainedBox(
                constraints: BoxConstraints(
                  minHeight: context.dsDensity.controlHeight,
                ),
                child: Padding(
                  padding: const EdgeInsets.symmetric(horizontal: DsSpace.s8),
                  child: ExcludeSemantics(
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Text(
                          label,
                          style: dsTextStyle(DsType.label)
                              .copyWith(color: c.textPrimary),
                        ),
                        if (tabs.length > 1) ...[
                          const SizedBox(width: DsSpace.s4),
                          DsIcon(
                            DsIcons.actionExpand,
                            size: DsIconSizeRole.sm,
                            color: c.textSecondary,
                            decorative: true,
                          ),
                        ],
                      ],
                    ),
                  ),
                ),
              ),
            ),
          ),
          OutlinedButton(onPressed: onDetach, child: Text(l.terminalDetach)),
          OutlinedButton(
            style: OutlinedButton.styleFrom(
              foregroundColor: c.dangerFg,
              side: BorderSide(color: c.dangerBorder),
            ),
            onPressed: onEnd,
            child: Text(l.terminalEnd),
          ),
        ],
      ),
    );
  }
}
