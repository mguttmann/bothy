// SPDX-License-Identifier: Apache-2.0

/// `SyncEngine`: the one composition root of the sync engine that the apps use (brief M4-17
/// Design note 2; 15 7.1).
library;

import 'dart:async';
import 'dart:math';

import 'package:api_client/api_client.dart' as api_model show Message;
import 'package:api_client/api_client.dart'
    show
        ApiClient,
        GatewayClientInfo,
        GatewayReady,
        GatewayResumed,
        Run,
        standardSerializers;
import 'package:auth/auth.dart';
import 'package:built_value/serializer.dart';
import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';

import 'catch_up.dart';
import 'connection.dart';
import 'connection_state.dart';
import 'ephemeral_store.dart';
import 'fetcher.dart';
import 'focus_manager.dart';
import 'frames.dart';
import 'host_events.dart';
import 'notifications.dart';
import 'outbox/executors.dart';
import 'outbox/operations/message_ops.dart';
import 'outbox/operations/personal_ops.dart';
import 'outbox/outbox.dart';
import 'ports.dart';
import 'runs/ports.dart';
import 'runs/run_store.dart';
import 'session_listener.dart';
import 'stats/stats_subscription.dart';
import 'status_events.dart';
import 'sync_events.dart';
import 'typing_throttle.dart';

/// The run states that end a run (04 11.3 item 6).
const Set<String> _endStates = {'completed', 'failed', 'cancelled'};

/// The sync engine of one signed-in device: gateway connection, catch-up and dispatcher,
/// outbox and run store, wired as 04 13 and 15 7 say.
///
/// [create] builds the parts of M4-13 to M4-16 and connects their hooks:
/// - the echo `message.created` of the dispatcher confirms the outbox entry
///   ([Outbox.confirmByEcho], 04 13.2 item 6);
/// - the outbox starts when the catch-up reports live (`catchup.complete`, or `ready`
///   without a cursor, 04 13.4) and pauses on every disconnect;
/// - `run.updated` goes to [RunStore.onRunUpdated], a `run.state_changed` to an end state to
///   [RunStore.onRunEnded] (04 11.3);
/// - a focused run puts its thread into the focus of the gateway ([EngineFocus.focusRun]).
///
/// The apps never wire these parts themselves (docs/DECISIONS.md D-M4-17-1).
///
/// Example:
/// ```dart
/// final engine = SyncEngine.create(
///   baseUrl: Uri.parse('https://chat.example.com'),
///   auth: authSession,
///   store: db,
///   transport: IoGatewayTransport(),
///   network: connectivity,
///   lifecycle: lifecycle,
///   ticker: schedulerTicker,
///   config: appConfig,
///   client: GatewayClientInfo((b) => b
///     ..platform = DevicePlatform.macos
///     ..version = '1.0.0'),
/// );
/// engine.start();
/// await engine.outbox.enqueue(OutboxRequest(
///   kind: MessageOps.createMessage,
///   threadId: thread,
///   payload: {'content_md': 'Hello Sam'},
/// ));
/// ```
final class SyncEngine {
  SyncEngine._({
    required this.api,
    required this.connection,
    required this.catchUp,
    required this.outbox,
    required this.runs,
    required this.focus,
    required this._resyncs,
  });

  final StreamSubscription<SyncNotice> _resyncs;

