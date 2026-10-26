// SPDX-License-Identifier: Apache-2.0

// The pages of the phone app that the screen tests do not build on their own (brief M5-10
// Design note 2; 01 5.3, 8): the Me page with the tab bar, the settings sections, Mentions,
// Search, the status page and "Not found", in the phone app with the fake ports, on iOS and on
// Android, and the language of the app in the semantics tree on the phone.

import 'dart:io';

import 'package:client/phone/phone_pages.dart' show phoneSettingsSections;
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../phone/support/phone_app.dart';

/// The paths of the pages, by the name the checklist uses.
final Map<String, String> pages = {
  'Me': '/settings',
  for (final section in phoneSettingsSections)
    'Settings ${section.path.split('/').last}': section.path,
  'Mentions': '/mentions',
  'Search': '/search?q=release',
  'Status': '/status',
  'Not found': '/c/unknown-channel',
};

void main() {
  for (final platform in ['ios', 'android']) {
    testWidgets(
      'the pages of the phone app on $platform: roles, labels, contrast and touch targets',
      (tester) async {
        final handle = tester.ensureSemantics();
        final root = Directory.systemTemp.createTempSync('a11y-pages');
        addTearDown(() => root.deleteSync(recursive: true));
        final app = await PhoneApp.start(
          tester,
          root: root,
          platform: platform,
        );
        for (final MapEntry(key: name, value: path) in pages.entries) {
          await app.go(path);
          await dsPumpIcons(tester, timeout: const Duration(minutes: 3));
          final problems = dsSemanticsProblems(tester);
          expect(problems, isEmpty, reason: '$name: ${problems.join('\n')}');
          await expectLater(tester, meetsGuideline(labeledTapTargetGuideline));
          await expectLater(tester, meetsGuideline(androidTapTargetGuideline));
          await expectLater(tester, meetsGuideline(iOSTapTargetGuideline));
        }
        handle.dispose();
      },
    );
  }

  for (final language in ['de', 'en']) {
    testWidgets('the phone app sets its language in the semantics tree: '
        '$language from the system', (tester) async {
      final root = Directory.systemTemp.createTempSync('a11y-language');
      addTearDown(() => root.deleteSync(recursive: true));
      tester.platformDispatcher.localesTestValue = [Locale(language)];
      addTearDown(tester.platformDispatcher.clearLocalesTestValue);
      await PhoneApp.start(tester, root: root);
      expect(
        tester.platformDispatcher.applicationLocale?.languageCode,
        language,
      );
    });
  }
}
