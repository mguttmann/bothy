// SPDX-License-Identifier: Apache-2.0

// The REST reads of the sync path have a deadline, and the snapshot of a resync never holds the
// frame queue of the connection (AUD-pkg-sync-engine-01, finding sync-engine-store-2): every
// read of `ApiFetcher` carries `client.sync.read_timeout` as the request option the auth
// interceptor turns into the timeouts of the request, and the first snapshot attempt of a
// resync runs outside the queue like its retries, so the close of the socket is handled while
// the snapshot is still on its way.

import 'dart:async';

import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart';
import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  test('every read of the fetcher carries client.sync.read_timeout', () async {
    final rest = FakeApi();
    const deadline = Duration(seconds: 7);
    final fetcher = ApiFetcher(
      createApiClient(
        baseUrl: serverUrl,
        traceContext: () => TraceContext(traceId: '2' * 32, sampled: false),
        readRetry: ReadRetryPolicy(
          base: Duration.zero,
          cap: Duration.zero,
          maxAttempts: 1,
        ),
        adapter: rest,
      ),
      readTimeout: deadline,
    );
    final space = SpaceId.parse(spaceId(1));
    final channel = ChannelId.parse(channelId(1));
    final thread = ThreadId.parse(threadId(1));
    rest
      ..routes['GET /sync/snapshot'] = ((_) => FakeReply(200, snapshotJson()))
      ..routes['GET /instance'] = ((_) => FakeReply(200, instanceJson()))
      ..routes['GET /threads/${thread.value}/messages'] = ((_) =>
          const FakeReply(200, {
            'items': <Object?>[],
            'has_more_before': false,
            'has_more_after': false,
          }))
      ..routes['GET /threads/${thread.value}'] = ((_) =>
          FakeReply(200, threadJson(1)))
      ..routes['GET /channels/${channel.value}'] = ((_) =>
          FakeReply(200, channelView(1)))
      ..routes['GET /spaces/${space.value}'] = ((_) =>
          FakeReply(200, spaceView(1)))
      ..routes['GET /spaces/${space.value}/channels'] = ((_) => FakeReply(200, {
        'items': [channelView(1)],
      }))
      ..routes['GET /channels/${channel.value}/threads'] = ((_) =>
          const FakeReply(200, {'items': <Object?>[]}))
      ..routes['GET /me/drafts'] = ((_) =>
          const FakeReply(200, {'items': <Object?>[]}))
      ..routes['GET /me/read-states'] = ((_) =>
          const FakeReply(200, {'items': <Object?>[]}))
      ..routes['GET /threads/${thread.value}/wakes'] = ((_) =>
          const FakeReply(200, {'items': <Object?>[]}));

    await fetcher.snapshot();
    await fetcher.instance();
    await fetcher.threadPage(thread);
    await fetcher.range(thread, const ThreadSeq(1), const ThreadSeq(3));
    await fetcher.thread(thread);
    await fetcher.channel(channel);
    await fetcher.space(space);
    await fetcher.loadStructure([
      TargetRef(RemovedTargetKind.space, space.value),
      TargetRef(RemovedTargetKind.channel, channel.value),
    ]);
    await fetcher.channelThreads(channel);
    await fetcher.myDrafts();
    await fetcher.myReadStates(channel);
    await fetcher.threadWakes(thread);
    // The two reads of a single object answer `404` here; their request counts all the same.
    await expectLater(
      fetcher.message(MessageId.parse(messageId(1, 1))),
      throwsA(anything),
    );
    await expectLater(
      fetcher.agent(PrincipalId.parse(robinId)),
      throwsA(anything),
    );

    expect(rest.requests.length, greaterThanOrEqualTo(17));
    expect(
      [
        for (final r in rest.requests)
          if (r.extra[requestTimeoutExtra] != deadline) '${r.method} ${r.path}',
      ],
      isEmpty,
      reason: 'reads without the deadline',
    );
  });

  test('the engine gives its reads the configured deadline', () async {
    final w = World(overrides: {'client.sync.read_timeout': '9s'});
    addTearDown(w.dispose);
    await w.connect();
    expect(w.rest.requests, isNotEmpty);
    expect([
      for (final r in w.rest.requests)
        if (r.extra[requestTimeoutExtra] != const Duration(seconds: 9))
          '${r.method} ${r.path}',
    ], isEmpty);
  });

  test('a resync snapshot without an answer does not hold the close of the '
      'connection', () async {
    final w = World();
    addTearDown(w.dispose);
    await w.connect(cursor: 30);
    await w.push('catchup.complete', {'head_seq': 30});
    final never = Completer<void>();
    addTearDown(never.complete);
    w.rest.gates['GET /sync/snapshot'] = never.future;

    await w.push('resync_required', {
      'reason': 'retention',
      'head_seq': 90,
      'sync_epoch': 'epoch-1',
    });
    expect(w.h.phase, ConnectionPhase.resyncing);
    // Frames that arrive meanwhile still wait for the snapshot.
    await w.dispatch([threadCreated(91, 1)]);
    expect(await w.cursor(), 30);

    w.ch.serverClose(1001);
    await settle();

    expect(
      w.h.phase,
      ConnectionPhase.backoff,
      reason: 'the close is handled while the snapshot read is open',
    );
  });
}
