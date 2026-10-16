// SPDX-License-Identifier: Apache-2.0

// A seeded randomized model test of the dependency rules of the outbox (docs/DECISIONS.md
// D-M4-15-10). Every sequence mixes, across threads and upload lanes, in random order:
// enqueue with dependencies, success, an own failure (`422`), a retry class answer (`503`),
// answers held back and given later, the echo, retry, edit, remove, discard, the passing of
// time and a restart of the outbox on the same database. A model of its own follows what
// every entry really depends on (replacements by retry and edit followed, discarded and
// confirmed entries recorded) and checks after every step:
//
// (a) nothing is handed to an executor while one of its dependencies is not confirmed;
// (b) no entry that is not final waits for an ID that no longer exists, unless that
//     dependency was confirmed;
// (c) once every answer is a success and the person retries or discards everything that
//     failed, the outbox empties (no permanent lock).
//
// OUTBOX_MODEL_SEEDS sets the number of sequences (default 500), OUTBOX_MODEL_FIRST_SEED the
// first seed (default 1). A failure names the seed and prints the whole sequence.

import 'dart:async';
import 'dart:io';
import 'dart:math';

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

import 'support.dart';

const _chainFailed = 'dependency_discarded';

/// Lets the outbox finish what does not wait for a timer or a held answer; shorter than
/// `settle` of the other tests, which keeps thousands of sequences fast.
Future<void> _settle() => pumpEventQueue(times: 40);

List<String> _deps(OutboxEntry e) => e.dependsOn == null || e.dependsOn!.isEmpty
    ? const []
    : e.dependsOn!.split(',');

/// Runs the sequence of [seed]; returns null or the report of the first broken invariant.
Future<String?> runModelSequence(int seed) async {
  final run = _Run(seed);
  try {
    await run.play();
    return null;
  } on _Violation catch (v) {
    return run.report(v.message);
  } finally {
    await run.close();
  }
}

void main() {
  final env = Platform.environment;
  final count = int.tryParse(env['OUTBOX_MODEL_SEEDS'] ?? '') ?? 500;
  final first = int.tryParse(env['OUTBOX_MODEL_FIRST_SEED'] ?? '') ?? 1;

  test(
    'dependency invariants hold for $count random sequences from seed $first',
    () async {
      for (var seed = first; seed < first + count; seed++) {
        final failure = await runModelSequence(seed);
        if (failure != null) fail(failure);
      }
    },
    timeout: Timeout.none,
  );
}

final class _Violation implements Exception {
  _Violation(this.message);
  final String message;
}

/// One executor call: the entry and the confirmed IDs at that moment.
typedef _Sent = (String id, Set<String> confirmedThen);

final class _Run {
  _Run(this.seed) : rnd = Random(seed);

  final int seed;
  final Random rnd;
  final FakeTime time = FakeTime();
  late final LocalDatabase db = openInMemoryForTests(clock: time.clock);
  final OutboxExecutors executors = OutboxExecutors();
  late Outbox outbox;
  int generation = 0;
  bool draining = false;
  final List<String> trace = [];

  // The model.
  final Map<String, List<String>> logical = {};
  final Map<String, String> kindOf = {};
  final Set<String> confirmed = {};
  final Set<String> discarded = {};
  final Map<String, String> replacedBy = {};
  final Set<String> bridged = {};

  /// IDs that were gone before the last restart: a new outbox has no record of them, so an
  /// entry that names one in `enqueue` fails and is never sent until the person sends it
  /// again (D-M4-15-10, the record of gone IDs lives as long as the outbox).
  final Set<String> forgotten = {};
  final List<(String, Completer<OutboxResponse>)> held = [];
  final List<_Sent> sent = [];
  final Map<String, String> names = {};

  static final List<ThreadId?> threads = [threadA, threadB, null];

  String name(String id) => names[id] ?? '?${id.substring(id.length - 4)}';

  void learn(String id, String kind, List<String> deps) {
    names.putIfAbsent(id, () => 'e${names.length + 1}');
    kindOf[id] = kind;
    logical[id] = deps;
  }

  String fate(String id) {
    if (forgotten.contains(id)) return 'gone before a restart';
    if (confirmed.contains(id)) return 'confirmed';
    if (discarded.contains(id)) return 'discarded';
    final next = replacedBy[id];
    if (next != null) return 'replaced by ${name(next)}';
    if (bridged.contains(id)) return 'dropped follow-up';
    return 'open';
  }

