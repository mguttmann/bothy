// SPDX-License-Identifier: Apache-2.0

// Focus, unfocus and the end of a run (15 7.4, 04 11.3 items 5 and 6).

import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  test('focus fetches the whole document', () async {
    final fetcher = FakeFetcher();
    final store = RunStore(fetch: fetcher, ticker: ManualFrameTicker());
    store.focus(runId);
    expect(fetcher.ids, [runId]);
    expect(store.documentOf(runId), isNull);
    await fetcher.respond(doc(2));
    expect(store.documentOf(runId)!.raw, doc(2));
  });

  test('a second focus while the document is held fetches nothing', () async {
    final (store, fetcher, _) = await focusedStore();
    store.focus(runId);
    expect(fetcher.calls, 1);
  });

  test('a second focus during the initial fetch fetches nothing', () async {
    final fetcher = FakeFetcher();
    final store = RunStore(fetch: fetcher, ticker: ManualFrameTicker());
    store.focus(runId);
    store.focus(runId);
    expect(fetcher.calls, 1);
  });

  test('focus after a failed initial fetch retries', () async {
    final fetcher = FakeFetcher();
    final store = RunStore(fetch: fetcher, ticker: ManualFrameTicker());
    store.focus(runId);
    await fetcher.fail();
    expect(store.documentOf(runId), isNull);
    store.focus(runId);
    expect(fetcher.calls, 2);
    await fetcher.respond(doc(1));
    expect(store.documentOf(runId)!.raw, doc(1));
  });

  test(
    'a fetched document without a version counts as a failed fetch',
    () async {
      final fetcher = FakeFetcher();
      final store = RunStore(fetch: fetcher, ticker: ManualFrameTicker());
      store.focus(runId);
      await fetcher.respond(doc(1)..remove('version'));
      expect(store.documentOf(runId), isNull);
    },
  );

  test('unfocus drops the document', () async {
    final (store, _, ticker) = await focusedStore();
    final recorder = Recorder(store, runId);
    await ticker.tick();
    store.unfocus(runId);
    expect(store.documentOf(runId), isNull);
    await ticker.tick();
    expect(recorder.events.last, isNull);
    await recorder.cancel();
  });

  test('onRunEnded discards the document', () async {
    final (store, fetcher, ticker) = await focusedStore();
    final recorder = Recorder(store, runId);
    await ticker.tick();
    store.onRunEnded(runId);
    expect(store.documentOf(runId), isNull);
    await ticker.tick();
    expect(recorder.events.last, isNull);
    store.onRunUpdated(patch(1));
    expect(
      store.documentOf(runId),
      isNull,
      reason: 'a late patch after the end',
    );
    expect(fetcher.calls, 1);
    await recorder.cancel();
  });

  test('patches for unfocused runs are ignored', () async {
    final (store, fetcher, _) = await focusedStore();
    store.onRunUpdated({...patch(1), 'run_id': otherRunId.value});
    store.onRunUpdated({...snapshotFrame(2), 'run_id': otherRunId.value});
    store.onRunUpdated({...truncatedFrame(5), 'run_id': otherRunId.value});
    expect(store.documentOf(otherRunId), isNull);
    expect(fetcher.calls, 1);
  });

  test('a fetch that returns after unfocus is ignored', () async {
    final fetcher = FakeFetcher();
    final store = RunStore(fetch: fetcher, ticker: ManualFrameTicker());
    store.focus(runId);
    store.unfocus(runId);
    await fetcher.respond(doc(1));
    expect(store.documentOf(runId), isNull);
  });

  test('a fetch that returns after the end of the run is ignored', () async {
    final (store, fetcher, _) = await focusedStore();
    store.onRunUpdated(patch(2)); // gap: refetch in flight
    store.onRunEnded(runId);
    await fetcher.respond(doc(3));
    expect(store.documentOf(runId), isNull);
  });

  test(
    'a late fetch of an earlier focus never replaces the one of a new focus',
    () async {
      final fetcher = FakeFetcher();
      final store = RunStore(fetch: fetcher, ticker: ManualFrameTicker());
      store.focus(runId);
      store.unfocus(runId);
      store.focus(runId);
      expect(fetcher.calls, 2);
      await fetcher.respond(doc(4)); // the first, stale request
      expect(store.documentOf(runId), isNull);
      await fetcher.respond(doc(2)); // the request of the current focus
      expect(store.documentOf(runId)!.raw, doc(2));
    },
  );

  test('two runs are independent', () async {
    final (store, fetcher, _) = await focusedStore();
    store.focus(otherRunId);
    await fetcher.respond({...doc(3), 'id': otherRunId.value});
    store.onRunUpdated(patch(1));
    expect(store.documentOf(runId)!.version.value, 2);
    expect(store.documentOf(otherRunId)!.version.value, 3);
  });
}
