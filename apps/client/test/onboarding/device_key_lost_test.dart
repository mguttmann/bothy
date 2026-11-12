// SPDX-License-Identifier: Apache-2.0

// A device that lost a key of its registration (08 5.4, R-ID-11; docs/decisions.d/
// AUD-pkg-device-key-01.md): the app never creates a key in the place of a registered one. It
// compares what the key storage holds with the keys the session was registered with, ends the
// account on the device with its own reason and registers as a new device at the next login.
//
// The path is the app's own from the plugin channel up: the real `DeviceKeyClient` with its
// generated channel code and codec, `PlatformDeviceKeyStore`, `InstanceServices.open`, the
// onboarding controller, the auth session and the engine, against the fake server over HTTP,
// which binds every session to the thumbprint of its DPoP key as the server does (08 7.4).
// Only the key storage of the platform is in memory (`package:device_key/testing.dart`); each
// group loses a key the way that platform does.

import 'dart:convert';
import 'dart:io';

import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart'
    show StepUpCredentials, StepUpPrompt, stepUpPurposeExtra;
import 'package:client/app/account_wiper.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:client/platform/device_key_signer.dart';
import 'package:device_key/device_key.dart' as dk;
import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:sync_engine/testing.dart';

import '../support/device_key_host.dart';
import '../support/fake_instance.dart';

/// The step-up prompt of the app: the person agrees.
class _Agreeing implements StepUpPrompt {
  @override
  Future<bool> ask(String purposeText) async => true;

  @override
  Future<StepUpCredentials?> askPassword(
    String purposeText, {
    bool rejected = false,
  }) async => (password: WorldSeed.password, totpCode: WorldSeed.totp);
}

