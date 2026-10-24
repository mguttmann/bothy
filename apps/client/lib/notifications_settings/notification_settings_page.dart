// SPDX-License-Identifier: Apache-2.0

/// D9 Notification settings (01 5.1 D9; 13 3; brief M4-33 Scope): the personal default level,
/// the explanation of the runs the person triggered (01 10.1), the content of notifications,
/// the quiet hours, the switch that lets questions break quiet hours, the administration
/// reminders for owners and the tree of levels per space, channel and thread.
///
/// Every change goes through the outbox and reaches the person's other devices (04 14).
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';

import '../settings/appearance_section.dart' show SettingChoice;
import '../settings/settings_providers.dart';
import '../settings/settings_shell.dart' show SettingRow;
import 'effective_level.dart';
import 'level_tree.dart';
import 'providers.dart';
import 'quiet_hours_editor.dart';

/// The rows of D9.
class NotificationSettingsPage extends ConsumerWidget {
  /// Creates the page; [timeZones] are the zones the quiet hours editor offers.
  const NotificationSettingsPage({super.key, this.timeZones = ianaTimeZones});

  /// The zones offered in the quiet hours editor.
  final List<String> Function() timeZones;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final settings = ref.watch(notifySettingsProvider);
    final writer = ref.watch(settingsRepositoryProvider);
    final owner = ref.watch(isOwnerProvider).value ?? false;
    // Loads the levels and quiet hours of the server once per session (04 6.3).
    ref.watch(notifyRefreshProvider);
    void set(String key, Object value) {
      if (writer != null) unawaited(writer.set(key, value));
    }

    return Column(
      key: const ValueKey('notify.page'),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        SettingRow(
          label: l.notifySettingsDefault,
          hint: l.notifySettingsDefaultHint,
          control: SettingChoice(
            key: const ValueKey('notify.defaultLevel'),
            value: (settings.personalDefault ?? instanceDefaultLevel).code,
            options: {
              for (final level in const [
                NotificationLevel.all,
                NotificationLevel.mentions,
                NotificationLevel.mute,
              ])
                level.code: levelLabel(l, level),
            },
            onChanged: writer == null
                ? null
                : (value) => set(NotifyKeys.defaultLevel, value),
          ),
        ),
        SettingRow(
          label: l.notifySettingsRuns,
          control: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 420),
            child: Text(
              l.notifySettingsRunsExplain,
              key: const ValueKey('notify.runs'),
              style: dsTextStyle(DsType.body)
                  .copyWith(color: colors.textSecondary),
            ),
          ),
        ),
        SettingRow(
          label: l.notifySettingsPreview,
          control: SettingChoice(
            key: const ValueKey('notify.previewMode'),
            value: settings.previewMode,
            options: {
              'full': l.notifyPreviewFull,
              'names': l.notifyPreviewNames,
              'none': l.notifyPreviewNone,
            },
            onChanged: writer == null
                ? null
                : (value) => set(NotifyKeys.previewMode, value),
          ),
        ),
        SettingRow(
          label: l.notifySettingsQuietHours,
          control: QuietHoursList(timeZones: timeZones),
        ),
        SettingRow(
          label: l.notifySettingsInputBreaks,
          hint: l.notifySettingsInputBreaksHint,
          control: Switch(
            key: const ValueKey('notify.inputBreaks'),
            value: settings.inputBreaksQuietHours,
            onChanged: writer == null
                ? null
                : (on) => set(NotifyKeys.inputBreaksQuietHours, on),
          ),
        ),
        if (owner)
          SettingRow(
            label: l.notifySettingsAdmin,
            hint: l.notifySettingsAdminHint,
            control: Switch(
              key: const ValueKey('notify.adminEnabled'),
              value: settings.adminEnabled,
              onChanged: writer == null
                  ? null
                  : (on) => set(NotifyKeys.adminEnabled, on),
            ),
          ),
        const SizedBox(height: DsSpace.s16),
        Semantics(
          header: true,
          child: Text(
            l.notifySettingsTree,
            style: dsTextStyle(DsType.heading)
                .copyWith(color: colors.textPrimary),
          ),
        ),
        const SizedBox(height: DsSpace.s8),
        const LevelTree(),
      ],
    );
  }
}
