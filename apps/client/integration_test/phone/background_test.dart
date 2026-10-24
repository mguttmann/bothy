// SPDX-License-Identifier: Apache-2.0

// The phone in the background (brief M5-02 Tests, fixed requirements 14, 16, 17; 15 6.3, 7.2):
// `inactive` covers the content, `paused` closes the gateway with 1000 (state Suspended, the
// fake sees the code), `resumed` removes the cover and connects again with `resume`; the
// database files get the protection class NSFileProtectionComplete before the first open (on a
// device the attribute is read back; the simulator keeps no protection class).

import 'dart:io';

import 'package:client/platform/app_lifecycle.dart';
import 'package:client/platform/ios_platform.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';
import 'package:sync_engine/sync_engine.dart';

import 'support/phone_flow.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('background suspends with 1000, the foreground resumes', (
    tester,
  ) async {
    final lifecycle = FlutterAppLifecycle(initial: AppLifecycleState.resumed);
    final app = await startPhone(tester, lifecycle: lifecycle);
    try {
      await signIn(app);
      final gateway = app.fake.server.gateway;
      final before = gateway.connections.length;
      final engine = app.state.services!.engine;

      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.inactive);
      await tester.pump();
      expect(find.byKey(const ValueKey('privacy.cover')), findsOneWidget);
      expect(find.byKey(const ValueKey('phone.shell')), findsNothing);

      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.hidden);
      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.paused);
      await until(
        tester,
        'suspended',
        () => engine.state.phase == ConnectionPhase.suspended,
      );
      await until(
        tester,
        'closed with 1000 at the fake',
        () => gateway.connections.every((c) => !c.open),
      );
      expect(gateway.connections[before - 1].closeCode, 1000);

      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.hidden);
      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.inactive);
      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
      await app.waitFor(find.byKey(const ValueKey('phone.shell')));
      expect(find.byKey(const ValueKey('privacy.cover')), findsNothing);
      await app.waitLive();
      final reconnected = gateway.connections.last.number;
      expect(reconnected, greaterThan(before));
      final firstOp = gateway.frames
          .firstWhere((f) => f.connection == reconnected && !f.fromServer)
          .op;
      expect(firstOp, 'resume');

      // 15 6.3: the database file and its companions got the protection class before the
      // first open (the runner's Pigeon call; a no-op on the Flutter tester).
      final db =
          '${app.fake.instanceFolder.path}${Platform.pathSeparator}local.db';
      expect(protectedPaths, containsAll([db, '$db-wal', '$db-shm']));
      if (Platform.isIOS && !inSimulator) {
        expect(await IosPlatformApi().fileProtection(db), 'complete');
      }
      expect(app.fake.server.violations, isEmpty);
    } finally {
      await app.stop();
      await lifecycle.dispose();
    }
  });
}
