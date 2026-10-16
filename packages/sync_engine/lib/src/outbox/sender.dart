// SPDX-License-Identifier: Apache-2.0

/// The sender of the outbox: one worker per FIFO lane, lanes in parallel (04 13.2 item 2,
/// 15 5, brief M4-15 Design note 4). Internal; `Outbox` is the public face.
library;

import 'dart:async';
import 'dart:convert';
import 'dart:math';

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';

import 'backoff.dart';
import 'classify.dart';
import 'executors.dart';

/// Called with a confirmed entry and its response before the entry leaves the outbox; stores
/// what the response returned (for example the server's message).
typedef OutboxConfirmationHook = Future<void> Function(
  OutboxEntry entry,
  OutboxResponse response,
);

/// Creates a timer; tests pass a fake that runs only when they move the clock.
typedef OutboxTimerFactory = Timer Function(
  Duration delay,
  void Function() callback,
);

/// The IDs [entry] waits for (stored comma-separated in `depends_on`, D-M4-15-6).
List<String> dependenciesOf(OutboxEntry entry) =>
    encodedDependencies(entry.dependsOn);

/// Splits the stored `depends_on` value [stored].
List<String> encodedDependencies(String? stored) =>
    stored == null || stored.isEmpty ? const [] : stored.split(',');

/// Runs async sections one after another, so a change by the person and a step of the sender
/// never interleave (an edit must never change a payload that is being sent).
final class SerialLock {
  Future<void> _tail = Future<void>.value();

  /// Runs [action] after every section that was queued before it.
  Future<T> run<T>(Future<T> Function() action) {
    final result = _tail.then((_) => action());
    _tail = result.then((_) {}, onError: (Object _) {});
    return result;
  }
}

/// The scheduler and the workers of the outbox.
final class OutboxSender {
  /// Creates the sender; see `Outbox` for the parameters.
  OutboxSender({
    required this.dao,
    required this.executors,
    required this.config,
    required this.now,
    required this.timer,
    required this.random,
    required this.lock,
    required this.onChanged,
    this.onConfirmed,
  });

  /// The outbox table.
  final OutboxDao dao;

  /// The registry.
  final OutboxExecutors executors;

  /// `client.outbox_retry_base` and `client.outbox_retry_cap`.
  final AppConfig config;

  /// The clock.
  final DateTime Function() now;

  /// The timer factory.
  final OutboxTimerFactory timer;

  /// The jitter source.
  final Random random;

  /// The lock shared with `Outbox`.
  final SerialLock lock;

  /// Called after every change of the table by the sender.
  final void Function() onChanged;

  /// Called for a confirmed entry with its response.
  final OutboxConfirmationHook? onConfirmed;

  final Set<String> _busy = {};
  bool _started = false;
  bool _recovered = false;
  bool _closed = false;
  bool _pumping = false;
  bool _again = false;
  Timer? _wake;
  Future<void> _pumpDone = Future<void>.value();

  /// Whether the sender may start requests.
  bool get started => _started;

  /// Starts sending. The first start resets `sending` entries to `pending`: their request may
  /// have reached the server, and the same ID makes the repeat safe (Design note 4).
  Future<void> start() async {
    if (_closed) return;
    if (!_recovered) {
      _recovered = true;
      await lock.run(() async {
        for (final e in await allEntries()) {
          if (e.state == OutboxState.sending) {
            await dao.markState(e.id, OutboxState.pending);
          }
        }
      });
    }
    _started = true;
    await pump();
  }

  /// Stops starting requests; running ones finish and apply their outcome.
  void pause() {
    _started = false;
    _wake?.cancel();
    _wake = null;
  }

  /// Stops for good and waits until no step of the sender touches the database any more;
  /// requests in flight are left alone and their entries stay `sending`, so the next run
  /// sends them again with the same ID.
  Future<void> close() async {
    pause();
    _closed = true;
    await _pumpDone;
    await lock.run(() async {});
  }

  /// Every entry in the table, thread by thread in FIFO order.
  Future<List<OutboxEntry>> allEntries() async {
    final threads = <ThreadId?>{...await dao.pendingThreads(), null};
    return [for (final t in threads) ...await dao.entriesFor(t)];
  }

  /// Looks for lanes that can send and starts their workers; safe to call at any time.
  Future<void> pump() async {
    if (_pumping) {
      _again = true;
      return;
    }
    _pumping = true;
    final done = Completer<void>();
    _pumpDone = done.future;
    try {
      do {
        _again = false;
        if (!_started || _closed) return;
        await _schedule();
      } while (_again);
    } finally {
      _pumping = false;
      done.complete();
    }
  }

