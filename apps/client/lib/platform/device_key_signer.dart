// SPDX-License-Identifier: Apache-2.0

/// The device keys of M4-10 and M4-11 behind the signer ports of `auth` (docs/spec/
/// 15-client-architecture.md 9, brief M4-18 Design note 1).
library;

import 'dart:typed_data';

import 'package:auth/auth.dart';
import 'package:device_key/device_key.dart' as dk;

import 'secure_store.dart' show FlutterSecureBackend, InstanceSecrets;

/// The device keys of one instance, created on demand and deleted on a wipe.
///
/// [PlatformDeviceKeyStore] uses the secure hardware of the platform; the tests and the
/// integration test entry use software keys.
abstract interface class DeviceKeyStore {
  /// Creates the `dpop` and `step_up` keys of [scope] if they do not exist and returns their
  /// signers; [DeviceKeys.stepUp] is `null` on a platform without a step-up key (Windows).
  /// Throws a `DeviceKeyException` when the platform cannot create a key it should have.
  Future<DeviceKeys> ensure(String scope);

  /// Deletes both keys of [scope]; missing keys are no error.
  Future<void> delete(String scope);
}

/// The scope of the device keys of one instance (docs/decisions.d/FIX-app-logout.md FIX-AL/4).
///
/// Where the keys lie in a store every app of the person reaches (the macOS login keychain of a
/// debug or profile build, the Windows key storage of the user; [shared] says so, none: always),
/// [appId] is the app identity and a new enrollment gets the scope `<appId>/<instance_id>`, so another bundle identifier never
/// signs with, or deletes, this app's keys. An instance copied from a build before this rule
/// keeps its keys under the `instance_id`, which the copy records as its scope
/// (docs/decisions.d/FIX-secure-store-migration.md FIX-SSM/2), so an update signs no one out; an
/// instance that already has other secrets keeps the `instance_id` as well. The choice is stored with the instance's secrets and goes with
/// them on a wipe. Without [appId] or in a store of the app's own (iOS, Android, the macOS Data
/// Protection keychain with its access group; there the app extensions open the keys by the
/// `instance_id`) the scope is the `instance_id`.
///
/// Example: `final scope = await DeviceKeyScopes(appId: 'com.example.chat').resolve(secrets);`
final class DeviceKeyScopes {
  /// Creates the rule; [appId] is null where the key store belongs to the app alone.
  const DeviceKeyScopes({this.appId, this.shared});

  /// The name of the stored scope in the instance's secrets.
  static const String keyName = FlutterSecureBackend.deviceKeyScopeKey;

  /// The app identity of a shared key store, or null.
  final String? appId;

  /// Whether the key store is shared between apps; none: it is whenever [appId] is set.
  final Future<bool> Function()? shared;

  /// Returns the scope of the instance of [secrets], storing it at the first call.
  Future<String> resolve(InstanceSecrets secrets) async {
    final instanceId = secrets.instanceId;
    final id = appId;
    final isShared = shared;
    if (id == null || (isShared != null && !await isShared())) {
      return instanceId;
    }
    final stored = await secrets.read(keyName);
    if (stored != null) return stored;
    final scope = await secrets.isEmpty() ? '$id/$instanceId' : instanceId;
    await secrets.write(keyName, scope);
    return scope;
  }
}

/// [DeviceKeyStore] over the `device_key` plugin.
///
/// The scope comes from [DeviceKeyScopes] and contains the `instance_id` of discovery, so a key
/// never serves two instances.
/// [stepUpPrompt] gives the texts of the system dialog of a step-up signature in the current
/// language.
///
/// Example: `final keys = await PlatformDeviceKeyStore(DeviceKeyClient(), prompt).ensure(id);`
final class PlatformDeviceKeyStore implements DeviceKeyStore {
  /// Creates the store over [client].
  PlatformDeviceKeyStore(this._client, this._stepUpPrompt);

  final dk.DeviceKeyClient _client;
  final dk.StepUpPrompt Function() _stepUpPrompt;

  /// A step-up key that the plugin reports as `not_available` is no error: Windows has none by
  /// design (08 6.1, D-M4-11-2), and the device registers without it ([DeviceKeys.stepUp]
  /// `null`). Every other failure of either key throws.
  @override
  Future<DeviceKeys> ensure(String scope) async {
    final dpop = await _client.ensure(scope, dk.KeyPurpose.dpop);
    dk.DeviceKeyInfo? stepUp;
    try {
      stepUp = await _client.ensure(scope, dk.KeyPurpose.stepUp);
    } on dk.DeviceKeyException catch (e) {
      if (e.code != dk.DeviceKeyError.notAvailable) rethrow;
    }
    return DeviceKeys(
      dpop: DeviceKeySignerAdapter(_client, scope, dk.KeyPurpose.dpop, dpop),
      stepUp: stepUp == null
          ? null
          : DeviceKeySignerAdapter(
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
