// SPDX-License-Identifier: Apache-2.0

// 15 7.4: the interface draws at most once per frame; patches between two ticks collapse into
// one emission with the newest state.

import 'package:domain/domain.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  test(
    'ten patches between two ticks produce one emission with the newest state',
    () async {
      final (store, _, ticker) = await focusedStore();
      final recorder = Recorder(store, runId);
      await ticker.tick();
      expect(recorder.events.single!.version, const ResourceVersion(1));

      var version = 1;
      for (var i = 0; i < 10; i++) {
        store.onRunUpdated({
          'run_id': runId.value,
          'base_version': version,
          'version': version + 1,
          'ops': [
            {'op': 'replace', 'path': '/status_line', 'value': 'line $i'},
          ],
        });
        version++;
      }
      await pumpQueue();
      expect(recorder.events, hasLength(1), reason: 'nothing before the tick');
      await ticker.tick();
      expect(recorder.events, hasLength(2));
      expect(recorder.events.last!.version, const ResourceVersion(11));
      expect(recorder.events.last!.statusLine, 'line 9');

      await ticker.tick();
      expect(recorder.events, hasLength(2), reason: 'no change, no emission');
      await recorder.cancel();
    },
  );

  test('the store requests one frame per dirty period', () async {
    final (store, _, ticker) = await focusedStore();
    final requestsAfterFetch = ticker.requests;
    store.onRunUpdated(patch(1));
    store.onRunUpdated(patch(2));
    store.onRunUpdated(patch(3));
    expect(ticker.requests, requestsAfterFetch);
    await ticker.tick();
    store.onRunUpdated(snapshotFrame(4));
    expect(
      ticker.requests,
      requestsAfterFetch,
      reason: 'a stale snapshot changes nothing',
    );
  });

  test(
    'a listener that starts later receives the current state on the next tick',
    () async {
      final (store, _, ticker) = await focusedStore(version: 2);
      await ticker.tick();
      final recorder = Recorder(store, runId);
      await pumpQueue();
      expect(recorder.events, isEmpty);
      await ticker.tick();
      expect(recorder.events.single!.raw, doc(2));
      await recorder.cancel();
    },
  );

  test('two listeners of one run each receive one emission per tick', () async {
    final (store, _, ticker) = await focusedStore();
    final first = Recorder(store, runId);
    final second = Recorder(store, runId);
    await ticker.tick();
    store.onRunUpdated(patch(1));
    store.onRunUpdated(patch(2));
    await ticker.tick();
    expect(first.events.map((d) => d!.version.value), [1, 3]);
    expect(second.events.map((d) => d!.version.value), [1, 3]);
    await first.cancel();
    await second.cancel();
  });

  test('a listener of a run without a document receives null', () async {
    final ticker = ManualFrameTicker();
    final store = RunStore(fetch: FakeFetcher(), ticker: ticker);
    final recorder = Recorder(store, otherRunId);
    await ticker.tick();
    expect(recorder.events, [null]);
    await recorder.cancel();
  });

  test('dispose closes the streams and ignores later input', () async {
    final (store, fetcher, ticker) = await focusedStore();
    var done = false;
    store.watch(runId).listen((_) {}, onDone: () => done = true);
    await store.dispose();
    await pumpQueue();
    expect(done, isTrue);
    store.onRunUpdated(patch(1));
    store.focus(otherRunId);
    await ticker.tick();
    expect(fetcher.calls, 1);
    expect(store.documentOf(runId), isNull);
    expect(() => store.watch(runId), throwsStateError);
  });
}
