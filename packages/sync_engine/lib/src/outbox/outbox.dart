// SPDX-License-Identifier: Apache-2.0

/// The outbox of the apps: every write goes through it (15 1 principle 3, 04 13.2, 15 7.5).
library;

import 'dart:async';
import 'dart:convert';
import 'dart:math';

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';

import 'entry.dart';
import 'executors.dart';
import 'sender.dart';

/// The one write path of the apps (04 13.2, 15 7.5).
///
/// [enqueue] stores an entry with its own UUIDv7 before anything is sent; while started, a
/// worker per FIFO lane (a thread, or the lane its executor names) sends the oldest entry,
/// waits for the response and applies the outcome (`classify`): confirmed entries leave the
/// table, retries wait with full jitter between `client.outbox_retry_base` and
/// `client.outbox_retry_cap` (or `Retry-After`) and keep their ID, final errors and
/// `412 version_conflict` stay as `failed` with the reason. Entries survive a restart;
/// [start] sends them again with their stored IDs. M4-17 calls [start] after
/// `catchup.complete` and [pause] when the connection drops (04 13.4), and passes every echo
/// `message.created` to [confirmByEcho].
///
/// Example:
/// ```dart
/// final outbox = Outbox(db: db, executors: executors, config: config, ids: ids);
/// final entry = await outbox.enqueue(OutboxRequest(
///   kind: MessageOps.createMessage,
///   threadId: thread,
///   payload: {'content_md': 'Hello Sam'},
/// ));
/// await outbox.start();
/// ```
final class Outbox {
  /// Creates the outbox on [db].
  ///
  /// [executors] sends the entries; [config] gives `client.outbox_retry_base`,
  /// `client.outbox_retry_cap` and `client.outbox_stuck_after`; [ids] creates the entry IDs.
  /// The clock is the database's (`LocalDatabase.clock`); [timer] creates the timers of the
  /// backoff and of the delayed display, [random] draws the jitter (a secure source by
  /// default), and [onConfirmed] stores what a `2xx` returned before the entry leaves.
  Outbox({
    required LocalDatabase db,
    required OutboxExecutors executors,
    required AppConfig config,
    required Uuid7Generator ids,
    OutboxTimerFactory? timer,
    Random? random,
    OutboxConfirmationHook? onConfirmed,
  }) : _dao = db.outboxDao,
       _executors = executors,
       _config = config,
       _newId = ids.next,
       _now = db.clock,
       _timer = timer ?? Timer.new {
    _sender = OutboxSender(
      dao: _dao,
      executors: executors,
      config: config,
      now: _now,
      timer: _timer,
      random: random ?? Random.secure(),
      lock: _lock,
      onChanged: _changed,
      onConfirmed: onConfirmed,
    );
  }

  final OutboxDao _dao;
  final OutboxExecutors _executors;
  final AppConfig _config;
  final String Function() _newId;
  final DateTime Function() _now;
  final OutboxTimerFactory _timer;
  final SerialLock _lock = SerialLock();
  final StreamController<void> _changes = StreamController<void>.broadcast();
  late final OutboxSender _sender;

  /// Stores [request] as a `pending` entry and returns it; sends it when the outbox is
  /// started and its turn comes.
  ///
  /// The ID is `request.id` or a new UUIDv7; it is the `client_msg_id`,
  /// `client_interaction_id` or `Idempotency-Key` of every attempt (04 5). Throws when the ID
  /// exists already (sending again takes a new ID, 04 13.2 item 5).
  Future<OutboxEntry> enqueue(OutboxRequest request) async {
    final entry = await _lock.run(() => _insert(request));
    _changed();
    unawaited(_sender.pump());
    return entry;
  }

