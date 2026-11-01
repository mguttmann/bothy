// SPDX-License-Identifier: Apache-2.0

// Fakes of the connection tests (brief M4-13, Tests): a fake transport that records frames and
// closes with any code and reason, a fake clock with `elapse`, a seeded random source, fake
// network status and lifecycle, an auth session with a software P-256 signer on a fake API
// (the pattern of packages/auth/test/support) and an in-memory local store. No test opens a
// socket.

import 'dart:async';
import 'dart:collection';
import 'dart:convert';
import 'dart:math';
import 'dart:typed_data';

import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart';
import 'package:dio/dio.dart';
import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

import 'software_p256_signer.dart';

/// The server address of every test.
final Uri serverUrl = Uri.parse('https://chat.example.com');

/// The gateway address of every test (04 6.1).
final Uri gatewayUrl = Uri.parse('wss://chat.example.com/api/v1/gateway');

/// The heartbeat interval the fake server announces in `hello`.
const Duration heartbeatInterval = Duration(seconds: 10);

/// The resume window the fake server announces in `hello`.
const Duration resumeWindow = Duration(seconds: 60);

/// Lets pending microtasks and zero timers run.
Future<void> settle() async {
  for (var i = 0; i < 40; i++) {
    await Future<void>.delayed(Duration.zero);
  }
}

// ---------------------------------------------------------------------------------------------
// Clock and random

/// A clock whose waits complete only when the test moves time with [elapse].
class FakeClock implements Clock {
  /// Starts at [start].
  FakeClock(this._now);

  DateTime _now;
  final List<({DateTime at, int order, Completer<void> done})> _waits = [];
  int _order = 0;

  /// Every wait asked for, in order.
  final List<Duration> waits = [];

  @override
  DateTime now() => _now;

  @override
  Future<void> delay(Duration duration) {
    waits.add(duration);
    final done = Completer<void>();
    _waits.add((at: _now.add(duration), order: _order++, done: done));
    return done.future;
  }

  /// Moves time forward by [d], completing every wait that falls due on the way, in order,
  /// and letting the code that waited run before the next one.
  Future<void> elapse(Duration d) async {
    final target = _now.add(d);
    await settle();
    while (true) {
      _waits.sort((a, b) {
        final c = a.at.compareTo(b.at);
        return c != 0 ? c : a.order.compareTo(b.order);
      });
      if (_waits.isEmpty || _waits.first.at.isAfter(target)) break;
      final next = _waits.removeAt(0);
      if (next.at.isAfter(_now)) _now = next.at;
      next.done.complete();
      await settle();
    }
    _now = target;
    await settle();
  }
}

/// A seeded [RandomSource], so every run draws the same values.
class SeededRandom implements RandomSource {
  /// Creates the source from [seed].
  SeededRandom(int seed) : _random = Random(seed);

  final Random _random;

  /// Every value drawn, in order.
  final List<double> drawn = [];

  @override
  double nextDouble() {
    final v = _random.nextDouble();
    drawn.add(v);
    return v;
  }
}

// ---------------------------------------------------------------------------------------------
// Transport

/// One fake WebSocket: the test pushes server frames and closes from the server side; every
/// frame the client sends is recorded.
class FakeChannel implements GatewayChannel {
  /// Creates the channel opened at [openedAt].
  FakeChannel(this.openedAt);

  /// When the client opened it.
  final DateTime openedAt;

  final StreamController<String> _in = StreamController<String>();

  /// Every text frame the client sent, in order.
  final List<String> sent = [];

  bool _closed = false;

  /// The close code the client used, when it closed.
  int? clientCloseCode;

  @override
  int? closeCode;

  @override
  String? closeReason;

  /// Whether the socket is closed.
  bool get isClosed => _closed;

  @override
  Stream<String> get frames => _in.stream;

  @override
  void send(String text) {
    if (_closed) throw StateError('send on a closed channel');
    sent.add(text);
  }

  /// When set, a close by the client completes only after this future (a slow close
  /// handshake).
  Future<void>? closeGate;

  @override
  Future<void> close(int code, [String? reason]) async {
    if (_closed) return;
    final gate = closeGate;
    if (gate != null) await gate;
    if (_closed) return;
    _closed = true;
    clientCloseCode = code;
    closeCode = code;
    closeReason = reason;
    unawaited(_in.close());
  }

  /// The sent frames decoded to JSON.
  List<Map<String, Object?>> get sentJson => [
    for (final s in sent) jsonDecode(s) as Map<String, Object?>,
  ];

