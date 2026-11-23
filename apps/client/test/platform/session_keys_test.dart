// SPDX-License-Identifier: Apache-2.0

// The names of the stored session (docs/decisions.d of AUD-PKG-P02-client-b): the app finds the
// refresh token and the device ID under the names `auth` writes them (`SessionSecretKeys`), so
// the start routing and the retirement of old secrets follow a change of the scheme.

import 'package:auth/auth.dart' show SessionSecretKeys;
import 'package:client/platform/secure_store.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/fakes.dart';

void main() {
  test('the refresh token auth writes under an instance is the one the '
      'secure store recognizes', () async {
    final backend = MemorySecureBackend();
    const instanceId = '0193f2a7-0000-7000-8000-000000000001';
    final server = Uri.parse('https://chat.example.com');
    final secrets = InstanceSecrets(backend, instanceId);
    await secrets.write(SessionSecretKeys.refreshToken(server), 'R1');
    await secrets.write(SessionSecretKeys.deviceId(server), 'D1');

    final stored = backend.values.keys.toList();
    expect(stored.where(FlutterSecureBackend.isRefreshToken), [
      'instance/$instanceId/${SessionSecretKeys.refreshToken(server)}',
    ]);
    expect(await secrets.read(SessionSecretKeys.refreshToken(server)), 'R1');
    expect(await secrets.read(SessionSecretKeys.deviceId(server)), 'D1');
  });
}
