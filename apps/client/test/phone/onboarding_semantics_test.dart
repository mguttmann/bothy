// SPDX-License-Identifier: Apache-2.0

// The onboarding rules of 01 5.6 in the phone form (brief M5-02 fixed requirement 13, Tests): the
// screens fill the screen, codes are read character by character, the remaining time is
// announced at most once per `a11y.live_announce_min_interval` with a fake clock, the TOTP field
// offers the system's one-time codes, E2 offers a passkey only where the build has one, and an
// invalid certificate offers no way on (R-PR-14).

import 'package:client/onboarding/e1_server_address.dart';
import 'package:client/onboarding/e2_login.dart';
import 'package:client/onboarding/e4_pending_device.dart';
import 'package:client/onboarding/e5_pairing.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/instances.dart';
import '../support/widgets.dart';
import 'support/phone_app.dart';
import 'support/phone_host.dart';

void main() {
  final interval = AppConfig.defaults().liveAnnounceMinInterval;

  testWidgets('on the phone the screens fill the screen without the card', (
    tester,
  ) async {
    phoneWindow(tester);
    await tester.pumpWidget(
      hostPhone(
        ServerAddressScreen(
          address: 'https://chat.example.com',
          status: ServerStatus.idle,
          onConnect: (_) {},
        ),
      ),
    );
    final screen = tester.getRect(
      find.byKey(const ValueKey('onboarding.phone')),
    );
    expect(screen.size, phoneSize);
    final field = tester.getRect(find.byKey(const ValueKey('e1.address')));
    // Full width but the padding of the screen.
    expect(field.width, greaterThan(phoneSize.width * 0.8));
  });

  testWidgets('codes are grouped and read character by character', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    phoneWindow(tester);
    final now = DateTime.utc(2026, 10, 17, 9);
    await tester.pumpWidget(
      hostPhone(
        PendingDeviceScreen(
          code: '481207',
          deadline: now.add(const Duration(minutes: 23)),
          clock: () => now,
          announceInterval: interval,
          announce: (_) {},
          onCancel: () {},
        ),
      ),
    );
    expect(
      find.bySemanticsLabel(en.onboardingCodeLabel('4, 8, 1, 2, 0, 7')),
      findsOneWidget,
    );
    await tester.pumpWidget(
      hostPhone(
        PairingScreen(
          code: 'HQ7P3MXR',
          deadline: now.add(const Duration(minutes: 4)),
          clock: () => now,
          announceInterval: interval,
          announce: (_) {},
          onNewCode: () {},
          onBack: () {},
        ),
      ),
    );
    expect(
      find.bySemanticsLabel(en.onboardingCodeLabel('H, Q, 7, P, 3, M, X, R')),
      findsOneWidget,
    );
    semantics.dispose();
  });

  testWidgets('the remaining time is announced at most once per interval', (
    tester,
  ) async {
    phoneWindow(tester);
    var now = DateTime.utc(2026, 10, 17, 9);
    final deadline = now.add(const Duration(minutes: 10, seconds: 1));
    final announced = <String>[];
    Future<void> pump(DateTime until) => tester.pumpWidget(
      hostPhone(
        PairingScreen(
          code: 'HQ7P3MXR',
          deadline: until,
          clock: () => now,
          announceInterval: interval,
          announce: announced.add,
          onNewCode: () {},
          onBack: () {},
        ),
      ),
    );
    await pump(deadline);
    expect(find.text(en.onboardingValidFor(10)), findsOneWidget);
    // A new code within the interval: the text changes at once, the announcement waits.
    now = now.add(interval ~/ 4);
    await pump(deadline.add(const Duration(minutes: 5)));
    final first = List.of(announced);
    now = now.add(interval ~/ 4);
    await pump(deadline.add(const Duration(minutes: 7)));
    expect(
      announced,
      first,
      reason: 'no second announcement within the interval',
    );
    now = now.add(interval);
    await tester.pump(interval);
    expect(announced.length, first.length + 1);
    final last = deadline.add(const Duration(minutes: 7));
    expect(
      announced.last,
      en.onboardingValidFor(last.difference(now).inMinutes),
    );
  });

  testWidgets('the TOTP field offers the one-time codes of the system', (
    tester,
  ) async {
    phoneWindow(tester);
    await tester.pumpWidget(
      hostPhone(
        LoginScreen(
          instance: localWithPassword,
          passkeysAvailable: false,
          onProvider: (_) {},
          onPasskey: (_) {},
          onPassword: (_, _, _) {},
          onEnroll: () {},
          onPair: () {},
          onChangeServer: () {},
        ),
      ),
    );
    // Without a passkey of the build (no associated domain) E2 hides the button (15 8).
    expect(find.byKey(const ValueKey('e2.passkey')), findsNothing);
    await tester.tap(find.byKey(const ValueKey('e2.password.open')));
    await tester.pump();
    final code = tester.widget<TextField>(
      find.byKey(const ValueKey('e2.code')),
    );
    expect(code.autofillHints, contains(AutofillHints.oneTimeCode));
    expect(code.keyboardType, TextInputType.number);
  });

  testWidgets('an invalid certificate offers no way on', (tester) async {
    phoneWindow(tester);
    await tester.pumpWidget(
      hostPhone(
        ServerAddressScreen(
          address: 'https://chat.example.com',
          status: ServerStatus.certificateInvalid,
          onConnect: (_) {},
        ),
      ),
    );
    expect(
      find.byKey(const ValueKey('e1.status.certificateInvalid')),
      findsOneWidget,
    );
    // The only button tries the address again; nothing continues past the certificate.
    expect(find.byType(FilledButton), findsOneWidget);
    expect(find.byType(TextButton), findsNothing);
    expect(find.byType(OutlinedButton), findsNothing);
  });
}
