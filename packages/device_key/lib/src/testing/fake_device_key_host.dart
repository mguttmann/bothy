// SPDX-License-Identifier: Apache-2.0

/// The native side of the plugin in memory, for tests on the host.
library;

import 'dart:async';

import 'package:flutter/services.dart';

import '../device_key.dart';
import '../generated/device_key_api.g.dart';

/// One key of a [FakeDeviceKeyHost]: its public point and its signatures.
///
/// The test brings the key material (a software P-256 key), so this package
/// needs no elliptic curve code of its own.
abstract interface class FakeHostKey {
  /// The x coordinate of the public key, 32 bytes big-endian.
  Uint8List get x;

  /// The y coordinate of the public key, 32 bytes big-endian.
  Uint8List get y;

  /// Signs [data] with ES256 and returns `R || S`, 64 bytes.
  Future<Uint8List> sign(Uint8List data);
}

/// The native side of the `device_key` plugin in memory.
///
/// [client] is a real [DeviceKeyClient] whose channel ends here: the generated
/// code of the plugin encodes every call, this host decodes it with the same
/// codec and answers in the reply form of Pigeon, errors as the stable codes
/// of [DeviceKeyError]. A test therefore runs the Dart half of the plugin as
/// the app does, and only the key storage of the platform is replaced.
///
/// The methods below the constructor change the storage behind the app's
/// back, the way each platform loses a key (docs/spec/08-identity.md 5.4).
///
/// Example:
/// ```dart
/// final host = FakeDeviceKeyHost(newKey: (scope, purpose, n) => myKey(n));
/// final info = await host.client.ensure('scope', KeyPurpose.dpop);
/// host.remove('scope', KeyPurpose.dpop); // the system deleted the item
/// expect(await host.client.keyInfo('scope', KeyPurpose.dpop), isNull);
/// ```
final class FakeDeviceKeyHost {
  /// Creates a host that takes each new key from [newKey]; `generation`
  /// counts the keys created so far, so two keys of one scope differ.
  ///
  /// [stepUpKey] false behaves as Windows does: `ensureKey` of the step-up
  /// key answers `not_available`.
  FakeDeviceKeyHost({required this.newKey, this.stepUpKey = true});

  /// Makes the key number `generation` of this host, for [KeyPurpose] and
  /// scope.
  final FakeHostKey Function(String scope, KeyPurpose purpose, int generation)
  newKey;

  /// Whether the platform has a step-up key.
  final bool stepUpKey;

  /// A real client over this host.
  late final DeviceKeyClient client = DeviceKeyClient(
    api: DeviceKeyHostApi(binaryMessenger: _Loopback(this)),
  );

  /// The calls the host received, as `<method> <purpose>` (`deleteKeys`
  /// without a purpose), in order.
  final List<String> calls = [];

  final Map<(String, KeyPurpose), FakeHostKey> _keys = {};
  final Set<(String, KeyPurpose)> _markerOnly = {};
  final Set<(String, KeyPurpose)> _invalidated = {};
  final Map<(String, KeyPurpose), DeviceKeyError> _failing = {};
  int _generation = 0;

  /// Whether the storage holds the key of [purpose] for [scope].
  bool has(String scope, KeyPurpose purpose) =>
      _keys.containsKey((scope, purpose));

  /// Removes the key as the system does (iOS deletes the step-up item when
  /// the device passcode is turned off): `keyInfo` finds nothing, and
  /// `ensureKey` would create another key.
  void remove(String scope, KeyPurpose purpose) =>
      _keys.remove((scope, purpose));

  /// Puts another key under the same name, as a second writer does (an app
  /// extension, or a build that created a missing key without asking).
  void replace(String scope, KeyPurpose purpose) =>
      _keys[(scope, purpose)] = newKey(scope, purpose, _generation++);

  /// Loses the key but keeps the record that the plugin created it (Windows:
  /// a marker whose key is gone, for example after a TPM clear): `ensureKey`,
  /// `keyInfo` and `sign` answer `key_invalidated` until `deleteKeys`.
  void loseKeepingMarker(String scope, KeyPurpose purpose) {
    _keys.remove((scope, purpose));
    _markerOnly.add((scope, purpose));
  }

