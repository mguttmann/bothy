// SPDX-License-Identifier: Apache-2.0

// Logout deletes database, file cache, keys and tokens (R-SY-14); 4012 from the fake does the
// same (brief M4-18 Tests).

import 'dart:io';

import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';

import 'support/test_app.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  Future<TestApp> signedIn(WidgetTester tester) async {
    final app = await TestApp.start(tester);
    await app.connect();
    await app.loginWithPassword();
    await app.waitFor(find.byKey(const ValueKey('column.sidebar')));
    await app.waitLive();
    await File('${app.fake.instanceFolder.path}/files/cached.bin')
        .create(recursive: true);
    return app;
  }

  testWidgets('logout deletes every local datum and shows E7', (tester) async {
    final app = await signedIn(tester);
    try {
      await tester.tap(find.byKey(const ValueKey('shell.logout')));
      await app.waitFor(find.byKey(const ValueKey('screen.e7')));
      expect(await app.fake.wiped(), isTrue);
      expect(app.fake.server.requestsOf('logout'), hasLength(1));
      await tester.tap(find.byKey(const ValueKey('e7.loginAgain')));
      await app.waitFor(find.byKey(const ValueKey('screen.e2')));
    } finally {
      await app.stop();
    }
  });

  testWidgets('4012 deletes every local datum and shows E7', (tester) async {
    final app = await signedIn(tester);
    try {
      app.fake.server.faults.revoke(deviceId: app.deviceId);
      await app.waitFor(find.byKey(const ValueKey('screen.e7')));
      expect(await app.fake.wiped(), isTrue);
    } finally {
      await app.stop();
    }
  });
}
