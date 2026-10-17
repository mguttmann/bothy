// SPDX-License-Identifier: Apache-2.0

// A waiting device rejected (wrong code on the other device) or expired leads to E7 and a
// wiped store (brief M4-18 Tests; 08 5.2; R-SY-14).

import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';

import '../test/support/fakes.dart' show fastConfig;
import 'support/test_app.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('rejection: E7 and a wiped store', (tester) async {
    final app = await TestApp.start(tester);
    final other = await app.otherDevice();
    try {
      app.fake.seed.alex.newDevicesPending = true;
      await app.connect();
      await app.loginWithPassword();
      await app.waitFor(find.byKey(const ValueKey('screen.e4')));
      await other.confirm(app.deviceId, '000000');
      await app.waitFor(find.byKey(const ValueKey('screen.e7')));
      expect(find.byKey(const ValueKey('e7.deleted')), findsOneWidget);
      expect(await app.fake.wiped(), isTrue);
    } finally {
      await other.dispose();
      await app.stop();
    }
  });

  testWidgets('expiry: E7 and a wiped store', (tester) async {
    final app = await TestApp.start(
      tester,
      config: {...fastConfig, 'auth.device_pending_ttl': '1s'},
    );
    final other = await app.otherDevice();
    try {
      app.fake.seed.alex.newDevicesPending = true;
      await app.connect();
      await app.loginWithPassword();
      await app.waitFor(find.byKey(const ValueKey('screen.e4')));
      await tester.pump(const Duration(seconds: 2));
      await Future<void>.delayed(const Duration(seconds: 1));
      app.fake.server.world.expirePendingDevice(app.deviceId);
      await app.waitFor(find.byKey(const ValueKey('screen.e7')));
      expect(await app.fake.wiped(), isTrue);
    } finally {
      await other.dispose();
      await app.stop();
    }
  });
}
