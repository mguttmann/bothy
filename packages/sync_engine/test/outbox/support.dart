// SPDX-License-Identifier: Apache-2.0

/// Fakes of the outbox tests: a clock with timers that run only when the test moves the time,
/// executors that answer from a script, and a harness around an in-memory `local_store`.
library;

import 'dart:async';
import 'dart:convert';
import 'dart:math';

import 'package:api_client/api_client.dart';
import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

/// A UTC time [minutes] after a fixed start.
DateTime at(int minutes) =>
    DateTime.utc(2026, 10, 1, 9).add(Duration(minutes: minutes));

/// Thread A of the tests.
final ThreadId threadA = ThreadId.parse('0192f4a0-7b1c-7000-8a00-00000000000a');

/// Thread B of the tests.
final ThreadId threadB = ThreadId.parse('0192f4a0-7b1c-7000-8a00-00000000000b');

/// A clock and a timer factory that run timers only in [advance].
final class FakeTime {
  /// The current time.
  DateTime now = at(0);
  final List<_FakeTimer> _timers = [];

  /// The time source.
  DateTime clock() => now;

  /// The timer factory the outbox receives.
  Timer timer(Duration delay, void Function() callback) {
    final t = _FakeTimer(now.add(delay), callback);
    _timers.add(t);
    return t;
  }

  /// Moves the time by [d] and runs every timer that is due, earliest first.
  Future<void> advance(Duration d) async {
    now = now.add(d);
    while (true) {
      final due =
          _timers.where((t) => t.isActive && !t.due.isAfter(now)).toList()
            ..sort((a, b) => a.due.compareTo(b.due));
      if (due.isEmpty) break;
      due.first.fire();
      await settle();
    }
    await settle();
  }
}

final class _FakeTimer implements Timer {
  _FakeTimer(this.due, this._callback);
  final DateTime due;
  final void Function() _callback;
  bool _active = true;

  void fire() {
    _active = false;
    _callback();
  }

  @override
  void cancel() => _active = false;

  @override
  bool get isActive => _active;

  @override
  int get tick => 0;
}

/// Lets the outbox finish everything that does not wait for a timer or a held answer.
Future<void> settle() => pumpEventQueue(times: 200);

/// A problem the fake server answers with.
ApiProblem problem(
  int status,
  String code, {
  String? reason,
  Duration? retryAfter,
  Map<String, Object?>? current,
}) => ApiProblem(
  status: status,
  code: code,
  reason: reason,
  retryAfter: retryAfter,
  current: current,
);

/// One call an executor received.
final class Call {
  /// Records a call.
  Call(this.id, this.kind, this.payload, this.idempotencyKey);

  /// Entry ID.
  final String id;

  /// Operation.
  final String kind;

  /// Decoded payload.
  final Map<String, Object?> payload;

  /// The idempotency key of the call context.
  final String idempotencyKey;
}

/// An executor that answers from a script: an [OutboxResponse], an error to throw, or a
/// [Completer] the test completes later. An empty script answers with an empty success.
final class ScriptedExecutor {
  /// The calls so far, in order.
  final List<Call> calls = [];
  final List<Object> _script = [];

  /// Calls that have not been answered yet.
  int inFlight = 0;

  /// Queues answers.
  void then(Iterable<Object> answers) => _script.addAll(answers);

  /// The executor function.
  Future<OutboxResponse> call(OutboxEntry entry, ApiCallContext ctx) async {
    calls.add(
      Call(
        entry.id,
        entry.kind,
        (jsonDecode(entry.payload) as Map).cast<String, Object?>(),
        ctx.idempotencyKey,
      ),
    );
    final next = _script.isEmpty ? null : _script.removeAt(0);
    inFlight++;
    try {
      if (next is Completer<OutboxResponse>) return await next.future;
      if (next is OutboxResponse) return next;
      if (next != null) throw next;
      return const OutboxResponse();
    } finally {
      inFlight--;
    }
  }

  /// IDs of the calls in order.
  List<String> get ids => [for (final c in calls) c.id];
}

/// An in-memory database, a registry, and an outbox on a fake clock.
final class Harness {
  Harness._(this.db, this.time, this.executors, this.outbox, this.confirmed);

  /// Opens the harness with [config] overrides; [db] replaces the in-memory database.
  static Harness open({
    Map<String, String> config = const {},
    LocalDatabase? db,
    FakeTime? time,
    OutboxExecutors? executors,
    int seed = 7,
    bool hookFails = false,
  }) {
    final t = time ?? FakeTime();
    final d = db ?? openInMemoryForTests(clock: t.clock);
    final registry = executors ?? OutboxExecutors();
    final random = Random(seed);
    final confirmed = <String>[];
    final outbox = Outbox(
      db: d,
      executors: registry,
      config: AppConfig.defaults().withOverrides(config),
      ids: Uuid7Generator(
        nowMillis: () => t.now.millisecondsSinceEpoch,
        random: (bits) => random.nextInt(1 << min(bits, 31)),
      ),
      timer: t.timer,
      random: Random(seed),
      onConfirmed: (entry, response) async {
        if (hookFails) throw StateError('storage failed');
        confirmed.add(entry.id);
      },
    );
    return Harness._(d, t, registry, outbox, confirmed);
  }

  /// The database.
  final LocalDatabase db;

  /// The fake clock.
  final FakeTime time;

  /// The registry.
  final OutboxExecutors executors;

  /// The outbox under test.
  final Outbox outbox;

  /// IDs passed to the confirmation hook, in order.
  final List<String> confirmed;

  /// Registers a scripted executor for [kind] and returns it.
  ScriptedExecutor fake(String kind, {OutboxLaneOf? lane}) {
    final e = ScriptedExecutor();
    executors.register(kind, e.call, lane: lane);
    return e;
  }

  /// Enqueues a message-like entry of [kind] in [thread] with the text [text].
  Future<OutboxEntry> add(
    String text, {
    ThreadId? thread,
    String kind = 'createMessage',
    List<String> dependsOn = const [],
  }) async {
    final e = await outbox.enqueue(
      OutboxRequest(
        kind: kind,
        threadId: thread,
        payload: {'content_md': text},
        dependsOn: dependsOn,
      ),
    );
    // Distinct creation times keep the FIFO order independent of the ID.
    time.now = time.now.add(const Duration(milliseconds: 1));
    return e;
  }

  /// The stored entry [id], or null.
  Future<OutboxEntry?> entry(String id) => db.outboxDao.byId(id);

  /// Closes outbox and database.
  Future<void> close() async {
    await outbox.close();
    await db.close();
  }
}
