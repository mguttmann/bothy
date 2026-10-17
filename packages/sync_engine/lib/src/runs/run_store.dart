// SPDX-License-Identifier: Apache-2.0

/// `RunStore`: the live box documents of the focused runs, in memory only (15 7.1, 7.4).
library;

import 'dart:async';

import 'package:domain/domain.dart';

import 'json_patch.dart';
import 'ports.dart';
import 'run_document.dart';

/// Keeps the run document of every focused run in memory and applies `run.updated` frames by
/// the client rule of 04 11.3 item 4.
///
/// Rule per frame of a focused run, with `local` the version of the held document:
/// - `version` not newer than `local`: ignore (a duplicate or an older frame).
/// - `snapshot` present: replace the document (04 11.3 item 3).
/// - `base_version` equal to `local` and `ops` present: apply the ops ([applyRunPatch]); an
///   invalid patch is a gap.
/// - anything else (a gap, a truncated frame with only `run_id` and `version`, no document yet):
///   refetch the whole document through [RunDocumentFetcher] and replace the local one when the
///   fetched version is newer.
///
/// While a fetch of a run is in flight, frames with ops of that run wait in a queue; when the
/// fetch returns, the rule runs again for each of them in arrival order, so frames not newer
/// than the fetched version drop out (docs/DECISIONS.md D-M4-16-4). A snapshot applies at once.
/// A failed fetch keeps the last good document, drops the queue and retries on the next frame
/// or [focus]. A fetch that returns after [unfocus] or [onRunEnded] of its run is ignored.
///
/// [watch] publishes at most one state per frame of the [FrameTicker], the newest one
/// (15 7.4, docs/DECISIONS.md D-M4-16-3). Documents never reach the local database (15 5); the
/// collapsed row after the end comes from the table `runs`.
///
/// The store depends on no transport (docs/DECISIONS.md D-M4-16-1): M4-17 passes decoded
/// payloads of the dispatcher to [onRunUpdated], calls [focus] and [unfocus] with the focus of
/// the gateway and [onRunEnded] on `run.state_changed` to an end state.
///
/// Example:
/// ```dart
/// final store = RunStore(fetch: restFetcher, ticker: schedulerTicker);
/// store.focus(runId);
/// store.watch(runId).listen((doc) => notifier.state = doc);
/// store.onRunUpdated(payload); // from the dispatcher
/// ```
final class RunStore {
  /// Creates a store that loads documents through [fetch] and publishes on the frames of
  /// [ticker].
  RunStore({required RunDocumentFetcher fetch, required FrameTicker ticker})
    : _fetcher = fetch,
      _frames = ticker;

  final RunDocumentFetcher _fetcher;
  final FrameTicker _frames;

  final Map<RunId, _Run> _runs = {};
  final Map<RunId, Set<_Watcher>> _watchers = {};
  final Set<RunId> _dirty = {};
  bool _frameRequested = false;
  bool _disposed = false;

  // Each fetch takes a new number; a result counts only while its run still waits for exactly
  // this number, so unfocus, the end of the run or a newer fetch make it stale.
  int _nextGeneration = 0;

  /// The document of [id] as held now; null when the run is not focused or has no document yet.
  RunDocument? documentOf(RunId id) => _runs[id]?.document;

  /// Registers interest in the run [id] and fetches its whole document.
  ///
  /// Does nothing when the run is focused already and holds a document or a fetch is in
  /// flight; a focused run without a document (its last fetch failed) fetches again. A
  /// snapshot that arrives before the fetch returns is applied, and the fetch result then
  /// replaces it only when newer.
  void focus(RunId id) {
    if (_disposed) return;
    final run = _runs.putIfAbsent(id, _Run.new);
    if (run.document == null && !run.fetching) _refetch(id, run);
  }

  /// Drops the run [id]: its document, its queue and any fetch in flight.
  void unfocus(RunId id) => _drop(id);

  /// Discards the document of [id] at the end of the run (15 7.4, 04 11.3 item 6).
  ///
  /// Watchers receive null; the collapsed row comes from the run summary in the local
  /// database. Late frames and late fetch results of the run are ignored.
  void onRunEnded(RunId id) => _drop(id);

  /// Applies one decoded `run.updated` payload (API `RunUpdatedPayload`).
  ///
  /// [payload] holds `run_id`, `version` and either `base_version` with `ops`, or `snapshot`,
  /// or neither (truncated). A payload whose `run_id` is no UUID or whose `version` is no
  /// integer is ignored, as is a payload of a run that is not focused: the server sends
  /// patches only for focused threads, and a late frame after unfocus is harmless.
  void onRunUpdated(Map<String, Object?> payload) {
    if (_disposed) return;
    final id = _runIdOf(payload['run_id']);
    final version = payload['version'];
    if (id == null || version is! int) return;
    final run = _runs[id];
    if (run == null) return;
    _applyRule(id, run, payload, ResourceVersion(version));
  }

  /// Streams the newest document of [id], at most once per frame.
  ///
  /// The first event comes at the next frame after listening and holds the current document,
  /// or null when there is none; later events follow changes, one per frame with the newest
  /// state. Null means the run is not focused, has ended or has no document yet. Several
  /// listeners may watch one run. Throws a [StateError] after [dispose].
  Stream<RunDocument?> watch(RunId id) {
    if (_disposed) throw StateError('RunStore is disposed');
    late final _Watcher watcher;
    final controller = StreamController<RunDocument?>(
      // Registered on listen, so a stream nobody listens to holds nothing and [dispose] never
      // waits for its done event.
      onListen: () {
        if (_disposed) return;
        (_watchers[id] ??= {}).add(watcher);
        _markDirty(id);
      },
      onCancel: () {
        final set = _watchers[id];
        set?.remove(watcher);
        if (set != null && set.isEmpty) _watchers.remove(id);
      },
    );
    watcher = _Watcher(controller);
    return controller.stream;
  }

