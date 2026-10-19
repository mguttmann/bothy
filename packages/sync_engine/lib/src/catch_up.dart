// SPDX-License-Identifier: Apache-2.0

/// Catch-up and reconciliation (15 7.3, 04 7; brief M4-14 Design note 1): the
/// [SessionListener] that applies the snapshot of `ready`, the pages of the catch-up, live
/// events, `cursor`, `catchup.complete` and `resync_required`, and wipes the account on a
/// revocation (04 8.5).
library;

import 'dart:async';

import 'package:api_client/api_client.dart'
    show
        GatewayReady,
        GatewayResumed,
        GatewayResyncRequired,
        GatewayResyncRequiredReasonEnum,
        Snapshot,
        standardSerializers;
import 'package:auth/auth.dart';
import 'package:built_value/serializer.dart';
import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';

import 'appliers/message_appliers.dart';
import 'appliers/personal_appliers.dart';
import 'appliers/thread_appliers.dart';
import 'backoff.dart';
import 'connection_state.dart';
import 'dispatcher.dart';
import 'ephemeral_store.dart';
import 'fetcher.dart';
import 'frames.dart';
import 'gap_filler.dart';
import 'isolate_decode.dart';
import 'ports.dart';
import 'session_listener.dart';
import 'snapshot_applier.dart';
import 'sync_events.dart';
import 'wire.dart';

/// Keeps the local database equal to what the server lets the person see (15 7.3).
///
/// A gateway page is the run of `dispatch` frames up to the next `cursor` frame or
/// `catchup.complete`; it is applied in one transaction with the cursor of that frame
/// (D-M4-14-1). In the live phase every durable event is its own transaction with `cursor =
/// seq`. `resync_required` empties the mirror (never outbox, drafts or share inbox), loads the
/// snapshot and continues live on the same connection (04 7.3). `4012` (watched through
/// [watchConnection]) and `session.revoked` of this session wipe the database and emit
/// [SignedOutNotice]; nothing is applied after that.
///
/// The hooks are no-ops by default; M4-17 wires the outbox ([onEcho]), the run store
/// ([onRunUpdated]) and the outbox start ([onLive]).
///
/// Example:
/// ```dart
/// final catchUp = CatchUpListener(
///   db: db,
///   fetcher: ApiFetcher(apiClient),
///   config: appConfig,
///   clock: const SystemClock(),
///   identity: AuthIdentity(authSession),
/// );
/// final connection = GatewayConnection(/* ... */ listener: catchUp);
/// catchUp.watchConnection(connection.states);
/// catchUp.notices.listen(handleNotice);
/// ```
final class CatchUpListener implements SessionListener {
  /// Creates the listener.
  ///
  /// [config] supplies `client.sync.parse_offload_bytes` and `sync.typing_ttl` (D-W-115);
  /// [openThreads] names the threads in focus, reloaded after a resync; [random] draws the
  /// jitter of the snapshot retries of a resync (`sync.reconnect_backoff_base`,
  /// `sync.reconnect_backoff_cap`); [logHook] receives one line per notable step, never
  /// content.
  CatchUpListener({
    required LocalDatabase db,
    required Fetcher fetcher,
    required AppConfig config,
    required Clock clock,
    LocalIdentity identity = const FixedIdentity(),
    FileCacheEraser? eraser,
    DraftEditing? draftEditing,
    Iterable<ThreadId> Function()? openThreads,
    EchoHook? onEcho,
    RunUpdatedHook? onRunUpdated,
    void Function()? onLive,
    this._logHook,
    RandomSource? random,
  }) : _db = db,
       _clock = clock,
       _offloadBytes = config.syncParseOffloadBytes,
       _openThreads = openThreads ?? (() => const <ThreadId>[]),
       _onLive = onLive ?? _noop {
    ephemeral = EphemeralStore(clock: clock, typingTtl: config.typingTtl);
    _sync = SyncContext(
      db: db,
      fetcher: fetcher,
      identity: identity,
      draftEditing: draftEditing ?? const _NobodyTyping(),
      eraser: eraser ?? const _NoEraser(),
      ephemeral: ephemeral,
      gaps: gaps,
      notify: _notify,
      onEcho: onEcho ?? (_, _) {},
      onRunUpdated: onRunUpdated ?? (_) {},
      log: _log,
    );
    _resyncBackoff = ReconnectBackoff(
      base: config.reconnectBackoffBase,
      cap: config.reconnectBackoffCap,
      random: random ?? SystemRandomSource(),
    );
    _dispatcher = Dispatcher(_sync);
    _snapshots = SnapshotApplier(_sync);
  }

