// SPDX-License-Identifier: Apache-2.0

// The switch "Show status on projects" (brief M4-52; 01 11 and 6.17 item 6): its default (on), a
// change written through the outbox as a personal setting and applied at once, the default
// deleting the key, and the switch in Settings, Appearance on the desktop and the phone, in
// English and German.

import 'dart:convert';
import 'dart:io';

import 'package:client/app/providers/ports.dart';
import 'package:client/app/providers/session.dart';
import 'package:client/app/providers/settings.dart';
import 'package:client/settings/appearance_section.dart';
import 'package:client/settings/settings_providers.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

import '../support/engine.dart';
import '../support/fakes.dart';
import '../support/widgets.dart';

void main() {
  test(
    'default of 01 11: on; a stored false applies, a non-bool is absent',
    () {
      PersonalSettings resolve(Map<String, Object?> stored) =>
          PersonalSettings.resolve(
            stored,
            desktop: true,
            hasCrashTarget: false,
          );
      expect(resolve(const {}).showProjectStatus, isTrue);
      expect(
        resolve(const {'sidebar.show_project_status': false}).showProjectStatus,
        isFalse,
      );
      expect(
        resolve(const {'sidebar.show_project_status': 'no'}).showProjectStatus,
        isTrue,
      );
      expect(SettingKeys.showProjectStatus, 'sidebar.show_project_status');
      expect(
        PersonalSettings.fixedDefaults[SettingKeys.showProjectStatus],
        true,
      );
    },
  );

  group('writes', () {
    late LocalDatabase db;
    late ({SyncEngine engine, dynamic auth}) idle;
    late ProviderContainer container;

    setUp(() {
      db = openInMemoryForTests();
      final e = idleEngine(db);
      idle = (engine: e.engine, auth: e.auth);
      container = ProviderContainer(
        overrides: [
          localDatabaseProvider.overrideWithValue(db),
          syncEngineProvider.overrideWithValue(idle.engine),
          appPortsProvider.overrideWithValue(
            testPorts(dataRoot: Directory.systemTemp),
          ),
        ],
      );
      container.listen(userSettingsProvider, (_, _) {});
    });

    tearDown(() async {
      container.dispose();
      await idle.engine.dispose();
      await idle.auth.dispose();
      await db.close();
    });

    Future<bool> settled() async {
      await Future<void>.delayed(const Duration(milliseconds: 30));
      return container.read(showProjectStatusProvider);
    }

    test('the switch writes the personal setting and applies at once; the '
        'default deletes it again', () async {
      expect(await settled(), isTrue);
      final writer = container.read(settingsRepositoryProvider)!;
      await writer.set(SettingKeys.showProjectStatus, false);
      expect(await settled(), isFalse);
      await writer.set(SettingKeys.showProjectStatus, true);
      expect(await settled(), isTrue);
      expect(
        [
          for (final e in await db.outboxDao.allEntries())
            {'kind': e.kind, ...jsonDecode(e.payload) as Map<String, Object?>},
        ],
        [
          {
            'kind': 'putMySetting',
            'key': 'sidebar.show_project_status',
            'value': false,
          },
          {'kind': 'deleteMySetting', 'key': 'sidebar.show_project_status'},
        ],
      );
    });
  });

  for (final (phone, locale, label) in [
    (false, const Locale('en'), 'Show status on projects'),
    (true, const Locale('de'), 'Status auf Projekten anzeigen'),
  ]) {
    testWidgets('Appearance shows the switch, on by default '
        '(${phone ? 'phone' : 'desktop'}, ${locale.languageCode})', (
      tester,
    ) async {
      desktopWindow(tester);
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            appPortsProvider.overrideWithValue(
              testPorts(
                dataRoot: Directory.systemTemp,
                platform: phone ? 'ios' : 'macos',
              ),
            ),
            personalSettingsProvider.overrideWithValue(
              PersonalSettings.resolve(
                const {},
                desktop: !phone,
                hasCrashTarget: false,
              ),
            ),
            settingsRepositoryProvider.overrideWithValue(null),
          ],
          child: hostScreen(
            const Scaffold(
              body: SingleChildScrollView(child: AppearanceSection()),
            ),
            locale: locale,
          ),
        ),
      );
      expect(find.text(label), findsOneWidget);
      expect(
        tester
            .widget<Switch>(
              find.byKey(const ValueKey('settings.showProjectStatus')),
            )
            .value,
        isTrue,
      );
    });
  }
}
