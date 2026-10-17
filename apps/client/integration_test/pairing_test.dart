// SPDX-License-Identifier: Apache-2.0

// E5 pairing with confirmation, with a wrong code on the other device, with expiry and "New
// code" (brief M4-18 Tests; 08 5.3).

import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';

import 'support/test_app.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets(
    'pairing: a wrong code, expiry, a new code and the confirmation',
    (tester) async {
      final app = await TestApp.start(tester);
      final other = await app.otherDevice();
      try {
        await app.connect();
        await tester.tap(find.byKey(const ValueKey('e2.pair')));
        await app.waitFor(find.byKey(const ValueKey('code.groups')));
        final first = app.state.code!;
        await expectLater(other.approvePairing('ZZZZZZZZ'), throwsA(anything));
        expect(find.byKey(const ValueKey('screen.e5')), findsOneWidget);
        app.fake.server.world.expirePairing(
          app.fake.server.world.pairings.keys.single,
        );
        await app.waitFor(find.byKey(const ValueKey('e5.expired')));
        await tester.tap(find.byKey(const ValueKey('e5.newCode')));
        await app.waitFor(find.byKey(const ValueKey('code.groups')));
        final second = app.state.code!;
        expect(second, isNot(first));
        await other.approvePairing(second);
        await app.waitFor(find.byKey(const ValueKey('column.sidebar')));
        await app.waitLive();
      } finally {
        await other.dispose();
        await app.stop();
      }
    },
  );
}
