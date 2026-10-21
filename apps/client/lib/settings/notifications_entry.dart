// SPDX-License-Identifier: Apache-2.0

/// The Notifications entry of D8; M4-30 adds the note that notifications arrive only while the
/// app runs and "Start at login", M4-33 fills it with D9 (docs/spec/01-product.md 5.1 D8, D9;
/// brief M4-25 Out of scope; brief M4-30 Scope).
library;

import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import 'notifications_note.dart';
import 'settings_shell.dart';

/// The Notifications section: its heading, the note of M4-30, the place M4-33 fills.
class NotificationsEntry extends StatelessWidget {
  /// Creates the entry.
  const NotificationsEntry({super.key});

  @override
  Widget build(BuildContext context) => SectionFrame(
    key: const ValueKey('settings.notifications'),
    title: AppLocalizations.of(context).settingsSectionNotifications,
    children: const [NotificationsNote()],
  );
}