  /// The ops sent, in order.
  List<String> get sentOps => [for (final f in sentJson) f['op']! as String];

  /// The `d` of the last sent frame with [op].
  Map<String, Object?>? lastData(String op) {
    final all = sentJson.where((f) => f['op'] == op).toList();
    return all.isEmpty ? null : all.last['d'] as Map<String, Object?>?;
  }

  /// Pushes a server frame `{op, d}`.
  void push(String op, Map<String, Object?> d) =>
      _in.add(jsonEncode({'op': op, 'd': d}));

  /// Pushes a raw text frame.
  void pushRaw(String text) => _in.add(text);

  /// Closes from the server side with [code] and [reason].
  void serverClose(int? code, [String? reason]) {
    if (_closed) return;
    _closed = true;
    closeCode = code;
    closeReason = reason;
    unawaited(_in.close());
  }
}

/// A transport that hands out [FakeChannel]s and records when each was opened.
class FakeTransport implements GatewayTransport {
  /// Creates the transport on [clock].
  FakeTransport(this.clock);

  /// The clock that stamps each connection.
  final FakeClock clock;

  /// Every channel opened, in order.
  final List<FakeChannel> channels = [];

  /// Every URL connected to.
  final List<Uri> urls = [];

  /// How many of the next connects fail with a network error.
  int failNext = 0;

  /// Connection attempts, failed ones included.
  int attempts = 0;

  /// How many of the next connects never complete (a stalled TCP connect or upgrade).
  int stallNext = 0;

  /// The newest channel.
  FakeChannel get last => channels.last;

  @override
  Future<GatewayChannel> connect(Uri url) async {
    attempts++;
    urls.add(url);
    if (failNext > 0) {
      failNext--;
      throw const SocketLikeError();
    }
    if (stallNext > 0) {
      stallNext--;
      return Completer<GatewayChannel>().future;
    }
    final ch = FakeChannel(clock.now());
    channels.add(ch);
    return ch;
  }
}

/// The error of a failed fake connect.
class SocketLikeError implements Exception {
  /// Creates the error.
  const SocketLikeError();
}

/// Network status the test switches.
class FakeNetwork implements NetworkStatus {
  final StreamController<bool> _changes = StreamController<bool>.broadcast();

  @override
  bool online = true;

  @override
  Stream<bool> get changes => _changes.stream;

  /// Reports a change to [value] (also the same value: a switch between networks).
  void set(bool value) {
    online = value;
    _changes.add(value);
  }
}

/// App lifecycle the test switches.
class FakeLifecycle implements AppLifecycle {
  /// Starts in [current].
  FakeLifecycle([this.current = AppActivity.foreground]);

  final StreamController<AppActivity> _changes =
      StreamController<AppActivity>.broadcast();

  @override
  AppActivity current;

  @override
  Stream<AppActivity> get changes => _changes.stream;

  /// Reports [value].
  void set(AppActivity value) {
    current = value;
    _changes.add(value);
  }
}

// ---------------------------------------------------------------------------------------------
// Fake API of the auth session

/// One REST request as the fake API saw it.
class SeenRequest {
  /// Records a request.
  SeenRequest(this.method, this.path, [this.query = const {}]);

  /// HTTP method in upper case.
  final String method;

  /// Path below `/api/v1`.
  final String path;

  /// The query parameters.
  final Map<String, String> query;
}

/// A canned answer of the fake API.
class FakeReply {
  /// Creates an answer.
  const FakeReply(this.status, [this.body]);

  /// An RFC 9457 problem with [code].
  factory FakeReply.problem(int status, String code) => FakeReply(status, {
    'type': '/api/v1/problems/$code',
    'title': code,
    'status': status,
    'code': code,
  });

  /// HTTP status.
  final int status;

  /// JSON body or `null`.
  final Object? body;
}

/// A Dio adapter that dispatches to routes by `METHOD path`.
class FakeApi implements HttpClientAdapter {
  /// Every request seen.
  final List<SeenRequest> requests = [];

  /// Routes by `METHOD /path`.
  final Map<String, FakeReply Function(SeenRequest r)> routes = {};

  /// Futures by `METHOD /path` that a request waits for before it is answered.
  final Map<String, Future<void>> gates = {};

