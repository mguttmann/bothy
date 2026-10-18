// SPDX-License-Identifier: Apache-2.0

// The headless client of the scenarios (brief M4-17 Design note 7): a `ScenarioClient` over
// `SyncEngine` with the in-memory local store, the dart:io gateway transport and the auth
// session of `package:auth`, all against the fake server on 127.0.0.1.

import 'dart:async';
import 'dart:math';

import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart';
import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:sync_engine/testing.dart';

import '../support/fakes.dart'
    show FakeLifecycle, FakeNetwork, MemorySecretStore;
import '../support/software_p256_signer.dart';

/// Short client values, so retries and reconnects take milliseconds.
const Map<String, String> fastConfig = {
  'sync.reconnect_backoff_base': '10ms',
  'sync.reconnect_backoff_cap': '100ms',
  'client.outbox_retry_base': '20ms',
  'client.outbox_retry_cap': '200ms',
};

/// Gateway values of the scenarios: quick coalescing, so the agent script stays short.
const GatewayTimings fastTimings = GatewayTimings(
  runPatchCoalesce: Duration(milliseconds: 40),
);

/// A frame ticker that runs the callback in the next microtask (no Flutter in this package).
class MicrotaskTicker implements FrameTicker {
  @override
  void requestFrame(void Function() onFrame) => scheduleMicrotask(onFrame);
}

class _Prompt implements StepUpPrompt {
  @override
  Future<bool> ask(String purposeText) async => true;
}

/// Starts a fake with the standard world; [run] gets it with a fresh engine client, and the
/// engine and the fake stop afterwards.
Future<void> withScenario(
  Future<void> Function(
    FakeServer server,
    WorldSeed seed,
    EngineScenarioClient client,
  )
  run,
) async {
  final server = await FakeServer.start(
    options: const FakeServerOptions(timings: fastTimings),
  );
  final seed = WorldSeed.standard(server.world);
  final client = EngineScenarioClient(server);
  try {
    await run(server, seed, client);
  } finally {
    await client.stop();
    await server.stop();
  }
}

/// [ScenarioClient] over [SyncEngine].
class EngineScenarioClient implements ScenarioClient {
  /// Builds the client for Alex against [server].
  EngineScenarioClient(this.server, {this.login = WorldSeed.alexLogin}) {
    final random = Random(7);
    auth = AuthSession(
      serverUrl: server.baseUri,
      keys: DeviceKeys(
        dpop: SoftwareP256Signer.seeded(11),
        stepUp: SoftwareP256Signer.seeded(12),
      ),
      secrets: MemorySecretStore(),
      clock: const SystemClock(),
      ids: Uuid7Generator(
        nowMillis: () => DateTime.now().millisecondsSinceEpoch,
        random: (bits) => random.nextInt(1 << (bits > 31 ? 31 : bits)),
      ),
      config: config,
      device: const DeviceDescription(
        name: 'Alex laptop',
        platform: 'macos',
        appVersion: '1.0.0',
      ),
      buildClient: (interceptors) => createApiClient(
        baseUrl: server.baseUri,
        traceContext: () => TraceContext(traceId: '1' * 32, sampled: false),
        readRetry: ReadRetryPolicy(
          base: Duration.zero,
          cap: Duration.zero,
          maxAttempts: 1,
        ),
        authInterceptors: interceptors,
      ),
      stepUpPrompt: _Prompt(),
    );
    engine = SyncEngine.create(
      baseUrl: server.baseUri,
      auth: auth,
      store: db,
      transport: IoGatewayTransport(),
      network: network,
      lifecycle: FakeLifecycle(),
      ticker: MicrotaskTicker(),
      config: config,
      client: GatewayClientInfo(
        (b) => b
          ..platform = DevicePlatform.macos
          ..version = '1.0.0',
      ),
      keepConnectedInBackground: true,
      allowPlainHttpForTests: true,
      logHook: log.add,
    );
    _notices = engine.notices.listen(
      (n) => notices.addAll(switch (n) {
        TargetsRemoved(:final targets) => [
          for (final t in targets) 'removed:${t.kind.name}:${t.id}',
        ],
        Resynced(:final reason) => ['resynced:$reason'],
        SignedOutNotice() => ['signed_out'],
        DraftOverwritten() => ['draft_overwritten'],
      }),
    );
  }

