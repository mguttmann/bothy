// SPDX-License-Identifier: Apache-2.0

// The goldens of the settings sections (brief M4-25 Tests): light and dark, two densities,
// text scaling 1 and 2, German and English. One file per section keeps each file below
// repo.max_file_kb.

import 'dart:io';

import 'package:admin_ui/admin_ui.dart';
import 'package:alchemist/alchemist.dart';
import 'package:client/app/providers/ports.dart';
import 'package:client/app/providers/session.dart';
import 'package:client/settings/about_section.dart';
import 'package:client/settings/appearance_section.dart';
import 'package:client/settings/devices_section.dart';
import 'package:client/settings/keyboard_section.dart';
import 'package:client/settings/language_section.dart';
import 'package:client/settings/notifications_entry.dart';
import 'package:client/settings/privacy_section.dart';
import 'package:client/settings/settings_providers.dart';
import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../support/fakes.dart';

final DateTime now = DateTime.utc(2026, 10, 17, 9);

class _Profile extends ProfileController {
  @override
  Future<Profile> build() async =>
      const Profile(locale: 'de', timeZone: 'Europe/Berlin');
}

class _Self implements SelfRepository {
  @override
  Future<List<SelfDevice>> devices() async => [
    SelfDevice(
      id: 'laptop',
      name: 'Alex laptop',
      platform: 'macos',
      state: SelfDeviceState.active,
      createdAt: DateTime.utc(2026, 10),
      appVersion: '1.4.0',
      lastSeenAt: now,
      isThisDevice: true,
    ),
    SelfDevice(
      id: 'tablet',
      name: 'Alex tablet',
      platform: 'android',
      state: SelfDeviceState.pending,
      createdAt: DateTime.utc(2026, 10, 17, 8, 55),
    ),
  ];

  @override
  Future<List<SelfSession>> sessions() async => [
    SelfSession(
      id: 's1',
      deviceId: 'laptop',
      deviceName: 'Alex laptop',
      authMethod: SelfAuthMethod.passwordTotp,
      lastUsedAt: now,
      current: true,
    ),
  ];

  @override
  Future<SelfDevice> approveDevice(
    String deviceId,
    String code, {
    required String purpose,
  }) => throw UnimplementedError();

  @override
  Future<SelfDevice> approvePairing(String code, {required String purpose}) =>
      throw UnimplementedError();

  @override
  Future<SelfDevice> revokeDevice(
    String deviceId, {
    required String reason,
    required String purpose,
  }) => throw UnimplementedError();

  @override
  Future<void> revokeSession(String sessionId) => throw UnimplementedError();
}

Widget _section(Widget child) => ProviderScope(
  overrides: [
    appPortsProvider.overrideWithValue(
      testPorts(dataRoot: Directory.systemTemp),
    ),
    personalSettingsProvider.overrideWithValue(
      PersonalSettings.resolve(
        const {'appearance.theme': 'dark', 'layout.column_widths': {}},
        desktop: true,
        hasCrashTarget: false,
      ),
    ),
    settingsRepositoryProvider.overrideWithValue(null),
    profileControllerProvider.overrideWith(_Profile.new),
    selfRepositoryProvider.overrideWithValue(_Self()),
    instanceDescriptionProvider.overrideWith((ref) async => null),
    currentInstanceProvider.overrideWithValue(null),
  ],
  child: Material(
    type: MaterialType.transparency,
    child: SizedBox(
      width: 760,
      child: Padding(padding: const EdgeInsets.all(24), child: child),
    ),
  ),
);

final Map<String, Widget Function()> sections = {
  'appearance': () => _section(const AppearanceSection()),
  'notifications': () => _section(const NotificationsEntry()),
  'language': () => _section(
    LanguageSection(timeZones: () => const ['Europe/Berlin', 'UTC']),
  ),
  'keyboard': () => _section(const KeyboardSection()),
  'devices': () => _section(DevicesSection(clock: () => now)),
  'privacy': () => _section(const PrivacySection()),
  'about': () => _section(const AboutSection()),
};

void main() {
  for (final MapEntry(key: name, value: section) in sections.entries) {
    goldenTest(
      '$name in light and dark, two densities, text scale 1 and 2, de and en',
      fileName: 'settings_$name',
      builder: () => dsGoldenMatrix(
        name: name,
        builder: section,
        densities: const [DsDensityMode.comfortable, DsDensityMode.compact],
        textScales: const [1.0, 2.0],
        locales: const [Locale('de'), Locale('en')],
      ),
    );
  }
}
