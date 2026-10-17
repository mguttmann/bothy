// SPDX-License-Identifier: Apache-2.0

/// `SyncEngine`: the one composition root of the sync engine that the apps use (brief M4-17
/// Design note 2; 15 7.1).
library;

import 'dart:async';
import 'dart:math';

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
import 'outbox/executors.dart';
import 'outbox/operations/message_ops.dart';
import 'outbox/operations/personal_ops.dart';
import 'outbox/outbox.dart';
import 'ports.dart';
import 'runs/ports.dart';
import 'runs/run_store.dart';
import 'session_listener.dart';
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
  });

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
    final outbox = Outbox(
      db: store,
      executors: executors,
      config: config,
      ids:
          ids ??
          Uuid7Generator(
            nowMillis: () => time.now().millisecondsSinceEpoch,
            random: _secureBits(Random.secure()),
          ),
    );
    final runs = RunStore(fetch: _ApiRunFetcher(api), ticker: ticker);
    late final EngineFocus focus;
    final catchUp = CatchUpListener(
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
    );
    catchUp.watchConnection(connection.states);
    focus = EngineFocus._(connection.focus, runs, catchUp);
    return SyncEngine._(
      api: api,
      connection: connection,
      catchUp: catchUp,
      outbox: outbox,
      runs: runs,
      focus: focus,
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

  /// The typing throttle of the connection.
  TypingThrottle get typing => connection.typing;

  /// Connects (when online and, on mobile, in the foreground); a second call does nothing.
  void start() => connection.start();

  /// Stops the connection and closes every part; the stored outbox entries stay.
  Future<void> dispose() async {
    await connection.dispose();
    await outbox.close();
    await catchUp.dispose();
    await runs.dispose();
  }
}

/// The focus of the app: the visible threads and the threads of focused runs together form
/// the `focus` of the gateway (04 6.2, 04 11.3 item 2).
///
/// Example:
/// ```dart
/// engine.focus.setVisible({thread}, idle: false);
/// engine.focus.focusRun(run, thread); // the run document follows its patches
/// ```
final class EngineFocus {
  EngineFocus._(this._manager, this._runs, this._catchUp);

  final FocusManager _manager;
  final RunStore _runs;
  final CatchUpListener _catchUp;
  Set<ThreadId> _visible = const {};
  bool _idle = false;
  final Map<RunId, ThreadId> _focusedRuns = {};

  /// The threads sent as focus: the visible ones and those of focused runs.
  Set<ThreadId> get threads => {..._visible, ..._focusedRuns.values};

  /// The focus the gateway confirmed last (sent and not rejected).
  Set<ThreadId> get current => _manager.current;

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
  void focusRun(RunId run, ThreadId thread) {
    _focusedRuns[run] = thread;
    _runs.focus(run);
    _push();
  }

  /// Drops the focus of [run]; its thread leaves the gateway focus unless it is visible or
  /// holds another focused run.
  void unfocusRun(RunId run) {
    _focusedRuns.remove(run);
    _runs.unfocus(run);
    _push();
  }

  Set<ThreadId> _push() => _manager.setVisible(threads, idle: _idle);
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
