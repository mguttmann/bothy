// SPDX-License-Identifier: Apache-2.0

// A hint of the gateway on the phone that is in front (13 4.6 item 1; 15 10; docs/decisions.d/
// AUD-client-app-01.md): the phone is connected, so the server sends it `notification.created`
// and no push, and the app shows the hint as a banner over its content. A tap opens the thread
// at the message; the thread that is on screen gets no banner.

import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:integration_test/integration_test.dart';
import 'package:sync_engine/sync_engine.dart' show NotificationCreated;

import 'support/phone_flow.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('a hint shows as a banner while the app is in front; a tap '
      'opens the thread at the message', (tester) async {
    final app = await startPhone(tester);
    try {
      await signIn(app);
      final seed = app.fake.seed;
      final world = app.fake.server.world
        ..notifyOnMessages = true
        ..setNotificationPref(seed.alex.id, 'channel', seed.general, 'all')
        ..setNotificationPref(seed.alex.id, 'channel', seed.support, 'all');
      final banner = find.byKey(const ValueKey('notification.banner'));
      final shell = find.byKey(const ValueKey('phone.shell'));
      String route() =>
          GoRouter.of(tester.element(shell))
              .routerDelegate
              .currentConfiguration
              .uri
              .toString();
      expect(banner, findsNothing);

      final message = seed.post(seed.sam, seed.releaseNotes, 'Build is green');
      // The server delivered the hint over the gateway: no push goes to this phone for it.
      expect(world.notifications.single.recipient, seed.alex.id);
      await app.waitFor(banner);
      expect(
        find.descendant(
          of: banner,
          matching: find.text('general · Release notes'),
        ),
        findsOneWidget,
      );
      expect(
        find.descendant(of: banner, matching: find.text('Sam: Build is green')),
        findsOneWidget,
      );

      await tester.tap(find.byKey(const ValueKey('notification.banner.open')));
      final target = '/t/${seed.releaseNotes}?m=${message['id']}';
      await until(tester, 'the thread at the message', () => route() == target);
      await app.waitFor(
        find.byKey(
          ValueKey('historyColumn.${seed.releaseNotes}.${message['id']}'),
        ),
      );
      expect(banner, findsNothing, reason: 'the opened banner is gone');

      // The thread on screen gets no banner: the app receives the hint and shows nothing.
      final engine = app.state.services!.engine;
      await until(
        tester,
        'the thread reported as on screen',
        () => engine.focus.threads.any((t) => t.value == seed.releaseNotes),
      );
      final hints = <NotificationCreated>[];
      final received = engine.notifications.listen(hints.add);
      seed.post(seed.sam, seed.releaseNotes, 'Deploy is done');
      await until(
        tester,
        'the hint of the open thread',
        () => hints.length == 1,
      );
      await tester.pump(const Duration(milliseconds: 200));
      expect(banner, findsNothing);

      // A hint of another thread shows over the open one.
      seed.post(seed.sam, seed.printer, 'Paper jam');
      await app.waitFor(banner);
      expect(
        find.descendant(of: banner, matching: find.text('support · Printer')),
        findsOneWidget,
      );
      await received.cancel();
      expect(app.fake.server.violations, isEmpty);
    } finally {
      await app.stop();
    }
  });
}
