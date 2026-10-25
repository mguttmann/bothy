// SPDX-License-Identifier: Apache-2.0

// Device keys of the E2E apps and actors. Software keys like the integration test entry (no test
// touches the keychain or the credential store of the machine that runs it, docs/decisions.d/
// M4-34.md), but random per store: every app and actor of a run is its own device of the same
// instance, and the server refuses a second device with a key it already knows.

import 'dart:math';

import 'package:auth/auth.dart' show DeviceKeys;
import 'package:client/platform/device_key_signer.dart';

import '../../test/support/software_keys.dart';

/// The hex text of 16 random bytes of the system's secure random source.
String randomNonce() {
  final random = Random.secure();
  return List.generate(
    16,
    (_) => random.nextInt(256).toRadixString(16).padLeft(2, '0'),
  ).join();
}

/// A [DeviceKeyStore] with software keys from a random seed: a new pair per scope and
/// generation, so the keys after a wipe differ from the keys before it.
final class E2eKeyStore implements DeviceKeyStore {
  final String _nonce = randomNonce();
  final Map<String, DeviceKeys> _keys = {};
  int _generation = 0;

  /// The scopes that currently hold keys.
  Set<String> get scopes => _keys.keys.toSet();

  @override
  Future<DeviceKeys> ensure(String scope) async =>
      _keys[scope] ??= keysFor('$_nonce/$scope/${_generation++}');

  @override
  Future<void> delete(String scope) async => _keys.remove(scope);
}

/// A fresh pair of software keys for [seed].
DeviceKeys keysFor(String seed) => DeviceKeys(
  dpop: SoftwareP256Signer.seeded('$seed/dpop'),
  stepUp: SoftwareP256Signer.seeded('$seed/step_up'),
);