  /// Builds the engine on the signed-in [auth] session.
  ///
  /// [baseUrl] is the server origin; the gateway is `wss://<host>/api/v1/gateway` (04 6.1).
  /// Only `https` is accepted: DPoP and the device binding assume a protected transport
  /// (15 8); [allowPlainHttpForTests] admits `http` (gateway `ws`) for the fake server of
  /// `package:sync_engine/testing.dart` and the app's test entry point, never a production
  /// build (D-M4-17-6). The REST client is the one of [auth] (`AuthSession.api`, built with
  /// its interceptors). [store] is the local database; [ticker] paces the run documents;
  /// [config] the client values of D-W-115; [client] names platform and version for
  /// `identify`. [clock] defaults to the system clock, [random] to a secure source, [ids] to
  /// UUIDv7 on [clock] and [random]. [keepConnectedInBackground] is the desktop behaviour of
  /// 15 7.2. [logHook] receives one line per notable step, never content.
  /// [registerExecutors] registers the app's own writes on the executor registry next to
  /// [MessageOps] and [PersonalOps], with the session's REST client (brief M4-19: the sidebar
  /// and forum writes of `apps/client/lib/sync/`); a kind it registers replaces a built-in
  /// one. [connectGate] is asked before every connection attempt (the app's version gate,
  /// 18 9 item 2; see [ConnectGate]).
  ///
  /// Throws an [ArgumentError] for a [baseUrl] that is not an absolute `https` origin (or
  /// `http` with [allowPlainHttpForTests]).
  static SyncEngine create({
    required Uri baseUrl,
    required AuthSession auth,
    required LocalDatabase store,
    required GatewayTransport transport,
    required NetworkStatus network,
    required AppLifecycle lifecycle,
    required FrameTicker ticker,
    required AppConfig config,
    required GatewayClientInfo client,
    Clock? clock,
    RandomSource? random,
    FileCacheEraser? eraser,
    Uuid7Generator? ids,
    DraftEditing? draftEditing,
    bool keepConnectedInBackground = false,
    bool allowPlainHttpForTests = false,
    void Function(String line)? logHook,
    void Function(OutboxExecutors executors, ApiClient api)? registerExecutors,
    ConnectGate? connectGate,
  }) {
    final gatewayUrl = gatewayUrlOf(
      baseUrl,
      allowPlainHttp: allowPlainHttpForTests,
    );
    final time = clock ?? const SystemClock();
    final rnd = random ?? SystemRandomSource();
    final api = auth.api;
    final executors = OutboxExecutors();
    MessageOps.register(executors, api);
    PersonalOps.register(executors, api);
    registerExecutors?.call(executors, api);
    late final CatchUpListener catchUp;
    final outbox = Outbox(
      db: store,
      executors: executors,
      config: config,
      // 04 13.2 item 6: the server message replaces the entry at once, not only at its echo.
      onConfirmed: (entry, response) async {
        final body = response.body;
        if (body is! api_model.Message) return;
        final json = standardSerializers.serialize(
          body,
          specifiedType: const FullType(api_model.Message),
        );
        if (json is! Map) return;
        await catchUp.storeConfirmedMessage(
          json.cast<String, Object?>(),
          created: entry.kind == MessageOps.createMessage,
        );
      },
      ids:
          ids ??
          Uuid7Generator(
            nowMillis: () => time.now().millisecondsSinceEpoch,
            random: _secureBits(Random.secure()),
          ),
    );
    final runs = RunStore(fetch: _ApiRunFetcher(api), ticker: ticker);
    late final EngineFocus focus;
    catchUp = CatchUpListener(
      db: store,
      fetcher: ApiFetcher(api),
      config: config,
      clock: time,
      identity: AuthIdentity(auth),
      eraser: eraser,
      draftEditing: draftEditing,
      openThreads: () => focus.threads,
      onEcho: (clientMsgId, message) => unawaited(
        outbox.confirmByEcho(
          clientMsgId.value,
          result: {
            'message_id': message.id.value,
            'version': message.version.value,
          },
        ),
      ),
      onRunUpdated: runs.onRunUpdated,
      onLive: () => unawaited(outbox.start()),
      logHook: logHook,
      random: rnd,
    );
    final connection = GatewayConnection(
      transport: transport,
      gatewayUrl: gatewayUrl,
      auth: auth,
      syncState: store.syncStateDao,
      config: config,
      clock: time,
      random: rnd,
      network: network,
      lifecycle: lifecycle,
      client: client,
      keepConnectedInBackground: keepConnectedInBackground,
      listener: _EngineListener(catchUp, outbox, runs),
      logHook: logHook,
      connectGate: connectGate,
    );
    catchUp.watchConnection(connection.states);
    focus = EngineFocus._(connection.focus, runs, catchUp);
    // 04 7.3: a resync skips the events of the gap, so a focused run that ended there gets no
    // run.state_changed; the new snapshot's runs decide (15 7.4).
    final resyncs = catchUp.notices.listen((n) {
      if (n is Resynced) unawaited(focus._dropEndedRuns(store));
    });
    return SyncEngine._(
      api: api,
      connection: connection,
      catchUp: catchUp,
      outbox: outbox,
      runs: runs,
      focus: focus,
      resyncs: resyncs,
    );
  }

