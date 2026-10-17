// SPDX-License-Identifier: Apache-2.0

// E4 and E5 (01 5.6; 08 5.2, 5.3): the remaining time as text (E4 from created_at plus
// auth.device_pending_ttl, E5 from expires_at), announced at most once per
// a11y.live_announce_min_interval; "Cancel" calls the wiper (through the controller, see
// controller_test.dart).

import 'package:client/onboarding/e4_pending_device.dart';
import 'package:client/onboarding/e5_pairing.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/widgets.dart';

void main() {
  final config = AppConfig.defaults();
  final interval = config.liveAnnounceMinInterval;

  testWidgets(
    'E4 shows the code and the time left until created_at plus the TTL',
    (tester) async {
      desktopWindow(tester);
      final createdAt = DateTime.utc(2026, 10, 17, 9);
      var now = createdAt.add(const Duration(minutes: 7));
      final announced = <String>[];
      var cancelled = 0;
      final deadline = createdAt.add(config.devicePendingTtl);
      await tester.pumpWidget(
        hostScreen(
          PendingDeviceScreen(
            code: '481207',
            deadline: deadline,
            clock: () => now,
            announceInterval: interval,
            announce: announced.add,
            onCancel: () => cancelled++,
          ),
        ),
      );
      final minutes = deadline.difference(now).inMinutes;
      expect(find.text(en.onboardingValidFor(minutes)), findsOneWidget);
      expect(find.text('481  207'), findsOneWidget);
      await tester.tap(find.byKey(const ValueKey('e4.cancel')));
      expect(cancelled, 1);

      // The text changes each minute; the announcements are at most one per interval.
      now = now.add(const Duration(minutes: 1));
      await tester.pump(const Duration(minutes: 1));
      expect(find.text(en.onboardingValidFor(minutes - 1)), findsOneWidget);
      expect(announced, [en.onboardingValidFor(minutes - 1)]);
      now = now.add(interval ~/ 2);
      await tester.pumpWidget(
        hostScreen(
          PendingDeviceScreen(
            code: '481207',
            deadline: deadline.subtract(const Duration(minutes: 1)),
            clock: () => now,
            announceInterval: interval,
            announce: announced.add,
            onCancel: () {},
          ),
        ),
      );
      expect(announced, hasLength(1));
    },
  );

  testWidgets(
    'a second change within the interval waits and is sent once when due',
    (tester) async {
      desktopWindow(tester);
      var now = DateTime.utc(2026, 10, 17, 9);
      final announced = <String>[];
      var deadline = now.add(const Duration(minutes: 10, seconds: 1));
      Future<void> pump() => tester.pumpWidget(
        hostScreen(
          PairingScreen(
            code: 'HQ7P3MXR',
            deadline: deadline,
            clock: () => now,
            announceInterval: interval,
            announce: announced.add,
            onNewCode: () {},
            onBack: () {},
          ),
        ),
      );
      await pump();
      expect(find.text(en.onboardingValidFor(10)), findsOneWidget);
      expect(find.text('HQ7P  3MXR'), findsOneWidget);
      deadline = deadline.subtract(const Duration(minutes: 1));
      await pump();
      deadline = deadline.subtract(const Duration(minutes: 1));
      await pump();
      expect(announced, [en.onboardingValidFor(9)]);
      now = now.add(interval);
      await tester.pump(interval);
      expect(announced, [en.onboardingValidFor(9), en.onboardingValidFor(8)]);
    },
  );

  testWidgets('E5: expired text, "New code", Enter and Esc', (tester) async {
    desktopWindow(tester);
    final calls = <String>[];
    Future<void> pump({
      bool expired = false,
      String? code,
      LoginNotice? notice,
    }) => tester.pumpWidget(
      hostScreen(
        PairingScreen(
          code: code,
          deadline: code == null ? null : DateTime.utc(2030),
          expired: expired,
          notice: notice,
          clock: () => DateTime.utc(2026),
          announceInterval: interval,
          onNewCode: () => calls.add('new'),
          onBack: () => calls.add('back'),
        ),
      ),
    );
    await pump(expired: true, code: 'HQ7P3MXR');
    expect(find.text(en.onboardingPairExpired), findsOneWidget);
    expect(find.text('HQ7P  3MXR'), findsNothing);
    await tester.tap(find.byKey(const ValueKey('e5.newCode')));
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await tester.sendKeyEvent(LogicalKeyboardKey.escape);
    await tester.tap(find.byKey(const ValueKey('e5.back')));
    expect(calls, ['new', 'new', 'back', 'back']);
    await pump(notice: LoginNotice.unreachable);
    expect(find.text(en.onboardingUnreachable), findsOneWidget);
  });
}
