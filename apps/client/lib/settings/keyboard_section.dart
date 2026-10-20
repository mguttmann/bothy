// SPDX-License-Identifier: Apache-2.0

/// Keyboard: Enter sends and where the list of shortcuts is (docs/spec/01-product.md 5.1 D8
/// and 7; the overlay itself is M4-24).
library;

import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import 'appearance_section.dart';
import 'settings_shell.dart';

/// The Keyboard section.
class KeyboardSection extends StatelessWidget {
  /// Creates the section.
  const KeyboardSection({super.key});

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    // App shortcuts use Cmd on macOS and Ctrl on Windows (01 7, R-PR-10).
    final shortcut = defaultTargetPlatform == TargetPlatform.macOS
        ? 'Cmd+/'
        : 'Ctrl+/';
    return SectionFrame(
      title: l.settingsSectionKeyboard,
      children: [
        const EnterSendsRow(),
        SettingRow(
          label: l.settingsSectionKeyboard,
          control: Text(
            l.settingsShortcuts(shortcut),
            key: const ValueKey('settings.shortcuts'),
          ),
        ),
      ],
    );
  }
}
