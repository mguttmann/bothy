// SPDX-License-Identifier: Apache-2.0

// The client rule of 04 11.3 item 4: equal base_version applies, a stale frame is ignored, a
// gap refetches the whole document once and replaces it.

import 'package:domain/domain.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  test('equal base_version applies the ops', () async {
    final (store, fetcher, _) = await focusedStore();
    store.onRunUpdated(patch(1));
    final current = store.documentOf(runId)!;
    expect(current.version, const ResourceVersion(2));
    expect(current.raw, doc(2));
    expect(fetcher.calls, 1, reason: 'only the initial fetch');
  });

  test('the fixtures chain from version 1 to 4', () async {
    final (store, fetcher, _) = await focusedStore();
    for (var base = 1; base <= 3; base++) {
      store.onRunUpdated(patch(base));
      expect(store.documentOf(runId)!.raw, doc(base + 1));
    }
    expect(fetcher.calls, 1);
  });

  test('a stale frame (version not newer) is ignored', () async {
    final (store, fetcher, _) = await focusedStore(version: 3);
    store.onRunUpdated(patch(1));
    store.onRunUpdated(patch(2));
    expect(store.documentOf(runId)!.raw, doc(3));
    expect(fetcher.calls, 1, reason: 'a stale frame never refetches');
  });

  test('a gapped frame refetches once and replaces the document', () async {
    final (store, fetcher, _) = await focusedStore();
    store.onRunUpdated(patch(3));
    expect(fetcher.calls, 2, reason: 'one refetch');
    expect(
      store.documentOf(runId)!.raw,
      doc(1),
      reason: 'the last good document stays visible during the refetch',
    );
    await fetcher.respond(doc(4));
    expect(store.documentOf(runId)!.raw, doc(4));
    expect(fetcher.calls, 2);
  });

  test(
    'patches during a refetch: newer ones apply afterwards, older ones drop',
    () async {
      final (store, fetcher, _) = await focusedStore();
      store.onRunUpdated(patch(2)); // gap: local 1, base 2
      expect(fetcher.calls, 2);
      store.onRunUpdated(
        patch(2),
      ); // arrives during the refetch, not newer than the fetch
      store.onRunUpdated(
        patch(3),
      ); // arrives during the refetch, newer than the fetch
      expect(
        fetcher.calls,
        2,
        reason: 'no second refetch while one is in flight',
      );
      await fetcher.respond(doc(3));
      expect(store.documentOf(runId)!.raw, doc(4));
      expect(fetcher.calls, 2, reason: 'one refetch for the whole scenario');
    },
  );

  test('a queued patch that still does not fit refetches again', () async {
    final (store, fetcher, _) = await focusedStore();
    store.onRunUpdated(patch(2)); // gap
    store.onRunUpdated(patch(3)); // queued
    await fetcher.respond(
      doc(2),
    ); // the server was at 2; 3 -> 4 still lacks 2 -> 3
    expect(fetcher.calls, 3);
    await fetcher.respond(doc(4));
    expect(store.documentOf(runId)!.raw, doc(4));
  });

  test(
    'a fetch error keeps the last good document and retries on the next patch',
    () async {
      final (store, fetcher, _) = await focusedStore();
      store.onRunUpdated(patch(3)); // gap
      store.onRunUpdated(patch(3)); // queued during the failing fetch
      await fetcher.fail();
      expect(store.documentOf(runId)!.raw, doc(1));
      expect(
        fetcher.calls,
        2,
        reason: 'a failed fetch does not loop over its queue',
      );
      store.onRunUpdated(patch(1)); // fits the kept document
      expect(store.documentOf(runId)!.raw, doc(2));
      store.onRunUpdated(patch(3)); // gap again: retry
      expect(fetcher.calls, 3);
      await fetcher.respond(doc(4));
      expect(store.documentOf(runId)!.raw, doc(4));
    },
  );

  test('a frame without a usable run_id or version is ignored', () async {
    final (store, fetcher, _) = await focusedStore();
    store.onRunUpdated({'run_id': 'not-a-uuid', 'version': 2});
    store.onRunUpdated({'version': 2});
    store.onRunUpdated({'run_id': runId.value});
    store.onRunUpdated({'run_id': runId.value, 'version': '2'});
    expect(store.documentOf(runId)!.raw, doc(1));
    expect(fetcher.calls, 1);
  });

  test('a frame with ops but no base_version is a gap', () async {
    final (store, fetcher, _) = await focusedStore();
    final frame = patch(1)..remove('base_version');
    store.onRunUpdated(frame);
    expect(fetcher.calls, 2);
  });

  test('ops that are not a list of objects are a gap', () async {
    final (store, fetcher, _) = await focusedStore();
    store.onRunUpdated({...patch(1), 'ops': 'replace'});
    expect(fetcher.calls, 2);
    await fetcher.respond(doc(2));
    store.onRunUpdated({
      ...patch(2),
      'ops': [1, 2],
    });
    expect(fetcher.calls, 3);
    expect(
      store.documentOf(runId)!.raw,
      doc(2),
      reason: 'never a partial state',
    );
  });
}
