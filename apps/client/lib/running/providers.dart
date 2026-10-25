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

import '../agents/providers.dart';
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
    final stalls = <RunId, DateTime>{};
    do {
      final page = (await runs.listActiveRuns(cursor: cursor)).data;
      if (page == null) return false;
      for (final item in page.items) {
        final stall = item.stallWarnedAt;
        if (stall != null) stalls[RunId.parse(item.id)] = stall;
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
    ref.read(activeRunStallsProvider.notifier).replace(stalls);
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

/// The two forms of "Running now" (01 D10): the control room cards (the default) and the rows.
enum RunningForm {
  /// The control room cards.
  cards,

  /// The rows of M4-23, also the form for screen readers.
  list,
}

/// The form of "Running now" the person chose; cards by default (D-RV-7). It lasts while the
/// app runs.
@Riverpod(keepAlive: true)
class RunningFormChoice extends _$RunningFormChoice {
  @override
  RunningForm build() => RunningForm.cards;

  /// Shows [form].
  void set(RunningForm form) => state = form;
}

/// `stall_warned_at` of the active runs as of the last `listActiveRuns` load (09 5.6): the
/// stall notice of a card without live details. The marking changes no state, so no
/// `run.state_changed` carries it and the local table keeps none.
@Riverpod(keepAlive: true)
class ActiveRunStalls extends _$ActiveRunStalls {
  @override
  Map<RunId, DateTime> build() => const {};

  /// Replaces the marks with those of a complete load.
  void replace(Map<RunId, DateTime> stalls) => state = Map.unmodifiable(stalls);
}

/// The cards of the control room on screen and which of them are live (brief M4-39 Design
/// note 4; 04 6.2 `background_thread_ids`, R-SY-27).
///
/// [show] hands the cards in on-screen order to `EngineFocus.setBackgroundRuns`: the threads
/// the app already shows stay in `thread_ids`, the rest of `sync.max_focused_threads` goes to
/// the cards in that order, and `idle` keeps following the window. The state is the set of
/// runs whose card is live; the others show "live details paused". Leaving the view frees the
/// places.
@riverpod
class ControlRoomFocus extends _$ControlRoomFocus {
  List<({RunId run, ThreadId thread})> _cards = const [];

  @override
  Set<RunId> build() {
    final engine = ref.watch(syncEngineProvider);
    ref.onDispose(() => engine?.focus.setBackgroundRuns(const []));
    // A new engine (a new session) gets the cards on screen again, so they do not stay
    // paused until the grid measures anew.
    return engine?.focus.setBackgroundRuns(_cards) ?? const <RunId>{};
  }

  /// The cards [cards] are on screen, in on-screen order.
  void show(List<({RunId run, ThreadId thread})> cards) {
    final engine = ref.read(syncEngineProvider);
    final same =
        cards.length == _cards.length &&
        [
          for (var i = 0; i < cards.length; i++)
            cards[i].run == _cards[i].run &&
                cards[i].thread == _cards[i].thread,
        ].every((x) => x);
    if (same && engine != null) {
      final live = engine.focus.liveBackgroundRuns;
      if (!_equal(live, state)) state = live;
      return;
    }
    _cards = List.unmodifiable(cards);
    final live = engine?.focus.setBackgroundRuns(_cards) ?? const <RunId>{};
    if (!_equal(live, state)) state = live;
  }

  static bool _equal(Set<RunId> a, Set<RunId> b) =>
      a.length == b.length && a.containsAll(b);
}

/// The run document of a live card from the run store (the control room focused it through
/// [ControlRoomFocus]); null while it loads or when the card is not live.
@riverpod
Stream<RunDocument?> cardDocument(Ref ref, String runId) {
  final engine = ref.watch(syncEngineProvider);
  return engine == null
      ? Stream.value(null)
      : engine.runs.watch(RunId.parse(runId));
}

/// The document of the run graph [runId] of [threadId]: the run is focused with its thread in
/// `background_thread_ids` while the graph shows (04 6.2; the graph shows no messages).
@riverpod
Stream<RunDocument?> runGraphDocument(Ref ref, String runId, String threadId) {
  final source = ref.watch(runSourceProvider);
  return source == null
      ? Stream.value(null)
      : source.watch(
          RunId.parse(runId),
          ThreadId.parse(threadId),
          background: true,
        );
}
