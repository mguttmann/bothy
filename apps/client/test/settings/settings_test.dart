// SPDX-License-Identifier: Apache-2.0

// The personal settings (brief M4-25 Tests): the defaults of 01 11 when keys are absent; a
// change writes one outbox entry and applies at once; reset deletes the key; theme, density
// and motion switch the theme; the sections show what 01 11, 15 16 and 15 17 say.

import 'dart:convert';
import 'dart:io';

import 'package:client/app/branding.dart';
import 'package:client/app/providers/ports.dart';
import 'package:client/app/providers/session.dart';
import 'package:client/app/providers/settings.dart';
import 'package:client/settings/appearance_section.dart';
import 'package:client/settings/keyboard_section.dart';
import 'package:client/settings/language_section.dart';
import 'package:client/settings/privacy_section.dart';
import 'package:client/settings/settings_providers.dart';
import 'package:design_system/design_system.dart';
import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_riverpod/misc.dart' show Override;
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

import '../support/engine.dart';
import '../support/fakes.dart';
import '../support/widgets.dart';

void main() {
  group('defaults of 01 11', () {
    test('absent keys give the defaults; Enter sends on desktop', () {
      final s = PersonalSettings.resolve(
        const {},
        desktop: true,
        hasCrashTarget: false,
      );
      expect(s.theme, 'system');
      expect(s.density, 'comfortable');
      expect(s.reduceMotion, 'system');
      expect(s.enterSends, isTrue);
      expect(s.crashReports, isFalse);
      expect(s.clientMetrics, isTrue);
      expect(s.hasColumnWidths, isFalse);
    });

    test('device and instance dependent defaults', () {
      final mobile = PersonalSettings.resolve(
        const {},
        desktop: false,
        hasCrashTarget: true,
      );
      expect(mobile.enterSends, isFalse);
      expect(mobile.crashReports, isTrue);
    });

    test(
      'a stored value applies; a value outside the list counts as absent',
      () {
        final s = PersonalSettings.resolve(
          const {
            'appearance.theme': 'dark',
            'appearance.density': 'touch',
            'composer.enter_sends': false,
            'privacy.client_metrics': 'yes',
            'layout.column_widths': {'sidebar': 300},
          },
          desktop: true,
          hasCrashTarget: false,
        );
        expect(s.theme, 'dark');
        expect(s.density, 'comfortable');
        expect(s.enterSends, isFalse);
        expect(s.clientMetrics, isTrue);
        expect(s.hasColumnWidths, isTrue);
      },
    );
  });

  group('SettingsRepository', () {
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

    Future<PersonalSettings> settled() async {
      await Future<void>.delayed(const Duration(milliseconds: 30));
      return container.read(personalSettingsProvider);
    }

    Future<List<Map<String, Object?>>> outbox() async => [
      for (final e in await db.outboxDao.allEntries())
        {'kind': e.kind, ...jsonDecode(e.payload) as Map<String, Object?>},
    ];

    SettingsRepository writer() => container.read(settingsRepositoryProvider)!;

    test('a change writes one outbox entry and applies at once', () async {
      await writer().set(SettingKeys.enterSends, false);

      expect((await settled()).enterSends, isFalse);
      expect(await outbox(), [
        {
          'kind': PersonalOps.putMySetting,
          'key': 'composer.enter_sends',
          'value': false,
        },
      ]);
    });

    test('the same value twice writes once', () async {
      await writer().set(SettingKeys.theme, 'dark');
      await writer().set(SettingKeys.theme, 'dark');
      expect(await outbox(), hasLength(1));
    });

    test('reset deletes the key and the default applies again', () async {
      await writer().set(SettingKeys.theme, 'dark');
      expect((await settled()).theme, 'dark');

      await writer().reset(SettingKeys.theme);

      expect((await settled()).theme, 'system');
      expect((await outbox()).last, {
        'kind': PersonalOps.deleteMySetting,
        'key': 'appearance.theme',
      });
    });

    test('choosing a fixed default deletes instead of storing it', () async {
      await writer().set(SettingKeys.density, 'compact');
      await writer().set(SettingKeys.density, 'comfortable');
      expect((await outbox()).map((e) => e['kind']), [
        PersonalOps.putMySetting,
        PersonalOps.deleteMySetting,
      ]);
      expect(await db.personalDao.watchSettings().first, isEmpty);
    });

    test('composer.paste_originals is off by default and on deletes nothing '
        'but stores true (FIX-scan-state-stale)', () async {
      expect(
        PersonalSettings.resolve(
          const {},
          desktop: true,
          hasCrashTarget: false,
        ).pasteOriginals,
        isFalse,
      );
      expect(
        PersonalSettings.resolve(
          const {SettingKeys.pasteOriginals: true},
          desktop: true,
          hasCrashTarget: false,
        ).pasteOriginals,
        isTrue,
      );
      await writer().set(SettingKeys.pasteOriginals, true);
      await writer().set(SettingKeys.pasteOriginals, false);
      expect((await outbox()).map((e) => e['kind']), [
        PersonalOps.putMySetting,
        PersonalOps.deleteMySetting,
      ]);
    });

    test('reset of an absent key sends nothing', () async {
      await writer().reset(SettingKeys.columnWidths);
      expect(await outbox(), isEmpty);
    });

    test('theme, density and motion switch the theme', () async {
      DsContext ds() =>
          container.read(appearanceProvider(Brightness.light, false));
      container.listen(appearanceProvider(Brightness.light, false), (_, _) {});
      expect(ds().theme, DsThemeMode.light);

      await writer().set(SettingKeys.theme, 'dark');
      await writer().set(SettingKeys.density, 'compact');
      await writer().set(SettingKeys.reduceMotion, 'on');
      await settled();

      expect(ds().theme, DsThemeMode.dark);
      expect(ds().density, DsDensityMode.compact);
      expect(ds().motion, DsMotionMode.reduced);
    });
  });

  group('sections', () {
    Widget section(
      Widget child, {
      PersonalSettings? settings,
      BuildBranding branding = testBranding,
      List<Override> extra = const [],
    }) => ProviderScope(
      overrides: [
        appPortsProvider.overrideWithValue(
          testPorts(dataRoot: Directory.systemTemp, branding: branding),
        ),
        personalSettingsProvider.overrideWithValue(
          settings ??
              PersonalSettings.resolve(
                const {},
                desktop: true,
                hasCrashTarget: branding.hasCrashTarget,
              ),
        ),
        settingsRepositoryProvider.overrideWithValue(null),
        ...extra,
      ],
      child: hostScreen(Scaffold(body: child)),
    );

    testWidgets('appearance shows the defaults', (tester) async {
      desktopWindow(tester);
      await tester.pumpWidget(section(const AppearanceSection()));
      final selected = tester.widget<ChoiceChip>(
        find.descendant(
          of: find.byKey(const ValueKey('settings.theme')),
          matching: find.byKey(const ValueKey('choice.system')),
        ),
      );
      expect(selected.selected, isTrue);
      expect(
        tester
            .widget<Switch>(find.byKey(const ValueKey('settings.enterSends')))
            .value,
        isTrue,
      );
      expect(
        tester
            .widget<OutlinedButton>(
              find.byKey(const ValueKey('settings.columnWidths.reset')),
            )
            .onPressed,
        isNull,
      );
      // A pasted screenshot goes as a smaller JPEG unless switched on (FIX-scan-state-stale).
      expect(
        tester
            .widget<Switch>(
              find.byKey(const ValueKey('settings.pasteOriginals')),
            )
            .value,
        isFalse,
      );
    });

    testWidgets('crash reports only with a crash target', (tester) async {
      desktopWindow(tester);
      await tester.pumpWidget(section(const PrivacySection()));
      expect(find.byKey(const ValueKey('settings.crashReports')), findsNothing);
      expect(
        find.byKey(const ValueKey('settings.clientMetrics')),
        findsOneWidget,
      );

      const withTarget = BuildBranding(
        appName: 'ExampleChat',
        urlScheme: 'chat-example',
        bundleId: 'com.example.chat',
        defaultServer: 'https://chat.example.com',
        appVersion: '1.0.0',
        crashDsn: 'https://public@crash.example.com/1',
      );
      await tester.pumpWidget(
        section(const PrivacySection(), branding: withTarget),
      );
      expect(
        tester
            .widget<Switch>(find.byKey(const ValueKey('settings.crashReports')))
            .value,
        isTrue,
      );
    });

    testWidgets('keyboard names the shortcut of the overlay', (tester) async {
      desktopWindow(tester);
      debugDefaultTargetPlatformOverride = TargetPlatform.windows;
      await tester.pumpWidget(section(const KeyboardSection()));
      expect(find.text('All keyboard shortcuts: Ctrl+/'), findsOneWidget);
      debugDefaultTargetPlatformOverride = null;
    });

    testWidgets('language changes the profile and shows a failure', (
      tester,
    ) async {
      desktopWindow(tester);
      final profile = _FakeProfile();
      await tester.pumpWidget(
        section(
          LanguageSection(timeZones: () => const ['Europe/Berlin', 'UTC']),
          extra: [profileControllerProvider.overrideWith(() => profile)],
        ),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey('settings.language')));
      await tester.pumpAndSettle();
      await tester.tap(find.text('English').last);
      await tester.pumpAndSettle();
      expect(profile.changes, ['locale=en']);

      profile.fail = true;
      await tester.tap(find.byKey(const ValueKey('settings.language')));
      await tester.pumpAndSettle();
      await tester.tap(find.text('German').last);
      await tester.pumpAndSettle();
      expect(
        find.text(
          'The profile could not be saved: No connection. Try again when you '
          'are online.',
        ),
        findsOneWidget,
      );
    });
  });
}

class _FakeProfile extends ProfileController {
  final List<String> changes = [];
  bool fail = false;

  @override
  Future<Profile> build() async =>
      const Profile(locale: 'de', timeZone: 'Europe/Berlin');

  @override
  Future<void> change({String? locale, String? timeZone}) async {
    if (fail) {
      throw DioException(requestOptions: RequestOptions(path: '/me/profile'));
    }
    changes.add('locale=$locale');
    state = AsyncData(Profile(locale: locale, timeZone: timeZone));
  }
}
