// SPDX-License-Identifier: Apache-2.0

/// D8 Settings: the list of sections on the left and the open section on the right
/// (docs/spec/01-product.md 5.1 D8; route `/settings/<section>`, brief M4-25 Scope).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:l10n/l10n.dart';

import 'about_section.dart';
import 'appearance_section.dart';
import 'devices_section.dart';
import 'keyboard_section.dart';
import 'language_section.dart';
import 'notifications_entry.dart';
import 'privacy_section.dart';

/// The sections of D8 in their order, with their path segment.
enum SettingsSection {
  /// Theme, density, motion, Enter sends, column widths.
  appearance('appearance'),

  /// The link M4-33 fills (D9).
  notifications('notifications'),

  /// Language and time zone of the profile.
  language('language'),

  /// Enter sends and the shortcut of the overlay.
  keyboard('keyboard'),

  /// Own devices and sessions.
  devices('devices'),

  /// Crash reports and performance data.
  privacy('privacy'),

  /// Version and licenses.
  about('about');

  const SettingsSection(this.segment);

  /// The path segment below `/settings/`.
  final String segment;

  /// The path, for example `/settings/devices`.
  String get path => '/settings/$segment';

  /// The section of [segment], or null for an unknown one.
  static SettingsSection? of(String? segment) =>
      values.where((s) => s.segment == segment).firstOrNull;

  /// The name in the person's language.
  String label(AppLocalizations l) => switch (this) {
    appearance => l.settingsSectionAppearance,
    notifications => l.settingsSectionNotifications,
    language => l.settingsSectionLanguage,
    keyboard => l.settingsSectionKeyboard,
    devices => l.settingsSectionDevices,
    privacy => l.settingsSectionPrivacy,
    about => l.settingsSectionAbout,
  };
}

/// The settings screen with [segment] open; an unknown segment shows "Not found or no access"
/// (01 6.7).
class SettingsShell extends StatelessWidget {
  /// Creates the screen.
  const SettingsShell({super.key, required this.segment});

  /// The path segment of the open section.
  final String? segment;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final section = SettingsSection.of(segment);
    return Material(
      key: const ValueKey('screen.settings'),
      color: colors.surfaceCanvas,
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          SizedBox(
            width: DsLayout.sidebarWidth,
            child: Material(
              color: colors.surfaceRaised,
              child: ListView(
                padding: const EdgeInsets.all(DsSpace.s12),
                children: [
                  Align(
                    alignment: Alignment.centerLeft,
                    child: TextButton(
                      key: const ValueKey('settings.back'),
                      onPressed: () => context.go('/'),
                      child: Text(l.settingsBack),
                    ),
                  ),
                  Padding(
                    padding: const EdgeInsets.all(DsSpace.s8),
                    child: Semantics(
                      header: true,
                      child: Text(
                        l.settingsTitle,
                        style: dsTextStyle(DsType.title)
                            .copyWith(color: colors.textPrimary),
                      ),
                    ),
                  ),
                  for (final s in SettingsSection.values)
                    ListTile(
                      key: ValueKey('settings.nav.${s.segment}'),
                      selected: s == section,
                      selectedColor: colors.textPrimary,
                      selectedTileColor: colors.accentSubtle,
                      textColor: colors.textSecondary,
                      title: Text(s.label(l)),
                      onTap: () => context.go(s.path),
                    ),
                ],
              ),
            ),
          ),
          Expanded(
            child: section == null
                ? Center(
                    child: Text(
                      l.accessNotFoundOrNoAccess,
                      key: const ValueKey('settings.notFound'),
                      style: dsTextStyle(DsType.body)
                          .copyWith(color: colors.textSecondary),
                    ),
                  )
                : SingleChildScrollView(
                    padding: const EdgeInsets.all(DsSpace.s24),
                    child: Align(
                      alignment: Alignment.topLeft,
                      child: ConstrainedBox(
                        constraints: const BoxConstraints(maxWidth: 760),
                        child: sectionBody(section),
                      ),
                    ),
                  ),
          ),
        ],
      ),
    );
  }
}

/// The body of [section].
Widget sectionBody(SettingsSection section) => switch (section) {
  SettingsSection.appearance => const AppearanceSection(),
  SettingsSection.notifications => const NotificationsEntry(),
  SettingsSection.language => const LanguageSection(),
  SettingsSection.keyboard => const KeyboardSection(),
  SettingsSection.devices => const DevicesSection(),
  SettingsSection.privacy => const PrivacySection(),
  SettingsSection.about => const AboutSection(),
};

/// The frame of a section: its heading and its rows.
class SectionFrame extends StatelessWidget {
  /// Creates the frame.
  const SectionFrame({super.key, required this.title, required this.children});

  /// The heading.
  final String title;

  /// The rows.
  final List<Widget> children;

  @override
  Widget build(BuildContext context) => Column(
    crossAxisAlignment: CrossAxisAlignment.stretch,
    mainAxisSize: MainAxisSize.min,
    children: [
      Semantics(
        header: true,
        child: Text(
          title,
          style: dsTextStyle(DsType.title)
              .copyWith(color: context.dsColors.textPrimary),
        ),
      ),
      const SizedBox(height: DsSpace.s16),
      ...children,
    ],
  );
}

/// One row of a section: a label (with an optional hint below) and its control.
class SettingRow extends StatelessWidget {
  /// Creates the row.
  const SettingRow({
    super.key,
    required this.label,
    required this.control,
    this.hint,
    this.mergeSemantics = true,
  });

  /// The label.
  final String label;

  /// The hint below the label, or null.
  final String? hint;

  /// The control.
  final Widget control;

  /// Whether label, hint and control form one node for screen readers, so the control is
  /// named by the row: "Send performance data, switch, on" instead of an unnamed switch
  /// (01 8, WCAG 4.1.2; brief M4-32). Pass false for a control of several entries of its own
  /// (a choice of chips, a list), whose entries must stay separate nodes.
  final bool mergeSemantics;

  @override
  Widget build(BuildContext context) {
    final colors = context.dsColors;
    final row = Padding(
      padding: const EdgeInsets.symmetric(vertical: DsSpace.s8),
      child: Wrap(
        spacing: DsSpace.s16,
        runSpacing: DsSpace.s8,
        crossAxisAlignment: WrapCrossAlignment.center,
        children: [
          SizedBox(
            width: 200,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  label,
                  style: dsTextStyle(DsType.bodyStrong)
                      .copyWith(color: colors.textPrimary),
                ),
                if (hint != null)
                  Text(
                    hint!,
                    style: dsTextStyle(DsType.bodySm)
                        .copyWith(color: colors.textSecondary),
                  ),
              ],
            ),
          ),
          control,
        ],
      ),
    );
    return mergeSemantics ? MergeSemantics(child: row) : row;
  }
}
