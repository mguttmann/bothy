// SPDX-License-Identifier: Apache-2.0

// The app lock on the phone (brief M5-08 Tests, fixed requirements 11 to 13): the person turns
// the lock on, the app goes to the background beyond `client.app_lock_after` (the fake reports
// its smallest value), and on the return E6 shows with no content; while the app is not in
// front the privacy cover is in the tree when the app switcher takes its picture; a failed
// authentication keeps the lock, a successful one opens it. The system authentication is the
// fake port of the lock tests: a person's biometric match is not automated here (the report
// of M5-08 names it as not proven).

import 'package:client/lock/app_lock_controller.dart';
import 'package:client/lock/app_lock_port.dart';
import 'package:client/lock/providers.dart';
import 'package:client/platform/app_lifecycle.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';

import '../../test/lock/support/lock_fakes.dart' show FakeAppLockPort;
import 'support/phone_flow.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets(
    'the return after the time shows E6; only the authentication opens it',
    (tester) async {
      final lifecycle = FlutterAppLifecycle(initial: AppLifecycleState.resumed);
      final port = FakeAppLockPort();
      final app = await startPhone(tester, lifecycle: lifecycle, appLock: port);
      try {
        app.fake.server.world.appLockAfterMs = 1;
        await signIn(app);
        final lock = app.container.read(appLockProvider)!;
        await until(tester, 'the policy', () => lock.policy?.afterMs == 1);
        expect(
          await lock.setEnabled(true, reason: 'on'),
          AppLockSwitchResult.changed,
        );

        tester.binding.handleAppLifecycleStateChanged(
          AppLifecycleState.inactive,
        );
        await tester.pump();
        // The app switcher takes its picture now: only the cover is painted.
        expect(find.byKey(const ValueKey('privacy.cover')), findsOneWidget);
        expect(find.byKey(const ValueKey('phone.shell')), findsNothing);
        tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.hidden);
        tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.paused);
        final away = DateTime.now();
        await untilInBackground(
          'longer than client.app_lock_after in the background',
          () =>
              DateTime.now().difference(away) > const Duration(milliseconds: 2),
        );
        tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.hidden);
        tester.binding.handleAppLifecycleStateChanged(
          AppLifecycleState.inactive,
        );
        tester.binding.handleAppLifecycleStateChanged(
          AppLifecycleState.resumed,
        );
        await app.waitFor(find.byKey(const ValueKey('screen.e6')));
        expect(
          find.byKey(const ValueKey('phone.shell'), skipOffstage: false),
          findsNothing,
        );

        port.results.add(AppLockAuthResult.failed);
        await tester.tap(find.byKey(const ValueKey('e6.unlock')));
        await app.waitFor(find.byKey(const ValueKey('e6.failed')));
        expect(lock.status, AppLockStatus.locked);

        // With the smallest time of the fake the lock closes again right after the unlock,
        // so the test follows the lock's events rather than the screen.
        final events = <AppLockEvent>[];
        final sub = lock.events.listen(events.add);
        try {
          await tester.tap(find.byKey(const ValueKey('e6.unlock')));
          await until(
            tester,
            'unlocked',
            () => events.contains(AppLockEvent.unlocked),
          );
        } finally {
          await sub.cancel();
        }
        expect(
          port.prompts,
          hasLength(3),
          reason: 'on, the failure, the success',
        );
        expect(app.fake.server.violations, isEmpty);
      } finally {
        await app.stop();
        await lifecycle.dispose();
      }
    },
  );
}