  /// Applies the edit [payload] to entry [id] after 04 13.3 and returns the ID of the entry
  /// that now carries the change.
  ///
  /// [payload] holds the changed members; they are merged into the stored payload.
  ///
  /// - Never attempted (`attempts == 0`): the payload is changed in place; same ID.
  /// - Failed: a new entry with a new ID and the merged payload replaces it, and the entries
  ///   that waited for the failed one wait for the new one; after a `412 version_conflict` the
  ///   new entry targets the version of the kept `current` revision (member `version` and the
  ///   expected version).
  /// - Attempted and not failed: the entry may be stored on the server, so it stays as it is;
  ///   a follow-up of its operation (`followUpEdit`) waits for it, or the follow-up that has
  ///   not been sent yet is changed in place.
  ///
  /// Throws a [StateError] for an unknown ID or an operation without a follow-up edit.
  Future<String> edit(String id, Map<String, Object?> payload) async {
    final result = await _lock.run(() async {
      final e = await _require(id);
      if (e.state == OutboxState.failed) {
        // The person's edit is the newest text: follow-up edits queued behind the failed entry
        // carry older texts and would overwrite it after the new entry (04 13.3).
        final editKind = _executors
            .lookup(e.kind)
            ?.followUpEdit
            ?.call(e, const {})
            .kind;
        final obsolete = [
          for (final d in await _chain(e))
            if (d.kind == editKind && d.attempts == 0 && !d.state.isFinal) d,
        ];
        final fresh = await _replaceFailed(e, payload);
        for (final d in obsolete) {
          await _bridge(d);
        }
        return fresh;
      }
      if (e.attempts == 0) {
        await _dao.updatePayload(
          id,
          jsonEncode({...decodePayload(e.payload), ...payload}),
        );
        return id;
      }
      final followUp = _executors.lookup(e.kind)?.followUpEdit;
      if (followUp == null) {
        throw StateError('${e.kind} cannot be edited after it was sent');
      }
      final request = followUp(e, payload);
      final tail = await _tail(e);
      if (tail.id != e.id &&
          tail.attempts == 0 &&
          tail.kind == request.kind &&
          tail.state == OutboxState.pending) {
        // The slots the outbox fills (null in a new follow-up) keep what is bound already.
        final merged = decodePayload(tail.payload);
        for (final MapEntry(:key, :value) in request.payload.entries) {
          if (value != null || !merged.containsKey(key)) merged[key] = value;
        }
        await _dao.updatePayload(tail.id, jsonEncode(merged));
        return tail.id;
      }
      return (await _insert(request.withDependsOn(await _waitable([tail.id]))))
          .id;
    });
    _changed();
    unawaited(_sender.pump());
    return result;
  }

  /// Removes entry [id] after 04 13.3; returns the ID of a follow-up delete, or null.
  ///
  /// A never attempted or failed entry is discarded ([discard]); an attempted one stays and a
  /// follow-up delete (`followUpRemove`) waits behind it and its pending follow-ups. Throws a
  /// [StateError] for an unknown ID or an operation without a follow-up delete.
  Future<String?> remove(String id) async {
    final result = await _lock.run<String?>(() async {
      final e = await _require(id);
      if (e.state == OutboxState.failed || e.attempts == 0) {
        await _discard(e);
        return null;
      }
      final followUp = _executors.lookup(e.kind)?.followUpRemove;
      if (followUp == null) {
        throw StateError('${e.kind} cannot be removed after it was sent');
      }
      final tail = await _tail(e);
      return (await _insert(
        followUp(e).withDependsOn(await _waitable([tail.id])),
      )).id;
    });
    _changed();
    unawaited(_sender.pump());
    return result;
  }

  /// Sends the failed entry [id] again with a new ID and discards the old one (04 13.2
  /// item 5); the entries that waited for it wait for the new one. Returns the new ID. Throws a [StateError] when the entry does not exist or has
  /// not failed.
  Future<String> retry(String id) async {
    final result = await _lock.run(() async {
      final e = await _require(id);
      if (e.state != OutboxState.failed) {
        throw StateError('only a failed entry is sent again');
      }
      return _replaceFailed(e, decodePayload(e.payload));
    });
    _changed();
    unawaited(_sender.pump());
    return result;
  }

  /// Deletes entry [id] at the person's request; the entries that depend on it fail with
  /// `dependency_discarded`, so nothing is sent without what it needs and nothing is dropped
  /// silently (04 13.2 item 5). An unknown ID changes nothing.
  Future<void> discard(String id) async {
    await _lock.run(() async {
      final e = await _dao.byId(id);
      if (e != null) await _discard(e);
    });
    _changed();
    unawaited(_sender.pump());
  }

  /// Confirms the entry whose ID is [clientMsgId] because its echo `message.created` arrived
  /// (04 13.2 item 6), also while its request still waits for a response (04 16.2).
  ///
  /// [result] carries what dependents need (`message_id`, `version` of the event's message).
  /// Returns false and changes nothing for an unknown ID.
  Future<bool> confirmByEcho(
    String clientMsgId, {
    Map<String, Object?> result = const {},
  }) async {
    final found = await _lock.run(() async {
      final e = await _dao.byId(clientMsgId);
      if (e == null) return false;
      await _sender.confirm(e, result);
      await _dao.replaceWithServer(e.id);
      return true;
    });
    if (found) {
      _changed();
      unawaited(_sender.pump());
    }
    return found;
  }

