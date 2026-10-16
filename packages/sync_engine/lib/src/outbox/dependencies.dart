// SPDX-License-Identifier: Apache-2.0

/// The one place where the outbox decides what an entry waits for (docs/DECISIONS.md
/// D-M4-15-6, D-M4-15-9, D-M4-15-10). Internal; `Outbox` and `OutboxSender` share one
/// instance and call it only under their common lock.
library;

import 'package:local_store/local_store.dart';

/// The IDs [entry] waits for (stored comma-separated in `depends_on`, D-M4-15-6).
List<String> dependenciesOf(OutboxEntry entry) =>
    encodedDependencies(entry.dependsOn);

/// Splits the stored `depends_on` value [stored].
List<String> encodedDependencies(String? stored) =>
    stored == null || stored.isEmpty ? const [] : stored.split(',');

/// The failure code of an entry that failed because an entry it waited for was discarded.
const dependencyDiscarded = 'dependency_discarded';

/// Whether an entry it waits for lets [entry] go.
enum Readiness {
  /// Every dependency is confirmed.
  ready,

  /// A dependency is still stored and not confirmed.
  waiting,

  /// A dependency is gone without a confirmation (discarded, or never known): the entry must
  /// fail with [dependencyDiscarded] instead of going without it.
  broken,
}

/// The dependency graph of the outbox over every stored entry, final ones included.
///
/// Every row the outbox deletes leaves through [removeEntry], which re-points every stored
/// entry that names it: a confirmed entry is dropped from the lists, a replaced one (retry,
/// edit) becomes its replacement, a dropped follow-up becomes what it waited for, and a
/// discarded one is dropped after its dependents failed. So a stored `depends_on` only names
/// stored entries, and every path reads the same lists. The session record of gone IDs only
/// resolves IDs a caller names in `enqueue`; a missing ID is never taken as confirmed unless
/// it is recorded as confirmed.
final class OutboxDependencies {
  /// Creates the graph on [dao].
  OutboxDependencies(this._dao);

  final OutboxDao _dao;

  /// What became of a deleted ID in this run: the IDs that replace it (empty: confirmed), or
  /// null for a discarded entry.
  final Map<String, List<String>?> _gone = {};

  /// Every stored entry, final ones included, in FIFO order.
  Future<List<OutboxEntry>> all() => _dao.allEntries();

  /// The stored entries that name [id], final ones included.
  Future<List<OutboxEntry>> dependentsOf(String id) async => [
    for (final e in await all())
      if (dependenciesOf(e).contains(id)) e,
  ];

  /// Every stored entry that waits for [root], directly or through others, final ones
  /// included.
  Future<List<OutboxEntry>> chainOf(String root) async {
    final entries = await all();
    final found = <String, OutboxEntry>{};
    final open = [root];
    while (open.isNotEmpty) {
      final id = open.removeLast();
      for (final e in entries) {
        if (e.id != root &&
            dependenciesOf(e).contains(id) &&
            found.putIfAbsent(e.id, () => e) == e) {
          open.add(e.id);
        }
      }
    }
    return [
      for (final e in entries)
        if (found.containsKey(e.id)) e,
    ];
  }

  /// The resolver: what an entry that names [ids] waits for.
  ///
  /// Replacements of this run are followed and confirmed IDs are dropped. With
  /// [personRetries] (the replacement of `retry` and `edit`, a follow-up of `edit` and
  /// `remove`, the bridge over a dropped follow-up) a stored entry that failed with its chain
  /// is replaced by what it waits for, and a discarded or unknown ID is dropped: the person
  /// sends the entry again knowing the dependency is gone (D-M4-15-9). Without it (`enqueue`
  /// and the check before sending) both stay; a discarded or unknown ID then breaks the entry.
  Future<List<String>> resolve(
    List<String> ids, {
    required bool personRetries,
  }) async {
    final result = <String>[];
    final seen = <String>{};
    Future<void> visit(String id) async {
      if (!seen.add(id)) return;
      final stored = await _dao.byId(id);
      if (stored != null) {
        if (personRetries &&
            stored.state == OutboxState.failed &&
            stored.lastErrorCode == dependencyDiscarded) {
          for (final d in dependenciesOf(stored)) {
            await visit(d);
          }
        } else if (stored.state != OutboxState.confirmed) {
          result.add(id);
        }
        return;
      }
      if (_gone.containsKey(id)) {
        final into = _gone[id];
        if (into != null) {
          for (final d in into) {
            await visit(d);
          }
          return;
        }
      }
      if (!personRetries) result.add(id);
    }

    for (final id in ids) {
      await visit(id);
    }
    return result;
  }

  /// Whether [entry] may be sent now: run before every claim.
  Future<Readiness> readiness(OutboxEntry entry) async {
    var waiting = false;
    for (final id in await resolve(
      dependenciesOf(entry),
      personRetries: false,
    )) {
      if (await _dao.byId(id) == null) return Readiness.broken;
      waiting = true;
    }
    return waiting ? Readiness.waiting : Readiness.ready;
  }

  /// Fails every entry that is not final in the chain behind [root] with
  /// [dependencyDiscarded], so none of them waits for an entry that can never be confirmed;
  /// [before] runs for each of them first.
  Future<void> failChain(
    String root, {
    Future<void> Function(OutboxEntry entry)? before,
  }) async {
    for (final d in await chainOf(root)) {
      if (d.state.isFinal) continue;
      await before?.call(d);
      await _dao.recordFailure(d.id, code: dependencyDiscarded);
    }
  }

  /// Deletes entry [id]; every stored entry that names it names [into] instead.
  ///
  /// [into] is empty for a confirmed entry, the replacement for a retried or edited one and
  /// what a dropped follow-up waited for; null for a discarded entry, whose chain the caller
  /// has failed first ([failChain]).
  Future<void> removeEntry(String id, {required List<String>? into}) async {
    _gone[id] = into;
    for (final d in await dependentsOf(id)) {
      final next = <String>[];
      for (final x in dependenciesOf(d)) {
        for (final y in x == id ? (into ?? const <String>[]) : [x]) {
          if (y != d.id && !next.contains(y)) next.add(y);
        }
      }
      await _dao.updateDependsOn(d.id, next.isEmpty ? null : next.join(','));
    }
    await _dao.delete(id);
  }
}
