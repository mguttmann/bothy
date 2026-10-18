// SPDX-License-Identifier: Apache-2.0

/// About: the version of the app and of the instance and the licenses (docs/spec/01-product.md
/// 5.1 D8; brief M4-25 Design note 5).
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';

import '../app/providers/ports.dart';
import '../app/providers/session.dart';
import 'settings_providers.dart';
import 'settings_shell.dart';

/// The About section: app version of the build, `spec_version` of `getInstance` and Flutter's
/// license page (`LicenseRegistry`).
class AboutSection extends ConsumerWidget {
  /// Creates the section.
  const AboutSection({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context);
    final branding = ref.watch(appPortsProvider).branding;
    final name = ref.watch(currentInstanceProvider)?.productName;
    final spec = ref.watch(instanceDescriptionProvider).value?.specVersion;
    return SectionFrame(
      title: l.settingsSectionAbout,
      children: [
        SettingRow(
          label: l.settingsAboutAppVersion,
          control: Text(
            branding.appVersion,
            key: const ValueKey('settings.appVersion'),
          ),
        ),
        SettingRow(
          label: l.settingsAboutServerVersion,
          control: Text(
            spec ?? '',
            key: const ValueKey('settings.specVersion'),
          ),
        ),
        SettingRow(
          label: l.settingsAboutLicenses,
          control: OutlinedButton(
            key: const ValueKey('settings.licenses'),
            onPressed: () => showLicensePage(
              context: context,
              applicationName: name ?? branding.appName,
              applicationVersion: branding.appVersion,
            ),
            child: Text(l.settingsAboutLicenses),
          ),
        ),
      ],
    );
  }
}
