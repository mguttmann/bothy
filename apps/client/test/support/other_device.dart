// SPDX-License-Identifier: Apache-2.0

// A second, already active device of Alex against the fake server: it confirms (or rejects)
// the waiting device of E4 with its verification code and approves the pairing code of E5, as
// a person would on the active device (08 5.2, 5.3).

import 'dart:math';

import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart';
import 'package:domain/domain.dart';
import 'package:sync_engine/testing.dart';

import 'fakes.dart';
import 'software_keys.dart';

/// Alex's other device.
class OtherDevice {
  /// Creates the device for [server]; [readRetry] replaces the policy that never retries a
  /// read.
  OtherDevice(this.server, {ReadRetryPolicy? readRetry}) {
    final random = Random(3);
    auth = AuthSession(
      serverUrl: server.baseUri,
      keys: DeviceKeys(
        dpop: SoftwareP256Signer.seeded('other/dpop'),
        stepUp: SoftwareP256Signer.seeded('other/step_up'),
      ),
      secrets: _Secrets(),
      clock: const SystemClock(),
      ids: Uuid7Generator(
        nowMillis: () => DateTime.now().millisecondsSinceEpoch,
        random: (bits) => random.nextInt(1 << (bits > 31 ? 31 : bits)),
      ),
      config: AppConfig.defaults().withOverrides(fastConfig),
      device: const DeviceDescription(name: 'Alex phone', platform: 'ios'),
      stepUpPrompt: const _Prompt(),
      buildClient: (interceptors) => createApiClient(
        baseUrl: server.baseUri,
        traceContext: () => TraceContext(traceId: '2' * 32, sampled: false),
        readRetry:
            readRetry ??
            ReadRetryPolicy(
              base: Duration.zero,
              cap: Duration.zero,
              maxAttempts: 1,
            ),
        authInterceptors: interceptors,
      ),
    );
  }

  /// The fake.
  final FakeServer server;

  /// The session of this device.
  late final AuthSession auth;

  int _keys = 0;

  /// Signs in as Alex; the device is active (Alex has no pending rule yet).
  Future<void> signIn() async {
    final outcome = await auth.login.loginWithPassword(
      loginName: WorldSeed.alexLogin,
      password: WorldSeed.password,
      totpCode: WorldSeed.totp,
    );
    if (outcome is! LoginSignedIn) throw StateError('other device: $outcome');
  }

  /// Types [code] for the waiting device [deviceId]; a wrong code rejects it.
  Future<void> confirm(String deviceId, String code) =>
      auth.api.getDevicesApi().approveDevice(
        deviceId: deviceId,
        deviceApprove: DeviceApprove((b) => b..verificationCode = code),
        idempotencyKey: _key(),
      );

  /// Types the pairing [code] of the new device.
  Future<void> approvePairing(String code) =>
      auth.api.getDevicesApi().approvePairing(
        pairingApprove: PairingApprove((b) => b..pairingCode = code),
        idempotencyKey: _key(),
      );

  String _key() =>
      '0193f2a7-0000-7000-8000-${(_keys++).toRadixString(16).padLeft(12, '0')}';

  /// Releases the session.
  Future<void> dispose() => auth.dispose();
}

class _Secrets implements SecretStore {
  final Map<String, String> _values = {};

  @override
  Future<String?> read(String key) async => _values[key];

  @override
  Future<void> write(String key, String value) async => _values[key] = value;

  @override
  Future<void> delete(String key) async => _values.remove(key);
}

class _Prompt implements StepUpPrompt {
  const _Prompt();

  @override
  Future<bool> ask(String purposeText) async => true;
}
