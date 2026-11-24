// SPDX-License-Identifier: Apache-2.0

// "Cancel" while an OIDC login waits for the system browser (AUD-PKG-P02-client-b): a closed
// browser tab reports nothing on the desktop, so E2 offers to end the wait; the screen takes the
// next attempt at once and the cancelled attempt changes nothing when it ends.

import 'dart:async';
import 'dart:io';

import 'package:auth/auth.dart' show RedirectReceiver;
import 'package:client/app/discovery.dart';
import 'package:client/onboarding/e2_login.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/fake_instance.dart';
import '../support/widgets.dart';

/// A receiver whose redirect address comes only when the test says so: the login waits.
class _HeldRedirects implements RedirectReceiver {
  final Completer<Uri> address = Completer<Uri>();

  @override
  Future<Uri> redirectUri() => address.future;

  @override
  Future<Uri> waitForRedirect(Uri expectedPrefix, Duration timeout) =>
      Completer<Uri>().future.timeout(timeout);
}

void main() {
  const provider = LoginProvider(id: 'sso', kind: 'oidc', name: 'Company SSO');

  test('"Cancel" ends the wait for the browser; the cancelled attempt '
      'changes nothing', () async {
    final saved = HttpOverrides.current;
    HttpOverrides.global = null;
    final redirects = _HeldRedirects();
    final f = await FakeInstance.start(redirects: () => redirects);
    final c = appContainer(f);
    try {
      final control = await connected(c, f);
      final login = control.loginWithProvider(provider);
      var state = c.read(onboardingControllerProvider);
      expect(state.busy, isTrue);
      expect(state.waitingForBrowser, isTrue);

      control.cancelBrowserLogin();
      state = c.read(onboardingControllerProvider);
      expect(state.step, OnboardingStep.login);
      expect(state.busy, isFalse);
      expect(state.waitingForBrowser, isFalse);

      // The attempt ends later (here it fails at the instance): the screen stays as it is.
      redirects.address.complete(Uri.parse('http://127.0.0.1:1/callback'));
      await login;
      state = c.read(onboardingControllerProvider);
      expect(state.step, OnboardingStep.login);
      expect(state.busy, isFalse);
      expect(state.notice, isNull);
    } finally {
      c.dispose();
      await f.stop();
      HttpOverrides.global = saved;
    }
  });

  test('a password login offers no "Cancel"', () async {
    final saved = HttpOverrides.current;
    HttpOverrides.global = null;
    final f = await FakeInstance.start();
    final c = appContainer(f);
    try {
      final control = await connected(c, f);
      final login = control.loginWithPassword('alex', 'pw', '123456');
      expect(c.read(onboardingControllerProvider).waitingForBrowser, isFalse);
      control.cancelBrowserLogin();
      expect(c.read(onboardingControllerProvider).busy, isTrue);
      await login;
    } finally {
      c.dispose();
      await f.stop();
      HttpOverrides.global = saved;
    }
  });

  testWidgets('E2 shows "Cancel" only while the browser is awaited', (
    tester,
  ) async {
    desktopWindow(tester);
    final instance = InstanceInfo(
      instanceId: '0193f2a7-0000-7000-8000-000000000001',
      address: Uri.parse('https://chat.example.com'),
      productName: 'ExampleChat',
      providers: const [provider],
      localEnabled: false,
      passwordEnabled: false,
    );
    var cancelled = 0;
    Widget screen({required bool waiting}) => hostScreen(
      LoginScreen(
        instance: instance,
        passkeysAvailable: false,
        busy: waiting,
        waitingForBrowser: waiting,
        onCancelBrowser: () => cancelled++,
        onProvider: (_) {},
        onPasskey: (_) {},
        onPassword: (_, _, _) {},
        onEnroll: () {},
        onPair: () {},
        onChangeServer: () {},
      ),
    );
    await tester.pumpWidget(screen(waiting: false));
    expect(find.byKey(const ValueKey('e2.cancelBrowser')), findsNothing);
    await tester.pumpWidget(screen(waiting: true));
    final cancel = find.byKey(const ValueKey('e2.cancelBrowser'));
    expect(
      find.descendant(of: cancel, matching: find.text(en.onboardingCancel)),
      findsOneWidget,
    );
    await tester.tap(cancel);
    expect(cancelled, 1);
  });
}
