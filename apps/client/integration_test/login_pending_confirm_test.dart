// SPDX-License-Identifier: Apache-2.0

// Login with password and one-time code; the device is pending (E4 shows the code); the other
// device of Alex confirms it; the main view appears with the sidebar frame (brief M4-18 Tests).

import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';

import 'support/test_app.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('login, waiting device, confirmation, main view', (tester) async {
    final app = await TestApp.start(tester);
    final other = await app.otherDevice();
    try {
      app.fake.seed.alex.newDevicesPending = true;
      await app.connect();
      await app.loginWithPassword();
      await app.waitFor(find.byKey(const ValueKey('screen.e4')));
      final code = app.state.code!;
      expect(
        find.text('${code.substring(0, 3)}  ${code.substring(3)}'),
        findsOneWidget,
      );
      await other.confirm(app.deviceId, code);
      await app.waitFor(find.byKey(const ValueKey('column.sidebar')));
      await app.waitLive();
      expect(app.fake.server.violations, isEmpty);
    } finally {
      await other.dispose();
      await app.stop();
    }
  });
}
