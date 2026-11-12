// SPDX-License-Identifier: Apache-2.0

// The deadline of the engine's REST reads on the real path (AUD-pkg-sync-engine-01, finding
// sync-engine-store-2): the engine over the auth session of `package:auth` and dio against the
// fake server on 127.0.0.1, which holds one answer back. The read ends after
// `client.sync.read_timeout` instead of waiting for the answer.

import 'package:dio/dio.dart';
import 'package:domain/domain.dart';
import 'package:sync_engine/testing.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  test('a read without an answer ends after client.sync.read_timeout', () async {
    final server = await FakeServer.start(
      options: const FakeServerOptions(timings: fastTimings),
    );
    final seed = WorldSeed.standard(server.world);
    final client = EngineScenarioClient(
      server,
      overrides: {'client.sync.read_timeout': '500ms'},
    );
    addTearDown(() async {
      await client.stop();
      await server.stop();
    });
    await client.start();
    // Far longer than the deadline, so a loaded machine does not blur the two.
    const held = Duration(seconds: 6);
    server.faults.delay(held, operationId: 'getThread');

    final watch = Stopwatch()..start();
    await expectLater(
      client.engine.catchUp.loadThread(ThreadId.parse(seed.printer)),
      throwsA(
        isA<DioException>().having(
          (e) => e.type,
          'type',
          anyOf(
            DioExceptionType.receiveTimeout,
            DioExceptionType.connectionTimeout,
            DioExceptionType.sendTimeout,
          ),
        ),
      ),
    );
    expect(
      watch.elapsed,
      lessThan(held),
      reason: 'the read ended before its answer came',
    );
  });
}
