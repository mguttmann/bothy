// SPDX-License-Identifier: Apache-2.0

// A sync engine over an in-memory database for tests that only need its outbox: the engine is
// never started, so nothing connects.

import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart';
import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

import 'fakes.dart';
import 'software_keys.dart';

/// An engine on [db] for `https://chat.example.com`, not started.
({SyncEngine engine, AuthSession auth}) idleEngine(LocalDatabase db) {
  final config = AppConfig.defaults().withOverrides(fastConfig);
  var n = 0;
  final ids = Uuid7Generator(
    nowMillis: () => DateTime.utc(2026, 10, 17).millisecondsSinceEpoch,
    random: (bits) => n++,
  );
  final base = Uri.parse('https://chat.example.com');
  final auth = AuthSession(
    serverUrl: base,
    keys: DeviceKeys(
      dpop: SoftwareP256Signer.seeded('e/dpop'),
      stepUp: SoftwareP256Signer.seeded('e/step_up'),
    ),
    secrets: _Secrets(),
    clock: const SystemClock(),
    ids: ids,
    config: config,
    device: const DeviceDescription(name: 'Alex laptop', platform: 'macos'),
    stepUpPrompt: const _Prompt(),
    buildClient: (interceptors) => createApiClient(
      baseUrl: base,
      traceContext: () => TraceContext(traceId: '1' * 32, sampled: false),
      readRetry: ReadRetryPolicy(
        base: Duration.zero,
        cap: Duration.zero,
        maxAttempts: 1,
      ),
      authInterceptors: interceptors,
    ),
  );
  final engine = SyncEngine.create(
    baseUrl: base,
    auth: auth,
    store: db,
    transport: IoGatewayTransport(),
    network: SwitchNetwork(),
    lifecycle: ForegroundLifecycle(),
    ticker: MicrotaskTicker(),
    config: config,
    client: GatewayClientInfo(
      (b) => b
        ..platform = DevicePlatform.macos
        ..version = '1.0.0',
    ),
    ids: ids,
  );
  return (engine: engine, auth: auth);
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
