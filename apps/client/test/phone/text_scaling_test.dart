// SPDX-License-Identifier: Apache-2.0

// Text scaling on the phone (brief M5-02 fixed requirement 12, Tests; 01 8, WCAG SC 1.4.4):
// every phone screen at text scale 2 lays out without an overflow (Flutter's overflow errors
// fail the test) and every action of the screen stays reachable.

import 'dart:io';

import 'package:client/phone/phone_navigation.dart';
import 'package:client/phone/phone_pages.dart' show phoneSettingsSections;
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

import '../goldens/onboarding/onboarding_golden_test.dart' as onboarding;
import 'support/phone_app.dart';
import 'support/phone_host.dart';
import 'support/phone_scenes.dart';

/// The actions each scene must keep reachable.
Map<String, List<Finder>> actions(AppLocalizations l) => {
  'm1_spaces': [
    find.byKey(const ValueKey('sidebar.jumpTo')),
    find.byKey(const ValueKey('sidebar.runningNow')),
    for (final tab in PhoneTab.values)
      find.byKey(ValueKey('phone.tab.${tab.name}')),
  ],
  'm2_post_list': [
    find.byKey(const ValueKey('phone.back')),
    find.byKey(const ValueKey('postList.newPost')),
    find.byKey(const ValueKey('postList.filter.waiting')),
  ],
  'm3_running': [
    find.byKey(const ValueKey('phone.back')),
    find.byKey(const ValueKey('thread.more')),
    find.bySemanticsLabel(l.liveBoxStopRun('Robin')),
    find.byKey(const ValueKey('ds-component-once')),
    find.byKey(const ValueKey('ds-component-session')),
    find.byKey(const ValueKey('ds-component-deny')),
    find.byType(TextField),
  ],
  'm3_waiting': [
    find.bySemanticsLabel(l.liveBoxStopRun('Robin')),
    find.byType(TextField),
  ],
  'm3_collapsed': [
    find.byKey(const ValueKey('scene.livebox')),
    find.byType(TextField),
  ],
  'm4_components': [
    find.byKey(const ValueKey('ds-component-select-model')),
    find.byKey(const ValueKey('ds-component-form-details')),
  ],
  'm7_offline': [
    find.byKey(const ValueKey('shell.offline')),
    find.text(l.messageEdit),
    find.text(l.messageDiscard),
    find.byKey(const ValueKey('phone.back')),
  ],
};

void main() {
  for (final locale in const [Locale('en'), Locale('de')]) {
    for (final MapEntry(key: name, value: scene) in phoneScenes.entries) {
      testWidgets('$name at text scale 2 in ${locale.languageCode}', (
        tester,
      ) async {
        final semantics = tester.ensureSemantics();
        phoneWindow(tester);
        await tester.pumpWidget(
          hostPhone(Center(child: scene()), textScale: 2, locale: locale),
        );
        await tester.pumpAndSettle();
        final error = tester.takeException();
        expect(
          error,
          isNull,
          reason: error is FlutterError ? error.toStringDeep() : '$error',
        );
        final l = lookupAppLocalizations(locale);
        for (final action in actions(l)[name]!) {
          // Scrolled into view where the screen scrolls, then hit by a tap.
          if (action.evaluate().isEmpty) {
            await tester.scrollUntilVisible(
              action,
              100,
              scrollable: find
                  .descendant(
                    of: find.byType(ListView),
                    matching: find.byType(Scrollable),
                  )
                  .first,
            );
          }
          await tester.ensureVisible(action.first);
          await tester.pumpAndSettle();
          expect(action.first.hitTestable(), findsOneWidget, reason: '$action');
        }
        semantics.dispose();
      });
    }
  }

  for (final MapEntry(key: name, value: screen)
      in onboarding.screens().entries) {
    testWidgets('onboarding $name at text scale 2 fills the phone', (
      tester,
    ) async {
      phoneWindow(tester);
      for (final locale in const [Locale('en'), Locale('de')]) {
        await tester.pumpWidget(
          hostPhone(screen(), textScale: 2, locale: locale),
        );
        await tester.pumpAndSettle();
        expect(tester.takeException(), isNull, reason: locale.toString());
        expect(find.byKey(const ValueKey('onboarding.phone')), findsOneWidget);
      }
    });
  }

  testWidgets('the pages of the tabs, search and settings at text scale 2', (
    tester,
  ) async {
    final root = Directory.systemTemp.createTempSync('scale');
    addTearDown(() => root.deleteSync(recursive: true));
    tester.platformDispatcher.textScaleFactorTestValue = 2;
    addTearDown(tester.platformDispatcher.clearTextScaleFactorTestValue);
    final app = await PhoneApp.start(tester, root: root);
    for (final path in [
      '/',
      '/mentions',
      '/running',
      '/settings',
      '/search?q=release',
      for (final section in phoneSettingsSections) section.path,
    ]) {
      await app.go(path);
      expect(tester.takeException(), isNull, reason: path);
    }
  });
}
