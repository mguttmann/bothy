// SPDX-License-Identifier: Apache-2.0

// E2 Login (01 5.6, fixed requirement 13): the buttons equal the methods of the discovery
// fixture and nothing more; "No login is set up" without methods; Enter triggers the main
// action and Esc goes back.

import 'package:client/app/discovery.dart';
import 'package:client/onboarding/e2_login.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/instances.dart';
import '../support/widgets.dart';

class Calls {
  final List<String> log = [];
}

Future<Calls> pumpE2(
  WidgetTester tester,
  InstanceInfo instance, {
  bool passkeys = true,
  LoginNotice? notice,
  bool busy = false,
}) async {
  desktopWindow(tester);
  final calls = Calls();
  await tester.pumpWidget(
    hostScreen(
      LoginScreen(
        instance: instance,
        passkeysAvailable: passkeys,
        notice: notice,
        busy: busy,
        onProvider: (p) => calls.log.add('provider ${p.name}'),
        onPasskey: (name) => calls.log.add('passkey $name'),
        onPassword: (n, p, c) => calls.log.add('password $n $p $c'),
        onEnroll: () => calls.log.add('enroll'),
        onPair: () => calls.log.add('pair'),
        onChangeServer: () => calls.log.add('change'),
      ),
    ),
  );
  return calls;
}

List<String> buttonTexts(WidgetTester tester) => [
  for (final b in tester.widgetList<ButtonStyleButton>(
    find.byWidgetPredicate((w) => w is ButtonStyleButton),
  ))
    if (b.child is Text) (b.child! as Text).data!,
];

void main() {
  testWidgets('OIDC only: one button per provider, the code and pairing links', (tester) async {
    await pumpE2(tester, oidcOnly);
    expect(buttonTexts(tester), [
      en.onboardingLoginWithProvider('Company SSO'),
      en.onboardingLoginWithProvider('Office proxy'),
      en.onboardingEnrollmentCodeLink,
      en.onboardingPairLink,
      en.onboardingChangeServer,
    ]);
    expect(find.byKey(const ValueKey('e2.loginName')), findsNothing);
    expect(find.text(en.onboardingLoginTitle('Team Chat')), findsOneWidget);
    expect(find.text(en.onboardingServerHost('chat.example.com')), findsOneWidget);
  });

  testWidgets('local with password: login name, passkey, password and code', (tester) async {
    final calls = await pumpE2(tester, localWithPassword);
    expect(buttonTexts(tester), [
      en.onboardingLoginWithPasskey,
      en.onboardingPasswordAndCode,
      en.onboardingEnrollmentCodeLink,
      en.onboardingPairLink,
      en.onboardingChangeServer,
    ]);
    await tester.enterText(find.byKey(const ValueKey('e2.loginName')), 'alex');
    await tester.tap(find.byKey(const ValueKey('e2.passkey')));
    await tester.tap(find.byKey(const ValueKey('e2.password.open')));
    await tester.pump();
    await tester.enterText(find.byKey(const ValueKey('e2.password')), 'pw');
    await tester.enterText(find.byKey(const ValueKey('e2.code')), '123456');
    await tester.tap(find.byKey(const ValueKey('e2.submit')));
    expect(calls.log, ['passkey alex', 'password alex pw 123456']);
  });

  testWidgets('without the passkey port the passkey button is not offered', (tester) async {
    await pumpE2(tester, localWithPassword, passkeys: false);
    expect(find.byKey(const ValueKey('e2.passkey')), findsNothing);
    final noPassword = InstanceInfo(
      address: fixtureAddress,
      instanceId: localWithPassword.instanceId,
      productName: 'Team Chat',
      localEnabled: true,
    );
    await pumpE2(tester, noPassword);
    expect(find.byKey(const ValueKey('e2.password.open')), findsNothing);
    expect(find.byKey(const ValueKey('e2.passkey')), findsOneWidget);
  });

  testWidgets('nothing: "No login is set up", enrollment and pairing stay', (tester) async {
    await pumpE2(tester, nothing);
    expect(find.text(en.onboardingNoMethod), findsOneWidget);
    expect(buttonTexts(tester), [
      en.onboardingEnrollmentCodeLink,
      en.onboardingPairLink,
      en.onboardingChangeServer,
    ]);
  });

  testWidgets('Enter triggers the main action and Esc goes back', (tester) async {
    var calls = await pumpE2(tester, oidcOnly);
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await tester.sendKeyEvent(LogicalKeyboardKey.escape);
    expect(calls.log, ['provider Company SSO', 'change']);
    calls = await pumpE2(tester, localWithPassword);
    await tester.tap(find.byKey(const ValueKey('e2.password.open')));
    await tester.pump();
    await tester.enterText(find.byKey(const ValueKey('e2.password')), 'pw');
    await tester.enterText(find.byKey(const ValueKey('e2.code')), '1');
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    expect(calls.log.last, startsWith('password'));
  });

  testWidgets('the links, the notice and the busy state', (tester) async {
    var calls = await pumpE2(tester, oidcOnly, notice: LoginNotice.unreachable);
    expect(find.text(en.onboardingUnreachable), findsOneWidget);
    await tester.tap(find.byKey(const ValueKey('e2.enroll')));
    await tester.tap(find.byKey(const ValueKey('e2.pair')));
    await tester.tap(find.byKey(const ValueKey('e2.change')));
    expect(calls.log, ['enroll', 'pair', 'change']);
    calls = await pumpE2(tester, oidcOnly, busy: true);
    await tester.tap(find.byKey(ValueKey('e2.provider.${oidcOnly.providers.first.id}')));
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    expect(calls.log, isEmpty);
    for (final n in LoginNotice.values) {
      await pumpE2(tester, oidcOnly, notice: n);
      expect(find.byKey(ValueKey('notice.${n.name}')), findsOneWidget);
    }
  });
}
