// SPDX-License-Identifier: Apache-2.0

// A failure to open the services of a found instance (docs/decisions.d/FIX-macos-keychain.md):
// E1 shows its own text, never "No instance at this address", because discovery did answer.
// A platform without a step-up key (Windows) opens and logs in with its DPoP key alone
// (docs/decisions.d/FIX-windows-keystore.md) and confirms step-up actions with password and
// TOTP (docs/decisions.d/FIX-stepup-macos.md).

import 'dart:convert';
import 'dart:io';

import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart'
    show DeviceKeys, StepUpCredentials, StepUpPrompt, stepUpPurposeExtra;
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:client/onboarding/start_route.dart';
import 'package:client/platform/device_key_signer.dart';
import 'package:device_key/device_key.dart' as dk;
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/testing.dart';

import '../support/fake_instance.dart';
import '../support/software_keys.dart';

/// Device keys that the platform refuses, as the keychain of an app without entitlements does.
class _RefusedKeys implements DeviceKeyStore {
  @override
  Future<DeviceKeys> ensure(String scope) async =>
      throw const dk.DeviceKeyException(dk.DeviceKeyError.notAvailable);

  @override
  Future<void> delete(String scope) async {}
}

/// The `device_key` plugin as on Windows (D-M4-11-2): the DPoP key exists in software here,
/// `ensure` of the step-up key fails with `not_available`.
class _WindowsLikeClient implements dk.DeviceKeyClient {
  final Map<String, SoftwareP256Signer> _dpop = {};

  SoftwareP256Signer _key(String scope) =>
      _dpop[scope] ??= SoftwareP256Signer.seeded('$scope/windows/dpop');

  static Uint8List _bytes(String b64) =>
      base64Url.decode(base64Url.normalize(b64));

  @override
  Future<dk.DeviceKeyInfo> ensure(String scope, dk.KeyPurpose purpose) async {
    if (purpose == dk.KeyPurpose.stepUp) {
      throw const dk.DeviceKeyException(dk.DeviceKeyError.notAvailable);
    }
    final jwk = _key(scope).publicJwk;
    return dk.DeviceKeyInfo(
      x: _bytes(jwk['x']!),
      y: _bytes(jwk['y']!),
      hardwareBound: true,
      requiresUserPresence: false,
    );
  }

  @override
  Future<Uint8List> sign(
    String scope,
    dk.KeyPurpose purpose,
    Uint8List data, {
    dk.StepUpPrompt? prompt,
  }) => _key(scope).sign(data);

  @override
  Future<void> delete(String scope) async => _dpop.remove(scope);

  @override
  Future<dk.DeviceKeyInfo?> keyInfo(String scope, dk.KeyPurpose purpose) =>
      throw UnimplementedError();

  @override
  Future<Uint8List?> attestation(
    String scope,
    dk.KeyPurpose purpose,
    Uint8List challenge,
  ) => throw UnimplementedError();
}

/// The step-up prompt of the app: confirms with the device key and types the seed's password
/// and code into the password variant.
class _Prompt implements StepUpPrompt {
  final List<String> asked = [];
  final List<String> askedPassword = [];

  @override
  Future<bool> ask(String purposeText) async {
    asked.add(purposeText);
    return true;
  }

  @override
  Future<StepUpCredentials?> askPassword(
    String purposeText, {
    bool rejected = false,
  }) async {
    askedPassword.add(purposeText);
    return (password: WorldSeed.password, totpCode: WorldSeed.totp);
  }
}

