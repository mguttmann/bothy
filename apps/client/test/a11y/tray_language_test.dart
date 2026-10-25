// SPDX-License-Identifier: Apache-2.0

// The menu of the notification area speaks the language of the app (brief M4-32, a gap found
// by the desktop checklist; 01 8 "Language", 01 9): Narrator reads the tray menu in the language
// the person chose (`users.locale`), not in the system language the app started with.

import 'dart:io';

import 'package:client/app/providers/ports.dart';
import 'package:client/desktop/providers.dart';
import 'package:client/settings/settings_providers.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

import '../support/desktop_fakes.dart';
import '../support/fakes.dart';

/// A profile whose language the test changes.
class _Profile extends ProfileController {
  @override
  Future<Profile> build() async => const Profile(locale: 'de');

  void choose(String locale) => state = AsyncData(Profile(locale: locale));
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test('the tray menu follows the language the person chose', () async {
    final root = Directory.systemTemp.createTempSync('tray');
    addTearDown(() => root.deleteSync(recursive: true));
    final binding = TestWidgetsFlutterBinding.instance;
    binding.platformDispatcher.localesTestValue = [const Locale('en')];
    addTearDown(binding.platformDispatcher.clearLocalesTestValue);
    final tray = FakeTray();
    final c = ProviderContainer(
      overrides: [
        appPortsProvider.overrideWithValue(
          testPorts(dataRoot: root, platform: 'windows', tray: tray),
        ),
        profileControllerProvider.overrideWith(_Profile.new),
      ],
    );
    addTearDown(c.dispose);
    await c.read(profileControllerProvider.future);
    await c.read(desktopShellProvider.future);
    final de = lookupAppLocalizations(const Locale('de'));
    final en = lookupAppLocalizations(const Locale('en'));
    expect(
      tray.shown.last,
      '${testBranding.appName}|${de.trayOpen}|${de.trayQuit}',
    );
    (c.read(profileControllerProvider.notifier) as _Profile).choose('en');
    await Future<void>.delayed(Duration.zero);
    expect(
      tray.shown.last,
      '${testBranding.appName}|${en.trayOpen}|${en.trayQuit}',
    );
  });
}
