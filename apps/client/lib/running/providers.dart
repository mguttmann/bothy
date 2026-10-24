// SPDX-License-Identifier: Apache-2.0

/// The providers of "Running now" (docs/spec/01-product.md 5.1 D10; brief M4-23 Design note 4):
/// the active runs of the local `runs` table (kept by `run.state_changed`, 04 11.3 item 1),
/// refreshed once from `listActiveRuns` when the view opens, the filters, the rows with their
/// path and the ticking clock of the durations.
library;

import 'package:api_client/api_client.dart' as rest;
import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:sync_engine/sync_engine.dart';

import '../app/providers/ports.dart';
import '../app/providers/session.dart';
import '../sidebar/providers.dart';
import '../sync/message_ops_app.dart';
import '../thread/providers.dart';

part 'providers.g.dart';

/// The filters of D10: one space, one agent and "mine only"; null means all.
final class RunningFilterState {
  /// Creates the filter state.
  const RunningFilterState({this.spaceId, this.agentId, this.mineOnly = false});

  /// Only runs in channels of this space.
  final SpaceId? spaceId;

  /// Only runs of this agent.
  final PrincipalId? agentId;

  /// Only runs the person triggered.
  final bool mineOnly;
}

/// One row of D10: the run with the names of its agent and path and whether Stop shows.
final class RunningRow {
  /// Creates the row.
  const RunningRow({
    required this.run,
    required this.agentName,
    required this.path,
    required this.spaceId,
    required this.canStop,
  });

  /// The run summary.
  final RunSummary run;

  /// The display name of the agent, or its ID while the principal is not stored.
  final String agentName;

  /// Space, channel and thread title, the parts that are stored (the context of the run).
  final List<String> path;

  /// The space of the run's channel, or null for a direct message.
  final SpaceId? spaceId;

  /// Whether the server's answers allow `run.cancel` here (01 D10).
  final bool canStop;
}

/// Whether Stop shows for [run] (01 D10; 05 `run.cancel`, R-AZ-07).
///
/// No own derivation from grants (15 1 principle 1): the client reads only what the server
/// reported for the channel ([access], the `ChannelView` of the snapshot and of
/// `access.changed`) and the run's `triggered_by`. Stop needs the level `trigger` and either the
/// person started the run or holds the capability `manage`; the server checks again and a
/// refusal shows as a failed write.
bool mayStop(RunSummary run, AccessView? access, PrincipalId? me) {
  if (access == null || access.level != AccessLevel.trigger) return false;
  return run.triggeredBy == me ||
      access.capabilities.contains(Capability.manage);
}

/// Builds the rows of [runs] from the stored spaces, channels, threads, principals and the
/// channel access in [db]; [me] is the signed-in person.
Future<List<RunningRow>> buildRunningRows(
  LocalDatabase db,
  List<RunSummary> runs,
  PrincipalId? me,
) async {
  final rows = <RunningRow>[];
  for (final run in runs) {
    final channel = await db.structureDao.channelById(run.channelId);
    final space = channel?.spaceId == null
        ? null
        : await db.structureDao.spaceById(channel!.spaceId!);
    final thread = await db.threadsDao.byId(run.threadId);
    final agent = await db.principalsDao.byId(run.agentId);
    final access = await db.structureDao
        .watchChannelAccess(run.channelId)
        .first;
    rows.add(
      RunningRow(
        run: run,
        agentName: agent?.displayName ?? run.agentId.value,
        path: [
          ?space?.name,
          ?channel?.name,
          if (thread != null && thread.title.isNotEmpty) thread.title,
        ],
        spaceId: channel?.spaceId,
        canStop: mayStop(run, access, me),
      ),
    );
  }
  return rows;
}

/// The filter state of D10.
@riverpod
class RunningFilter extends _$RunningFilter {
  @override
  RunningFilterState build() => const RunningFilterState();

  /// Shows only the runs of [spaceId], or every space for null.
  void setSpace(SpaceId? spaceId) => state = RunningFilterState(
    spaceId: spaceId,
    agentId: state.agentId,
    mineOnly: state.mineOnly,
  );

