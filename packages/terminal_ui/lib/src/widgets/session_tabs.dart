// SPDX-License-Identifier: Apache-2.0

/// The tabs of the terminal view, one per own session on the host, with "Detach" and "End"
/// on the right (screen D11 of docs/spec/01-product.md; docs/design/components.md 12 "Tabs").
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';

/// One tab: its label (profile and number, `terminalTabLabel`).
@immutable
final class SessionTabData {
  /// Creates the tab data.
  const SessionTabData({required this.label});

  /// The visible label.
  final String label;
}

/// The actions of the selected session: "Close" alone for an ended one ([onClose] set), else
/// "Detach" and "End" (a null callback disables its button).
List<Widget> sessionActions(
  BuildContext context,
  VoidCallback? onDetach,
  VoidCallback? onEnd,
  VoidCallback? onClose,
) {
  final l = dsL10n(context);
  final c = context.dsColors;
  if (onClose != null) {
    return [
      OutlinedButton(
        key: const ValueKey('terminal.close'),
        onPressed: onClose,
        child: Text(l.terminalCloseTab),
      ),
    ];
  }
  return [
    OutlinedButton(onPressed: onDetach, child: Text(l.terminalDetach)),
    const SizedBox(width: DsSpace.s8),
    OutlinedButton(
      style: OutlinedButton.styleFrom(
        foregroundColor: c.dangerFg,
        side: BorderSide(color: c.dangerBorder),
      ),
      onPressed: onEnd,
      child: Text(l.terminalEnd),
    ),
  ];
}

/// The row of tabs and the actions; a null callback disables its button.
///
/// ```dart
/// SessionTabs(tabs: const [SessionTabData(label: 'shell 1')], selected: 0,
///     onSelect: select, onDetach: controller.detach, onEnd: controller.end)
/// ```
class SessionTabs extends StatelessWidget {
  /// Creates the row.
  const SessionTabs({
    super.key,
    required this.tabs,
    required this.selected,
    this.onSelect,
    this.onDetach,
    this.onEnd,
    this.onClose,
    this.onSessions,
  });

  /// The tabs, in order.
  final List<SessionTabData> tabs;

  /// The index of the selected tab.
  final int selected;

  /// Selects a tab.
  final ValueChanged<int>? onSelect;

  /// Detaches the selected session; null disables "Detach".
  final VoidCallback? onDetach;

  /// Ends the selected session; null disables "End".
  final VoidCallback? onEnd;

  /// Closes the tab of the selected session, which ended; when set, "Close" replaces "Detach"
  /// and "End" (FIX-terminal-session-lifecycle).
  final VoidCallback? onClose;

  /// Shows the list of the running sessions; null hides "Sessions".
  final VoidCallback? onSessions;

  @override
  Widget build(BuildContext context) {
    final l = dsL10n(context);
    final c = context.dsColors;
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: DsSpace.s8),
      child: Row(
        children: [
          Expanded(
            child: SingleChildScrollView(
              scrollDirection: Axis.horizontal,
              child: Row(
                children: [
                  for (var i = 0; i < tabs.length; i++)
                    Padding(
                      padding: const EdgeInsets.only(right: DsSpace.s4),
                      child: Semantics(
                        selected: i == selected,
                        button: true,
                        child: Material(
                          color: i == selected
                              ? c.surfaceSelected
                              : c.surfaceCanvas,
                          borderRadius: BorderRadius.circular(DsRadius.md),
                          child: InkWell(
                            borderRadius: BorderRadius.circular(DsRadius.md),
                            onTap: onSelect == null ? null : () => onSelect!(i),
                            child: Padding(
                              padding: const EdgeInsets.symmetric(
                                horizontal: DsSpace.s12,
                                vertical: DsSpace.s6,
                              ),
                              child: Text(
                                tabs[i].label,
                                style: dsTextStyle(DsType.label)
                                    .copyWith(color: c.textPrimary),
                              ),
                            ),
                          ),
                        ),
                      ),
                    ),
                ],
              ),
            ),
          ),
          if (onSessions case final sessions?) ...[
            const SizedBox(width: DsSpace.s8),
            TextButton(
              key: const ValueKey('terminal.sessions'),
              onPressed: sessions,
              child: Text(l.terminalSessions),
            ),
          ],
          const SizedBox(width: DsSpace.s8),
          ...sessionActions(context, onDetach, onEnd, onClose),
        ],
      ),
    );
  }
}
