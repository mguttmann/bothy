// SPDX-License-Identifier: Apache-2.0

// Who retries a refused read of the agent interface (rulings AUD-CA01/2 and AUD-CA01/3), counted
// at the Dart fake server on 127.0.0.1 with the app's real API client and its read retry: the
// document of an ended run line (`getRun`) is requested as often as the read retry of the API
// client allows and never again by Riverpod; the session read of `/status` and `/resume` is one
// request whose failure reaches the command, while the read of the session column keeps the
// read retry and reads a failure as "no sessions".

import 'dart:io';

import 'package:api_client/api_client.dart' show ReadRetryPolicy;
import 'package:client/agents/providers.dart';
import 'package:client/app/providers/ports.dart';
import 'package:dio/dio.dart';
import 'package:domain/domain.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/testing.dart';

import '../support/app_scenario_client.dart';
import '../support/fake_instance.dart';
import '../support/fakes.dart';
import 'ended_runs_fake_server_test.dart' show failedRun;

void main() {
  /// The app of Alex signed in on a fake where [before] ran first.
  Future<void> withApp(
    Future<void> Function(FakeInstance f, ProviderContainer c) body, {
    void Function(FakeInstance f)? before,
  }) async {
    final saved = HttpOverrides.current;
    HttpOverrides.global = null;
    final f = await FakeInstance.start();
    before?.call(f);
    final c = ProviderContainer(
      overrides: [appPortsProvider.overrideWithValue(f.ports)],
    );
    final client = AppScenarioClient(
      c,
      f.server,
      f.ports.network as SwitchNetwork,
    );
    try {
      await client.start();
      await body(f, c);
    } finally {
      await client.stop();
      c.dispose();
      await f.stop();
      HttpOverrides.global = saved;
    }
  }

  int requests(FakeInstance f, String operationId) =>
      f.server.rest.requests.where((r) => r.operationId == operationId).length;

  /// The attempts the read retry of the app's API client makes for one read.
  int readAttempts(ProviderContainer c) {
    final config = c.read(appPortsProvider).config;
    return ReadRetryPolicy.fromBackoff(
      base: config.outboxRetryBase,
      cap: config.outboxRetryCap,
    ).maxAttempts;
  }

  test('a refused getRun of an ended run line is retried by the API client '
      'alone, never again by Riverpod', () async {
    late String runId;
    await withApp(before: (f) => runId = failedRun(f), (f, c) async {
      final attempts = readAttempts(c);
      expect(attempts, greaterThan(1));
      f.server.faults.onRequest(
        'getRun',
        status: 429,
        code: 'rate_limited',
        times: attempts * 20,
      );
      final document = c.listen(endedRunDocumentProvider(runId), (_, _) {});
      await eventually('the failed read', () => document.read().hasError);
      expect(document.read().error, isA<DioException>());
      expect(requests(f, 'getRun'), attempts);
      // Riverpod's default would run the provider again after 200 ms, then 400 ms, each
      // time with every attempt of the read retry.
      await Future<void>.delayed(const Duration(seconds: 2));
      expect(requests(f, 'getRun'), attempts);
      expect(document.read().hasError, isTrue);
      document.close();
    });
  });

  test('the session read of /status and /resume is one request and its '
      'failure reaches the command; the session column keeps the read '
      'retry', () async {
    await withApp((f, c) async {
      final w = f.seed;
      final thread = ThreadId.parse(w.releaseNotes);
      FakeAgentInterface.of(f.server.world)
          .addEarlierSession(w.releaseNotes, w.robin.id, 'sess-earlier');
      final reads = c.read(agentReadsProvider);
      expect(readAttempts(c), greaterThan(1));

      f.server.faults.onRequest(
        'listAgentSessions',
        status: 503,
        code: 'unavailable',
      );
      await expectLater(
        reads.sessionsNow(thread),
        throwsA(isA<DioException>()),
      );
      expect(requests(f, 'listAgentSessions'), 1);
      // The fault is used up: the same read answers at once.
      expect(
        [for (final s in await reads.sessionsNow(thread)) s.externalSessionRef],
        ['sess-earlier'],
      );
      expect(requests(f, 'listAgentSessions'), 2);

      // The session column: the refused read is tried again by the API client and answers.
      f.server.faults.onRequest(
        'listAgentSessions',
        status: 503,
        code: 'unavailable',
      );
      expect(await reads.sessions(thread), hasLength(1));
      expect(requests(f, 'listAgentSessions'), 4);
      // ... and a read that stays refused reads as no sessions.
      f.server.faults.onRequest(
        'listAgentSessions',
        status: 503,
        code: 'unavailable',
        times: readAttempts(c),
      );
      expect(await reads.sessions(thread), isEmpty);
      expect(requests(f, 'listAgentSessions'), 4 + readAttempts(c));
      expect(f.server.violations, isEmpty);
    });
  });
}