  /// The requests to [path].
  int count(String path) => requests.where((r) => r.path == path).length;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    if (requestStream != null) await requestStream.drain<void>();
    final path = options.uri.path.replaceFirst('/api/v1', '');
    final seen = SeenRequest(
      options.method.toUpperCase(),
      path,
      options.uri.queryParameters,
    );
    requests.add(seen);
    final gate = gates['${seen.method} $path'];
    if (gate != null) await gate;
    final route = routes['${seen.method} $path'];
    final r = route == null ? FakeReply.problem(404, 'not_found') : route(seen);
    final problem = r.body is Map && (r.body! as Map)['code'] is String;
    return ResponseBody.fromString(
      r.body == null ? '' : jsonEncode(r.body),
      r.status,
      headers: {
        if (r.body != null)
          Headers.contentTypeHeader: [
            problem ? 'application/problem+json' : 'application/json',
          ],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

/// A secret store in memory.
class MemorySecretStore implements SecretStore {
  /// Stored values.
  final Map<String, String> values = {};

  @override
  Future<String?> read(String key) async => values[key];

  @override
  Future<void> write(String key, String value) async => values[key] = value;

  @override
  Future<void> delete(String key) async => values.remove(key);
}

class _Prompt implements StepUpPrompt {
  @override
  Future<bool> ask(String purposeText) async => true;

  /// These tests confirm with the step-up key; a password prompt cancels.
  @override
  Future<StepUpCredentials?> askPassword(
    String purposeText, {
    bool rejected = false,
  }) async => null;
}

/// IDs of the tests.
const String sessionId = '0192f4a0-7b1c-7000-8000-000000000001';

/// IDs of the tests.
const String userId = '0192f4a0-7b1c-7000-8000-000000000002';

/// The device of the tests.
const String deviceId = '0192f4a0-7b1c-7000-8000-0000000000d1';

/// A JWT-shaped access token whose payload carries [exp] and a [tag].
String accessToken(String tag, DateTime exp) {
  String enc(Map<String, Object?> m) =>
      base64Url.encode(utf8.encode(jsonEncode(m))).replaceAll('=', '');
  return '${enc({'alg': 'EdDSA', 'typ': 'at+jwt'})}.'
      '${enc({'sub': userId, 'sid': sessionId, 'tag': tag, 'exp': exp.millisecondsSinceEpoch ~/ 1000})}.'
      'c2ln';
}

/// The `tag` of an access token built by [accessToken].
String tokenTag(String token) {
  final payload = token.split('.')[1];
  final json =
      jsonDecode(utf8.decode(b64Decode(payload))) as Map<String, Object?>;
  return json['tag']! as String;
}

/// A `TokenResponse` body.
Map<String, Object?> tokenBody({
  required String access,
  required String refresh,
}) => {
  'access_token': access,
  'token_type': 'DPoP',
  'expires_in': 3600,
  'refresh_token': refresh,
  'refresh_expires_at': '2026-11-01T00:00:00.000Z',
  'session': {
    'id': sessionId,
    'user_id': userId,
    'device_id': deviceId,
    'auth_method': 'oidc',
    'created_at': '2026-10-15T10:00:00.000Z',
    'expires_at': '2026-11-01T00:00:00.000Z',
  },
  'device': {
    'id': deviceId,
    'user_id': userId,
    'name': 'Alex laptop',
    'platform': 'macos',
    'state': 'active',
    'created_at': '2026-10-15T10:00:00.000Z',
  },
};

/// A `Device` body in [state].
Map<String, Object?> deviceBody(String state) => {
  'id': deviceId,
  'user_id': userId,
  'name': 'Alex laptop',
  'platform': 'macos',
  'state': state,
  'created_at': '2026-10-15T10:00:00.000Z',
};

/// The decoded payload of a compact JWS.
Map<String, Object?> jwsPayload(String jws) =>
    jsonDecode(utf8.decode(b64Decode(jws.split('.')[1])))
        as Map<String, Object?>;

// ---------------------------------------------------------------------------------------------
// Listener

/// A [SessionListener] that records every call and can set a phase on ready.
class RecordingListener implements SessionListener {
  /// Every call as text, in order.
  final List<String> calls = [];

  /// Frames handed to [onFrame].
  final List<GatewayFrame> frames = [];

  /// The control of the last ready or resumed.
  SessionControl? control;

  @override
  Future<void> onReady(GatewayReady ready, SessionControl control) async {
    this.control = control;
    calls.add('ready ${ready.gatewaySessionId}');
  }

  @override
  Future<void> onResumed(GatewayResumed resumed, SessionControl control) async {
    this.control = control;
    calls.add('resumed ${resumed.gatewaySessionId}');
  }

  @override
  Future<void> onFrame(GatewayFrame frame, SessionControl control) async {
    frames.add(frame);
    calls.add('frame ${frame.op}');
  }

  @override
  void onDisconnected() => calls.add('disconnected');
}

// ---------------------------------------------------------------------------------------------
// Server frames

/// The `d` of a `hello`.
Map<String, Object?> helloData({
  String nonce = 'nonce-1',
  List<int> versions = const [1],
  Duration interval = heartbeatInterval,
  Duration window = resumeWindow,
  String epoch = 'epoch-1',
}) => {
  'heartbeat_interval_ms': interval.inMilliseconds,
  'resume_window_ms': window.inMilliseconds,
  'protocol_versions': versions,
  'dpop_nonce': nonce,
  'server_time': '2026-10-15T10:00:00.000Z',
  'sync_epoch': epoch,
};

/// The `d` of a `ready`.
Map<String, Object?> readyData({
  String gatewaySessionId = 'gs-1',
  int headSeq = 40,
  int permissionVersion = 3,
  String epoch = 'epoch-1',
}) => {
  'gateway_session_id': gatewaySessionId,
  'principal': {
    'id': userId,
    'kind': 'user',
    'handle': 'alex',
    'display_name': 'Alex',
  },
  'device_id': deviceId,
  'permission_version': permissionVersion,
  'head_seq': headSeq,
  'sync_epoch': epoch,
  'snapshot': {
    'head_seq': headSeq,
    'permission_version': permissionVersion,
    'sync_epoch': epoch,
    'principal': {
      'id': userId,
      'kind': 'user',
      'handle': 'alex',
      'display_name': 'Alex',
    },
    'is_owner': false,
    'spaces': <Object?>[],
    'channels': <Object?>[],
    'sidebar': <Object?>[],
    'unread_threads': <Object?>[],
    'active_runs': <Object?>[],
    'agents': <Object?>[],
    'settings': <Object?>[],
    'drafts': <Object?>[],
    'features': <Object?>[],
  },
};

/// The `d` of a `resumed`.
Map<String, Object?> resumedData({
  String gatewaySessionId = 'gs-1',
  int headSeq = 40,
  String epoch = 'epoch-1',
}) => {
  'gateway_session_id': gatewaySessionId,
  'head_seq': headSeq,
  'sync_epoch': epoch,
};

/// A thread ID of the tests.
ThreadId thread(int n) => ThreadId.parse(
  '0192f4a0-7b1c-78f9-8074-${n.toRadixString(16).padLeft(12, '0')}',
);

// ---------------------------------------------------------------------------------------------
// Harness

/// A connection with every fake around it.
class Harness {
  /// Builds the fakes; [overrides] change `AppConfig` values by key.
  Harness({
    Map<String, String> overrides = const {},
    int seed = 1,
    this.keepConnectedInBackground = false,
    AppActivity activity = AppActivity.foreground,
    this._listener,
    this.listenerFor,
    this.connectGate,
  }) : clock = FakeClock(DateTime.utc(2026, 10, 15, 10)),
       random = SeededRandom(seed),
       lifecycle = FakeLifecycle(activity),
       config = AppConfig.defaults().withOverrides(overrides) {
    transport = FakeTransport(clock);
    db = openInMemoryForTests(clock: clock.now);
    var counter = 0;
    auth = AuthSession(
      serverUrl: serverUrl,
      keys: DeviceKeys(dpop: dpopKey, stepUp: SoftwareP256Signer.seeded(2)),
      secrets: secrets,
      clock: clock,
      ids: Uuid7Generator(
        nowMillis: () => clock.now().millisecondsSinceEpoch,
        random: (bits) => (counter++) & ((1 << bits) - 1),
      ),
      config: config,
      device: const DeviceDescription(
        name: 'Alex laptop',
        platform: 'macos',
        appVersion: '1.0.0',
      ),
      buildClient: (interceptors) => createApiClient(
        baseUrl: serverUrl,
        traceContext: () => TraceContext(traceId: '1' * 32, sampled: false),
        readRetry: ReadRetryPolicy(
          base: Duration.zero,
          cap: Duration.zero,
          maxAttempts: 1,
        ),
        authInterceptors: interceptors,
        adapter: api,
      ),
      stepUpPrompt: _Prompt(),
    );
    auth.events.listen(authEvents.add);
    api.routes['POST /auth/token/refresh'] = (_) {
      refreshes++;
      return FakeReply(
        200,
        tokenBody(
          access: accessToken(
            'A${refreshes + 1}',
            clock.now().add(const Duration(hours: 1)),
          ),
          refresh: 'R${refreshes + 1}',
        ),
      );
    };
  }

  /// The clock.
  final FakeClock clock;

  /// The random source of the connection.
  final SeededRandom random;

  /// The lifecycle.
  final FakeLifecycle lifecycle;

  /// The network status.
  final FakeNetwork network = FakeNetwork();

  /// The configuration.
  final AppConfig config;

  /// The fake API of the auth session.
  final FakeApi api = FakeApi();

  /// The secret store.
  final MemorySecretStore secrets = MemorySecretStore();

  /// The DPoP key.
  final SoftwareP256Signer dpopKey = SoftwareP256Signer.seeded(1);

  /// Desktop behaviour.
  final bool keepConnectedInBackground;

  final SessionListener? _listener;

  /// Builds the listener from the harness (its database, clock and configuration), when given.
  final SessionListener Function(Harness h)? listenerFor;

  /// The listener, if any.
  late final SessionListener? listener = listenerFor?.call(this) ?? _listener;

  /// The version gate before every attempt (brief M7-13), if any.
  final ConnectGate? connectGate;

  /// The transport.
  late final FakeTransport transport;

  /// The local database.
  late final LocalDatabase db;

  /// The auth session.
  late final AuthSession auth;

  /// Auth events so far.
  final List<AuthEvent> authEvents = [];

  /// Number of refreshes the fake API answered.
  int refreshes = 0;

  /// States published by the connection.
  final List<ConnectionState> states = [];

  GatewayConnection? _connection;

  /// Lines of the log hook.
  final List<String> log = [];

  /// The connection (built on first use).
  GatewayConnection get connection => _connection ??= _build();

  GatewayConnection _build() {
    final c = GatewayConnection(
      transport: transport,
      gatewayUrl: gatewayUrl,
      auth: auth,
      syncState: db.syncStateDao,
      config: config,
      clock: clock,
      random: random,
      network: network,
      lifecycle: lifecycle,
      client: GatewayClientInfo(
        (b) => b
          ..platform = DevicePlatform.macos
          ..version = '1.0.0',
      ),
      keepConnectedInBackground: keepConnectedInBackground,
      listener: listener,
      logHook: log.add,
      connectGate: connectGate,
    );
    c.states.listen(states.add);
    return c;
  }

  /// The current phase.
  ConnectionPhase get phase => connection.state.phase;

  /// Signs in: a password login on the fake API, so the session holds `A1` and `R1`.
  Future<void> signIn() async {
    api.routes['POST /auth/password/login'] = (_) => FakeReply(
      201,
      tokenBody(
        access: accessToken('A1', clock.now().add(const Duration(hours: 1))),
        refresh: 'R1',
      ),
    );
    final outcome = await auth.login.loginWithPassword(
      loginName: 'alex',
      password: 'pw',
      totpCode: '123456',
    );
    if (outcome is! LoginSignedIn) throw StateError('sign-in failed');
    api.requests.clear();
  }

  /// Stores a cursor and epoch as M4-14 would.
  Future<void> storeCursor(int cursor, {String epoch = 'epoch-1'}) => db
      .syncStateDao
      .commitPage((_) async {}, cursor: GlobalSeq(cursor), syncEpoch: epoch);

  /// Signs in, starts the connection and returns the first channel (before `hello`).
  Future<FakeChannel> open() async {
    if (auth.session == null) await signIn();
    connection.start();
    await settle();
    return transport.last;
  }

  /// Opens a connection and brings it live with `hello` and `ready`.
  Future<FakeChannel> goLive({int cursor = 30}) async {
    await storeCursor(cursor);
    final ch = await open();
    ch.push('hello', helloData());
    await settle();
    ch.push('ready', readyData());
    await settle();
    return ch;
  }

  /// Answers `hello` on the newest channel and returns the frame the client sent.
  Future<Map<String, Object?>> answerHello({String nonce = 'nonce-2'}) async {
    final ch = transport.last;
    ch.push('hello', helloData(nonce: nonce));
    await settle();
    return ch.sentJson.last;
  }

  /// Stops the connection and closes the database.
  Future<void> dispose() async {
    await _connection?.dispose();
    await db.close();
    await auth.dispose();
  }
}

/// The ops of [frames] in order.
List<String> opsOf(Iterable<Map<String, Object?>> frames) =>
    UnmodifiableListView([for (final f in frames) f['op']! as String]);