  /// Starts or resumes sending (after `catchup.complete`, 04 13.4); the first call resets
  /// entries left in `sending` by an earlier run to `pending`.
  Future<void> start() => _sender.start();

  /// Stops starting requests; running ones finish and apply their outcome.
  Future<void> pause() async => _sender.pause();

  /// Stops the outbox and its streams; the stored entries stay.
  Future<void> close() async {
    await _sender.close();
    await _changes.close();
  }

  /// The entries of [threadId] that are not confirmed, in FIFO order, for the display after
  /// the last confirmed `thread_seq` (04 13.2 item 3); a new list on every change and when an
  /// entry becomes `delayed`.
  Stream<List<OutboxEntryView>> watchThread(ThreadId threadId) {
    late StreamController<List<OutboxEntryView>> controller;
    StreamSubscription<List<OutboxEntry>>? sub;
    Timer? delayTimer;
    List<OutboxEntry> last = const [];
    var generation = 0;

    Future<void> publish() async {
      final mine = ++generation;
      delayTimer?.cancel();
      final now = _now();
      final views = <OutboxEntryView>[];
      DateTime? nextDelay;
      for (final e in last) {
        if (e.state == OutboxState.confirmed) continue;
        final view = await _view(e, now);
        views.add(view);
        if (view.stateLabel == OutboxStateLabel.waiting) {
          final at = e.createdAt.add(_config.outboxStuckAfter);
          if (nextDelay == null || at.isBefore(nextDelay)) nextDelay = at;
        }
      }
      if (mine != generation || controller.isClosed) return;
      controller.add(views);
      if (nextDelay != null) {
        delayTimer = _timer(
          nextDelay.difference(now),
          () => unawaited(publish()),
        );
      }
    }

    controller = StreamController<List<OutboxEntryView>>(
      onListen: () {
        sub = _dao.watchEntriesFor(threadId).listen((entries) {
          last = entries;
          unawaited(publish());
        }, onError: controller.addError);
      },
      onCancel: () async {
        delayTimer?.cancel();
        await sub?.cancel();
      },
    );
    return controller.stream;
  }

  /// The number of entries that are not final, on listen and after every change (for
  /// `client.outbox.depth`, M4-31).
  Stream<int> get depth {
    late StreamController<int> controller;
    StreamSubscription<void>? sub;
    int? last;
    Future<void> emit() async {
      final count = (await _sender.allEntries())
          .where((e) => !e.state.isFinal)
          .length;
      if (count != last && !controller.isClosed) {
        last = count;
        controller.add(count);
      }
    }

    controller = StreamController<int>(
      onListen: () {
        sub = _changes.stream.listen((_) => unawaited(emit()));
        unawaited(emit());
      },
      onCancel: () => sub?.cancel(),
    );
    return controller.stream;
  }

  Future<OutboxEntryView> _view(OutboxEntry e, DateTime now) async {
    String? blockedCode;
    String? blockedReason;
    for (final id in dependenciesOf(e)) {
      final parent = await _dao.byId(id);
      if (parent != null && parent.state == OutboxState.failed) {
        blockedCode = parent.lastErrorCode;
        blockedReason = parent.lastErrorReason;
        break;
      }
    }
    final label = switch (e.state) {
      OutboxState.failed => OutboxStateLabel.failed,
      OutboxState.sending => OutboxStateLabel.sending,
      OutboxState.waiting =>
        now.difference(e.createdAt) >= _config.outboxStuckAfter
            ? OutboxStateLabel.delayed
            : OutboxStateLabel.waiting,
      _ => OutboxStateLabel.pending,
    };
    return OutboxEntryView(
      entry: e,
      stateLabel: label,
      blockedByCode: blockedCode,
      blockedByReason: blockedReason,
    );
  }

  void _changed() {
    if (!_changes.isClosed) _changes.add(null);
  }

  Future<OutboxEntry> _require(String id) async {
    final e = await _dao.byId(id);
    if (e == null) throw StateError('no outbox entry $id');
    return e;
  }

  Future<OutboxEntry> _insert(OutboxRequest r) => _dao.enqueue(
    id: r.id ?? _newId(),
    kind: r.kind,
    payload: jsonEncode(r.payload),
    threadId: r.threadId,
    dependsOn: r.dependsOn.isEmpty ? null : r.dependsOn.join(','),
    expectedVersion: r.expectedVersion,
  );

