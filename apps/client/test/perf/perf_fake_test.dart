// SPDX-License-Identifier: Apache-2.0

// The fake server of the mobile performance level on its own isolate (brief M5-11 Design note
// 5: "patches from the fake server of M4-17 inside the app, so the measurement isolates the
// client"): it starts with the world of the measurement, answers on 127.0.0.1, adds messages,
// streams the status line of a run and stops, each command through the isolate's port.

import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

import '../../perf_test/phone/perf_fake.dart';

void main() {
  test('the fake runs on its own isolate with the world of the measurement', () async {
    final fake = await PerfFake.spawn();
    try {
      expect(fake.baseUri.host, '127.0.0.1');
      expect(fake.threads, hasLength(openThreads));
      expect(fake.threads.toSet(), hasLength(openThreads));
      expect(fake.spaceId, isNotEmpty);
      expect(fake.instanceId, isNotEmpty);

      // The discovery document of the instance comes from the other isolate, over a real
      // socket (the test binding answers every HttpClient of its own with 400).
      await HttpOverrides.runWithHttpOverrides(() async {
        final client = HttpClient();
        try {
          final request = await client.getUrl(
            fake.baseUri.replace(path: '/api/v1/instance/discovery'),
          );
          final response = await request.close();
          final body = await response.transform(utf8.decoder).join();
          expect(response.statusCode, 200, reason: body);
          expect(body, contains(fake.instanceId));
        } finally {
          client.close(force: true);
        }
      }, _RealSockets());

      // Each thread starts with the messages of the seed; posts add to one thread only.
      expect(await fake.post(fake.threads[1], 0), messagesPerThread);
      expect(await fake.post(fake.threads[1], 25), messagesPerThread + 25);
      expect(await fake.post(fake.threads[0], 0), messagesPerThread);

      // A stream posts its trigger and patches the run's status line in the background.
      final run = await fake.stream(
        fake.threads[0],
        patches: 3,
        every: const Duration(milliseconds: 1),
      );
      expect(run, isNotEmpty);
      expect(await fake.streamed(run), 3);
      expect(await fake.post(fake.threads[0], 0), messagesPerThread + 1);

      await fake.disconnect('EXAMPLE-device');
      expect(await fake.violations(), isEmpty);
    } finally {
      await fake.stop();
    }
  });

  test('a failed command names its error in the test isolate', () async {
    final fake = await PerfFake.spawn();
    try {
      await expectLater(
        fake.post('no-such-thread', 1),
        throwsA(isA<PerfFakeError>()),
      );
    } finally {
      await fake.stop();
    }
  });
}

/// The `dart:io` client itself, in place of the test binding's.
final class _RealSockets extends HttpOverrides {}