  void openOutbox() {
    generation++;
    final idRandom = Random(seed * 7919 + generation);
    outbox = Outbox(
      db: db,
      executors: executors,
      config: AppConfig.defaults(),
      ids: Uuid7Generator(
        nowMillis: () => time.now.millisecondsSinceEpoch,
        random: (bits) => idRandom.nextInt(1 << min(bits, 31)),
      ),
      timer: time.timer,
      random: Random(seed),
      onConfirmed: (entry, response) async => confirmed.add(entry.id),
    );
  }

  Future<OutboxResponse> send(OutboxEntry e, ApiCallContext ctx) async {
    sent.add((e.id, {...confirmed}));
    final r = draining ? 0 : rnd.nextInt(100);
    if (r < 45) return const OutboxResponse();
    if (r < 60) throw problem(422, 'validation_failed');
    if (r < 75) throw problem(503, 'unavailable');
    final c = Completer<OutboxResponse>();
    held.add((e.id, c));
    return c.future;
  }

  Future<void> close() async {
    for (final (_, c) in held) {
      if (!c.isCompleted) c.complete(const OutboxResponse());
    }
    await _settle();
    await outbox.close();
    await db.close();
  }

  Future<Map<String, OutboxEntry>> stored() async => {
    for (final t in threads)
      for (final e in await db.outboxDao.entriesFor(t)) e.id: e,
  };

  // ---- the model's rules -------------------------------------------------------------------

  /// What an entry waits for: [ids] with replacements followed, dropped follow-ups bridged
  /// and confirmed entries removed. For a replacement the person asked for ([forRetry]) a
  /// discarded dependency is dropped and an entry that failed with its chain is replaced by
  /// what it waited for (D-M4-15-9); otherwise both stay.
  List<String> resolve(
    List<String> ids,
    Map<String, OutboxEntry> table, {
    required bool forRetry,
  }) {
    final out = <String>[];
    final seen = <String>{};
    void visit(String d) {
      if (!seen.add(d)) return;
      if (forgotten.contains(d) && !table.containsKey(d)) {
        if (!forRetry) out.add(d);
        return;
      }
      if (confirmed.contains(d)) return;
      final next = replacedBy[d];
      if (next != null) return visit(next);
      if (bridged.contains(d)) {
        (logical[d] ?? const []).forEach(visit);
        return;
      }
      if (discarded.contains(d)) {
        if (!forRetry) out.add(d);
        return;
      }
      final s = table[d];
      if (forRetry &&
          s != null &&
          s.state == OutboxState.failed &&
          s.lastErrorCode == _chainFailed) {
        (logical[d] ?? const []).forEach(visit);
        return;
      }
      out.add(d);
    }

    ids.forEach(visit);
    return out;
  }

  void rewrite(String from, List<String> to) {
    for (final k in logical.keys) {
      final list = logical[k]!;
      if (!list.contains(from)) continue;
      final next = <String>[];
      for (final d in list) {
        for (final x in d == from ? to : [d]) {
          if (!next.contains(x)) next.add(x);
        }
      }
      logical[k] = next;
    }
  }

  Set<String> dependentsClosure(String root) {
    final found = <String>{};
    final open = [root];
    while (open.isNotEmpty) {
      final id = open.removeLast();
      for (final MapEntry(:key, :value) in logical.entries) {
        if (value.contains(id) && found.add(key)) open.add(key);
      }
    }
    return found;
  }

  // ---- checks ------------------------------------------------------------------------------

  Never violate(String message) => throw _Violation(message);

  Future<void> check() async {
    await _settle();
    // (a) every executor call since the last check.
    for (final (id, confirmedThen) in sent) {
      final deps = logical[id];
      if (deps == null) violate('(a) an unknown entry ${name(id)} was sent');
      if (fate(id) != 'open' && !confirmed.contains(id)) {
        violate('(a) ${name(id)} was sent although it is ${fate(id)}');
      }
      final open = [...deps];
      final seen = <String>{};
      while (open.isNotEmpty) {
        final d = open.removeLast();
        if (!seen.add(d) || confirmedThen.contains(d)) continue;
        violate(
          '(a) ${name(id)} was sent while its dependency ${name(d)} '
          '(${fate(d)}) was not confirmed',
        );
      }
    }
    sent.clear();
    // (b) no open entry waits for an ID that is gone without being confirmed.
    final table = await stored();
    for (final e in table.values) {
      if (e.state.isFinal) continue;
      for (final d in _deps(e)) {
        if (!table.containsKey(d) && !confirmed.contains(d)) {
          violate(
            '(b) ${name(e.id)} waits for ${name(d)} (${fate(d)}), which no '
            'longer exists and was not confirmed',
          );
        }
      }
    }
  }

