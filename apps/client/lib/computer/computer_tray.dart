// SPDX-License-Identifier: Apache-2.0

/// The tray entry of computer use (docs/spec/27-computer-use.md 11 item 4; brief CU-07
/// requirement 15): while a computer session of the owner is live the tray menu of the
/// desktop app has "Stop computer control (`desktops`)", which stops every live session; the
/// entry goes once none is live.
library;

import 'dart:async';

import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../desktop/tray.dart';
import 'computer_status.dart';
import 'providers.dart';

/// Keeps the computer entry of [tray] in step with the owner's live sessions.
///
/// [label] builds the entry's text from the desktops' names joined by a comma (the
/// localized `trayComputerStop`). A stop that fails leaves its session live, so the entry stays
/// and the chip in the thread shows the session; nothing of the error goes into a log line with
/// a desktop's name (R-CL-18).
///
/// Example:
/// ```dart
/// ComputerTray(tray: tray, ref: ref, label: l.trayComputerStop);
/// ```
final class ComputerTray {
  /// Starts following [computerSessionsProvider] through [ref].
  ComputerTray({required this.tray, required Ref ref, required this.label})
    : _ref = ref {
    ref.listen<ComputerSessionsState>(
      computerSessionsProvider,
      (_, now) => unawaited(_update(now)),
      fireImmediately: true,
    );
  }

  /// The tray of the desktop app.
  final DesktopTray tray;

  /// The entry's text for the desktops' names.
  final String Function(String desktops) label;

  final Ref _ref;

  Future<void> _update(ComputerSessionsState sessions) {
    if (sessions.live.isEmpty) return tray.setComputerEntry(null);
    final names = {for (final s in sessions.live.values) s.desktop}.toList()
      ..sort();
    return tray.setComputerEntry(label(names.join(', ')), onStop: _stopAll);
  }

  void _stopAll() {
    final notifier = _ref.read(computerSessionsProvider.notifier);
    final live = _ref.read(computerSessionsProvider).live.keys.toList();
    unawaited(() async {
      for (final id in live) {
        try {
          await notifier.stop(id);
        } on Object {
          // The session stays live and keeps the entry; the thread's chip names the failure
          // when the owner stops there.
        }
      }
    }());
  }
}
