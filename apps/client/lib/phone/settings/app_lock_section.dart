// SPDX-License-Identifier: Apache-2.0

/// Settings, Security on the phone: the app lock switch (docs/spec/01-product.md 11; 08 9;
/// brief M5-08 Design note 4; docs/decisions.d/M5-08.md D-M5-08-4).
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';

import '../../app/providers/session.dart';
import '../../lock/app_lock_controller.dart';
import '../../lock/providers.dart';
import '../../settings/settings_shell.dart';

/// The Security section: the app lock switch, device-local and not synced. While
/// `security.app_lock_required` holds the switch shows on and disabled with the reason;
/// turning it on asks for the system authentication once; a device without a device
/// credential keeps it off with the way to the system settings.
class AppLockSection extends ConsumerStatefulWidget {
  /// Creates the section.
  const AppLockSection({super.key});

  @override
  ConsumerState<AppLockSection> createState() => _AppLockSectionState();
}

class _AppLockSectionState extends ConsumerState<AppLockSection> {
  AppLockSwitchResult? _last;

  Future<void> _set(AppLockController lock, bool on, String reason) async {
    final result = await lock.setEnabled(on, reason: reason);
    if (mounted) setState(() => _last = result);
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final lock = ref.watch(appLockProvider);
    final productName = ref.watch(currentInstanceProvider)?.productName ?? '';
    return SectionFrame(
      title: l.settingsSectionSecurity,
      children: [
        if (lock != null)
          ListenableBuilder(
            listenable: lock,
            builder: (context, _) {
              final notice = switch (_last) {
                AppLockSwitchResult.noCredential =>
                  l.settingsAppLockNoCredential,
                AppLockSwitchResult.failed => l.settingsAppLockFailed,
                _ => null,
              };
              return Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                mainAxisSize: MainAxisSize.min,
                children: [
                  SettingRow(
                    label: l.settingsAppLock,
                    hint: lock.required
                        ? l.settingsAppLockRequired
                        : l.settingsAppLockHint,
                    control: Switch(
                      key: const ValueKey('settings.appLock'),
                      value: lock.enabled || lock.required,
                      onChanged: lock.required || lock.busy
                          ? null
                          : (on) => unawaited(
                              _set(lock, on, l.appLockReason(productName)),
                            ),
                    ),
                  ),
                  if (notice != null)
                    DsBanner(
                      key: const ValueKey('settings.appLock.notice'),
                      tone: DsTone.warning,
                      text: notice,
                      liveRegion: true,
                    ),
                ],
              );
            },
          ),
      ],
    );
  }
}