  /// The gateway address of [baseUrl]: `wss` for `https`, `ws` for `http` when
  /// [allowPlainHttp] (tests only), path `/api/v1/gateway`.
  ///
  /// Throws an [ArgumentError] for any other scheme, a URL without host, or `http` without
  /// [allowPlainHttp].
  static Uri gatewayUrlOf(Uri baseUrl, {bool allowPlainHttp = false}) {
    final scheme = baseUrl.scheme.toLowerCase();
    final plain = scheme == 'http';
    if (!baseUrl.isAbsolute ||
        baseUrl.host.isEmpty ||
        !(scheme == 'https' || (plain && allowPlainHttp))) {
      throw ArgumentError.value(
        '${baseUrl.scheme}://${baseUrl.host}',
        'baseUrl',
        plain
            ? 'plain http is accepted only by test builds'
            : 'must be an absolute https origin',
      );
    }
    return baseUrl.replace(
      scheme: plain ? 'ws' : 'wss',
      path: '/api/v1/gateway',
      query: null,
      fragment: null,
    );
  }

  /// The REST client of the session (`AuthSession.api`).
  final ApiClient api;

  /// The gateway connection (M4-13).
  final GatewayConnection connection;

  /// Catch-up, dispatcher and reconciliation (M4-14).
  final CatchUpListener catchUp;

  /// The one write path of the app (M4-15).
  final Outbox outbox;

  /// The run documents of the live box (M4-16).
  final RunStore runs;

  /// The focus of visible threads and runs.
  final EngineFocus focus;

  /// The connection state now (15 7.2).
  ConnectionState get state => connection.state;

  /// Every change of [state].
  Stream<ConnectionState> get states => connection.states;

  /// Typing and agent presence.
  EphemeralStore get ephemeral => catchUp.ephemeral;

  /// Notices to the app: removed targets, an overwritten draft, a resync, a sign-out.
  Stream<SyncNotice> get notices => catchUp.notices;

  /// The hints the server sends over the gateway instead of a push (`notification.created`,
  /// 13 5.3 and 6.3). The server decides who is notified, with which kind and preview
  /// (R-PN-01); the app only shows them, for example as a desktop notification. Nothing is
  /// stored; a malformed hint is logged without content and skipped.
  ///
  /// Example:
  /// ```dart
  /// engine.notifications.listen((n) => tray.show(n.threadTitle, n.excerpt));
  /// ```
  Stream<NotificationCreated> get notifications => catchUp.notifications;

  /// The changes of hosts and of the person's terminal sessions (brief M4-36; 15 23.1): the
  /// `HostEvents` port of the terminal view listens here. The stream of a terminal never
  /// runs through the engine (15 8).
  ///
  /// Example:
  /// ```dart
  /// engine.hostEvents.listen((e) => reloadHosts());
  /// ```
  Stream<HostEvent> get hostEvents => catchUp.hostEvents;

  /// The changes of the status page (brief M7-21): the status view of the app reads
  /// `getStatus` again on each one.
  ///
  /// Example:
  /// ```dart
  /// engine.statusUpdates.listen((_) => reloadStatus());
  /// ```
  Stream<StatusUpdated> get statusUpdates => catchUp.statusUpdates;

  /// The typing throttle of the connection.
  TypingThrottle get typing => connection.typing;

  /// The `stats.live` subscription of the connection (25 5.1; brief M6-16): the live frames
  /// of the visualizations arrive on [StatsSubscription.updates] and are never stored.
  StatsSubscription get stats => connection.stats;

