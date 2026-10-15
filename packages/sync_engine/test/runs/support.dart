// SPDX-License-Identifier: Apache-2.0

/// Fixtures and fakes of the run store tests: documents at versions 1 to 4, the patch frames
/// between them, a fetcher that counts calls and answers when the test says so, and a frame
/// ticker that runs its callbacks only on [ManualFrameTicker.tick].
library;

import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:domain/domain.dart';
import 'package:sync_engine/sync_engine.dart';

/// ID of the run of the fixtures.
final RunId runId = RunId.parse('0192f4a0-7b1c-7000-8a00-000000000001');

/// ID of a second run that is never focused in the fixtures.
final RunId otherRunId = RunId.parse('0192f4a0-7b1c-7000-8a00-000000000009');

/// Step IDs of the fixtures.
const String stepA1 = '0192f4a0-7b1c-7000-8a00-0000000000a1';

/// Second tool call.
const String stepA2 = '0192f4a0-7b1c-7000-8a00-0000000000a2';

/// Subagent step.
const String stepA3 = '0192f4a0-7b1c-7000-8a00-0000000000a3';

/// Tool call inside the subagent.
const String stepA4 = '0192f4a0-7b1c-7000-8a00-0000000000a4';

/// The follow-up of the fixtures.
const String followupB1 = '0192f4a0-7b1c-7000-8a00-0000000000b1';

Map<String, Object?> _read(String name) =>
    jsonDecode(File('test/runs/fixtures/$name.json').readAsStringSync())
        as Map<String, Object?>;

/// The run document at [version] (1 to 4), freshly decoded.
Map<String, Object?> doc(int version) => _read('run_v$version');

/// The `run.updated` payload from [base] to `base + 1` (1 to 3), freshly decoded.
Map<String, Object?> patch(int base) => _read('patch_v${base}_v${base + 1}');

/// A `run.updated` payload with `snapshot` holding the document at [version].
Map<String, Object?> snapshotFrame(int version) => {
  'run_id': runId.value,
  'version': version,
  'snapshot': doc(version),
};

/// A truncated `run.updated` payload: only `run_id` and `version`.
Map<String, Object?> truncatedFrame(int version) => {
  'run_id': runId.value,
  'version': version,
};

/// A fetcher whose answers the test releases one by one; counts every call.
final class FakeFetcher implements RunDocumentFetcher {
  final List<(RunId, Completer<Map<String, Object?>>)> _pending = [];

  /// Number of calls so far.
  int calls = 0;

  /// IDs of the calls in order.
  final List<RunId> ids = [];

  /// Number of calls that have not been answered yet.
  int get pending => _pending.length;

  @override
  Future<Map<String, Object?>> fetchRun(RunId id) {
    calls++;
    ids.add(id);
    final completer = Completer<Map<String, Object?>>();
    _pending.add((id, completer));
    return completer.future;
  }

  /// Answers the oldest open call with [document] and lets the store react.
  Future<void> respond(Map<String, Object?> document) async {
    _pending.removeAt(0).$2.complete(document);
    await pumpQueue();
  }

  /// Fails the oldest open call and lets the store react.
  Future<void> fail() async {
    _pending.removeAt(0).$2.completeError(StateError('network down'));
    await pumpQueue();
  }
}

/// A ticker that runs the requested frame callbacks only when the test calls [tick].
final class ManualFrameTicker implements FrameTicker {
  final List<void Function()> _callbacks = [];

  /// Number of frame requests so far.
  int requests = 0;

  @override
  void requestFrame(void Function() onFrame) {
    requests++;
    _callbacks.add(onFrame);
  }

  /// Runs every requested callback once and lets the listeners receive their events.
  Future<void> tick() async {
    final due = List.of(_callbacks);
    _callbacks.clear();
    for (final callback in due) {
      callback();
    }
    await pumpQueue();
  }
}

/// Lets pending microtasks and stream events run.
Future<void> pumpQueue() async {
  for (var i = 0; i < 20; i++) {
    await Future<void>.delayed(Duration.zero);
  }
}

/// Collects what [RunStore.watch] emits for [id].
final class Recorder {
  /// Starts listening on [store] for [id].
  Recorder(RunStore store, RunId id) {
    _sub = store.watch(id).listen(events.add);
  }

  late final StreamSubscription<RunDocument?> _sub;

  /// Everything emitted so far.
  final List<RunDocument?> events = [];

  /// Stops listening.
  Future<void> cancel() => _sub.cancel();
}

/// A store with a fake fetcher and a manual ticker, focused on [runId] and holding the
/// document at [version] after the initial fetch.
Future<(RunStore, FakeFetcher, ManualFrameTicker)> focusedStore({
  int version = 1,
}) async {
  final fetcher = FakeFetcher();
  final ticker = ManualFrameTicker();
  final store = RunStore(fetch: fetcher, ticker: ticker);
  store.focus(runId);
  await fetcher.respond(doc(version));
  return (store, fetcher, ticker);
}
