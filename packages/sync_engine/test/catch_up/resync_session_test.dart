// SPDX-License-Identifier: Apache-2.0

// The retry of a failed resync snapshot runs outside the ordered frame queue of the connection
// (D-M4-14-9). A disconnect and a new `ready` while the retry is between its awaits must end it:
// it may not set the new session live, report `Resynced` for it or strand its frames (04 9,
// D-M4-14-1).

import 'dart:async';
import 'dart:convert';

import 'package:api_client/api_client.dart';
import 'package:domain/domain.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

import 'support.dart';

/// A [SessionControl] that records the phases it is set to.
final class _Control implements SessionControl {
  final List<ConnectionPhase> phases = [];

  @override
  void setPhase(ConnectionPhase phase) => phases.add(phase);

  @override
  bool send(GatewayFrame frame) => true;

  @override
  set lastProcessedSeq(GlobalSeq seq) {}

  @override
  String? get syncEpoch => 'epoch-1';
}

GatewayFrame _frame(String op, Map<String, Object?> d) =>
    decodeFrame(jsonEncode({'op': op, 'd': d}))!;

GatewayReady _ready() {
  final d = readyData(headSeq: 40);
  d['snapshot'] = snapshotJson(headSeq: 40);
  return (_frame('ready', d) as ReadyFrame).model;
}

void main() {
  test(
    'a new session during the snapshot retry stays in its catch-up',
    () async {
      final h = Harness();
      final rest = FakeApi();
      var fail = true;
      rest.routes['GET /instance'] = (_) => FakeReply(200, instanceJson());
      rest.routes['GET /sync/snapshot'] = (_) => fail
          ? FakeReply.problem(503, 'unavailable')
          : FakeReply(200, snapshotJson(headSeq: 90, epoch: 'epoch-2'));
      late CatchUpListener listener;
      final second = _Control();
      var armed = false;
      // Called by the retry between its commit and its follow-up steps: here the old socket
      // ends and the next one sends `ready`, as the connection queue would while the retry waits.
      Iterable<ThreadId> openThreads() {
        if (armed) {
          armed = false;
          listener.onDisconnected();
          unawaited(listener.onReady(_ready(), second));
        }
        return const [];
      }

      listener = CatchUpListener(
        db: h.db,
        fetcher: ApiFetcher(
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
        ),
        config: h.config,
        clock: h.clock,
        openThreads: openThreads,
      );
      final notices = <SyncNotice>[];
      listener.notices.listen(notices.add);
      addTearDown(() async {
        await listener.dispose();
        await h.dispose();
      });

      await h.storeCursor(30);
      final first = _Control();
      await listener.onReady(_ready(), first);
      await listener.onFrame(
        _frame('resync_required', {
          'reason': 'epoch_changed',
          'head_seq': 90,
          'sync_epoch': 'epoch-2',
        }),
        first,
      );
      expect(first.phases.last, ConnectionPhase.resyncing);

      fail = false;
      armed = true;
      await h.clock.elapse(const Duration(seconds: 2));
      await settle();
      await listener.idle();
      expect(armed, isFalse, reason: 'the retry reached its follow-up steps');

      expect(
        notices.whereType<Resynced>(),
        isEmpty,
        reason: 'no resync for the new session',
      );
      expect(first.phases.last, ConnectionPhase.resyncing);
      expect(second.phases, isNot(contains(ConnectionPhase.live)));

      // The new session is in its catch-up: a dispatch frame waits for its page and moves no
      // cursor on its own.
      final before = (await h.db.syncStateDao.read()).cursor?.value;
      await listener.onFrame(_frame('dispatch', threadCreated(91, 4)), second);
      expect((await h.db.syncStateDao.read()).cursor?.value, before);
    },
  );

  test(
    'frames of a new session during the drain of held frames are kept',
    () async {
      final h = Harness();
      final rest = FakeApi();
      var fail = true;
      rest.routes['GET /instance'] = (_) => FakeReply(200, instanceJson());
      rest.routes['GET /sync/snapshot'] = (_) => fail
          ? FakeReply.problem(503, 'unavailable')
          : FakeReply(200, snapshotJson(headSeq: 90, epoch: 'epoch-2'));
      final listener = CatchUpListener(
        db: h.db,
        fetcher: ApiFetcher(
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
        ),
        config: h.config,
        clock: h.clock,
      );
      addTearDown(() async {
        await listener.dispose();
        await h.dispose();
      });
      final second = _Control();
      var armed = false;
      // A notice of a held frame that the drain applies: here the old socket ends, the next one
      // sends `ready` and its first catch-up frame, while the old drain is still in that frame.
      listener.notices.listen((n) {
        if (!armed || n is! TargetsRemoved) return;
        armed = false;
        listener.onDisconnected();
        unawaited(listener.onReady(_ready(), second));
        unawaited(
          listener.onFrame(_frame('dispatch', threadCreated(93, 5)), second),
        );
      });

      await h.storeCursor(30);
      final first = _Control();
      await listener.onReady(_ready(), first);
      await listener.onFrame(
        _frame('resync_required', {
          'reason': 'epoch_changed',
          'head_seq': 90,
          'sync_epoch': 'epoch-2',
        }),
        first,
      );
      // Held while the snapshot is missing; the second one ends in a TargetsRemoved notice.
      await listener.onFrame(_frame('dispatch', threadCreated(91, 4)), first);
      await listener.onFrame(
        _frame(
          'dispatch',
          envelope(
            'thread.moved_out',
            seq: 92,
            payload: {'thread_id': threadId(4), 'channel_id': channelId(1)},
            resourceKind: 'channel',
            resourceId: channelId(1),
          ),
        ),
        first,
      );
      fail = false;
      armed = true;
      await h.clock.elapse(const Duration(seconds: 2));
      await settle();
      await listener.idle();
      expect(armed, isFalse, reason: 'the drain applied the moved_out frame');

      await listener.onFrame(
        _frame('catchup.complete', {'head_seq': 93}),
        second,
      );
      await listener.idle();
      expect(
        await h.db.threadsDao.byId(ThreadId.parse(threadId(5))),
        isNotNull,
        reason: 'the catch-up frame of the new session was not stranded',
      );
      expect((await h.db.syncStateDao.read()).cursor?.value, 93);
    },
  );
}
