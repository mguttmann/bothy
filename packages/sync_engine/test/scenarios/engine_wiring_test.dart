// SPDX-License-Identifier: Apache-2.0

// The wiring of SyncEngine (brief M4-17 Design note 2): the outbox does not send before
// catchup.complete and pauses on disconnect; the echo confirms; run patches reach the run
// store; a run end discards the document; only test builds accept plain http.

import 'package:domain/domain.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:sync_engine/testing.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  test(
    'the outbox sends only after catchup.complete and pauses on disconnect',
    () async {
      await withScenario((server, w, client) async {
        final sentAfterCatchUp = <bool>[];
        final original = server.world;
        server.handle('createMessage', (r) {
          final number = server.gateway.connections.last.number;
          sentAfterCatchUp.add(
            server.gateway
                .sentOn(number)
                .any((f) => f.op == 'catchup.complete'),
          );
          final result = original.createMessage(
            r.person.id,
            r.param('thread_id'),
            r.json,
          );
          return FakeResponse(result.replay ? 200 : 201, result.message);
        });
        await client.start();
        await client.goOffline();
        for (var i = 0; i < 5; i++) {
          w.post(w.sam, w.releaseNotes, 'Catch-up $i');
        }
        server.world.catchupPageSize = 2;
        await client.send(w.releaseNotes, 'Queued while offline');
        await Future<void>.delayed(const Duration(milliseconds: 150));
        expect(
          server.requestsOf('createMessage'),
          isEmpty,
          reason: 'paused while offline',
        );
        expect(await client.outbox(), hasLength(1));
        await client.goOnline();
        await waitFor('sent', () async => (await client.outbox()).isEmpty);
        expect(sentAfterCatchUp, [true]);
        expect(server.violations, isEmpty);
      });
    },
  );

  test('the echo confirms the entry while the response is held', () async {
    await withScenario((server, w, client) async {
      await client.start();
      server.faults.delay(
        const Duration(seconds: 2),
        operationId: 'createMessage',
      );
      await client.send(w.releaseNotes, 'Echo first');
      await waitFor('confirmed', () async => (await client.outbox()).isEmpty);
      expect(server.requestsOf('createMessage'), isEmpty);
    });
  });

  test(
    'run patches reach the run store and the run end discards the document',
    () async {
      await withScenario((server, w, client) async {
        await client.start();
        server.world.createMessage(w.alex.id, w.releaseNotes, {
          'client_msg_id': server.world.nextId(),
          'content_md': 'Start a run',
        });
        final message =
            server.world.messagesOf(w.releaseNotes).last['id']! as String;
        final run = server.world.createRun(
          agentId: w.robin.id,
          triggeringMessageId: message,
          triggeredBy: w.alex.id,
        );
        final runId = run['id']! as String;
        await client.focusRun(runId, w.releaseNotes);
        final store = client.engine.runs;
        final id = RunId.parse(runId);
        await waitFor('the document', () => store.documentOf(id) != null);
        expect(
          client.engine.focus.current,
          contains(ThreadId.parse(w.releaseNotes)),
          reason: 'the run puts its thread into the gateway focus',
        );
        server.world.patchRun(runId, [
          {'op': 'add', 'path': '/status_line', 'value': 'Reading'},
        ]);
        await waitFor(
          'the patch',
          () => store.documentOf(id)?.statusLine == 'Reading',
        );
        server.world.setRunState(runId, 'completed', triggerState: 'done');
        await waitFor(
          'the discarded document',
          () => store.documentOf(id) == null,
        );
        expect(server.violations, isEmpty);
      });
    },
  );

  test('only test builds accept plain http', () {
    expect(
      SyncEngine.gatewayUrlOf(Uri.parse('https://chat.example.com')),
      Uri.parse('wss://chat.example.com/api/v1/gateway'),
    );
    expect(
      SyncEngine.gatewayUrlOf(
        Uri.parse('http://127.0.0.1:4711'),
        allowPlainHttp: true,
      ),
      Uri.parse('ws://127.0.0.1:4711/api/v1/gateway'),
    );
    expect(
      () => SyncEngine.gatewayUrlOf(Uri.parse('http://chat.example.com')),
      throwsArgumentError,
    );
    expect(
      () => SyncEngine.gatewayUrlOf(Uri.parse('ftp://chat.example.com')),
      throwsArgumentError,
    );
  });
}
