// SPDX-License-Identifier: Apache-2.0

// The end of an account when something goes wrong (docs/decisions.d/
// AUD-client-onboarding-01.md; R-SY-14): a wipe step that fails is logged and remembered, E7
// says that not everything was deleted and the next start deletes the rest; a logout against
// an instance that never answers ends within its limit. The real controller, auth session,
// local database and engine run against the fake server.

import 'dart:convert';
import 'dart:io';

import 'package:auth/auth.dart' show DeviceKeys;
import 'package:client/app/account_wiper.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:client/platform/device_key_signer.dart';
import 'package:client/platform/secure_store.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:sync_engine/testing.dart';

import '../support/fake_instance.dart';
import '../support/silent_listener.dart';
import '../support/software_keys.dart';

/// Device keys the platform refuses to delete while [refuse] is set, as a key store does
/// that is locked or busy.
class _StubbornKeys implements DeviceKeyStore {
  final SoftwareKeyStore inner = SoftwareKeyStore();
  bool refuse = false;

  @override
  Future<DeviceKeys> ensure(String scope) => inner.ensure(scope);

  @override
  Future<void> delete(String scope) async {
    if (refuse) throw StateError('the key store refuses the delete');
    await inner.delete(scope);
  }
}

void main() {
  // flutter_test answers every HttpClient request with 400; these tests talk to the fake.
  HttpOverrides? saved;
  late FakeInstance f;
  late ProviderContainer c;
  SilentListener? silent;

  setUp(() {
    saved = HttpOverrides.current;
    HttpOverrides.global = null;
  });

  tearDown(() async {
    c.dispose();
    await silent?.close();
    silent = null;
    await f.stop();
    HttpOverrides.global = saved;
  });

  OnboardingState state() => c.read(onboardingControllerProvider);

  Future<OnboardingController> signedIn() async {
    c = appContainer(f);
    final control = await connected(c, f);
    await control.loginWithPassword(
      WorldSeed.alexLogin,
      WorldSeed.password,
      WorldSeed.totp,
    );
    await eventually(
      'live',
      () => state().services?.engine.state.phase == ConnectionPhase.live,
    );
    return control;
  }

  group('a wipe step that fails', () {
    late _StubbornKeys keys;

    setUp(() async {
      keys = _StubbornKeys();
      f = await FakeInstance.start(deviceKeys: keys);
    });

    bool secretsLeft() =>
        f.secure.values.keys.any((k) => k.contains(f.server.world.instanceId));

    Future<UnfinishedWipe?> remembered() async {
      final json = await AppPreferences(f.secure).unfinishedWipe();
      return json == null ? null : UnfinishedWipe.fromJson(jsonDecode(json));
    }

    test('E7 says that not everything was deleted and the next start deletes '
        'the rest', () async {
      final control = await signedIn();
      keys.refuse = true;
      await control.logout();
      expect(state().step, OnboardingStep.signedOut);
      expect(state().cause, SignOutCause.loggedOut);
      expect(state().wipeIncomplete, isTrue);
      // Every other step ran: the files and the secrets are gone, the keys are left.
      expect(f.instanceFolder.existsSync(), isFalse);
      expect(secretsLeft(), isFalse);
      expect(keys.inner.scopes, {f.server.world.instanceId});
      expect((await remembered())?.instanceId, f.server.world.instanceId);
      expect((await remembered())?.keyScope, f.server.world.instanceId);
      final left = (await keys.inner.ensure(f.server.world.instanceId))
          .dpop
          .publicJwk;
      final deletes = keys.inner.deletes;

      // The app quits and starts again; the key store lets go now.
      c.dispose();
      keys.refuse = false;
      c = appContainer(f);
      c.read(onboardingControllerProvider);
      await eventually(
        'the entry is gone',
        () async => await remembered() == null,
      );
      await eventually('E2', () => state().step == OnboardingStep.login);
      expect(state().wipeIncomplete, isFalse);
      // The rest is deleted. The same start opens the instance again and makes new device
      // keys for it, so the key store is empty only for a moment and no poll may wait for
      // that: what holds from here on is that the delete ran and that the keys of the scope
      // are not the ones the wipe left.
      expect(keys.inner.deletes, deletes + 1);
      final now = (await keys.inner.ensure(f.server.world.instanceId))
          .dpop
          .publicJwk;
      expect(now, isNot(left));
    });

    test(
      '"Log in again" deletes the rest before it opens the instance',
      () async {
        final control = await signedIn();
        final before = (await keys.inner.ensure(f.server.world.instanceId))
            .dpop
            .publicJwk;
        keys.refuse = true;
        await control.logout();
        expect(state().wipeIncomplete, isTrue);
        keys.refuse = false;
        await control.loginAgain();
        expect(state().step, OnboardingStep.login);
        expect(await remembered(), isNull);
        // New device keys, not the ones the wipe left.
        final after = (await keys.inner.ensure(f.server.world.instanceId))
            .dpop
            .publicJwk;
        expect(after, isNot(before));
      },
    );

    test('a new session keeps what a later start must not delete', () async {
      final control = await signedIn();
      keys.refuse = true;
      await control.logout();
      expect(await remembered(), isNotNull);
      // The rest stays undeletable; the person logs in again on top of it.
      await control.loginAgain();
      expect(await remembered(), isNotNull);
      await control.loginWithPassword(
        WorldSeed.alexLogin,
        WorldSeed.password,
        WorldSeed.totp,
      );
      expect(state().step, OnboardingStep.signedIn);
      await eventually(
        'the entry is gone',
        () async => await remembered() == null,
      );
      // The next start resumes this session instead of wiping it.
      final closing = state().services!.close();
      c.dispose();
      await closing;
      keys.refuse = false;
      c = appContainer(f);
      c.read(onboardingControllerProvider);
      await eventually(
        'the main view',
        () => state().step == OnboardingStep.signedIn,
      );
      expect(keys.inner.scopes, {f.server.world.instanceId});
    });

    test('a complete wipe remembers nothing', () async {
      final control = await signedIn();
      await control.logout();
      expect(state().wipeIncomplete, isFalse);
      expect(await remembered(), isNull);
      expect(keys.inner.scopes, isEmpty);
    });
  });

  test('logout ends on E7 with the local data deleted while the instance never '
      'answers', () async {
    f = await FakeInstance.start(
      serverLogoutLimit: const Duration(milliseconds: 300),
    );
    final control = await signedIn();
    silent = await SilentListener.replace(f.server);
    final watch = Stopwatch()..start();
    await control.logout().timeout(
      const Duration(seconds: 30),
      onTimeout: () => fail('the logout waits for the instance'),
    );
    expect(watch.elapsed, greaterThanOrEqualTo(f.ports.serverLogoutLimit));
    expect(state().step, OnboardingStep.signedOut);
    expect(state().cause, SignOutCause.loggedOut);
    expect(state().wipeIncomplete, isFalse);
    expect(await f.wiped(), isTrue);
  });
}
