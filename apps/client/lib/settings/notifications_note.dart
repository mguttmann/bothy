// SPDX-License-Identifier: Apache-2.0

/// The note of the Notifications settings (brief M4-30 Scope; docs/spec/
/// 13-push-benachrichtigungen.md 5.3, 15-client-architektur.md 10.3): notifications arrive only
/// while the app runs, and "Start at login", a device setting that is off by default.
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';

import '../desktop/providers.dart';
import 'settings_shell.dart';

/// The sentence of 13 5.3 and the switch "Start at login"; where the system offers no login
/// start for the app, a sentence says so instead of the switch.
class NotificationsNote extends ConsumerWidget {
  /// Creates the note.
  const NotificationsNote({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context);
    final login = ref.watch(startAtLoginSettingProvider);
    final value = login.value;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Text(
          l.settingsNotificationsWhileRunning,
          key: const ValueKey('settings.notificationsWhileRunning'),
          style: dsTextStyle(DsType.body)
              .copyWith(color: context.dsColors.textSecondary),
        ),
        if (value != null && value.supported)
          SettingRow(
            label: l.settingsStartAtLogin,
            control: Switch(
              key: const ValueKey('settings.startAtLogin'),
              value: value.enabled,
              onChanged: login.isLoading
                  ? null
                  : (on) => unawaited(
                      ref.read(startAtLoginSettingProvider.notifier).set(on),
                    ),
            ),
          )
        else if (value != null)
          Text(
            l.settingsStartAtLoginUnavailable,
            style: dsTextStyle(DsType.bodySm)
                .copyWith(color: context.dsColors.textSecondary),
          ),
      ],
    );
  }
}