  Future<void> _schedule() async {
    final lanes = <String, List<OutboxEntry>>{};
    for (final e in await allEntries()) {
      if (e.state.isFinal) continue;
      (lanes[executors.laneOf(e)] ??= []).add(e);
    }
    DateTime? nextWake;
    final current = now();
    for (final MapEntry(key: lane, value: entries) in lanes.entries) {
      if (_busy.contains(lane)) continue;
      // Strict FIFO: only the oldest entry of a lane may go; later ones never overtake it. A
      // dependent never goes before its parent in the same lane, even when the clock stepped
      // back between the two writes and gave the dependent the older `created_at`.
      var head = entries.first;
      final inLane = {for (final e in entries) e.id: e};
      final seen = <String>{};
      while (seen.add(head.id)) {
        final parent = dependenciesOf(head)
            .map((id) => inLane[id])
            .whereType<OutboxEntry>()
            .firstOrNull;
        if (parent == null) break;
        head = parent;
      }
      if (!await _dependenciesConfirmed(head)) continue;
      final due = head.nextAttemptAt;
      if (head.state == OutboxState.waiting &&
          due != null &&
          due.isAfter(current)) {
        if (nextWake == null || due.isBefore(nextWake)) nextWake = due;
        continue;
      }
      _busy.add(lane);
      unawaited(_work(lane, head.id));
    }
    _wake?.cancel();
    _wake = nextWake == null
        ? null
        : timer(nextWake.difference(current), () => unawaited(pump()));
  }

  /// A confirmed entry leaves the table, so a missing dependency counts as confirmed; a
  /// discarded one fails its dependents first (`Outbox.discard`).
  Future<bool> _dependenciesConfirmed(OutboxEntry entry) async {
    for (final id in dependenciesOf(entry)) {
      final parent = await dao.byId(id);
      if (parent != null && parent.state != OutboxState.confirmed) return false;
    }
    return true;
  }

  Future<void> _work(String lane, String id) async {
    try {
      final claimed = await lock.run(() async {
        if (_closed) return null;
        final e = await dao.byId(id);
        if (e == null || e.state.isFinal || !_started) return null;
        if (executors.lookup(e.kind) == null) {
          await dao.recordFailure(id, code: 'no_executor');
          return null;
        }
        await dao.markState(id, OutboxState.sending);
        return dao.byId(id);
      });
      onChanged();
      if (claimed == null) return;
      final op = executors.lookup(claimed.kind)!;
      Classification outcome;
      OutboxResponse? response;
      try {
        response = await op.send(
          claimed,
          ApiCallContext(idempotencyKey: claimed.id),
        );
        outcome = const Classification(OutcomeClass.confirmed);
      } on Object catch (error) {
        outcome = classify(error);
      }
      await lock.run(() async {
        if (!_closed) await _apply(claimed, outcome, response);
      });
      onChanged();
    } finally {
      _busy.remove(lane);
      if (!_closed) unawaited(pump());
    }
  }

  Future<void> _apply(
    OutboxEntry sent,
    Classification outcome,
    OutboxResponse? response,
  ) async {
    // The echo may have confirmed the entry while the request ran (04 16.2): then the late
    // response changes nothing and adds no second display entry.
    final stored = await dao.byId(sent.id);
    if (stored == null) return;
    switch (outcome.outcome) {
      case OutcomeClass.confirmed:
        final r = response ?? const OutboxResponse();
        await confirm(stored, r.result);
        try {
          await onConfirmed?.call(stored, r);
        } on Object {
          // The write is stored on the server; its echo or the catch-up brings the result
          // again, so a failing hook must not keep the entry and send it once more.
        }
        await dao.replaceWithServer(stored.id);
      case OutcomeClass.retry:
        final wait =
            outcome.retryAfter ??
            fullJitterBackoff(
              base: config.outboxRetryBase,
              cap: config.outboxRetryCap,
              retry: stored.attempts - 1,
              random: random,
            );
        await dao.markState(
          stored.id,
          OutboxState.waiting,
          nextAttemptAt: now().add(wait),
        );
      case OutcomeClass.failed:
        await dao.recordFailure(
          stored.id,
          code: outcome.code ?? 'failed',
          reason: outcome.reason,
        );
      case OutcomeClass.conflict:
        await dao.recordFailure(
          stored.id,
          code: outcome.code ?? 'version_conflict',
          reason: outcome.reason,
          conflictCurrent: outcome.current == null
              ? null
              : jsonEncode(outcome.current),
        );
    }
  }

  /// Fills [result] into the members the dependents of [entry] hold as null (D-M4-15-6), for
  /// example the server's `message_id` of a follow-up edit. Runs inside the lock.
  Future<void> confirm(OutboxEntry entry, Map<String, Object?> result) async {
    if (result.isEmpty) return;
    for (final d in await dependentsOf(entry.id)) {
      if (d.state.isFinal) continue;
      final payload = decodePayload(d.payload);
      var changed = false;
      for (final MapEntry(:key, :value) in result.entries) {
        if (payload.containsKey(key) && payload[key] == null) {
          payload[key] = value;
          changed = true;
        }
      }
      if (changed) await dao.updatePayload(d.id, jsonEncode(payload));
    }
  }

  /// The entries that depend on [id].
  Future<List<OutboxEntry>> dependentsOf(String id) async => [
    for (final e in await allEntries())
      if (dependenciesOf(e).contains(id)) e,
  ];
}