  // ---- steps -------------------------------------------------------------------------------

  Future<void> step(String label, Future<void> Function() action) async {
    trace.add('${trace.length + 1}. $label');
    await action();
    await check();
  }

  T pick<T>(List<T> items) => items[rnd.nextInt(items.length)];

  Future<void> enqueue(Map<String, OutboxEntry> table) async {
    if (table.length >= 14) return;
    final upload = rnd.nextInt(4) == 0;
    if (upload) {
      final e = await outbox.enqueue(
        const OutboxRequest(kind: 'upload', payload: {'file': 'f'}),
      );
      learn(e.id, 'upload', const []);
      trace.add('${trace.length + 1}. enqueue upload ${name(e.id)}');
      await check();
      return;
    }
    final thread = pick(threads);
    final roll = rnd.nextInt(10);
    final count = roll < 4 ? 0 : (roll < 8 ? 1 : 2);
    final named = <String>[];
    for (var i = 0; i < count; i++) {
      final pool = rnd.nextInt(7) == 0 || table.isEmpty
          ? logical.keys.toList()
          : table.keys.toList();
      if (pool.isEmpty) break;
      final d = pick(pool);
      if (!named.contains(d)) named.add(d);
    }
    final resolved = resolve(named, table, forRetry: false);
    final e = await outbox.enqueue(
      OutboxRequest(
        kind: 'op',
        threadId: thread,
        payload: const {'v': 0},
        dependsOn: named,
      ),
    );
    learn(e.id, 'op', resolved);
    trace.add(
      '${trace.length + 1}. enqueue ${name(e.id)} in '
      '${thread == null ? 'no thread' : (thread == threadA ? 'A' : 'B')} after '
      '[${[for (final d in named) '${name(d)} (${fate(d)})'].join(', ')}]',
    );
    await check();
  }

  Future<void> complete({bool succeed = false}) async {
    final open = [
      for (final h in held)
        if (!h.$2.isCompleted) h,
    ];
    if (open.isEmpty) return;
    final (id, c) = pick(open);
    final r = succeed ? 0 : rnd.nextInt(3);
    await step(
      'answer held ${name(id)}: ${['success', '422', '503'][r]}',
      () async {
        switch (r) {
          case 0:
            c.complete(const OutboxResponse());
          case 1:
            c.completeError(problem(422, 'validation_failed'));
          default:
            c.completeError(problem(503, 'unavailable'));
        }
      },
    );
  }

  /// Retry or edit of the failed [x]; the model replaces it and bridges dropped follow-ups.
  Future<void> replace(
    OutboxEntry x,
    Map<String, OutboxEntry> before, {
    required bool viaEdit,
  }) async {
    final label = viaEdit ? 'edit' : 'retry';
    final fresh = viaEdit
        ? await outbox.edit(x.id, const {'v': 1})
        : await outbox.retry(x.id);
    final deps = resolve(logical[x.id]!, before, forRetry: true);
    final chain = dependentsClosure(x.id);
    learn(fresh, x.kind, deps);
    replacedBy[x.id] = fresh;
    rewrite(x.id, [fresh]);
    final after = await stored();
    final dropped = [
      for (final id in before.keys)
        if (id != x.id && !after.containsKey(id) && !confirmed.contains(id)) id,
    ];
    for (final o in dropped) {
      final e = before[o]!;
      final obsolete =
          viaEdit &&
          e.kind == 'opEdit' &&
          e.attempts == 0 &&
          !e.state.isFinal &&
          chain.contains(o);
      if (!obsolete) {
        violate('$label of ${name(x.id)} deleted ${name(o)} (${e.state.code})');
      }
      bridged.add(o);
      rewrite(o, logical[o] ?? const []);
    }
    trace.add(
      '${trace.length + 1}. $label ${name(x.id)} -> ${name(fresh)}'
      '${dropped.isEmpty ? '' : ', dropped ${dropped.map(name).join(', ')}'}',
    );
    await check();
  }

  Future<void> discard(OutboxEntry x) async {
    await step('discard ${name(x.id)} (${x.state.code})', () async {
      await outbox.discard(x.id);
      discarded.add(x.id);
    });
  }

