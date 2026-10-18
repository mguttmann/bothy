// SPDX-License-Identifier: Apache-2.0

/// The Notifications entry of D8; M4-33 fills it with D9 (docs/spec/01-product.md 5.1 D8, D9;
/// brief M4-25 Out of scope).
library;

import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import 'settings_shell.dart';

/// The Notifications section: its heading, the place M4-33 fills.
class NotificationsEntry extends StatelessWidget {
  /// Creates the entry.
  const NotificationsEntry({super.key});

  @override
  Widget build(BuildContext context) => SectionFrame(
    key: const ValueKey('settings.notifications'),
    title: AppLocalizations.of(context).settingsSectionNotifications,
    children: const [],
  );
}