  /// A new entry with a new ID replaces the failed [e] (04 13.2 item 5); after a `412` it
  /// targets the version of the kept revision (04 13.3).
  Future<String> _replaceFailed(
    OutboxEntry e,
    Map<String, Object?> payload,
  ) async {
    var expected = e.expectedVersion;
    // The edit is merged into the stored payload, so `message_id` and similar members stay.
    final next = {...decodePayload(e.payload), ...payload};
    final current = e.conflictCurrent;
    if (current != null) {
      final version = (jsonDecode(current) as Map)['version'];
      if (version is int) {
        expected = ResourceVersion(version);
        if (next.containsKey('version')) next['version'] = version;
      }
    }
    final fresh = await _insert(
      OutboxRequest(
        kind: e.kind,
        payload: next,
        threadId: e.threadId,
        dependsOn: await _waitable(dependenciesOf(e)),
        expectedVersion: expected,
      ),
    );
    // Entries that waited for the failed one now wait for its replacement (D-M4-15-6).
    for (final d in await _sender.dependentsOf(e.id)) {
      if (d.state.isFinal) continue;
      await _dao.updateDependsOn(
        d.id,
        [for (final id in dependenciesOf(d)) id == e.id ? fresh.id : id]
            .join(','),
      );
    }
    await _dao.delete(e.id);
    return fresh.id;
  }

  Future<void> _discard(OutboxEntry e) async {
    // What the discarded entry had bound (a follow-up edit holds the server's `message_id`)
    // goes to the slots its dependents still hold as null, so sending one of them again later
    // reaches the same message (04 13.3).
    final bound = <String, Object?>{
      for (final MapEntry(:key, :value) in decodePayload(e.payload).entries)
        key: ?value,
    };
    // The whole chain behind it fails, not only the direct dependents: an entry that waits for
    // a failed one would block its lane for good (04 13.2 item 2).
    for (final d in await _chain(e)) {
      if (d.state.isFinal) continue;
      await _sender.fillSlots(d, bound);
      await _dao.recordFailure(d.id, code: 'dependency_discarded');
    }
    await _dao.delete(e.id);
  }

  /// The dependencies an entry the outbox creates or re-points may wait for: [ids] with
  /// every entry stored as `failed` replaced by what that entry waited for, recursively.
  ///
  /// A failed entry is never sent, so an entry waiting for it would block its lane for good
  /// (04 13.2 item 2). A failed entry with a stored parent exists only after a discard failed
  /// its chain (`dependency_discarded`); that parent is gone, and its bound members are filled
  /// into the dependents already. Every path that writes dependencies of its own (retry,
  /// edit, remove, the bridge over obsolete follow-ups) goes through here, so the replacement
  /// of a chain-failed entry never waits for another chain-failed one. Dependencies the
  /// caller names in [enqueue] stay as they are: a message waits for its failed upload until
  /// the person retries or discards it.
  Future<List<String>> _waitable(List<String> ids) async {
    final result = <String>[];
    final seen = <String>{};
    final open = [...ids.reversed];
    while (open.isNotEmpty) {
      final id = open.removeLast();
      if (!seen.add(id)) continue;
      final parent = await _dao.byId(id);
      if (parent != null && parent.state == OutboxState.failed) {
        open.addAll(dependenciesOf(parent).reversed);
      } else {
        result.add(id);
      }
    }
    return result;
  }

  /// Every entry that waits for [e], directly or through others, final ones included.
  Future<List<OutboxEntry>> _chain(OutboxEntry e) async {
    final found = <String, OutboxEntry>{};
    final open = [e.id];
    while (open.isNotEmpty) {
      for (final d in await _sender.dependentsOf(open.removeLast())) {
        if (d.id != e.id && found.putIfAbsent(d.id, () => d) == d) {
          open.add(d.id);
        }
      }
    }
    return found.values.toList();
  }

  /// Deletes the never sent [e] and lets the entries that waited for it wait for what [e]
  /// waited for.
  Future<void> _bridge(OutboxEntry e) async {
    final fresh = (await _dao.byId(e.id)) ?? e;
    final parents = await _waitable(dependenciesOf(fresh));
    for (final d in await _sender.dependentsOf(e.id)) {
      await _dao.updateDependsOn(
        d.id,
        [
          for (final id in dependenciesOf(d))
            if (id == e.id) ...parents else id,
        ].join(','),
      );
    }
    await _dao.delete(e.id);
  }

  /// The newest entry of the follow-up chain of [e]: entries that wait for [e], then for that
  /// one, and so on (04 13.3).
  Future<OutboxEntry> _tail(OutboxEntry e) async {
    var tail = e;
    while (true) {
      final next = (await _sender.dependentsOf(tail.id))
          .where((d) => !d.state.isFinal && d.threadId == e.threadId)
          .toList();
      if (next.isEmpty) return tail;
      tail = next.last;
    }
  }
}
