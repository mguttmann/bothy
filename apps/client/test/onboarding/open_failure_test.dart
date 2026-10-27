// SPDX-License-Identifier: Apache-2.0

// A failure to open the services of a found instance (docs/decisions.d/FIX-macos-keychain.md):
// E1 shows its own text, never "No instance at this address", because discovery did answer.

import 'dart:io';

import 'package:auth/auth.dart' show DeviceKeys;
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:client/onboarding/start_route.dart';
import 'package:client/platform/device_key_signer.dart';
import 'package:device_key/device_key.dart' as dk;
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/fake_instance.dart';

/// Device keys that the platform refuses, as the keychain of an app without entitlements does.
class _RefusedKeys implements DeviceKeyStore {
  @override
  Future<DeviceKeys> ensure(String scope) async =>
      throw const dk.DeviceKeyException(dk.DeviceKeyError.notAvailable);

  @override
  Future<void> delete(String scope) async {}
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
}
