// SPDX-License-Identifier: Apache-2.0

/// The screen "Instance and branding" of 01 5.5: instance settings, branding, login providers
/// and feature flags under one heading (brief M6-09 Scope).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../data/admin_repository.dart';
import 'branding_section.dart';
import 'flags_section.dart';
import 'instance_common.dart';
import 'providers_section.dart';
import 'settings_section.dart';

/// The parts of the instance screen.
enum InstanceTab {
  /// The instance settings ([SettingsSection]).
  settings,

  /// Product name, logo and accent ([BrandingSection]).
  branding,

  /// The login providers ([ProvidersSection]).
  providers,

  /// The feature flags ([FlagsSection]).
  flags,
}

/// The instance administration for owners: a choice of [InstanceTab] and its section.
///
/// Parameters: [repository] serves every section; [pickLogo] is the host's file dialog for
/// the logo (null: no upload); [onBrandingChanged] runs after a change of the branding or the
/// logo (the console reads discovery again, so its theme follows); [instanceChanges] are the
/// desktop's `instance.updated` events, after which the settings and the flags load again
/// (null in the console, which reloads after its own change); [onChanged] runs after every
/// change. The screen holds no permission logic: a list the server refuses shows the server's
/// reason (15 1 principle 1).
///
/// Keys: `instance.view`, `instance.tabs`, `instance.tab.<tab>`.
///
/// Example: `InstanceView(repository: admin, pickLogo: picker)`.
class InstanceView extends StatefulWidget {
  /// Creates the view.
  const InstanceView({
    super.key,
    required this.repository,
    this.initialTab = InstanceTab.settings,
    this.pickLogo,
    this.onBrandingChanged,
    this.instanceChanges,
    this.onChanged,
    this.timeZone = 'UTC',
    this.clock = DateTime.now,
  });

  /// The administration's calls.
  final AdminRepository repository;

  /// The part shown first.
  final InstanceTab initialTab;

  /// The host's file dialog for the logo, or null.
  final LogoPicker? pickLogo;

  /// Runs after a change of the branding or the logo.
  final VoidCallback? onBrandingChanged;

  /// Opens the `instance.updated` events of the desktop (a new stream per call, one per
  /// section), or null.
  final Stream<void> Function()? instanceChanges;

  /// Runs after every change.
  final VoidCallback? onChanged;

  /// The time zone of the signed-in person.
  final String timeZone;

  /// The current time.
  final DateTime Function() clock;

  @override
  State<InstanceView> createState() => _InstanceViewState();
}

class _InstanceViewState extends State<InstanceView> {
  late InstanceTab _tab = widget.initialTab;

  String _label(AppLocalizations l, InstanceTab tab) => switch (tab) {
    InstanceTab.settings => l.instanceTabSettings,
    InstanceTab.branding => l.instanceTabBranding,
    InstanceTab.providers => l.instanceTabProviders,
    InstanceTab.flags => l.instanceTabFlags,
  };

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final changes = widget.instanceChanges;
    return Column(
      key: const ValueKey('instance.view'),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Semantics(
          header: true,
          child: Text(
            l.instanceTitle,
            style: dsTextStyle(DsType.title)
                .copyWith(color: colors.textPrimary),
          ),
        ),
        const SizedBox(height: DsSpace.s12),
        Align(
          alignment: Alignment.centerLeft,
          child: SingleChildScrollView(
            scrollDirection: Axis.horizontal,
            child: SegmentedButton<InstanceTab>(
              key: const ValueKey('instance.tabs'),
              showSelectedIcon: false,
              segments: [
                for (final tab in InstanceTab.values)
                  ButtonSegment(
                    value: tab,
                    label: Text(
                      _label(l, tab),
                      key: ValueKey('instance.tab.${tab.name}'),
                    ),
                  ),
              ],
              selected: {_tab},
              onSelectionChanged: (s) => setState(() => _tab = s.first),
            ),
          ),
        ),
        const SizedBox(height: DsSpace.s16),
        KeyedSubtree(
          key: ValueKey('instance.section.${_tab.name}'),
          child: switch (_tab) {
            InstanceTab.settings => SettingsSection(
              repository: widget.repository,
              timeZone: widget.timeZone,
              clock: widget.clock,
              changes: changes,
              onChanged: widget.onChanged,
            ),
            InstanceTab.branding => BrandingSection(
              repository: widget.repository,
              pickLogo: widget.pickLogo,
              onBrandingChanged: () {
                widget.onBrandingChanged?.call();
                widget.onChanged?.call();
              },
            ),
            InstanceTab.providers => ProvidersSection(
              repository: widget.repository,
              onChanged: widget.onChanged,
            ),
            InstanceTab.flags => FlagsSection(
              repository: widget.repository,
              timeZone: widget.timeZone,
              clock: widget.clock,
              changes: changes,
              onChanged: widget.onChanged,
            ),
          },
        ),
      ],
    );
  }
}
