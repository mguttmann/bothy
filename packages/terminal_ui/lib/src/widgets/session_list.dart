// SPDX-License-Identifier: Apache-2.0

/// The list of the person's running sessions on one host (FIX-terminal-session-lifecycle,
/// docs/spec/24-terminal.md 12 "session list per host with state, reattach"): each session with
/// its name, its start and, while no device of this app is attached, since when it is detached and
/// "Reattach". The same list on the desktop and on the phone.
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:terminal_ui/src/controller/session_state.dart';

/// One row of the list.
@immutable
final class TerminalSessionEntry {
  /// Creates a row.
  const TerminalSessionEntry({
    required this.label,
    required this.phase,
    this.startedAt,
    this.detachedAt,
    this.onReattach,
    this.onSelect,
  });

  /// The name of the session (profile and number, `terminalTabLabel`).
  final String label;

  /// The phase of its tab.
  final TerminalPhase phase;

  /// `opened_at` of the session.
  final DateTime? startedAt;

  /// Since when it is detached; null while attached or unknown.
  final DateTime? detachedAt;

  /// Attaches the session again; null hides "Reattach".
  final VoidCallback? onReattach;

  /// Shows the session's tab; null makes the row inert.
  final VoidCallback? onSelect;
}

/// Formats [t] in local time as `HH:mm`, with the date `yyyy-MM-dd` in front when it is not on
/// the day of [now].
String sessionTime(DateTime t, DateTime now) {
  final l = t.toLocal();
  final n = now.toLocal();
  String two(int v) => v.toString().padLeft(2, '0');
  final time = '${two(l.hour)}:${two(l.minute)}';
  if (l.year == n.year && l.month == n.month && l.day == n.day) return time;
  return '${l.year}-${two(l.month)}-${two(l.day)} $time';
}

/// The rows of [entries], or a note without any.
///
/// ```dart
/// TerminalSessionList(entries: [TerminalSessionEntry(label: 'shell 1',
///     phase: TerminalPhase.detached, startedAt: opened, detachedAt: since, onReattach: attach)])
/// ```
class TerminalSessionList extends StatelessWidget {
  /// Creates the list; [now] gives the day the times are compared with (null: the system time).
  const TerminalSessionList({super.key, required this.entries, this.now});

  /// The sessions, in tab order.
  final List<TerminalSessionEntry> entries;

  /// The clock of the date check.
  final DateTime Function()? now;

  @override
  Widget build(BuildContext context) {
    final l = dsL10n(context);
    final c = context.dsColors;
    final today = (now ?? DateTime.now)();
    final caption = dsTextStyle(DsType.caption)
        .copyWith(color: c.textSecondary);
    if (entries.isEmpty) {
      return Padding(
        padding: const EdgeInsets.all(DsSpace.s12),
        child: Text(l.terminalSessionsNone, style: caption),
      );
    }
    return Column(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        for (final (i, e) in entries.indexed)
          InkWell(
            key: ValueKey('terminal.sessions.$i'),
            onTap: e.onSelect,
            child: Padding(
              padding: const EdgeInsets.symmetric(
                horizontal: DsSpace.s12,
                vertical: DsSpace.s8,
              ),
              child: Row(
                children: [
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          e.label,
                          style: dsTextStyle(DsType.label)
                              .copyWith(color: c.textPrimary),
                        ),
                        Text(
                          [
                            if (e.startedAt case final t?)
                              l.terminalSessionStarted(sessionTime(t, today)),
                            if (e.phase == TerminalPhase.connected)
                              l.terminalSessionConnected
                            else if (e.detachedAt case final t?)
                              l.terminalSessionDetachedSince(
                                sessionTime(t, today),
                              ),
                          ].join(' · '),
                          style: caption,
                        ),
                      ],
                    ),
                  ),
                  if (e.onReattach case final reattach?)
                    TextButton(
                      key: ValueKey('terminal.sessions.$i.reattach'),
                      onPressed: reattach,
                      child: Text(l.terminalReattach),
                    ),
                ],
              ),
            ),
          ),
      ],
    );
  }
}

/// Shows [entries] in a dialog titled "Running sessions"; a tap on "Reattach" or a row closes
/// it and runs its callback.
Future<void> showTerminalSessionList(
  BuildContext context,
  List<TerminalSessionEntry> entries, {
  DateTime Function()? now,
}) => showDialog<void>(
  context: context,
  builder: (context) {
    final l = dsL10n(context);
    VoidCallback? closing(VoidCallback? f) => f == null
        ? null
        : () {
            Navigator.of(context).pop();
            f();
          };
    return AlertDialog(
      title: Text(l.terminalSessionsTitle),
      content: SingleChildScrollView(
        child: TerminalSessionList(
          now: now,
          entries: [
            for (final e in entries)
              TerminalSessionEntry(
                label: e.label,
                phase: e.phase,
                startedAt: e.startedAt,
                detachedAt: e.detachedAt,
                onReattach: closing(e.onReattach),
                onSelect: closing(e.onSelect),
              ),
          ],
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: Text(l.terminalCloseTab),
        ),
      ],
    );
  },
);