  static void _noop() {}

  final LocalDatabase _db;
  final Clock _clock;
  final int _offloadBytes;
  final Iterable<ThreadId> Function() _openThreads;
  final void Function() _onLive;
  final void Function(String line)? _logHook;
  late final SyncContext _sync;
  late final Dispatcher _dispatcher;
  late final SnapshotApplier _snapshots;

  /// Typing and agent presence in memory.
  late final EphemeralStore ephemeral;

  /// The gaps of thread positions; [onFocus] retries a failed fill.
  final GapFiller gaps = GapFiller();

  final StreamController<SyncNotice> _notices =
      StreamController<SyncNotice>.broadcast(sync: true);
  final List<Map<String, Object?>> _page = [];
  StreamSubscription<ConnectionState>? _states;
  Future<void> _background = Future<void>.value();
  int _pendingReads = 0;
  bool _live = false;
  bool _halted = false;
  bool _resyncing = false;

  /// The session whose held frames are being drained, or null; a drain of an ended session
  /// does not hold the frames of the next one.
  int? _drainingSession;
  int _session = 0;
  String _resyncReason = '';
  final List<GatewayFrame> _held = [];
  late final ReconnectBackoff _resyncBackoff;
  bool _wiped = false;

  /// The notices to the app: removed targets, an overwritten draft, a resync, a sign-out.
  Stream<SyncNotice> get notices => _notices.stream;

  /// Unread messages and mentions of [threadId] between two server values (04 14).
  ThreadCounts countsOf(ThreadId threadId) => _sync.counter.countsOf(threadId);

  /// The threads whose counts changed.
  Stream<ThreadId> get countChanges => _sync.counterChanges.stream;

  /// The signed-in person as the last snapshot named it (`principal`), or null before the
  /// first `ready` of this process (brief M4-19: "Waiting for me" and the own avatar).
  PrincipalId? get me => _sync.me;

  /// The counts of every thread of [channelId] this engine counts (04 14: the server's values
  /// plus what it counted since), by thread. A thread whose counter it never touched is
  /// absent; the caller then uses the stored read state. The sidebar sums them per channel
  /// (brief M4-19 Design note 1).
  Map<ThreadId, ThreadCounts> countsOfChannel(ChannelId channelId) => {
    for (final t in _sync.countedThreads)
      if (_sync.counterChannels[t] == channelId) t: _sync.counter.countsOf(t),
  };

  /// Loads one page of the threads of [channelId] (`listChannelThreads`, newest first by
  /// [sort]) from [cursor] and stores them by the version rule; returns the cursor of the next
  /// page, null on the last one (brief M4-19 Design note 3: the post list loads further pages
  /// when scrolled near the end). Nothing is stored for a channel this device does not hold
  /// or that was removed meanwhile, nor after a wipe. Throws what the read throws.
  Future<String?> loadThreadPage(
    ChannelId channelId, {
    String? cursor,
    ForumSort? sort,
    int? limit,
  }) async {
    if (_wiped) return null;
    final page = await _sync.fetcher.channelThreads(
      channelId,
      cursor: cursor,
      sort: sort,
      limit: limit,
    );
    await _db.transaction(() async {
      if (_wiped || !await _sync.holdsChannel(_db, channelId)) return;
      for (final thread in page.threads) {
        if (thread.channelId != channelId ||
            _sync.removedTargets.contains(thread.id.value)) {
          continue;
        }
        await storeThread(_db, thread);
      }
    });
    return page.nextCursor;
  }

  /// Whether the database was wiped after a revocation; nothing is applied any more.
  bool get wiped => _wiped;

  /// Watches the [states] of the connection: `signedOut` with the reason `revoked` (`4012`,
  /// 04 8.5) wipes the database and emits [SignedOutNotice].
  void watchConnection(Stream<ConnectionState> states) {
    unawaited(_states?.cancel());
    _states = states.listen((s) {
      if (s.phase == ConnectionPhase.signedOut &&
          s.reason == ConnectionReason.revoked) {
        unawaited(_wipe());
      }
    });
  }

