// SPDX-License-Identifier: Apache-2.0

// The three switches of the sidebar ordering (brief M4-51; 01 11 and 6.15; D-M4-51-5): their
// defaults (spaces and channels off, chats on), a change written through the outbox as a
// personal setting and applied at once, the default deleting the key, the arrangement the
// sidebar reads, and the switches in Settings, Appearance on the desktop and the phone, in
// English and German.

import 'dart:convert';
import 'dart:io';

import 'package:client/app/providers/ports.dart';
import 'package:client/app/providers/session.dart';
import 'package:client/app/providers/settings.dart';
import 'package:client/settings/appearance_section.dart';
import 'package:client/settings/settings_providers.dart';
import 'package:client/sidebar/sidebar_arrange.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

import '../support/engine.dart';
import '../support/fakes.dart';
import '../support/widgets.dart';

void main() {
  test('defaults of 01 11: spaces and channels manual, chats by activity', () {
    final s = PersonalSettings.resolve(
      const {},
      desktop: true,
      hasCrashTarget: false,
    );
    expect(s.arrangement, const SidebarArrangement());
    expect(s.arrangement.autoChats, isTrue);
    expect(s.arrangement.autoSpaces, isFalse);
    expect(s.arrangement.autoChannels, isFalse);
  });

  test('stored values apply; a value that is no bool counts as absent', () {
    final s = PersonalSettings.resolve(
      const {
        'sidebar.auto_arrange_spaces': true,
        'sidebar.auto_arrange_channels': 'yes',
        'sidebar.auto_arrange_chats': false,
      },
      desktop: false,
      hasCrashTarget: false,
    );
    expect(
      s.arrangement,
      const SidebarArrangement(autoSpaces: true, autoChats: false),
    );
  });

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

    Future<SidebarArrangement> settled() async {
      await Future<void>.delayed(const Duration(milliseconds: 30));
      return container.read(sidebarArrangementProvider);
    }

    Future<List<Map<String, Object?>>> outbox() async => [
      for (final e in await db.outboxDao.allEntries())
        {'kind': e.kind, ...jsonDecode(e.payload) as Map<String, Object?>},
    ];

    test('a switch writes the personal setting and applies at once; the '
        'default deletes it again', () async {
      final writer = container.read(settingsRepositoryProvider)!;
      await writer.set(SettingKeys.autoArrangeChannels, true);
      expect((await settled()).autoChannels, isTrue);
      await writer.set(SettingKeys.autoArrangeChats, false);
      expect((await settled()).autoChats, isFalse);
      await writer.set(SettingKeys.autoArrangeChats, true);
      expect((await settled()).autoChats, isTrue);
      expect(await outbox(), [
        {
          'kind': 'putMySetting',
          'key': 'sidebar.auto_arrange_channels',
          'value': true,
        },
        {
          'kind': 'putMySetting',
          'key': 'sidebar.auto_arrange_chats',
          'value': false,
        },
        {'kind': 'deleteMySetting', 'key': 'sidebar.auto_arrange_chats'},
      ]);
    });
  });

  for (final (phone, locale, labels) in [
    (
      false,
      const Locale('en'),
      [
        'Arrange spaces automatically',
        'Arrange channels and projects automatically',
        'Arrange chats automatically',
      ],
    ),
    (
      true,
      const Locale('de'),
      [
        'Bereiche automatisch anordnen',
        // language-check: off
        'Kanäle und Projekte automatisch anordnen',
        // language-check: on
        'Chats automatisch anordnen',
      ],
    ),
  ]) {
    testWidgets('Appearance shows the three switches with their defaults '
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
      for (final label in labels) {
        expect(find.text(label), findsOneWidget);
      }
      bool on(String key) =>
          tester.widget<Switch>(find.byKey(ValueKey(key))).value;
      expect(on('settings.autoArrangeSpaces'), isFalse);
      expect(on('settings.autoArrangeChannels'), isFalse);
      expect(on('settings.autoArrangeChats'), isTrue);
    });
  }
}
