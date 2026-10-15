// SPDX-License-Identifier: Apache-2.0

// `run.updated` with `snapshot` (04 11.3 item 3) and truncated frames.

import 'package:domain/domain.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  test('a snapshot with a newer version replaces the document', () async {
    final (store, fetcher, _) = await focusedStore();
    store.onRunUpdated(snapshotFrame(3));
    expect(store.documentOf(runId)!.raw, doc(3));
    expect(fetcher.calls, 1, reason: 'a snapshot needs no refetch');
  });

  test('an older snapshot is ignored', () async {
    final (store, fetcher, _) = await focusedStore(version: 3);
    store.onRunUpdated(snapshotFrame(2));
    store.onRunUpdated(snapshotFrame(3));
    expect(store.documentOf(runId)!.raw, doc(3));
    expect(fetcher.calls, 1);
  });

  test('the frame version wins over the version inside the snapshot', () async {
    final (store, _, _) = await focusedStore();
    store.onRunUpdated({...snapshotFrame(2), 'version': 3});
    expect(store.documentOf(runId)!.version, const ResourceVersion(3));
  });

  test('a snapshot that is not an object is a gap', () async {
    final (store, fetcher, _) = await focusedStore();
    store.onRunUpdated({...snapshotFrame(2), 'snapshot': 'whole'});
    expect(fetcher.calls, 2);
    expect(store.documentOf(runId)!.raw, doc(1));
  });

  test('a truncated frame with a newer version refetches', () async {
    final (store, fetcher, _) = await focusedStore();
    store.onRunUpdated(truncatedFrame(3));
    expect(fetcher.calls, 2);
    await fetcher.respond(doc(3));
    expect(store.documentOf(runId)!.raw, doc(3));
  });

  test('a truncated frame that is not newer is ignored', () async {
    final (store, fetcher, _) = await focusedStore(version: 3);
    store.onRunUpdated(truncatedFrame(3));
    expect(fetcher.calls, 1);
  });

  test('a slow initial fetch older than a snapshot received meanwhile does not replace it', () async {
    final fetcher = FakeFetcher();
    final ticker = ManualFrameTicker();
    final store = RunStore(fetch: fetcher, ticker: ticker);
    store.focus(runId);
    store.onRunUpdated(snapshotFrame(3));
    expect(store.documentOf(runId)!.raw, doc(3));
    await fetcher.respond(doc(2));
    expect(store.documentOf(runId)!.raw, doc(3));
    expect(fetcher.calls, 1);
  });

  test(
    'patches after a snapshot during the initial fetch apply once it returns',
    () async {
      final fetcher = FakeFetcher();
      final store = RunStore(fetch: fetcher, ticker: ManualFrameTicker());
      store.focus(runId);
      store.onRunUpdated(snapshotFrame(2));
      store.onRunUpdated(patch(2));
      store.onRunUpdated(patch(3));
      await fetcher.respond(doc(1));
      expect(store.documentOf(runId)!.raw, doc(4));
      expect(fetcher.calls, 1);
    },
  );
}