  @override
  Future<void> onReady(GatewayReady ready, SessionControl control) async {
    if (_wiped) return;
    _startSession(control);
    _sync.me = PrincipalId.parse(ready.principal.id);
    final snapshot = snapshotFromJson(
      (standardSerializers.serialize(
        ready.snapshot,
        specifiedType: const FullType(Snapshot),
      ) as Map).cast<String, Object?>(),
    );
    final stored = (await _db.syncStateDao.read()).cursor;
    final after = AfterCommit();
    if (stored == null) {
      // The first connection, or a cursor dropped by a failed resync: whatever the mirror
      // holds belongs to no known time line, so it is replaced by the snapshot (D-M4-14-2).
      final head = GlobalSeq(ready.headSeq);
      await _db.syncStateDao.commitPage(
        (txn) async {
          await _db.syncStateDao.clearMirror(txn);
          await _snapshots.applyFull(txn, snapshot, after);
        },
        cursor: head,
        syncEpoch: ready.syncEpoch,
      );
      gaps.clear();
      await _db.syncStateDao.markSnapshot(_clock.now());
      control.lastProcessedSeq = head;
      await _afterCommit(after);
      // Without a cursor in identify the server sends no catch-up and no catchup.complete:
      // the session is live right after ready (04 6.3 item 4).
      _goLive(control);
      return;
    } else {
      // The catch-up from the cursor delivers the events; only what is not an event comes
      // from the snapshot (Design note 1).
      await _db.syncStateDao.commitPage(
        (txn) => _snapshots.applyNonEvent(txn, snapshot, after),
        cursor: stored,
      );
      control.lastProcessedSeq = stored;
    }
    await _afterCommit(after);
  }

  @override
  Future<void> onResumed(GatewayResumed resumed, SessionControl control) async {
    if (_wiped) return;
    _startSession(control);
  }

  void _startSession(SessionControl control) {
    _endSession();
    _live = false;
    _halted = false;
    control.setPhase(ConnectionPhase.catchingUp);
  }

  @override
  Future<void> onFrame(GatewayFrame frame, SessionControl control) async {
    if (_wiped || _halted) return;
    if (_resyncing || _drainingSession == _session) {
      _held.add(frame);
      return;
    }
    await _handle(frame, control);
  }

  Future<void> _handle(GatewayFrame frame, SessionControl control) async {
    switch (frame) {
      case DispatchFrame(:final envelope):
        if (!_live) {
          _page.add(envelope);
          return;
        }
        final seq = envelope['seq'];
        if (seq is int) {
          await _commit([envelope], seq, control);
        } else {
          final decoded = (await decodeEnvelopes([
            envelope,
          ], offloadBytes: _offloadBytes)).single.event;
          if (decoded != null) _dispatcher.applyEphemeral(decoded);
        }
      case CursorFrame(:final model):
        await _commit(_takePage(), model.seq, control);
      case CatchupCompleteFrame(:final model):
        await _commit(_takePage(), model.headSeq, control);
        if (_wiped || _halted) return;
        _goLive(control);
      case ResyncRequiredFrame(:final model):
        await _resync(model, control);
      case _:
        break;
    }
  }

  @override
  void onDisconnected() {
    // An unfinished page is dropped: its events come again from the stored cursor (04 9).
    _endSession();
    ephemeral.clearTyping();
  }

  void _endSession() {
    _session++;
    _page.clear();
    _held.clear();
    _live = false;
    _resyncing = false;
  }

  List<Map<String, Object?>> _takePage() {
    final page = List.of(_page);
    _page.clear();
    return page;
  }

  void _goLive(SessionControl control) {
    final wasLive = _live;
    _live = true;
    control.setPhase(ConnectionPhase.live);
    if (!wasLive) _onLive();
  }

  /// Applies [envelopes] in one transaction with the cursor [cursorSeq] (never below the
  /// stored one) and the sync epoch (04 7.5 item 2). A failing page leaves rows and cursor as
  /// they were and halts the listener until the next session, which catches up from the stored
  /// cursor (D-M4-14-8); `session.revoked` of this session stops the page and wipes.
  Future<void> _commit(
    List<Map<String, Object?>> envelopes,
    int cursorSeq,
    SessionControl control,
  ) async {
    final decoded = await decodeEnvelopes(
      envelopes,
      offloadBytes: _offloadBytes,
    );
    if (_wiped) return;
    final stored = (await _db.syncStateDao.read()).cursor;
    final target = GlobalSeq(
      stored != null && stored.value > cursorSeq ? stored.value : cursorSeq,
    );
    final after = AfterCommit();
    try {
      await _db.syncStateDao.commitPage(
        (txn) async {
          for (final e in decoded) {
            final outcome = await _dispatcher.apply(
              txn,
              e,
              cursor: stored,
              after: after,
            );
            if (outcome == ApplyOutcome.stopped) throw const _Revoked();
          }
        },
        cursor: target,
        syncEpoch: control.syncEpoch,
      );
    } on _Revoked {
      await _wipe();
      return;
    } on Object catch (e) {
      _halted = true;
      _log('page failed, halted until the next session: ${e.runtimeType}');
      return;
    }
    control.lastProcessedSeq = target;
    await _afterCommit(after);
  }