void main() {
  test('each cause of a failed open maps to its status', () {
    final cases = <Object, ServerStatus>{
      const dk.DeviceKeyException(dk.DeviceKeyError.notAvailable):
          ServerStatus.keychainUnavailable,
      const dk.DeviceKeyException(dk.DeviceKeyError.internal):
          ServerStatus.keychainUnavailable,
      PlatformException(
        code: 'Unexpected security result code',
        details: -25308,
      ): ServerStatus.keychainUnavailable,
      PlatformException(code: 'protect_failed'): ServerStatus.openFailed,
      const FileSystemException('denied', '/x'): ServerStatus.openFailed,
      StateError('database'): ServerStatus.openFailed,
    };
    for (final MapEntry(key: error, value: status) in cases.entries) {
      expect(openFailureStatus(error), status, reason: '$error');
      expect(openFailureStatus(error), isNot(ServerStatus.noInstance));
    }
  });

  group('the controller', () {
    HttpOverrides? saved;
    late FakeInstance f;
    late ProviderContainer c;

    setUp(() async {
      saved = HttpOverrides.current;
      HttpOverrides.global = null;
      f = await FakeInstance.start(deviceKeys: _RefusedKeys());
      c = appContainer(f);
    });

    tearDown(() async {
      c.dispose();
      await f.stop();
      HttpOverrides.global = saved;
    });

    test('shows the keychain text when the device keys are refused', () async {
      await connected(c, f);
      final state = c.read(onboardingControllerProvider);
      expect(state.step, OnboardingStep.server);
      expect(state.serverStatus, ServerStatus.keychainUnavailable);
      expect(state.services, isNull);
    });
  });

  group('a platform without a step-up key (Windows)', () {
    HttpOverrides? saved;
    late FakeInstance f;
    late ProviderContainer c;
    late _Prompt prompt;

    setUp(() async {
      saved = HttpOverrides.current;
      HttpOverrides.global = null;
      prompt = _Prompt();
      f = await FakeInstance.start(
        platform: 'windows',
        stepUpPrompt: prompt,
        deviceKeys: PlatformDeviceKeyStore(
          _WindowsLikeClient(),
          () =>
              const dk.StepUpPrompt(title: 'Confirm', negativeButton: 'Cancel'),
        ),
      );
      c = appContainer(f);
    });

    tearDown(() async {
      c.dispose();
      await f.stop();
      HttpOverrides.global = saved;
    });

    test(
      'connects to E2 and registers the device without a step-up key',
      () async {
        final control = await connected(c, f);
        var state = c.read(onboardingControllerProvider);
        expect(state.serverStatus, isNot(ServerStatus.keychainUnavailable));
        expect(state.step, OnboardingStep.login);
        await control.loginWithPassword(
          WorldSeed.alexLogin,
          WorldSeed.password,
          WorldSeed.totp,
        );
        state = c.read(onboardingControllerProvider);
        expect(state.step, OnboardingStep.signedIn);
        final id = state.services!.auth.session!.deviceId.value;
        final device = f.server.world.devices[id]!;
        expect(device.stepUpJwk, isNull);
        expect(f.server.violations, isEmpty);
      },
    );

    test(
      'a step-up action asks for password and code and then succeeds',
      () async {
        final control = await connected(c, f);
        await control.loginWithPassword(
          WorldSeed.alexLogin,
          WorldSeed.password,
          WorldSeed.totp,
        );
        final auth = c.read(onboardingControllerProvider).services!.auth;
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
        f.server.requests.clear();

        final revoked = await auth.api.getDevicesApi().revokeDevice(
          deviceId: other,
          reasonBody: ReasonBody((b) => b..reason = 'lost'),
          extra: {
            idempotencyKeyExtra: '0193f2a7-0000-7000-8000-0000000000a1',
            stepUpPurposeExtra: 'Revoke the device Old phone',
          },
        );

        expect(revoked.data!.state, DeviceStateEnum.revoked);
        expect(prompt.askedPassword, ['Revoke the device Old phone']);
        expect(prompt.asked, isEmpty);
        const stepUpFlow = {
          'revokeDevice',
          'createStepUpChallenge',
          'completeStepUpWithDeviceKey',
          'stepUpWithPassword',
        };
        expect(
          [
            for (final r in f.server.requests)
              if (stepUpFlow.contains(r.operationId)) r.operationId,
          ],
          [
            'revokeDevice',
            'createStepUpChallenge',
            'stepUpWithPassword',
            'revokeDevice',
          ],
        );
        expect(f.server.violations, isEmpty);
      },
    );
  });
}
