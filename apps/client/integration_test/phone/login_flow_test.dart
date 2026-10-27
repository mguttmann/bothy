// SPDX-License-Identifier: Apache-2.0

// Login on the phone (brief M5-02 Tests; 01 5.6; 08 5.2; R-SY-14): E1 with the fake's discovery,
// E2 with password and TOTP, E4 waiting until the other device confirms, the phone shell; logout
// from the Me tab leads to E7 and deletes the local data.

import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';

import 'support/phone_flow.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('E1, E2, E4, the phone shell, then logout to E7', (tester) async {
    final app = await startPhone(tester);
    final other = await app.otherDevice();
    try {
      app.fake.seed.alex.newDevicesPending = true;
      await app.connect();
      // The phone form: the screen is filled, no card.
      expect(find.byKey(const ValueKey('onboarding.phone')), findsOneWidget);
      await app.loginWithPassword();
      await app.waitFor(find.byKey(const ValueKey('screen.e4')));
      await other.confirm(app.deviceId, app.state.code!);
      await app.waitFor(find.byKey(const ValueKey('phone.shell')));
      await app.waitLive();
      expect(find.byKey(const ValueKey('phone.tabBar')), findsOneWidget);
      expect(find.byKey(const ValueKey('column.sidebar')), findsNothing);

      await tester.tap(find.byKey(const ValueKey('phone.tab.me')));
      // The Me list is longer than the phone window (M5-12), and the Emergency entry of the
      // owner Alex comes with `getMe` after the page shows, pushing "Log out" down by one row:
      // scroll to the button only once the list has that entry, else it can end below the
      // window (two iOS failures: never found, or gone right after it was found).
      await app.waitFor(
        find.byKey(const ValueKey('phone.me.emergency'), skipOffstage: false),
      );
      final logout = find.byKey(
        const ValueKey('shell.logout'),
        skipOffstage: false,
      );
      await app.waitFor(logout);
      await tester.ensureVisible(logout);
      await tester.pump();
      await tester.tap(find.byKey(const ValueKey('shell.logout')));
      await app.waitFor(find.byKey(const ValueKey('screen.e7')));
      await eventually('local data deleted', app.fake.wiped);
      expect(app.fake.server.violations, isEmpty);
    } finally {
      await other.dispose();
      await app.stop();
    }
  });
}
