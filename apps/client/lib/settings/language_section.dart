// SPDX-License-Identifier: Apache-2.0

/// Language and time zone of the profile (docs/spec/01-product.md 9: the language follows
/// `users.locale`; brief M4-25 Design note 2).
library;

import 'package:api_client/api_client.dart' show ApiProblem;
import 'package:design_system/design_system.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';

import 'settings_providers.dart';
import 'settings_shell.dart';

/// The Language section: German or English and the IANA time zone, both stored in the profile
/// with `updateMyProfile`; the app switches its language after the answer.
class LanguageSection extends ConsumerStatefulWidget {
  /// Creates the section.
  const LanguageSection({super.key, this.timeZones = ianaTimeZones});

  /// The time zones offered; the time zone database by default.
  final List<String> Function() timeZones;

  @override
  ConsumerState<LanguageSection> createState() => _LanguageSectionState();
}

class _LanguageSectionState extends ConsumerState<LanguageSection> {
  late final List<String> _zones = widget.timeZones();
  String? _error;

  Future<void> _change({String? locale, String? timeZone}) async {
    final l = AppLocalizations.of(context);
    try {
      await ref
          .read(profileControllerProvider.notifier)
          .change(locale: locale, timeZone: timeZone);
      if (mounted) setState(() => _error = null);
    } on DioException catch (e) {
      final problem = ApiProblem.of(e);
      if (!mounted) return;
      setState(
        () => _error = l.settingsProfileFailed(
          problem == null
              ? l.selfOffline
              : problemText(l, problem.code, reason: problem.reason),
        ),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final profile = ref.watch(profileControllerProvider).value;
    final ready = profile != null;
    final locale = profile?.locale;
    final zone = profile?.timeZone;
    return SectionFrame(
      title: l.settingsSectionLanguage,
      children: [
        if (_error case final error?)
          Padding(
            padding: const EdgeInsets.only(bottom: DsSpace.s8),
            child: DsBanner(
              key: const ValueKey('settings.profileError'),
              tone: DsTone.danger,
              text: error,
              liveRegion: true,
            ),
          ),
        SettingRow(
          label: l.settingsLanguage,
          control: DropdownButton<String>(
            key: const ValueKey('settings.language'),
            value: locale == 'de' || locale == 'en' ? locale : null,
            items: [
              DropdownMenuItem(
                value: 'de',
                child: Text(l.settingsLanguageGerman),
              ),
              DropdownMenuItem(
                value: 'en',
                child: Text(l.settingsLanguageEnglish),
              ),
            ],
            onChanged: !ready
                ? null
                : (value) {
                    if (value != null && value != locale) {
                      _change(locale: value);
                    }
                  },
          ),
        ),
        SettingRow(
          label: l.settingsTimeZone,
          control: DropdownMenu<String>(
            key: const ValueKey('settings.timeZone'),
            width: 280,
            enabled: ready,
            enableFilter: true,
            requestFocusOnTap: true,
            initialSelection: zone,
            menuHeight: 320,
            dropdownMenuEntries: [
              for (final z in _zones) DropdownMenuEntry(value: z, label: z),
            ],
            onSelected: (value) {
              if (value != null && value != zone) _change(timeZone: value);
            },
          ),
        ),
      ],
    );
  }
}