  /// Keeps the key but lets every signature answer `key_invalidated`
  /// (Android after a new fingerprint or face was enrolled).
  void invalidate(String scope, KeyPurpose purpose) =>
      _invalidated.add((scope, purpose));

  /// Lets every call for the key answer [error], as a key storage that
  /// cannot answer right now does (a locked device); null ends it.
  void failWith(String scope, KeyPurpose purpose, DeviceKeyError? error) {
    if (error == null) {
      _failing.remove((scope, purpose));
    } else {
      _failing[(scope, purpose)] = error;
    }
  }

  /// Lets `deleteKeys` answer [error] and delete nothing, as a key storage
  /// that is locked or busy does; null ends it.
  DeviceKeyError? deleteError;

  Future<Object?> _handle(String method, List<Object?> args) async {
    final scope = args[0]! as String;
    if (method == 'deleteKeys') {
      calls.add(method);
      final refused = deleteError;
      if (refused != null) throw _error(refused);
      for (final purpose in KeyPurpose.values) {
        _keys.remove((scope, purpose));
        _markerOnly.remove((scope, purpose));
        _invalidated.remove((scope, purpose));
      }
      return null;
    }
    final purpose = switch (args[1]! as KeyPurposeMessage) {
      KeyPurposeMessage.dpop => KeyPurpose.dpop,
      KeyPurposeMessage.stepUp => KeyPurpose.stepUp,
    };
    calls.add('$method ${purpose.name}');
    final id = (scope, purpose);
    final failing = _failing[id];
    if (failing != null) throw _error(failing);
    if (_markerOnly.contains(id)) throw _error(DeviceKeyError.keyInvalidated);
    switch (method) {
      case 'ensureKey':
        if (purpose == KeyPurpose.stepUp && !stepUpKey) {
          throw _error(DeviceKeyError.notAvailable);
        }
        return _message(
          _keys[id] ??= newKey(scope, purpose, _generation++),
          purpose,
        );
      case 'keyInfo':
        final key = _keys[id];
        return key == null ? null : _message(key, purpose);
      case 'sign':
        final key = _keys[id];
        if (key == null) throw _error(DeviceKeyError.notAvailable);
        if (_invalidated.contains(id)) {
          throw _error(DeviceKeyError.keyInvalidated);
        }
        return key.sign(args[2]! as Uint8List);
      case 'attestation':
        if (!_keys.containsKey(id)) throw _error(DeviceKeyError.notAvailable);
        return null;
    }
    throw _error(DeviceKeyError.internal);
  }

  static DeviceKeyInfoMessage _message(FakeHostKey key, KeyPurpose purpose) =>
      DeviceKeyInfoMessage(
        x: key.x,
        y: key.y,
        hardwareBound: false,
        requiresUserPresence: purpose == KeyPurpose.stepUp,
      );

  static PlatformException _error(DeviceKeyError error) => PlatformException(
    code: error.wire,
    message: 'the fake host answers ${error.wire}',
  );
}

/// Carries the messages of the generated host API to a [FakeDeviceKeyHost]
/// and its replies back, both through the codec of the channel.
final class _Loopback implements BinaryMessenger {
  _Loopback(this._host);

  static const String _prefix =
      'dev.flutter.pigeon.device_key.DeviceKeyHostApi.';
  static const MessageCodec<Object?> _codec =
      DeviceKeyHostApi.pigeonChannelCodec;

  final FakeDeviceKeyHost _host;

  @override
  Future<ByteData?> send(String channel, ByteData? message) async {
    // No handler on a channel of another plugin, as on a platform without it.
    if (!channel.startsWith(_prefix)) return null;
    final args = _codec.decodeMessage(message)! as List<Object?>;
    List<Object?> reply;
    try {
      reply = [await _host._handle(channel.substring(_prefix.length), args)];
    } on PlatformException catch (e) {
      reply = [e.code, e.message, e.details];
    }
    return _codec.encodeMessage(reply);
  }

  @override
  Future<void> handlePlatformMessage(
    String channel,
    ByteData? data,
    PlatformMessageResponseCallback? callback,
  ) async {
    callback?.call(null);
  }

  @override
  void setMessageHandler(String channel, MessageHandler? handler) {}
}
