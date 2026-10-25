// SPDX-License-Identifier: Apache-2.0

/// The dispatcher (15 7.1, brief M4-14 Design note 2): it applies one durable event inside the
/// transaction of its page by the duplicate rule (04 9 item 2), the catalogue, the version rule
/// (04 9 item 3) and the applier of its type, and hands ephemeral events to the
/// [EphemeralStore], the run store hook and the hint hook.
library;

import 'dart:async';

import 'package:api_client/api_client.dart' show DecodedEvent, UnknownEvent;
import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';

import 'appliers/access_appliers.dart';
import 'appliers/command_appliers.dart';
import 'appliers/file_appliers.dart';
import 'appliers/host_appliers.dart';
import 'appliers/instance_appliers.dart';
import 'appliers/message_appliers.dart';
import 'appliers/personal_appliers.dart';
import 'appliers/principal_appliers.dart';
import 'appliers/run_summary_appliers.dart';
import 'appliers/structure_appliers.dart';
import 'appliers/thread_appliers.dart';
import 'ephemeral_store.dart';
import 'fetcher.dart';
import 'gap_filler.dart';
import 'host_events.dart';
import 'isolate_decode.dart';
import 'notifications.dart';
import 'sync_events.dart';
import 'wire.dart';

/// Called when the echo `message.created` of a message this person sent arrives (04 13.2 item
/// 6); the outbox of M4-15 confirms the entry with [clientMsgId].
typedef EchoHook = void Function(MessageId clientMsgId, Message message);

/// Called with the raw payload of every `run.updated`, truncated ones included (04 11.3 item 4);
/// the run store of M4-16 decides by version and refetches.
typedef RunUpdatedHook = void Function(Map<String, Object?> payload);

/// Writes one event type to the local database inside the page transaction of [c].
typedef Applier = Future<void> Function(ApplyContext c);

/// What [Dispatcher.apply] did with an event.
enum ApplyOutcome {
  /// The applier ran (or a truncated event scheduled its reload).
  applied,

  /// `seq <= cursor`: discarded (04 9 item 2).
  duplicate,

  /// Unknown type, no local table, a malformed envelope, or a versioned type without
  /// `resource.version`: skipped; the cursor still advances over it (04 6.1).
  skipped,

  /// `session.revoked` of this session: the page stops, everything is wiped (04 8.5).
  stopped,
}

/// Work that waits for the commit of the page: what must not happen when the transaction rolls
/// back (notices, file deletion, the echo, counters) and the REST reads that follow an event.
final class AfterCommit {
  /// Steps that run right after the commit, in order.
  final List<FutureOr<void> Function()> steps = [];

  /// REST reads that run in the background after the commit, in order (truncated events, the
  /// structure of added targets, gap fills).
  final List<Future<void> Function()> reads = [];
}

/// What the appliers share beyond one page: the database, the REST reads, the identity of this
/// device and the in-memory stores.
final class SyncContext {
  /// Creates the context.
  SyncContext({
    required this.db,
    required this.fetcher,
    required this.identity,
    required this.draftEditing,
    required this.eraser,
    required this.ephemeral,
    required this.gaps,
    required this.notify,
    required this.onEcho,
    required this.onRunUpdated,
    required this.onNotification,
    required this.log,
  });

  /// The local database.
  final LocalDatabase db;

  /// The REST reads.
  final Fetcher fetcher;

  /// This device and session.
  final LocalIdentity identity;

  /// Whether a draft is being typed on this device.
  final DraftEditing draftEditing;

  /// Deletes cached file bytes of removed targets.
  final FileCacheEraser eraser;

  /// Typing and presence.
  final EphemeralStore ephemeral;

  /// The gap filler of thread positions.
  final GapFiller gaps;

  /// Emits a notice to the app.
  final void Function(SyncNotice notice) notify;

  /// The echo hook of the outbox.
  final EchoHook onEcho;

  /// The `run.updated` hook of the run store.
  final RunUpdatedHook onRunUpdated;

  /// Receives every well-formed `notification.created` (13 6.3); nothing of it is stored.
  final void Function(NotificationCreated n) onNotification;

  /// One line per notable step, never content.
  final void Function(String line) log;

  /// Unread and mention counters between two server values (04 14); every change passes the
  /// global `seq` of its event, so the counter itself skips what its server values hold
  /// (D-M4-19-10).
  UnreadCounter counter = UnreadCounter();

  /// The threads [counter] holds values for, so a new snapshot can announce the ones it resets.
  final Set<ThreadId> countedThreads = {};