  /// 04 7.3: the connection stays; the mirror is replaced by the snapshot, cursor and epoch
  /// are its values, outbox, drafts and share inbox stay, the focused threads load again.
  Future<void> _resync(
    GatewayResyncRequired model,
    SessionControl control,
  ) async {
    control.setPhase(ConnectionPhase.resyncing);
    _page.clear();
    _live = false;
    _resyncing = true;
    _resyncBackoff.reset();
    _resyncReason =
        standardSerializers.serialize(
              model.reason,
              specifiedType: const FullType(GatewayResyncRequiredReasonEnum),
            )!
            as String;
    await _attemptResync(control, _session);
  }

  /// Loads the snapshot and replaces the mirror. A failed load is retried after the reconnect
  /// backoff on the same connection while frames wait in memory; mirror and cursor stay until
  /// a snapshot arrives, and a disconnect ends the retries (the next session gets
  /// `resync_required` again from its stored cursor) (D-M4-14-9). The retry runs outside the
  /// frame queue of the connection, so it checks its session after its awaits: a new session
  /// that started meanwhile is never set live or reported as resynced by it, while the
  /// follow-up steps of a committed snapshot still run.
  Future<void> _attemptResync(SessionControl control, int session) async {
    final SnapshotData snapshot;
    try {
      snapshot = await _sync.fetcher.snapshot();
    } on Object catch (e) {
      if (_wiped || session != _session || !_resyncing) return;
      final wait = _resyncBackoff.next();
      _log(
        'resync snapshot failed (${e.runtimeType}), '
        'retry in ${wait.inMilliseconds} ms',
      );
      unawaited(
        _clock.delay(wait).then((_) async {
          if (_wiped || session != _session || !_resyncing) return;
          await _attemptResync(control, session);
        }),
      );
      return;
    }
    if (_wiped || session != _session) return;
    final after = AfterCommit();
    await _db.syncStateDao.commitPage(
      (txn) async {
        await _db.syncStateDao.clearMirror(txn);
        await _snapshots.applyFull(txn, snapshot, after);
      },
      cursor: snapshot.headSeq,
      syncEpoch: snapshot.syncEpoch,
    );
    // The steps that belong to the committed snapshot run even when the session ended meanwhile
    // (the instance row and the read states come only from its reads); what the session sees
    // runs only for the session of the retry.
    gaps.clear();
    await _db.syncStateDao.markSnapshot(_clock.now());
    for (final threadId in _openThreads()) {
      after.reads.add(() => _loadNewestPage(threadId));
    }
    await _afterCommit(after);
    if (_wiped || session != _session) return;
    control.lastProcessedSeq = snapshot.headSeq;
    _resyncing = false;
    _goLive(control);
    _notify(Resynced(_resyncReason));
    await _drainHeld(control, session);
  }

  /// Applies the frames that arrived during a resync, in order, with the duplicate rule (04
  /// 7.3); new frames keep waiting behind them.
  Future<void> _drainHeld(SessionControl control, int session) async {
    if (_drainingSession == session) return;
    _drainingSession = session;
    try {
      while (_held.isNotEmpty &&
          !_resyncing &&
          !_wiped &&
          !_halted &&
          session == _session) {
        await _handle(_held.removeAt(0), control);
      }
    } finally {
      if (_drainingSession == session) _drainingSession = null;
    }
  }

  /// The newest page of [threadId] through REST, stored with its loaded range.
  Future<void> _loadNewestPage(ThreadId threadId) async {
    final page = await _sync.fetcher.threadPage(threadId);
    if (page.messages.isEmpty) return;
    await _db.transaction(() async {
      for (final b in page.messages) {
        final m = b.message;
        if (!await _sync.holdsContent(_db, m.channelId, threadId)) return;
        await storeMessage(_db, b, me: _sync.me);
      }
      await _db.messagesDao.addLoadedRange(
        threadId,
        page.messages.first.message.threadSeq,
        page.messages.last.message.threadSeq,
      );
    });
  }