  /// Connects (when online and, on mobile, in the foreground); a second call does nothing.
  void start() => connection.start();

  /// Stops the connection and closes every part; the stored outbox entries stay.
  Future<void> dispose() async {
    await _resyncs.cancel();
    await connection.dispose();
    await outbox.close();
    await catchUp.dispose();
    await runs.dispose();
  }
}

/// The focus of the app: the visible threads and the threads of focused runs together form
/// the `focus` of the gateway (04 6.2, 04 11.3 item 2); the threads of background runs (a run
/// graph, the control room cards) go into `background_thread_ids` within the same limit
/// (R-SY-27; brief M4-39 Design note 4).
///
/// Example:
/// ```dart
/// engine.focus.setVisible({thread}, idle: false);
/// engine.focus.focusRun(run, thread); // the run document follows its patches
/// final live = engine.focus.setBackgroundRuns([(run: r, thread: t)]); // the cards on screen
/// ```
final class EngineFocus {
  EngineFocus._(this._manager, this._runs, this._catchUp);

  final FocusManager _manager;
  final RunStore _runs;
  final CatchUpListener _catchUp;
  Set<ThreadId> _visible = const {};
  bool _idle = false;
  final Map<RunId, ThreadId> _focusedRuns = {};
  final Map<RunId, ThreadId> _backgroundRuns = {};
  List<({RunId run, ThreadId thread})> _cards = const [];
  Set<RunId> _liveCards = const {};
  final Set<RunId> _inStore = {};

  /// The threads sent as focus: the visible ones and those of focused runs.
  Set<ThreadId> get threads => {..._visible, ..._focusedRuns.values};

  /// The focus the gateway confirmed last (sent and not rejected).
  Set<ThreadId> get current => _manager.current;

  /// The `idle` flag given to [setVisible] last ("open but idle", 13 4.4); false before the
  /// first call.
  bool get idle => _idle;

  /// The card runs of [setBackgroundRuns] whose thread is live now.
  Set<RunId> get liveBackgroundRuns => Set.unmodifiable(_liveCards);

  /// Sets the threads on screen; [idle] reports "open but idle" (13 4.4). Returns the threads
  /// that did not fit into `sync.max_focused_threads`. A recorded gap of a visible thread is
  /// loaded again.
  Set<ThreadId> setVisible(Set<ThreadId> visible, {required bool idle}) {
    final added = visible.difference(_visible);
    _visible = Set.of(visible);
    _idle = idle;
    for (final t in added) {
      unawaited(_catchUp.onFocus(t));
    }
    return _push();
  }

  /// Focuses [run] of [thread]: the run store fetches its document and the thread joins the
  /// gateway focus, so `run.updated` reaches it (04 11.3 items 2 and 5).
  ///
  /// With [background] the thread goes into `background_thread_ids` instead (a run graph shows
  /// the run without the thread's messages, 04 6.2): it gets the document and its patches, no
  /// typing, and makes nobody "Active in the thread"; it comes before the control room cards.
  void focusRun(RunId run, ThreadId thread, {bool background = false}) {
    if (background) {
      _backgroundRuns[run] = thread;
    } else {
      _focusedRuns[run] = thread;
    }
    _push();
    // A focus call fetches again a run whose last fetch failed (RunStore.focus).
    _runs.focus(run);
  }

  /// Drops the focus of [run] that [focusRun] set with the same [background]; its thread
  /// leaves the gateway focus unless it is visible or holds another focused run. The other
  /// kind of focus of the same run stays (the live box and a run graph of one run may hand
  /// over to each other in either order).
  void unfocusRun(RunId run, {bool background = false}) {
    if (background) {
      _backgroundRuns.remove(run);
    } else {
      _focusedRuns.remove(run);
    }
    _push();
  }

  /// Sets the control room cards on screen, in on-screen order (04 6.2, 09 5.6): their threads
  /// go into `background_thread_ids` as far as `sync.max_focused_threads` leaves room after
  /// the visible threads and the background runs of [focusRun]. Returns the runs whose thread
  /// is live (it got a place, or it is focused anyway); the run store holds their documents.
  /// The other cards show their summary. `idle` stays as [setVisible] set it.
  Set<RunId> setBackgroundRuns(List<({RunId run, ThreadId thread})> cards) {
    _cards = List.unmodifiable(cards);
    _push();
    return liveBackgroundRuns;
  }