  /// The channel of every thread in [countedThreads], from the snapshot's `unread_threads`, the
  /// read state and the message events; the sidebar sums the counters per channel without the
  /// thread being stored (brief M4-19 Design note 1).
  final Map<ThreadId, ChannelId> counterChannels = {};

  /// IDs of spaces, channels and threads removed in this process (04 8.3); a REST read that
  /// was in flight during the removal must not store their content again (code review of
  /// M4-14). An `access.changed` with `added` and a snapshot that lists a target take it out.
  final Set<String> removedTargets = {};

  /// Whether a background read may store content of [threadId] in [channelId]: the channel is
  /// stored here and neither it nor the thread was removed. Call it inside the transaction of
  /// the write, so a removal cannot slip in between.
  Future<bool> holdsContent(
    LocalDatabase db,
    ChannelId channelId,
    ThreadId threadId,
  ) async =>
      !removedTargets.contains(threadId.value) &&
      await holdsChannel(db, channelId);

  /// Whether the channel [id] is stored here and was not removed, nor its space.
  Future<bool> holdsChannel(LocalDatabase db, ChannelId id) async {
    if (removedTargets.contains(id.value)) return false;
    final row = await db.structureDao.channelById(id);
    if (row == null) return false;
    final space = row.spaceId;
    return space == null || !removedTargets.contains(space.value);
  }

  /// Whether a channel of [spaceId] (null for a direct message) with [channelId] may be stored
  /// by a background read: neither was removed.
  bool mayStoreChannel(ChannelId channelId, SpaceId? spaceId) =>
      !removedTargets.contains(channelId.value) &&
      (spaceId == null || !removedTargets.contains(spaceId.value));

  /// The person, from the snapshot of `ready`; null before the first `ready`.
  PrincipalId? me;

  /// Thread counters that changed, for the interface.
  final StreamController<ThreadId> counterChanges =
      StreamController<ThreadId>.broadcast(sync: true);

  /// The changes of hosts and terminal sessions (brief M4-36); nothing of them is stored.
  final HostEventsHub hosts = HostEventsHub();
}

/// One event inside its page transaction.
final class ApplyContext {
  /// Creates the context of [event] in [txn].
  ApplyContext({
    required this.txn,
    required this.event,
    required this.after,
    required this.sync,
  });

  /// The page transaction; every write goes through `txn.db`.
  final LocalTxn txn;

  /// The event.
  final DecodedEvent event;

  /// Work after the commit.
  final AfterCommit after;

  /// The shared context.
  final SyncContext sync;

  /// Set by an applier to stop the page (`session.revoked`).
  bool stop = false;

  /// The database inside the transaction.
  LocalDatabase get db => txn.db;

  /// The envelope as received.
  Json get raw => event.raw;

  /// The `payload` object.
  Json get payload => obj(raw, 'payload');

  /// `resource.version`, or null.
  ResourceVersion? get resourceVersion {
    final v = optInt(obj(raw, 'resource'), 'version');
    return v == null ? null : ResourceVersion(v);
  }

  /// `resource.id`, or null.
  String? get resourceId => optStr(obj(raw, 'resource'), 'id');

  /// `client_msg_id` of the envelope, or null.
  String? get clientMsgId => optStr(raw, 'client_msg_id');
}

/// The appliers by event type: every durable type with a local table (README, applier table).
final Map<String, Applier> _appliers = {
  ...messageAppliers,
  ...threadAppliers,
  ...structureAppliers,
  ...runSummaryAppliers,
  ...principalAppliers,
  ...personalAppliers,
  ...instanceAppliers,
  ...fileAppliers,
  ...commandAppliers,
  ...accessAppliers,
  ...hostAppliers,
};

/// The event types the dispatcher writes to a local table.
Set<String> get appliedEventTypes => Set.unmodifiable(_appliers.keys);

/// Applies events to the local database (brief M4-14 Design note 2).
///
/// Example:
/// ```dart
/// await db.syncStateDao.commitPage((txn) async {
///   for (final e in page) {
///     await dispatcher.apply(txn, e, cursor: stored, after: after);
///   }
/// }, cursor: GlobalSeq(pageCursor));
/// ```
final class Dispatcher {
  /// Creates the dispatcher over [sync].
  Dispatcher(this.sync);

  /// The shared context.
  final SyncContext sync;

