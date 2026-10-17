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
import 'dependencies.dart';
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
    required this.dependencies,
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

  /// The dependency graph shared with `Outbox`.
  final OutboxDependencies dependencies;

  /// Called after every change of the table by the sender.
  final void Function() onChanged;

  /// Called for a confirmed entry with its response.
  final OutboxConfirmationHook? onConfirmed;

  final Set<String> _busy = {};
  bool _started = false;
  bool _recovered = false;
  bool _ready = false;
  bool _closed = false;
  bool _pumping = false;
  bool _again = false;
  Timer? _wake;
  Future<void> _pumpDone = Future<void>.value();

  /// Whether the sender may start requests.
  bool get started => _started;

  /// Starts sending. The first start resets `sending` entries to `pending`: their request may
  /// have reached the server, and the same ID makes the repeat safe (Design note 4). A
  /// [pause] while the reset runs holds: nothing is sent until the next start.
  Future<void> start() async {
    if (_closed) return;
    _started = true;
    if (!_recovered) {
      _recovered = true;
      // Queued on the lock before any schedule that this start allows, so no entry is claimed
      // before the reset.
      await lock.run(() async {
        for (final e in await allEntries()) {
          if (e.state == OutboxState.sending) {
            await dao.markState(e.id, OutboxState.pending);
          }
        }
        _ready = true;
      });
    }
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

  /// Every entry in the table in FIFO order, final ones included.
  Future<List<OutboxEntry>> allEntries() => dependencies.all();

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

  /// Picks the head of every idle lane and claims it, all under the lock on fresh data: a
  /// change by the person (a replaced upload, a new entry) or the outcome of another request
  /// can never slip between the choice and the claim (lane head, dependencies, due time).
  Future<void> _schedule() async {
    final claims = await lock.run(() async {
      final claimed = <(String, OutboxEntry)>[];
      if (!_started || !_ready || _closed) return claimed;
      final lanes = <String, List<OutboxEntry>>{};
      final open = <String, OutboxEntry>{};
      for (final e in await allEntries()) {
        if (e.state.isFinal) continue;
        open[e.id] = e;
        (lanes[executors.laneOf(e)] ??= []).add(e);
      }
      final order = _order(open);
      DateTime? nextWake;
      final current = now();
      for (final MapEntry(key: lane, value: entries) in lanes.entries) {
        if (_busy.contains(lane)) continue;
        final head = entries.reduce((a, b) => order(b, a) < 0 ? b : a);
        switch (await dependencies.readiness(head)) {
          case Readiness.waiting:
            continue;
          case Readiness.broken:
            // A dependency is gone without a confirmation: the entry and its chain fail
            // visibly instead of going without it (D-M4-15-10).
            await dao.recordFailure(head.id, code: dependencyDiscarded);
            await dependencies.failChain(head.id);
            _again = true;
            continue;
          case Readiness.ready:
        }
        final due = head.nextAttemptAt;
        if (head.state == OutboxState.waiting &&
            due != null &&
            due.isAfter(current)) {
          if (nextWake == null || due.isBefore(nextWake)) nextWake = due;
          continue;
        }
        if (executors.lookup(head.kind) == null) {
          await dao.recordFailure(head.id, code: 'no_executor');
          // The next entry of the lane may go now.
          _again = true;
          continue;
        }
        await dao.markState(head.id, OutboxState.sending);
        _busy.add(lane);
        claimed.add((lane, (await dao.byId(head.id))!));
      }
      _wake?.cancel();
      _wake = nextWake == null
          ? null
          : timer(nextWake.difference(current), () => unawaited(pump()));
      return claimed;
    });
    if (claims.isEmpty) return;
    onChanged();
    for (final (lane, entry) in claims) {
      unawaited(_send(lane, entry));
    }
  }

  /// Orders the entries [open] that are not final for [_schedule]: the first entry of a lane
  /// in this order goes next.
  ///
  /// FIFO by `created_at`, with one exception: an entry goes no later than the oldest entry
  /// that waits for it, directly or through others in any lane. So a dependent never goes
  /// before its parent (also when a clock step gave it the older `created_at`), and a
  /// replacement that `retry` or `edit` put at the end of its lane never waits behind an
  /// entry that waits for it through another lane, which would block both lanes for good.
  /// Ties go to the entry with the shorter chain of open dependencies, so a parent always
  /// comes before its dependent (D-M4-15-10).
  static Comparator<OutboxEntry> _order(Map<String, OutboxEntry> open) {
    final waiters = <String, List<OutboxEntry>>{};
    for (final e in open.values) {
      for (final d in dependenciesOf(e)) {
        if (open.containsKey(d)) (waiters[d] ??= []).add(e);
      }
    }
    final due = <String, DateTime>{};
    DateTime dueOf(OutboxEntry e) {
      final known = due[e.id];
      if (known != null) return known;
      due[e.id] = e.createdAt; // guards a cycle, which the outbox never writes
      var at = e.createdAt;
      for (final w in waiters[e.id] ?? const <OutboxEntry>[]) {
        final other = dueOf(w);
        if (other.isBefore(at)) at = other;
      }
      return due[e.id] = at;
    }

    final depth = <String, int>{};
    int depthOf(OutboxEntry e) {
      final known = depth[e.id];
      if (known != null) return known;
      depth[e.id] = 0;
      var d = 0;
      for (final id in dependenciesOf(e)) {
        final parent = open[id];
        if (parent != null) d = max(d, depthOf(parent) + 1);
      }
      return depth[e.id] = d;
    }

    return (a, b) {
      final byDue = dueOf(a).compareTo(dueOf(b));
      if (byDue != 0) return byDue;
      final byDepth = depthOf(a).compareTo(depthOf(b));
      if (byDepth != 0) return byDepth;
      final byAge = a.createdAt.compareTo(b.createdAt);
      return byAge != 0 ? byAge : a.id.compareTo(b.id);
    };
  }

  /// Sends the claimed [entry] and applies the outcome; its lane is busy until then.
  Future<void> _send(String lane, OutboxEntry entry) async {
    try {
      final op = executors.lookup(entry.kind)!;
      Classification outcome;
      OutboxResponse? response;
      try {
        response = await op.send(
          entry,
          ApiCallContext(idempotencyKey: entry.id),
        );
        outcome = const Classification(OutcomeClass.confirmed);
      } on Object catch (error) {
        outcome = classify(error);
      }
      await lock.run(() async {
        if (!_closed) await _apply(entry, outcome, response);
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
        await dependencies.removeEntry(stored.id, into: const []);
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
      await fillSlots(d, result);
    }
  }

  /// Fills [values] into the members [entry] holds as null. Runs inside the lock.
  Future<void> fillSlots(OutboxEntry entry, Map<String, Object?> values) async {
    final payload = decodePayload(entry.payload);
    var changed = false;
    for (final MapEntry(:key, :value) in values.entries) {
      if (payload.containsKey(key) && payload[key] == null) {
        payload[key] = value;
        changed = true;
      }
    }
    if (changed) await dao.updatePayload(entry.id, jsonEncode(payload));
  }

  /// The entries that depend on [id], final ones included.
  Future<List<OutboxEntry>> dependentsOf(String id) =>
      dependencies.dependentsOf(id);
}