  Set<ThreadId> _push() {
    final background = <ThreadId>[
      ..._backgroundRuns.values,
      for (final c in _cards) c.thread,
    ];
    final dropped = _manager.setVisible(
      threads,
      idle: _idle,
      background: background,
    );
    final granted = _manager.grantedBackground;
    final focused = threads.difference(dropped);
    _liveCards = {
      for (final c in _cards)
        if (granted.contains(c.thread) || focused.contains(c.thread)) c.run,
    };
    final wanted = {
      ..._focusedRuns.keys,
      ..._backgroundRuns.keys,
      ..._liveCards,
    };
    for (final run in _inStore.difference(wanted).toList()) {
      _inStore.remove(run);
      _runs.unfocus(run);
    }
    for (final run in wanted) {
      // Only a run new to the store fetches here; an ended run keeps its end (onRunEnded).
      if (_inStore.add(run)) _runs.focus(run);
    }
    return dropped;
  }

  /// Ends the documents of focused runs that the local run summaries list as ended or no longer
  /// hold (after a resync replaced the mirror).
  Future<void> _dropEndedRuns(LocalDatabase db) async {
    for (final run in List.of(_inStore)) {
      final summary = await db.runsDao.byId(run);
      if (summary == null || _endStates.contains(summary.state.code)) {
        _runs.onRunEnded(run);
      }
    }
  }
}

/// Wraps the catch-up listener: pauses the outbox on every disconnect and ends the run
/// documents of runs that ended.
final class _EngineListener implements SessionListener {
  _EngineListener(this._inner, this._outbox, this._runs);

  final CatchUpListener _inner;
  final Outbox _outbox;
  final RunStore _runs;

  @override
  Future<void> onReady(GatewayReady ready, SessionControl control) =>
      _inner.onReady(ready, control);

  @override
  Future<void> onResumed(GatewayResumed resumed, SessionControl control) =>
      _inner.onResumed(resumed, control);

  @override
  Future<void> onFrame(GatewayFrame frame, SessionControl control) async {
    await _inner.onFrame(frame, control);
    if (frame is! DispatchFrame ||
        frame.envelope['type'] != 'run.state_changed') {
      return;
    }
    final payload = frame.envelope['payload'];
    final run = payload is Map ? payload['run'] : null;
    if (run is! Map || !_endStates.contains(run['state'])) return;
    final id = run['id'];
    if (id is! String) return;
    try {
      _runs.onRunEnded(RunId.parse(id));
    } on FormatException {
      // A malformed ID belongs to no focused run.
    }
  }

  @override
  void onDisconnected() {
    _inner.onDisconnected();
    // 04 13.4: nothing is sent between a disconnect and the next catchup.complete.
    unawaited(_outbox.pause());
  }
}

/// Uniform random values of up to 62 bits from [random], 30 bits at a time.
int Function(int bits) _secureBits(Random random) => (bits) {
  var value = 0;
  for (var left = bits; left > 0; left -= 30) {
    final take = left > 30 ? 30 : left;
    value = (value << take) | random.nextInt(1 << take);
  }
  return value;
};

/// [RunDocumentFetcher] on `getRun` (04 11.3 item 4).
final class _ApiRunFetcher implements RunDocumentFetcher {
  _ApiRunFetcher(this._api);

  final ApiClient _api;

  @override
  Future<Map<String, Object?>> fetchRun(RunId id) async {
    final run = (await _api.getRunsApi().getRun(runId: id.value)).data;
    if (run == null) throw const FormatException('an empty run');
    final json = standardSerializers.serialize(
      run,
      specifiedType: const FullType(Run),
    );
    if (json is! Map) {
      throw const FormatException('a run that is not an object');
    }
    return json.cast<String, Object?>();
  }
}
