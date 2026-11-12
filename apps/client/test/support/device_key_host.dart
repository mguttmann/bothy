// SPDX-License-Identifier: Apache-2.0

// The `device_key` plugin of the host tests: the real `DeviceKeyClient`, its channel code and
// codec and the app's `PlatformDeviceKeyStore`, over the in-memory host of
// `package:device_key/testing.dart` with software P-256 keys, so the server of a test verifies
// real signatures and a test can lose a key the way a platform does.

import 'dart:convert';
import 'dart:typed_data';

import 'package:client/platform/device_key_signer.dart';
import 'package:device_key/device_key.dart' as dk;
import 'package:device_key/testing.dart';

import 'software_keys.dart';

/// A software key of the fake host.
class _SoftwareHostKey implements FakeHostKey {
  _SoftwareHostKey(this._signer);

  final SoftwareP256Signer _signer;

  static Uint8List _bytes(String b64) =>
      base64Url.decode(base64Url.normalize(b64));

  @override
  Uint8List get x => _bytes(_signer.publicJwk['x']!);

  @override
  Uint8List get y => _bytes(_signer.publicJwk['y']!);

  @override
  Future<Uint8List> sign(Uint8List data) => _signer.sign(data);
}

/// The plugin of one test device: [host] is its key storage, [store] what the app gets as its
/// `DeviceKeyStore`.
final class PluginKeys {
  /// Creates the plugin; [stepUpKey] false is a platform without a step-up key (Windows).
  PluginKeys({bool stepUpKey = true})
    : host = FakeDeviceKeyHost(
        newKey: (scope, purpose, generation) => _SoftwareHostKey(
          SoftwareP256Signer.seeded('$scope/${purpose.name}/$generation'),
        ),
        stepUpKey: stepUpKey,
      ) {
    store = PlatformDeviceKeyStore(
      host.client,
      () => const dk.StepUpPromptTexts(
        title: 'Confirm',
        negativeButton: 'Cancel',
      ),
    );
  }

  /// The key storage of the platform.
  final FakeDeviceKeyHost host;

  /// The app's store over the real client.
  late final PlatformDeviceKeyStore store;
}
