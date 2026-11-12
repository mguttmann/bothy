// SPDX-License-Identifier: Apache-2.0

/// The device keys of M4-10 and M4-11 behind the signer ports of `auth` (docs/spec/
/// 15-client-architecture.md 9, brief M4-18 Design note 1).
library;

import 'dart:convert';
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
  ///
  /// Only a device without a session calls this ([RegisteredDeviceKeys.forLogin]): a key
  /// created in the place of a registered one is another key (08 5.4).
  Future<DeviceKeys> ensure(String scope);

  /// The `dpop` key of [scope], or with [stepUp] its `step_up` key, as the store holds it
  /// now; never creates one. `null` when the store has no such key. Throws a
  /// `DeviceKeyException`: `keyInvalidated` for a key that is gone for good, any other code
  /// when the store cannot tell right now (a locked keychain).
  Future<DeviceKeySigner?> existing(String scope, {required bool stepUp});

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
  final dk.StepUpPromptTexts Function() _stepUpPrompt;

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
  Future<DeviceKeySigner?> existing(
    String scope, {
    required bool stepUp,
  }) async {
    final purpose = stepUp ? dk.KeyPurpose.stepUp : dk.KeyPurpose.dpop;
    final info = await _client.keyInfo(scope, purpose);
    if (info == null) return null;
    return DeviceKeySignerAdapter(
      _client,
      scope,
      purpose,
      info,
      prompt: stepUp ? _stepUpPrompt : null,
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
    dk.StepUpPromptTexts Function()? prompt,
  }) : _prompt = prompt; // ignore: prefer_initializing_formals

  final dk.DeviceKeyClient _client;
  final String _scope;
  final dk.KeyPurpose _purpose;
  final dk.DeviceKeyInfo _info;
  final dk.StepUpPromptTexts Function()? _prompt;

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

/// A device key of the stored session is gone, is another key or no longer signs (08 5.4):
/// the session cannot be used any more, and the app registers as a new device.
final class DeviceKeyLost implements Exception {
  /// Creates the error for the keys of [keyScope].
  const DeviceKeyLost(this.keyScope);

  /// The scope of the instance's device keys ([DeviceKeyScopes]).
  final String keyScope;

  @override
  String toString() =>
      'DeviceKeyLost: a registered device key is no longer there';
}

/// The device keys an instance registered with, compared at every start with what the key
/// store holds (08 5.4, R-ID-11; docs/decisions.d/AUD-pkg-device-key-01.md).
///
/// A key store creates a missing key without a word, and a system deletes keys behind the
/// app's back (the step-up item when the device passcode is turned off, a TPM that is cleared,
/// a restore onto another device). A key created in the place of a registered one is another
/// key: the server answers its DPoP proofs with `invalid_dpop_proof` for good and counts its
/// step-up confirmations as wrong. So keys are created only while no session is stored
/// ([forLogin]), which also writes their RFC 7638 thumbprints into the instance's secrets
/// ([keyName]); with a session the store is only read ([ofSession]), and a missing, replaced
/// or invalidated key is [DeviceKeyLost].
///
/// Example: `final keys = await RegisteredDeviceKeys(store, secrets, scope).ofSession();`
final class RegisteredDeviceKeys {
  /// Creates the rule for the keys of [_scope] in [_store], recorded in [_secrets].
  const RegisteredDeviceKeys(this._store, this._secrets, this._scope);

  /// The name of the record in an instance's secrets: a JSON object with the thumbprint
  /// `dpop` and, for a device that has one, `step_up`. It goes with the secrets on a wipe.
  static const String keyName = 'device_keys';

  final DeviceKeyStore _store;
  final SecretStore _secrets;
  final String _scope;

  /// The keys for a login of a device without a session: created where they are missing and
  /// recorded. What a lost key left behind (`keyInvalidated`: a marker without its key on
  /// Windows) is deleted first, because no session uses it.
  Future<DeviceKeys> forLogin() async {
    DeviceKeys keys;
    try {
      keys = await _store.ensure(_scope);
    } on dk.DeviceKeyException catch (e) {
      if (e.code != dk.DeviceKeyError.keyInvalidated) rethrow;
      await _store.delete(_scope);
      keys = await _store.ensure(_scope);
    }
    await _record(keys);
    return keys;
  }

