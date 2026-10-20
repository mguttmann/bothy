// SPDX-License-Identifier: Apache-2.0

/// Privacy: crash reports and performance data (docs/spec/01-product.md 11; 15 16, 17; brief
/// M4-25 Design note 4). This section only stores the switches; M4-31 sends.
library;

import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';

import '../app/providers/ports.dart';
import 'settings_providers.dart';
import 'settings_shell.dart';

/// The Privacy section: `privacy.crash_reports` only where the build names a crash target,
/// `privacy.client_metrics` always.
class PrivacySection extends ConsumerWidget {
  /// Creates the section.
  const PrivacySection({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context);
    final settings = ref.watch(personalSettingsProvider);
    final writer = ref.watch(settingsRepositoryProvider);
    final crashTarget = ref.watch(appPortsProvider).branding.hasCrashTarget;
    ValueChanged<bool>? set(String key) =>
        writer == null ? null : (on) => unawaited(writer.set(key, on));
    return SectionFrame(
      title: l.settingsSectionPrivacy,
      children: [
        if (crashTarget)
          SettingRow(
            label: l.settingsCrashReports,
            hint: l.settingsCrashReportsHint,
            control: Switch(
              key: const ValueKey('settings.crashReports'),
              value: settings.crashReports,
              onChanged: set(SettingKeys.crashReports),
            ),
          ),
        SettingRow(
          label: l.settingsClientMetrics,
          hint: l.settingsClientMetricsHint,
          control: Switch(
            key: const ValueKey('settings.clientMetrics'),
            value: settings.clientMetrics,
            onChanged: set(SettingKeys.clientMetrics),
          ),
        ),
      ],
    );
  }
}
