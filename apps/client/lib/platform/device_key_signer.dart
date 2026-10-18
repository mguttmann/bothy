// SPDX-License-Identifier: Apache-2.0

/// The device keys of M4-10 and M4-11 behind the signer ports of `auth` (docs/spec/
/// 15-client-architecture.md 9, brief M4-18 Design note 1).
library;

import 'dart:typed_data';

import 'package:auth/auth.dart';
import 'package:device_key/device_key.dart' as dk;

/// The two device keys of one instance, created on demand and deleted on a wipe.
///
/// [PlatformDeviceKeyStore] uses the secure hardware of the platform; the tests and the
/// integration test entry use software keys.
abstract interface class DeviceKeyStore {
  /// Creates the `dpop` and `step_up` keys of [scope] if they do not exist and returns their
  /// signers. Throws a `DeviceKeyException` when the platform cannot create a key.
  Future<DeviceKeys> ensure(String scope);

  /// Deletes both keys of [scope]; missing keys are no error.
  Future<void> delete(String scope);
}

/// [DeviceKeyStore] over the `device_key` plugin.
///
/// The scope is the `instance_id` of discovery, so a key never serves two instances.
/// [stepUpPrompt] gives the texts of the system dialog of a step-up signature in the current
/// language.
///
/// Example: `final keys = await PlatformDeviceKeyStore(DeviceKeyClient(), prompt).ensure(id);`
final class PlatformDeviceKeyStore implements DeviceKeyStore {
  /// Creates the store over [client].
  PlatformDeviceKeyStore(this._client, this._stepUpPrompt);

  final dk.DeviceKeyClient _client;
  final dk.StepUpPrompt Function() _stepUpPrompt;

  @override
  Future<DeviceKeys> ensure(String scope) async {
    final dpop = await _client.ensure(scope, dk.KeyPurpose.dpop);
    final stepUp = await _client.ensure(scope, dk.KeyPurpose.stepUp);
    return DeviceKeys(
      dpop: DeviceKeySignerAdapter(_client, scope, dk.KeyPurpose.dpop, dpop),
      stepUp: DeviceKeySignerAdapter(
        _client,
        scope,
        dk.KeyPurpose.stepUp,
        stepUp,
        prompt: _stepUpPrompt,
      ),
    );
  }

  @override
  Future<void> delete(String scope) => _client.delete(scope);
}

/// One device key as the [DeviceKeySigner] of `auth`: ES256, the JWK and thumbprint of
/// [DeviceKeyInfo], signatures from the plugin in the JOSE form `R || S`.
final class DeviceKeySignerAdapter implements DeviceKeySigner {
  /// Wraps the key [purpose] of [scope] that [info] describes; [prompt] gives the texts of a
  /// step-up confirmation.
  DeviceKeySignerAdapter(
    this._client,
    this._scope,
    this._purpose,
    this._info, {
    dk.StepUpPrompt Function()? prompt,
  }) : _prompt = prompt; // ignore: prefer_initializing_formals

  final dk.DeviceKeyClient _client;
  final String _scope;
  final dk.KeyPurpose _purpose;
  final dk.DeviceKeyInfo _info;
  final dk.StepUpPrompt Function()? _prompt;

  @override
  String get alg => 'ES256';

  @override
  Map<String, String> get publicJwk => _info.publicJwk;

  @override
  String get thumbprint => _info.thumbprint;

  @override
  Future<Uint8List> sign(Uint8List signingInput) =>
      _client.sign(_scope, _purpose, signingInput, prompt: _prompt?.call());
}