  /// The keys of the stored session; never creates one.
  ///
  /// Throws [DeviceKeyLost] when the DPoP key or a recorded step-up key is missing, has
  /// another thumbprint than recorded, or the store reports it as `keyInvalidated`. A session
  /// of a build before the record keeps the keys the store holds and gets its record here; a
  /// device recorded without a step-up key has none ([DeviceKeys.stepUp] `null`), whatever
  /// the store holds. A step-up key the store cannot read right now (`notAvailable`: a locked
  /// device) is no loss: this run has no step-up key, and the next start looks again. Every
  /// other failure of the store throws its `DeviceKeyException`, and nothing is deleted.
  Future<DeviceKeys> ofSession() async {
    final recorded = _decode(await _secrets.read(keyName));
    final dpop = await _existing(stepUp: false);
    if (dpop == null ||
        (recorded != null && recorded.dpop != dpop.thumbprint)) {
      throw DeviceKeyLost(_scope);
    }
    if (recorded != null && recorded.stepUp == null) {
      return DeviceKeys(dpop: dpop);
    }
    final DeviceKeySigner? stepUp;
    try {
      stepUp = await _existing(stepUp: true);
    } on dk.DeviceKeyException catch (e) {
      if (e.code != dk.DeviceKeyError.notAvailable) rethrow;
      return DeviceKeys(dpop: dpop);
    }
    if (recorded == null) {
      final keys = DeviceKeys(dpop: dpop, stepUp: stepUp);
      await _record(keys);
      return keys;
    }
    if (stepUp?.thumbprint != recorded.stepUp) throw DeviceKeyLost(_scope);
    return DeviceKeys(dpop: dpop, stepUp: stepUp);
  }

  Future<DeviceKeySigner?> _existing({required bool stepUp}) async {
    try {
      return await _store.existing(_scope, stepUp: stepUp);
    } on dk.DeviceKeyException catch (e) {
      if (e.code == dk.DeviceKeyError.keyInvalidated) {
        throw DeviceKeyLost(_scope);
      }
      rethrow;
    }
  }

  Future<void> _record(DeviceKeys keys) => _secrets.write(
    keyName,
    jsonEncode({
      'dpop': keys.dpop.thumbprint,
      'step_up': ?keys.stepUp?.thumbprint,
    }),
  );

  static ({String dpop, String? stepUp})? _decode(String? json) {
    if (json == null) return null;
    try {
      final map = jsonDecode(json);
      if (map is! Map) return null;
      final dpop = map['dpop'];
      final stepUp = map['step_up'];
      if (dpop is! String || (stepUp != null && stepUp is! String)) {
        return null;
      }
      return (dpop: dpop, stepUp: stepUp as String?);
    } on FormatException {
      return null;
    }
  }
}

/// [keys] with signers that call [onInvalidated] when a signature fails with
/// `keyInvalidated`: a key the system no longer accepts although the store still holds it
/// (Android after new biometrics were enrolled, an enclave key bound to the biometrics of
/// before, a key that vanished while the app ran). The failure itself still reaches the
/// caller.
DeviceKeys watchInvalidation(DeviceKeys keys, void Function() onInvalidated) {
  final stepUp = keys.stepUp;
  return DeviceKeys(
    dpop: _WatchedSigner(keys.dpop, onInvalidated),
    stepUp: stepUp == null ? null : _WatchedSigner(stepUp, onInvalidated),
  );
}

final class _WatchedSigner implements DeviceKeySigner {
  _WatchedSigner(this._inner, this._onInvalidated);

  final DeviceKeySigner _inner;
  final void Function() _onInvalidated;

  @override
  String get alg => _inner.alg;

  @override
  Map<String, String> get publicJwk => _inner.publicJwk;

  @override
  String get thumbprint => _inner.thumbprint;

  @override
  Future<Uint8List> sign(Uint8List signingInput) async {
    try {
      return await _inner.sign(signingInput);
    } on dk.DeviceKeyException catch (e) {
      if (e.code == dk.DeviceKeyError.keyInvalidated) _onInvalidated();
      rethrow;
    }
  }
}