void main() {
  // flutter_test answers every HttpClient request with 400; these tests talk to the fake.
  HttpOverrides? saved;
  late FakeInstance f;
  late ProviderContainer c;
  late PluginKeys plugin;

  Future<void> startInstance({
    bool stepUpKey = true,
    String platform = 'macos',
  }) async {
    plugin = PluginKeys(stepUpKey: stepUpKey);
    f = await FakeInstance.start(
      platform: platform,
      deviceKeys: plugin.store,
      stepUpPrompt: _Agreeing(),
    );
    c = appContainer(f);
  }

  setUp(() {
    saved = HttpOverrides.current;
    HttpOverrides.global = null;
  });

  tearDown(() async {
    final open = c.read(onboardingControllerProvider).services;
    c.dispose();
    await open?.close();
    await f.stop();
    HttpOverrides.global = saved;
  });

  OnboardingState state() => c.read(onboardingControllerProvider);
  String scope() => f.server.world.instanceId;
  String recordKey() =>
      'instance/${f.server.world.instanceId}/${RegisteredDeviceKeys.keyName}';
  bool sessionStored() =>
      f.secure.values.keys.any((k) => k.endsWith('/refresh_token'));

  /// Signs Alex in and waits until the engine is live. Returns the ID of the device.
  Future<String> signIn() async {
    final control = await connected(c, f);
    expect(state().step, OnboardingStep.login, reason: 'E2 after "Connect"');
    await control.loginWithPassword(
      WorldSeed.alexLogin,
      WorldSeed.password,
      WorldSeed.totp,
    );
    expect(state().step, OnboardingStep.signedIn);
    await eventually(
      'live',
      () => state().services?.engine.state.phase == ConnectionPhase.live,
    );
    return state().services!.auth.session!.deviceId.value;
  }

  /// Quits the app; its session, its keys and its local data stay.
  Future<void> quit() async {
    final closing = state().services!.close();
    c.dispose();
    await closing;
  }

  /// Starts the app again and waits for its first screen.
  Future<void> startAgain() async {
    plugin.host.calls.clear();
    c = appContainer(f);
    c.read(onboardingControllerProvider);
    await eventually(
      'the first screen',
      () => !state().starting && !state().busy,
      timeout: const Duration(seconds: 10),
    );
  }

  /// E7 shows the lost key, nothing of the account is left on the device and no key was
  /// created in the place of the lost one.
  void expectEndedForLostKey() {
    expect(state().step, OnboardingStep.signedOut);
    expect(state().cause, SignOutCause.deviceKeyLost);
    expect(state().services, isNull);
    expect(state().wipeIncomplete, isFalse);
    expect(sessionStored(), isFalse);
    expect(
      f.secure.values.keys.where((k) => k.contains(scope())),
      isEmpty,
      reason: 'every secret of the instance is deleted',
    );
    expect(f.instanceFolder.existsSync(), isFalse);
    expect(plugin.host.has(scope(), dk.KeyPurpose.dpop), isFalse);
    expect(plugin.host.has(scope(), dk.KeyPurpose.stepUp), isFalse);
    expect(
      plugin.host.calls.where((call) => call.startsWith('ensureKey')),
      isEmpty,
      reason: 'no key is created while a session is stored',
    );
  }

  /// "Log in again" registers this device anew: another device ID with other keys, while the
  /// old registration stays for the person to revoke (08 5.4).
  Future<void> expectRegistersAnew(String oldDevice) async {
    final control = c.read(onboardingControllerProvider.notifier);
    await control.loginAgain();
    expect(state().step, OnboardingStep.login);
    await control.loginWithPassword(
      WorldSeed.alexLogin,
      WorldSeed.password,
      WorldSeed.totp,
    );
    expect(state().step, OnboardingStep.signedIn);
    final devices = f.server.world.devices;
    final newDevice = state().services!.auth.session!.deviceId.value;
    expect(newDevice, isNot(oldDevice));
    expect(devices[newDevice]!.jkt, isNot(devices[oldDevice]!.jkt));
    expect(devices[oldDevice]!.state, isNot('revoked'));
    await eventually(
      'live with the new keys',
      () => state().services?.engine.state.phase == ConnectionPhase.live,
    );
    expect(f.server.violations, isEmpty);
  }

  group('iOS and macOS: the system deleted a key', () {
    test(
      'a step-up key that is gone (device passcode turned off and on) ends '
      'the account with its reason and the next login is a new device',
      () async {
        await startInstance();
        final old = await signIn();
        await quit();
        // The keychain drops a `WhenPasscodeSetThisDeviceOnly` item with the passcode.
        plugin.host.remove(scope(), dk.KeyPurpose.stepUp);

        await startAgain();

        expectEndedForLostKey();
        await expectRegistersAnew(old);
      },
    );

    test('another DPoP key under the same name is not taken for the registered '
        'one', () async {
      await startInstance();
      final old = await signIn();
      await quit();
      // A second writer put a new key there (an app extension, or a build that created a
      // missing key without asking): the server answers its proofs with invalid_dpop_proof
      // for good, which the session treats as temporary (08 7.3).
      plugin.host.replace(scope(), dk.KeyPurpose.dpop);

      await startAgain();

      expectEndedForLostKey();
      await expectRegistersAnew(old);
    });

    test('a DPoP key that is gone is not created again', () async {
      await startInstance();
      await signIn();
      await quit();
      plugin.host.remove(scope(), dk.KeyPurpose.dpop);

      await startAgain();

      expectEndedForLostKey();
    });

    test('a keychain that cannot answer right now (a locked device) ends '
        'nothing', () async {
      await startInstance();
      await signIn();
      await quit();
      plugin.host.failWith(
        scope(),
        dk.KeyPurpose.stepUp,
        dk.DeviceKeyError.notAvailable,
      );

      await startAgain();

      expect(state().step, OnboardingStep.signedIn);
      expect(sessionStored(), isTrue);
      await quit();

      // The DPoP key unreadable: E1 with the keychain text, and still nothing is deleted.
      plugin.host.failWith(
        scope(),
        dk.KeyPurpose.dpop,
        dk.DeviceKeyError.notAvailable,
      );
      await startAgain();

      expect(state().step, OnboardingStep.server);
      expect(state().serverStatus, ServerStatus.keychainUnavailable);
      expect(sessionStored(), isTrue);
      expect(f.instanceFolder.existsSync(), isTrue);
      expect(plugin.host.has(scope(), dk.KeyPurpose.dpop), isTrue);
      expect(plugin.host.has(scope(), dk.KeyPurpose.stepUp), isTrue);

      // The keychain answers again: the session is as it was.
      plugin.host
        ..failWith(scope(), dk.KeyPurpose.dpop, null)
        ..failWith(scope(), dk.KeyPurpose.stepUp, null);
      c.dispose();
      await startAgain();
      expect(state().step, OnboardingStep.signedIn);
      expect(state().restored, isTrue);
    });
  });

  group('a wipe step that fails after a lost key', () {
    test('E7 says that not everything was deleted, and "Log in again" '
        'deletes the rest before the new registration', () async {
      await startInstance();
      final old = await signIn();
      await quit();
      plugin.host.remove(scope(), dk.KeyPurpose.stepUp);
      // The key storage refuses the delete, as one that is locked or busy does.
      plugin.host.deleteError = dk.DeviceKeyError.internal;

      await startAgain();

      expect(state().step, OnboardingStep.signedOut);
      expect(state().cause, SignOutCause.deviceKeyLost);
      expect(state().wipeIncomplete, isTrue);
      // Every other step ran: the session and the folder are gone, the DPoP key is left.
      expect(sessionStored(), isFalse);
      expect(f.instanceFolder.existsSync(), isFalse);
      expect(plugin.host.has(scope(), dk.KeyPurpose.dpop), isTrue);
      expect(f.secure.values['app/unfinished_wipe'], contains(scope()));

      plugin.host.deleteError = null;
      await expectRegistersAnew(old);
      expect(f.secure.values['app/unfinished_wipe'], isNull);
    });
  });

  group('Android: new biometrics invalidate the step-up key', () {
    test('the signature of a confirmation answers key_invalidated and the '
        'account ends with the reason of the lost key', () async {
      await startInstance(platform: 'android');
      final old = await signIn();
      final auth = state().services!.auth;
      final world = f.server.world;
      final other = world.nextId();
      world.devices[other] = FakeDevice(
        id: other,
        userId: f.seed.alex.id,
        name: 'Old phone',
        platform: 'android',
        state: 'active',
        jkt: 'jkt-$other',
        createdAt: world.stamp(),
      );
      // The Keystore keeps the key but refuses every signature from now on.
      plugin.host.invalidate(scope(), dk.KeyPurpose.stepUp);
      plugin.host.calls.clear();

      await expectLater(
        auth.api.getDevicesApi().revokeDevice(
          deviceId: other,
          reasonBody: ReasonBody((b) => b..reason = 'lost'),
          extra: {
            idempotencyKeyExtra: '0193f2a7-0000-7000-8000-0000000000b1',
            stepUpPurposeExtra: 'Revoke the device Old phone',
          },
        ),
        throwsA(isA<DioException>()),
      );

      await eventually('E7', () => state().step == OnboardingStep.signedOut);
      expectEndedForLostKey();
      expect(world.devices[other]!.state, 'active');
      await expectRegistersAnew(old);
    });
  });

  group('Windows: a marker whose key is gone', () {
    test('with a session the account ends with the reason of the lost key, '
        'not on E1 with the keychain text', () async {
      await startInstance(stepUpKey: false, platform: 'windows');
      final old = await signIn();
      await quit();
      // After a TPM clear the plugin answers key_invalidated and never creates the key again.
      plugin.host.loseKeepingMarker(scope(), dk.KeyPurpose.dpop);

      await startAgain();

      expectEndedForLostKey();
      await expectRegistersAnew(old);
    });

    test('without a session the leftover is deleted and E2 shows', () async {
      await startInstance(stepUpKey: false, platform: 'windows');
      plugin.host.loseKeepingMarker(scope(), dk.KeyPurpose.dpop);

      final control = await connected(c, f);

      expect(state().serverStatus, isNot(ServerStatus.keychainUnavailable));
      expect(state().step, OnboardingStep.login);
      expect(plugin.host.has(scope(), dk.KeyPurpose.dpop), isTrue);
      await control.loginWithPassword(
        WorldSeed.alexLogin,
        WorldSeed.password,
        WorldSeed.totp,
      );
      expect(state().step, OnboardingStep.signedIn);
      expect(f.server.violations, isEmpty);
    });

    test(
      'a device registered without a step-up key starts as before',
      () async {
        await startInstance(stepUpKey: false, platform: 'windows');
        final device = await signIn();
        expect(f.server.world.devices[device]!.stepUpJwk, isNull);
        await quit();

        await startAgain();

        expect(state().step, OnboardingStep.signedIn);
        expect(state().restored, isTrue);
        expect(
          plugin.host.calls.where((call) => call.startsWith('ensureKey')),
          isEmpty,
        );
      },
    );
  });

  group('the record of the registered keys', () {
    test('holds the thumbprints the server registered', () async {
      await startInstance();
      final device = await signIn();
      final record =
          jsonDecode(f.secure.values[recordKey()]!) as Map<String, Object?>;
      expect(record['dpop'], f.server.world.devices[device]!.jkt);
      final stepUp = await plugin.host.client.keyInfo(
        scope(),
        dk.KeyPurpose.stepUp,
      );
      expect(record['step_up'], stepUp!.thumbprint);
    });

    test('a session of a build before the record keeps its keys and gets the '
        'record at its next start', () async {
      await startInstance();
      await signIn();
      await quit();
      final written = f.secure.values.remove(recordKey());
      expect(written, isNotNull);

      await startAgain();

      expect(state().step, OnboardingStep.signedIn);
      expect(state().restored, isTrue);
      expect(f.secure.values[recordKey()], written);
      expect(
        plugin.host.calls.where((call) => call.startsWith('ensureKey')),
        isEmpty,
      );
    });

    test('an unreadable record counts as none: the keys the storage holds '
        'stay and are recorded again', () async {
      await startInstance();
      await signIn();
      await quit();
      final written = f.secure.values[recordKey()];
      f.secure.values[recordKey()] = 'not json';

      await startAgain();

      expect(state().step, OnboardingStep.signedIn);
      expect(f.secure.values[recordKey()], written);
      await quit();

      f.secure.values[recordKey()] = '["a list"]';
      await startAgain();
      expect(state().step, OnboardingStep.signedIn);
      expect(f.secure.values[recordKey()], written);
    });

    test('a session of a build before the record without its DPoP key ends '
        'as well', () async {
      await startInstance();
      await signIn();
      await quit();
      f.secure.values.remove(recordKey());
      plugin.host.remove(scope(), dk.KeyPurpose.dpop);

      await startAgain();

      expectEndedForLostKey();
    });
  });
}