  Future<void> followUp(OutboxEntry x, {required bool edit}) async {
    final label = edit ? 'edit' : 'remove';
    String? id;
    try {
      id = edit
          ? await outbox.edit(x.id, const {'v': 2})
          : await outbox.remove(x.id);
    } on StateError {
      if (x.kind == 'op') {
        violate('$label of ${name(x.id)} (${x.kind}) threw');
      }
      trace.add('${trace.length + 1}. $label ${name(x.id)} refused');
      return;
    }
    if (id != null && !logical.containsKey(id)) {
      final e = (await db.outboxDao.byId(id))!;
      learn(id, e.kind, _deps(e));
    }
    trace.add(
      '${trace.length + 1}. $label ${name(x.id)} (${x.state.code}, attempts '
      '${x.attempts}) -> ${id == null ? '-' : name(id)}',
    );
    await check();
  }

  Future<void> randomStep() async {
    final table = await stored();
    final entries = table.values.toList();
    final roll = rnd.nextInt(100);
    if (roll < 28 || entries.isEmpty) return enqueue(table);
    if (roll < 40) {
      final d = Duration(seconds: rnd.nextInt(600));
      return step('advance ${d.inSeconds}s', () => time.advance(d));
    }
    if (roll < 52) return complete();
    final x = pick(entries);
    if (roll < 62) {
      if (x.state != OutboxState.failed) return;
      return replace(x, table, viaEdit: false);
    }
    if (roll < 72) {
      if (x.state == OutboxState.failed) {
        return replace(x, table, viaEdit: true);
      }
      return followUp(x, edit: true);
    }
    if (roll < 80) {
      if (x.state == OutboxState.failed || x.attempts == 0) {
        await step('remove ${name(x.id)} (${x.state.code})', () async {
          await outbox.remove(x.id);
          discarded.add(x.id);
        });
        return;
      }
      return followUp(x, edit: false);
    }
    if (roll < 88) return discard(x);
    if (roll < 96) {
      if (x.attempts == 0 || x.state == OutboxState.failed) return;
      return step('echo ${name(x.id)}', () async {
        if (await outbox.confirmByEcho(x.id)) confirmed.add(x.id);
      });
    }
    return step('restart', () async {
      final table = await stored();
      forgotten.addAll([
        for (final id in logical.keys)
          if (!table.containsKey(id)) id,
      ]);
      await outbox.close();
      openOutbox();
      await outbox.start();
    });
  }

  Future<void> play() async {
    executors
      ..register(
        'op',
        send,
        followUpEdit: (e, edited) => OutboxRequest(
          kind: 'opEdit',
          threadId: e.threadId,
          payload: {...edited},
        ),
        followUpRemove: (e) => OutboxRequest(
          kind: 'opRemove',
          threadId: e.threadId,
          payload: const {},
        ),
      )
      ..register('opEdit', send)
      ..register('opRemove', send)
      ..register('upload', send, lane: (e) => 'upload:${e.id}');
    openOutbox();
    await outbox.start();
    final steps = 15 + rnd.nextInt(45);
    for (var i = 0; i < steps; i++) {
      await randomStep();
    }
    // (c) every answer succeeds now; the person retries or discards everything that failed.
    draining = true;
    trace.add(
      '-- drain: every answer succeeds, every failed entry is retried or discarded',
    );
    for (var round = 0; round < 40; round++) {
      while (held.any((h) => !h.$2.isCompleted)) {
        await complete(succeed: true);
      }
      var table = await stored();
      if (table.isEmpty) return;
      for (final id in table.keys.toList()) {
        table = await stored();
        final x = table[id];
        if (x == null || x.state != OutboxState.failed) continue;
        if (rnd.nextInt(10) < 7) {
          await replace(x, table, viaEdit: false);
        } else {
          await discard(x);
        }
      }
      await step('advance 1h', () => time.advance(const Duration(hours: 1)));
    }
    final left = await stored();
    violate(
      '(c) the outbox never empties; left: '
      '${[for (final e in left.values) '${name(e.id)} ${e.kind} ${e.state.code} '
            '${e.lastErrorCode ?? ''} after [${_deps(e).map((d) => '${name(d)} (${fate(d)})').join(', ')}]'].join('; ')}',
    );
  }

  String report(String message) {
    return 'seed $seed: $message\n'
        'sequence:\n  ${trace.join('\n  ')}\n'
        'rerun: OUTBOX_MODEL_FIRST_SEED=$seed OUTBOX_MODEL_SEEDS=1 '
        'dart test test/outbox/dependency_model_test.dart';
  }
}
