// SPDX-License-Identifier: Apache-2.0

/// Where the anchor of a run loads the runs of its thread when the local `runs` table lacks its
/// run (`listThreadRuns`; ruling AUD-CA01/1). The table holds a run only from the active runs of
/// the snapshot and from `run.state_changed` since then (04 11.3), so a new device and a device
/// after a resync know no run that ended before; its line (01 3.2, 6.2) and "Retry" (09 5.7)
/// need the summary. Tests replace the port.
library;

import 'package:api_client/api_client.dart' as wire;
import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

/// Loads the run summaries of a thread into the local table.
abstract interface class ThreadRunsLoader {
  /// Loads the runs of [thread]; errors are the caller's to swallow (the view keeps what the
  /// table holds).
  Future<void> load(ThreadId thread);
}

/// [ThreadRunsLoader] through the REST client of [engine] into [db].
final class EngineThreadRunsLoader implements ThreadRunsLoader {
  /// Creates the loader.
  const EngineThreadRunsLoader(this.engine, this.db);

  /// The engine.
  final SyncEngine engine;

  /// The local database of the engine.
  final LocalDatabase db;

  /// Reads every page of `listThreadRuns` (newest first) and stores each summary by the
  /// version rule, like `run.state_changed`. A page is stored only while the thread and its
  /// channel are stored here, checked in the transaction of the write: an answer that arrives
  /// after the device lost the thread stores nothing and ends the load. Throws what the read
  /// throws.
  @override
  Future<void> load(ThreadId thread) async {
    final api = engine.api;
    String? cursor;
    do {
      final page = (await api.getRunsApi().listThreadRuns(
        threadId: thread.value,
        cursor: cursor,
      )).data;
      if (page == null) return;
      final runs = [
        for (final item in page.items)
          runFromJson(
            (api.serializers.serializeWith(wire.RunSummary.serializer, item)!
                    as Map<Object?, Object?>)
                .cast<String, Object?>(),
          ),
      ];
      final held = await db.transaction(() async {
        final stored = await db.threadsDao.byId(thread);
        if (stored == null ||
            await db.structureDao.channelById(stored.channelId) == null) {
          return false;
        }
        for (final run in runs) {
          await db.runsDao.upsertIfNewer(run);
        }
        return true;
      });
      if (!held) return;
      cursor = page.nextCursor;
    } while (cursor != null);
  }
}

/// The loader without an engine (before the login): loads nothing.
final class NoThreadRunsLoader implements ThreadRunsLoader {
  /// Creates the loader.
  const NoThreadRunsLoader();

  @override
  Future<void> load(ThreadId thread) async {}
}