  /// Closes every stream and drops every document; later calls do nothing.
  Future<void> dispose() async {
    if (_disposed) return;
    _disposed = true;
    _runs.clear();
    _dirty.clear();
    final controllers = [
      for (final set in _watchers.values)
        for (final watcher in set) watcher.controller,
    ];
    _watchers.clear();
    await Future.wait(controllers.map((c) => c.close()));
  }

  void _applyRule(
    RunId id,
    _Run run,
    Map<String, Object?> payload,
    ResourceVersion version,
  ) {
    final local = run.document?.version;
    if (local != null && version <= local) return;

    if (payload.containsKey('snapshot')) {
      final snapshot = payload['snapshot'];
      if (snapshot is Map<String, Object?>) {
        // The frame's version is the version of the snapshot (04 11.3 item 3).
        _replace(id, run, {...snapshot, 'version': version.value});
        return;
      }
      _gap(id, run);
      return;
    }

    if (run.fetching) {
      run.queue.add(payload);
      return;
    }

    final base = payload['base_version'];
    final ops = payload['ops'];
    if (local == null ||
        base is! int ||
        ResourceVersion(base) != local ||
        ops == null) {
      _gap(id, run);
      return;
    }
    final Map<String, Object?> next;
    try {
      next = applyRunPatch(run.document!.raw, _opsOf(ops));
    } on RunPatchError {
      // An invalid patch is a gap, never a partial state (docs/DECISIONS.md D-M4-16-2).
      _gap(id, run);
      return;
    }
    // The result's root map is new and held only here, so writing the version is safe.
    next['version'] = version.value;
    _replace(id, run, next);
  }

  void _gap(RunId id, _Run run) {
    if (!run.fetching) _refetch(id, run);
  }

  void _refetch(RunId id, _Run run) {
    final generation = ++_nextGeneration;
    run.fetching = true;
    run.generation = generation;
    unawaited(_load(id, generation));
  }

  Future<void> _load(RunId id, int generation) async {
    Map<String, Object?>? fetched;
    try {
      fetched = await _fetcher.fetchRun(id);
    } on Object {
      fetched = null;
    }
    final run = _runs[id];
    if (_disposed || run == null || run.generation != generation) return;
    run.fetching = false;
    final queued = List.of(run.queue);
    run.queue.clear();
    final RunDocument document;
    try {
      if (fetched == null) throw const FormatException('fetch failed');
      document = RunDocument.fromJson(fetched);
    } on FormatException {
      // Keep the last good document; the next frame or focus retries. The queue goes too,
      // otherwise a fetch that always fails would refetch for every queued frame forever.
      return;
    }
    final local = run.document?.version;
    if (local == null || document.version > local) {
      _setDocument(id, run, document);
    }
    for (final payload in queued) {
      final version = payload['version']! as int;
      _applyRule(id, run, payload, ResourceVersion(version));
    }
  }

  void _replace(RunId id, _Run run, Map<String, Object?> json) {
    _setDocument(id, run, RunDocument.fromJson(json));
  }

  void _setDocument(RunId id, _Run run, RunDocument document) {
    run.document = document;
    _markDirty(id);
  }

  void _drop(RunId id) {
    if (_disposed) return;
    if (_runs.remove(id) != null) _markDirty(id);
  }

  void _markDirty(RunId id) {
    if (_disposed) return;
    _dirty.add(id);
    if (_frameRequested) return;
    _frameRequested = true;
    _frames.requestFrame(_onFrame);
  }

  void _onFrame() {
    _frameRequested = false;
    if (_disposed) return;
    final due = List.of(_dirty);
    _dirty.clear();
    for (final id in due) {
      final document = _runs[id]?.document;
      for (final watcher in List.of(_watchers[id] ?? const <_Watcher>{})) {
        watcher.publish(document);
      }
    }
  }
}

/// State of one focused run.
final class _Run {
  RunDocument? document;
  bool fetching = false;
  int generation = 0;
  final List<Map<String, Object?>> queue = [];
}

/// One listener of [RunStore.watch].
final class _Watcher {
  _Watcher(this.controller);

  final StreamController<RunDocument?> controller;
  bool _published = false;
  RunDocument? _last;

  /// Adds [document] unless this listener has already seen exactly this state.
  void publish(RunDocument? document) {
    if (_published && identical(_last, document)) return;
    _published = true;
    _last = document;
    controller.add(document);
  }
}

RunId? _runIdOf(Object? value) {
  if (value is! String) return null;
  try {
    return RunId.parse(value);
  } on FormatException {
    return null;
  }
}

/// Checks that [ops] is a list of objects; anything else is an invalid patch.
List<Map<String, Object?>> _opsOf(Object ops) {
  if (ops is! List<Object?>) throw const RunPatchError(0, 'ops is not a list');
  final result = <Map<String, Object?>>[];
  for (var i = 0; i < ops.length; i++) {
    final op = ops[i];
    if (op is! Map<String, Object?>) {
      throw RunPatchError(i, 'operation is not an object');
    }
    result.add(op);
  }
  return result;
}
