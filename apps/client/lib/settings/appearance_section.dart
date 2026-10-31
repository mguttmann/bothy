// SPDX-License-Identifier: Apache-2.0

/// Appearance: theme, density, motion, Enter sends and the column widths reset
/// (docs/spec/01-product.md 5.1 D8 and 11).
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';

import '../app/providers/ports.dart';
import 'settings_providers.dart';
import 'settings_shell.dart';

/// A choice of one value out of a few, as a row of chips (one selected).
class SettingChoice extends StatelessWidget {
  /// Creates the control.
  const SettingChoice({
    super.key,
    required this.value,
    required this.options,
    required this.onChanged,
  });

  /// The current value.
  final String value;

  /// The values and their labels, in order.
  final Map<String, String> options;

  /// Called with the chosen value; null disables the control.
  final ValueChanged<String>? onChanged;

  @override
  Widget build(BuildContext context) => Wrap(
    spacing: DsSpace.s8,
    runSpacing: DsSpace.s8,
    children: [
      for (final MapEntry(:key, value: label) in options.entries)
        ChoiceChip(
          key: ValueKey('choice.$key'),
          label: Text(label),
          showCheckmark: false,
          selected: key == value,
          onSelected: onChanged == null ? null : (_) => onChanged!(key),
        ),
    ],
  );
}

/// The switch of `composer.enter_sends`, shared by Appearance and Keyboard.
class EnterSendsRow extends ConsumerWidget {
  /// Creates the row.
  const EnterSendsRow({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context);
    final settings = ref.watch(personalSettingsProvider);
    final writer = ref.watch(settingsRepositoryProvider);
    return SettingRow(
      label: l.settingsEnterSends,
      hint: l.settingsEnterSendsHint,
      control: Switch(
        key: const ValueKey('settings.enterSends'),
        value: settings.enterSends,
        onChanged: writer == null
            ? null
            : (on) => unawaited(writer.set(SettingKeys.enterSends, on)),
      ),
    );
  }
}

/// The switch of `composer.paste_originals` (FIX-scan-state-stale): off, a pasted bitmap goes
/// as a smaller JPEG when that is much smaller; files attached as files never change.
class PasteOriginalsRow extends ConsumerWidget {
  /// Creates the row.
  const PasteOriginalsRow({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context);
    final settings = ref.watch(personalSettingsProvider);
    final writer = ref.watch(settingsRepositoryProvider);
    return SettingRow(
      label: l.settingsPasteOriginals,
      hint: l.settingsPasteOriginalsHint,
      control: Switch(
        key: const ValueKey('settings.pasteOriginals'),
        value: settings.pasteOriginals,
        onChanged: writer == null
            ? null
            : (on) => unawaited(writer.set(SettingKeys.pasteOriginals, on)),
      ),
    );
  }
}

/// The three switches of the sidebar ordering (01 6.16 and 11, brief M4-51): each level is
/// arranged by activity while its switch is on; off, the person's own order applies.
class SidebarArrangeRows extends ConsumerWidget {
  /// Creates the rows.
  const SidebarArrangeRows({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context);
    final a = ref.watch(personalSettingsProvider).arrangement;
    final writer = ref.watch(settingsRepositoryProvider);
    Widget row(String key, String setting, String label, bool value) =>
        SettingRow(
          label: label,
          hint: l.settingsAutoArrangeHint,
          control: Switch(
            key: ValueKey(key),
            value: value,
            onChanged: writer == null
                ? null
                : (on) => unawaited(writer.set(setting, on)),
          ),
        );
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Padding(
          padding: const EdgeInsets.only(top: DsSpace.s16, bottom: DsSpace.s4),
          child: Semantics(
            header: true,
            child: Text(
              l.settingsSidebarHeading,
              style: dsTextStyle(DsType.bodyStrong),
            ),
          ),
        ),
        row(
          'settings.autoArrangeSpaces',
          SettingKeys.autoArrangeSpaces,
          l.settingsAutoArrangeSpaces,
          a.autoSpaces,
        ),
        row(
          'settings.autoArrangeChannels',
          SettingKeys.autoArrangeChannels,
          l.settingsAutoArrangeChannels,
          a.autoChannels,
        ),
        row(
          'settings.autoArrangeChats',
          SettingKeys.autoArrangeChats,
          l.settingsAutoArrangeChats,
          a.autoChats,
        ),
      ],
    );
  }
}

/// The Appearance section.
class AppearanceSection extends ConsumerWidget {
  /// Creates the section.
  const AppearanceSection({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context);
    final settings = ref.watch(personalSettingsProvider);
    final writer = ref.watch(settingsRepositoryProvider);
    // 01 11: the phone has the density `touch` fixed and no columns (brief M5-02).
    final phone = ref.watch(appPortsProvider).formFactor.isPhone;
    ValueChanged<String>? set(String key) =>
        writer == null ? null : (value) => unawaited(writer.set(key, value));
    return SectionFrame(
      title: l.settingsSectionAppearance,
      children: [
        SettingRow(
          label: l.settingsTheme,
          mergeSemantics: false,
          control: SettingChoice(
            key: const ValueKey('settings.theme'),
            value: settings.theme,
            options: {
              'system': l.settingsThemeSystem,
              'light': l.settingsThemeLight,
              'dark': l.settingsThemeDark,
            },
            onChanged: set(SettingKeys.theme),
          ),
        ),
        if (!phone)
          SettingRow(
            label: l.settingsDensity,
            mergeSemantics: false,
            control: SettingChoice(
              key: const ValueKey('settings.density'),
              value: settings.density,
              options: {
                'comfortable': l.settingsDensityComfortable,
                'compact': l.settingsDensityCompact,
              },
              onChanged: set(SettingKeys.density),
            ),
          ),
        SettingRow(
          label: l.settingsMotion,
          mergeSemantics: false,
          control: SettingChoice(
            key: const ValueKey('settings.motion'),
            value: settings.reduceMotion,
            // `on` reduces motion, `off` keeps the full motion (01 11).
            options: {
              'system': l.settingsMotionSystem,
              'off': l.settingsMotionFull,
              'on': l.settingsMotionReduced,
            },
            onChanged: set(SettingKeys.reduceMotion),
          ),
        ),
        const EnterSendsRow(),
        // Pasting a bitmap is a desktop way of attaching (15 14).
        if (!phone) const PasteOriginalsRow(),
        // 01 6.16: the order of the sidebar levels, on the desktop and the phone.
        const SidebarArrangeRows(),
        if (!phone)
          SettingRow(
            label: l.settingsColumnWidths,
            control: OutlinedButton(
              key: const ValueKey('settings.columnWidths.reset'),
              onPressed: writer == null || !settings.hasColumnWidths
                  ? null
                  : () => unawaited(writer.reset(SettingKeys.columnWidths)),
              child: Text(l.settingsColumnWidthsReset),
            ),
          ),
      ],
    );
  }
}