  /// The fake.
  final FakeServer server;

  /// The login name.
  final String login;

  /// The client values.
  final AppConfig config = AppConfig.defaults().withOverrides(fastConfig);

  /// The local database.
  final LocalDatabase db = openInMemoryForTests();

  /// The network status.
  final FakeNetwork network = FakeNetwork();

  /// The auth session.
  late final AuthSession auth;

  /// The engine under test.
  late final SyncEngine engine;

  /// The log lines of the engine.
  final List<String> log = [];

  late final StreamSubscription<SyncNotice> _notices;
  final Map<RunId, RunDocument?> _documents = {};
  final List<StreamSubscription<RunDocument?>> _watches = [];
  Set<ThreadId> _visible = {};
  bool _stopped = false;

  @override
  final List<String> notices = [];

  @override
  bool get live => engine.state.phase == ConnectionPhase.live;

  @override
  Future<void> start() async {
    final outcome = await auth.login.loginWithPassword(
      loginName: login,
      password: WorldSeed.password,
      totpCode: WorldSeed.totp,
    );
    if (outcome is! LoginSignedIn) throw StateError('sign-in failed: $outcome');
    engine.start();
    await waitFor('live', () => live);
  }

  @override
  Future<String> send(
    String threadId,
    String text, {
    List<String> mentionIds = const [],
  }) async {
    final entry = await engine.outbox.enqueue(
      OutboxRequest(
        kind: MessageOps.createMessage,
        threadId: ThreadId.parse(threadId),
        payload: {
          'content_md': text,
          if (mentionIds.isNotEmpty) 'mention_ids': mentionIds,
        },
      ),
    );
    return entry.id;
  }

  @override
  Future<void> goOffline() async {
    network.set(false);
    server.disconnect(deviceId: auth.session!.deviceId.value);
    await waitFor('offline', () => !live);
  }

  @override
  Future<void> goOnline() async {
    network.set(true);
    await waitFor('live', () => live);
  }

  @override
  Future<List<ScenarioMessage>> messages(String threadId) async => [
    for (final m in await db.messagesDao.pageBefore(
      ThreadId.parse(threadId),
      null,
      1000,
    ))
      ScenarioMessage(
        id: m.id.value,
        clientMsgId: m.clientMsgId.value,
        threadSeq: m.threadSeq.value,
        text: m.contentMd,
        authorId: m.authorId.value,
        triggerState: m.triggerState?.code,
      ),
  ];

  @override
  Future<List<ScenarioOutboxEntry>> outbox() async => [
    for (final e in await db.outboxDao.allEntries())
      if (e.state != OutboxState.confirmed)
        ScenarioOutboxEntry(
          id: e.id,
          failed: e.state == OutboxState.failed,
          code: e.lastErrorCode,
        ),
  ];

  @override
  Future<bool> holdsChannel(String channelId) async =>
      await db.structureDao.channelById(ChannelId.parse(channelId)) != null;

  @override
  Future<int?> cursor() async => (await db.syncStateDao.read()).cursor?.value;

  @override
  Future<void> focusThread(String threadId) async {
    _visible = {..._visible, ThreadId.parse(threadId)};
    engine.focus.setVisible(_visible, idle: false);
    await waitFor(
      'focus sent',
      () => engine.focus.current.contains(ThreadId.parse(threadId)),
    );
  }

  @override
  Future<void> focusRun(String runId, String threadId) async {
    final id = RunId.parse(runId);
    engine.focus.focusRun(id, ThreadId.parse(threadId));
    _watches.add(engine.runs.watch(id).listen((doc) => _documents[id] = doc));
  }

  @override
  Map<String, Object?>? runDocument(String runId) =>
      _documents[RunId.parse(runId)]?.raw;

  @override
  Future<String?> runState(String runId) async =>
      (await db.runsDao.byId(RunId.parse(runId)))?.state.code;

  @override
  Future<void> stop() async {
    if (_stopped) return;
    _stopped = true;
    for (final w in _watches) {
      await w.cancel();
    }
    await engine.dispose();
    await _notices.cancel();
    await auth.dispose();
    await db.close();
  }
}