  /// The thread [threadId] got the focus: a recorded gap is loaded again (Design note 4).
  Future<void> onFocus(ThreadId threadId) async {
    if (_wiped || !gaps.pending.containsKey(threadId)) return;
    await _inBackground(() => gaps.fill(_sync, threadId));
  }

  /// Stores the server message a confirmed write returned (API `Message` of `createMessage`
  /// or `updateMessage`), so the entry that leaves the outbox is replaced by it at once and
  /// not only by its echo (04 13.2 item 6). The version rule applies; a message of a removed
  /// channel or thread is not stored; after a wipe nothing happens. [created] marks the answer
  /// of a new message (gap and counter rules of `message.created`).
  Future<void> storeConfirmedMessage(
    Map<String, Object?> message, {
    required bool created,
  }) async {
    if (_wiped) return;
    await storeFetchedMessage(
      _sync,
      messageFromJson(message),
      created: created,
    );
  }

  /// Stores the thread a confirmed `createThread` returned (API `Thread`), so the new post
  /// shows at once and not only by its `thread.created`. The version rule applies and the
  /// title is indexed (14 6); a thread of a channel this device does not hold or of a removed
  /// target is not stored; after a wipe nothing happens.
  Future<void> storeConfirmedThread(Map<String, Object?> json) async {
    if (_wiped) return;
    final thread = threadFromJson(json);
    await _db.transaction(() async {
      if (_wiped ||
          _sync.removedTargets.contains(thread.id.value) ||
          !await _sync.holdsChannel(_db, thread.channelId)) {
        return;
      }
      await storeThread(_db, thread);
    });
  }

  /// Stores the answer of `putThreadDraft` of the app's write path (API `DraftPutResult`) and
  /// emits [DraftOverwritten] when it reports `overwrote_newer` (04 14).
  Future<void> applyDraftPutResult(Map<String, Object?> result) =>
      storeDraftPutResult(_sync, result);

  Future<void> _afterCommit(AfterCommit after) async {
    for (final step in after.steps) {
      if (_wiped) return;
      try {
        await step();
      } on Object catch (e) {
        _log('after-commit step failed: ${e.runtimeType}');
      }
    }
    for (final read in after.reads) {
      unawaited(_inBackground(read));
    }
  }

  /// Runs [read] after every earlier background read; its error is logged, never thrown, so a
  /// failed REST read (kept as a gap or a truncated row) never stops the sync.
  Future<void> _inBackground(Future<void> Function() read) {
    _pendingReads++;
    final run = _background.then((_) async {
      if (_wiped) return;
      try {
        await read();
      } on Object catch (e) {
        _log('background read failed: ${e.runtimeType}');
      }
    });
    _background = run.whenComplete(() => _pendingReads--);
    return _background;
  }

  /// Completes when every background read scheduled so far is done.
  Future<void> idle() async {
    while (_pendingReads > 0) {
      await _background;
    }
  }

  /// R-SY-14: everything of the account goes; nothing is applied after that.
  Future<void> _wipe() async {
    if (_wiped) return;
    _wiped = true;
    _page.clear();
    ephemeral.clearTyping();
    try {
      await _db.wipe();
    } on Object catch (e) {
      _log('wipe failed: ${e.runtimeType}');
    }
    _notify(const SignedOutNotice(deleteLocalData: true));
  }

  void _notify(SyncNotice notice) {
    if (!_notices.isClosed) _notices.add(notice);
  }

  void _log(String line) => _logHook?.call('catch-up: $line');

  /// Stops watching the connection and closes the streams.
  Future<void> dispose() async {
    await _states?.cancel();
    await idle();
    await ephemeral.dispose();
    await _sync.counterChanges.close();
    await _notices.close();
  }
}

final class _Revoked implements Exception {
  const _Revoked();
}

final class _NobodyTyping implements DraftEditing {
  const _NobodyTyping();

  @override
  bool isEditing({required ChannelId channelId, ThreadId? threadId}) => false;
}

final class _NoEraser implements FileCacheEraser {
  const _NoEraser();

  @override
  Future<void> erase(List<String> paths) async {}
}