  /// Applies the durable event [e] inside [txn]: (a) `seq <= cursor` is a duplicate; (b) a
  /// type the catalogue does not know or without a local table is skipped; (c) a versioned
  /// type without `resource.version` is skipped (only a greater version may change an object),
  /// the comparison with the stored version is the conditional upsert of the DAO (D-M4-12-3);
  /// (d) the applier of the type writes; (e) a truncated event schedules its reload through the
  /// [Fetcher] instead.
  ///
  /// Errors of the applier propagate, so the page transaction rolls back and the cursor stays.
  Future<ApplyOutcome> apply(
    LocalTxn txn,
    DecodedEnvelope e, {
    required GlobalSeq? cursor,
    required AfterCommit after,
  }) async {
    final seq = e.seq;
    if (seq != null && isDuplicate(GlobalSeq(seq), cursor)) {
      return ApplyOutcome.duplicate;
    }
    final event = e.event;
    if (event == null) {
      sync.log('event skipped: malformed envelope');
      return ApplyOutcome.skipped;
    }
    final info = EventCatalog.lookup(event.type);
    if (info == null) return ApplyOutcome.skipped;
    // The generated event union holds only durable types, so an ephemeral event of the
    // catalogue decodes as unknown; [applyEphemeral] reads its raw payload.
    if (!info.durable) {
      after.steps.add(() => applyEphemeral(event));
      return ApplyOutcome.applied;
    }
    if (event is UnknownEvent) return ApplyOutcome.skipped;
    final applier = _appliers[event.type];
    if (applier == null) return ApplyOutcome.skipped;
    final c = ApplyContext(txn: txn, event: event, after: after, sync: sync);
    final decision = decideApply(
      local: null,
      incoming: c.resourceVersion,
      typeCarriesVersion: info.carriesResourceVersion,
    );
    if (decision == ApplyDecision.ignoreStale) {
      sync.log('event skipped: ${event.type} without resource.version');
      return ApplyOutcome.skipped;
    }
    if (event.truncated) {
      _scheduleReload(c, info.resourceKind);
      return ApplyOutcome.applied;
    }
    await applier(c);
    return c.stop ? ApplyOutcome.stopped : ApplyOutcome.applied;
  }

  /// Applies an ephemeral event at once (04 2.1): typing and presence go to the
  /// [EphemeralStore], `run.updated` raw to the run store hook (also when truncated, Design
  /// note 2h), `notification.created` to the hint hook (13 6.3, no local table),
  /// `host.presence_changed` to the host hub (M4-36); other ephemeral types are skipped. A malformed payload is logged without its content.
  void applyEphemeral(DecodedEvent e) {
    try {
      final payload = obj(e.raw, 'payload');
      switch (e.type) {
        case 'typing.started':
          sync.ephemeral.typingStarted(
            ThreadId.parse(str(payload, 'thread_id')),
            PrincipalId.parse(str(payload, 'principal_id')),
            time(payload, 'expires_at'),
          );
        case 'agent.presence_changed':
          sync.ephemeral.presenceChanged(
            PrincipalId.parse(str(payload, 'agent_id')),
            presenceFromJson(payload),
          );
        case 'run.updated':
          sync.onRunUpdated(payload);
        case 'notification.created':
          sync.onNotification(NotificationCreated.fromPayload(payload));
        case 'host.presence_changed':
          applyHostPresence(sync, payload);
      }
    } on FormatException {
      sync.log('ephemeral event skipped: ${e.type} malformed');
    }
  }

  /// `truncated: true` (04 2.2): the object is loaded through REST after the commit and stored
  /// by the version rule.
  void _scheduleReload(ApplyContext c, String kind) {
    final id = c.resourceId;
    if (id == null) return;
    final type = c.event.type;
    final clientMsgId = c.clientMsgId;
    final seq = c.event.seq;
    c.after.reads.add(() async {
      switch (kind) {
        case 'message':
          final bundle = await sync.fetcher.message(MessageId.parse(id));
          await storeFetchedMessage(
            sync,
            bundle,
            created: type == 'message.created',
            clientMsgId: clientMsgId,
            seq: seq,
          );
        case 'thread':
          final thread = await sync.fetcher.thread(ThreadId.parse(id));
          await sync.db.transaction(() async {
            if (await sync.holdsContent(sync.db, thread.channelId, thread.id)) {
              await storeThread(sync.db, thread);
            }
          });
        case 'channel':
          final view = await sync.fetcher.channel(ChannelId.parse(id));
          await sync.db.transaction(() async {
            if (await sync.holdsChannel(sync.db, view.channel.id)) {
              await storeChannelView(sync.db, view);
            }
          });
        case 'space':
          final view = await sync.fetcher.space(SpaceId.parse(id));
          await sync.db.transaction(() async {
            final known = await sync.db.structureDao.spaceById(view.space.id);
            if (known != null &&
                !sync.removedTargets.contains(view.space.id.value)) {
              await storeSpaceView(sync.db, view);
            }
          });
        default:
          sync.log('truncated $type not reloaded: no read for $kind');
      }
    });
  }
}
