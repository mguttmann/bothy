// SPDX-License-Identifier: Apache-2.0

// Settings in sync across two simulated devices of Alex (brief M4-25 Tests): theme and Enter
// sends changed in the settings of the app appear on the second device, and a reset too
// (04 14).

import 'package:client/app/providers/settings.dart';
import 'package:client/settings/settings_providers.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';

import 'support/test_app.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('theme and Enter sends reach the other device, a reset too', (
    tester,
  ) async {
    final app = await TestApp.start(tester);
    final second = await app.secondApp();
    try {
      await app.connect();
      await app.loginWithPassword();
      await app.waitFor(find.byKey(const ValueKey('column.sidebar')));
      await app.waitLive();
      second.container.listen(userSettingsProvider, (_, _) {});
      await second.signIn();

      await app.openSettings('appearance');
      await tester.tap(find.byKey(const ValueKey('choice.dark')));
      await tester.tap(find.byKey(const ValueKey('settings.enterSends')));
      await tester.pumpAndSettle();
      await eventually('the second device follows', () {
        final s = second.container.read(personalSettingsProvider);
        return s.theme == 'dark' && !s.enterSends;
      });

      // Theme and motion both offer `system`; the reset is the one of the theme.
      await tester.tap(
        find.descendant(
          of: find.byKey(const ValueKey('settings.theme')),
          matching: find.byKey(const ValueKey('choice.system')),
        ),
      );
      await tester.pumpAndSettle();
      await eventually(
        'the reset reaches the second device',
        () => second.container.read(personalSettingsProvider).theme == 'system',
      );
      expect(app.fake.server.requestsOf('deleteMySetting'), hasLength(1));
      expect(app.fake.server.violations, isEmpty);
    } finally {
      await second.stop();
      await app.stop();
    }
  });
}
