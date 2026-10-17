// SPDX-License-Identifier: Apache-2.0

// The ScenarioClient of M4-17 over the app (brief M4-18 Design note 9): the login runs through
// the app's onboarding controller, and every read and write goes through the engine and the
// local database the app's providers hand out, so the five scenarios of 04 16 run in the app.

import 'package:client/app/providers/session.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:domain/domain.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:sync_engine/testing.dart';

import 'fakes.dart';

/// [ScenarioClient] over the providers of [container].
class AppScenarioClient implements ScenarioClient {
  /// Creates the client; [network] is the network port of the app's test ports.
  AppScenarioClient(this.container, this.server, this.network);

  /// The app's providers.
  final ProviderContainer container;

  /// The fake.
  final FakeServer server;

  /// The network port of the app.
  final SwitchNetwork network;

  final Map<RunId, RunDocument?> _documents = {};
  Set<ThreadId> _visible = {};
  final List<ProviderSubscription<Object?>> _subs = [];

  SyncEngine get _engine => container.read(syncEngineProvider)!;
  LocalDatabase get _db => container.read(localDatabaseProvider)!;

  @override
  final List<String> notices = [];

  @override
  bool get live =>
      container.read(syncEngineProvider)?.state.phase == ConnectionPhase.live;

  @override
  Future<void> start() async {
    container.read(onboardingControllerProvider);
    await Future<void>.delayed(Duration.zero);
    final control = container.read(onboardingControllerProvider.notifier);
    await control.connect(server.baseUri.toString());
    await control.loginWithPassword(
      WorldSeed.alexLogin,
      WorldSeed.password,
      WorldSeed.totp,
    );
    if (container.read(onboardingControllerProvider).step !=
        OnboardingStep.signedIn) {
      throw StateError('sign-in failed');
    }
    _engine.notices.listen(
      (n) => notices.addAll(switch (n) {
        TargetsRemoved(:final targets) => [
          for (final t in targets) 'removed:${t.kind.name}:${t.id}',
        ],
        Resynced(:final reason) => ['resynced:$reason'],
        SignedOutNotice() => ['signed_out'],
        DraftOverwritten() => ['draft_overwritten'],
      }),
    );
    await waitFor('live', () => live);
  }

  @override
  Future<String> send(
    String threadId,
    String text, {
    List<String> mentionIds = const [],
  }) async => (await _engine.outbox.enqueue(
    OutboxRequest(
      kind: MessageOps.createMessage,
      threadId: ThreadId.parse(threadId),
      payload: {
        'content_md': text,
        if (mentionIds.isNotEmpty) 'mention_ids': mentionIds,
      },
    ),
  )).id;

  @override
  Future<void> goOffline() async {
    network.set(false);
    server.disconnect(
      deviceId: container
          .read(signedInServicesProvider)!
          .auth
          .session!
          .deviceId
          .value,
    );
    await waitFor('offline', () => !live);
  }

  @override
  Future<void> goOnline() async {
    network.set(true);
    await waitFor('live', () => live);
  }

  @override
  Future<List<ScenarioMessage>> messages(String threadId) async => [
    for (final m in await _db.messagesDao.pageBefore(
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
    for (final e in await _db.outboxDao.allEntries())
      if (e.state != OutboxState.confirmed)
        ScenarioOutboxEntry(
          id: e.id,
          failed: e.state == OutboxState.failed,
          code: e.lastErrorCode,
        ),
  ];

  @override
  Future<bool> holdsChannel(String channelId) async =>
      await _db.structureDao.channelById(ChannelId.parse(channelId)) != null;

  @override
  Future<int?> cursor() async => (await _db.syncStateDao.read()).cursor?.value;

  @override
  Future<void> focusThread(String threadId) async {
    _visible = {..._visible, ThreadId.parse(threadId)};
    _engine.focus.setVisible(_visible, idle: false);
    await waitFor(
      'focus sent',
      () => _engine.focus.current.contains(ThreadId.parse(threadId)),
    );
  }

  @override
  Future<void> focusRun(String runId, String threadId) async {
    final id = RunId.parse(runId);
    _engine.focus.focusRun(id, ThreadId.parse(threadId));
    _engine.runs.watch(id).listen((doc) => _documents[id] = doc);
  }

  @override
  Map<String, Object?>? runDocument(String runId) =>
      _documents[RunId.parse(runId)]?.raw;

  @override
  Future<String?> runState(String runId) async =>
      (await _db.runsDao.byId(RunId.parse(runId)))?.state.code;

  @override
  Future<void> stop() async {
    for (final s in _subs) {
      s.close();
    }
  }
}