  /// Shows only the runs of [agentId], or every agent for null.
  void setAgent(PrincipalId? agentId) => state = RunningFilterState(
    spaceId: state.spaceId,
    agentId: agentId,
    mineOnly: state.mineOnly,
  );

  /// Shows only the person's own runs when [mineOnly] is true.
  void setMineOnly(bool mineOnly) => state = RunningFilterState(
    spaceId: state.spaceId,
    agentId: state.agentId,
    mineOnly: mineOnly,
  );
}

/// Loads every page of `listActiveRuns` into the local `runs` table by the version rule when the
/// view opens (Design note 4); returns whether the server answered. A failure leaves the table
/// as it is: the view then shows what the device knows with a note.
@riverpod
Future<bool> runningRefresh(Ref ref) async {
  final services = ref.watch(signedInServicesProvider);
  if (services == null) return false;
  final api = services.auth.api;
  final runs = api.getRunsApi();
  try {
    String? cursor;
    do {
      final page = (await runs.listActiveRuns(cursor: cursor)).data;
      if (page == null) return false;
      for (final item in page.items) {
        final json = api.serializers.serializeWith(
          rest.RunSummary.serializer,
          item,
        );
        await services.database.runsDao.upsertIfNewer(
          runFromJson((json! as Map<Object?, Object?>).cast<String, Object?>()),
        );
      }
      cursor = page.nextCursor;
    } while (cursor != null);
    return true;
  } on Object {
    return false;
  }
}

/// Every active run of the local table as a row, in queue order.
@riverpod
Stream<List<RunningRow>> runningRows(Ref ref) async* {
  final db = ref.watch(localDatabaseProvider);
  if (db == null) {
    yield const [];
    return;
  }
  final me = ref.watch(myPrincipalIdProvider);
  yield* db.runsDao.watchActive().asyncMap(
    (runs) => buildRunningRows(db, runs, me),
  );
}

/// The rows after the filters of [RunningFilter].
@riverpod
List<RunningRow> filteredRunningRows(Ref ref) {
  final rows = ref.watch(runningRowsProvider).value ?? const [];
  final filter = ref.watch(runningFilterProvider);
  final me = ref.watch(myPrincipalIdProvider);
  return [
    for (final row in rows)
      if ((filter.spaceId == null || row.spaceId == filter.spaceId) &&
          (filter.agentId == null || row.run.agentId == filter.agentId) &&
          (!filter.mineOnly || row.run.triggeredBy == me))
        row,
  ];
}

/// The number of active runs in the local table: the counter of the sidebar entry.
@riverpod
int activeRunCount(Ref ref) =>
    (ref.watch(activeRunsProvider).value ?? const []).length;

/// The time the durations count to: the app clock now and then once a second, so a row's
/// duration counts up locally without a patch (01 6.2).
@riverpod
Stream<DateTime> runningClock(Ref ref) async* {
  final clock = ref.watch(appPortsProvider).clock;
  yield clock.now();
  yield* Stream<void>.periodic(const Duration(seconds: 1))
      .map((_) => clock.now());
}

/// The used share of the context window of [runId] in percent, while the run store holds its
/// document (the run is focused elsewhere, 15 7.4); null otherwise. D10 never focuses runs
/// itself: one subscription per active run would make the server send every patch.
@riverpod
Stream<double?> runContext(Ref ref, String runId) {
  final engine = ref.watch(syncEngineProvider);
  if (engine == null) return Stream.value(null);
  return engine.runs
      .watch(RunId.parse(runId))
      .map((doc) => doc?.metrics.contextPct);
}

/// Stops [run] through the outbox (`cancelRun`, 15 1 principle 3); the run's lane keeps a
/// second Stop behind the first.
Future<void> stopRun(SyncEngine engine, RunSummary run) =>
    engine.outbox.enqueue(
      OutboxRequest(
        kind: AppMessageOps.cancelRun,
        payload: {'run_id': run.id.value},
      ),
    );
