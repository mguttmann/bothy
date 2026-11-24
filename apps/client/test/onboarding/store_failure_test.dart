// SPDX-License-Identifier: Apache-2.0

// A secret store that refuses a read or write outside `InstanceServices.open` (AUD-PKG-P02-
// client-b): "Connect", the start and the login of a waiting device end on a screen that takes
// the next attempt, never on a "Connecting" that only a restart ends; two "Connect" never run
// at once.

import 'dart:async';
import 'dart:io';

import 'package:auth/auth.dart' show SessionSecretKeys;
import 'package:client/app/discovery.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:domain/domain.dart' show ClientVersionDecision;
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/testing.dart';

import '../support/fake_instance.dart';
import '../support/other_device.dart';

/// Counts the discoveries and holds them until [release].
class _HeldDiscovery implements DiscoveryClient {
  _HeldDiscovery(this._real);

  final DiscoveryClient _real;
  int asked = 0;
  final Completer<void> release = Completer<void>();

  @override
  Future<DiscoveryOutcome> discover(Uri address) async {
    asked++;
    await release.future;
    return _real.discover(address);
  }

  @override
  Future<ClientVersionDecision?> checkVersion(Uri address) =>
      _real.checkVersion(address);
}

void main() {
  HttpOverrides? saved;

  setUp(() {
    saved = HttpOverrides.current;
    HttpOverrides.global = null;
  });

  tearDown(() => HttpOverrides.global = saved);

  String instanceKey(FakeInstance f, String key) =>
      'instance/${f.server.world.instanceId}/$key';

  test('a refused read of the stored session ends "Connect" on E1 with the '
      'keychain text', () async {
    final f = await FakeInstance.start();
    final c = appContainer(f);
    try {
      f.secure.lockedKeys.add(
        instanceKey(f, SessionSecretKeys.refreshToken(f.server.baseUri)),
      );
      await connected(c, f);
      final state = c.read(onboardingControllerProvider);
      expect(state.step, OnboardingStep.server);
      expect(state.serverStatus, ServerStatus.keychainUnavailable);
      expect(state.busy, isFalse);
      expect(state.services, isNull);
    } finally {
      c.dispose();
      await f.stop();
    }
  });

  test('a refused read at the start shows E1 with the keychain text', () async {
    final f = await FakeInstance.start();
    f.secure.values['app/server_address'] = f.server.baseUri.toString();
    f.secure.lockedKeys.add('app/server_address');
    final c = appContainer(f);
    try {
      c.read(onboardingControllerProvider);
      await eventually(
        'E1',
        () => !c.read(onboardingControllerProvider).starting,
      );
      final state = c.read(onboardingControllerProvider);
      expect(state.step, OnboardingStep.server);
      expect(state.serverStatus, ServerStatus.keychainUnavailable);
      expect(state.busy, isFalse);
    } finally {
      c.dispose();
      await f.stop();
    }
  });

  test(
    'a second "Connect" while one runs waits for it and opens nothing more',
    () async {
      late _HeldDiscovery held;
      final f = await FakeInstance.start(
        discovery: (real) => held = _HeldDiscovery(real),
      );
      final c = appContainer(f);
      try {
        c.read(onboardingControllerProvider);
        await Future<void>.delayed(Duration.zero);
        final control = c.read(onboardingControllerProvider.notifier);
        final first = control.connect(f.server.baseUri.toString());
        final second = control.connect(f.server.baseUri.toString());
        held.release.complete();
        await Future.wait([first, second]);
        expect(held.asked, 1);
        expect(c.read(onboardingControllerProvider).step, OnboardingStep.login);
      } finally {
        c.dispose();
        await f.stop();
      }
    },
  );

  test('a refused write of a waiting device\'s code leaves E2 with the '
      'failure text', () async {
    final f = await FakeInstance.start();
    final other = OtherDevice(f.server);
    final c = appContainer(f);
    try {
      await other.signIn();
      f.seed.alex.newDevicesPending = true;
      final control = await connected(c, f);
      f.secure.refusedWrites.add(
        instanceKey(f, OnboardingController.pendingKey),
      );
      await control.loginWithPassword(
        WorldSeed.alexLogin,
        WorldSeed.password,
        WorldSeed.totp,
      );
      final state = c.read(onboardingControllerProvider);
      expect(state.step, OnboardingStep.login);
      expect(state.notice, LoginNotice.failed);
      expect(state.busy, isFalse);
    } finally {
      await other.dispose();
      c.dispose();
      await f.stop();
    }
  });
}
