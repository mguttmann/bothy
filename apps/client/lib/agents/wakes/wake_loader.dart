// SPDX-License-Identifier: Apache-2.0

/// Where a thread view loads its wake-ups when it opens (`listThreadWakes`, 09 5.8.2; brief
/// AR-08): the engine stores them in the local table, which `run.wake_changed` keeps current
/// afterwards. Tests replace the port.
library;

import 'package:domain/domain.dart';
import 'package:sync_engine/sync_engine.dart';

/// Loads the wake-ups of a thread into the local table.
abstract interface class WakeLoader {
  /// Loads the wake-ups of [thread]; errors are the caller's to swallow (the view keeps what
  /// the table holds).
  Future<void> load(ThreadId thread);
}

/// [WakeLoader] through `CatchUpListener.loadThreadWakes` of [engine].
final class EngineWakeLoader implements WakeLoader {
  /// Creates the loader on [engine].
  const EngineWakeLoader(this.engine);

  /// The engine.
  final SyncEngine engine;

  @override
  Future<void> load(ThreadId thread) => engine.catchUp.loadThreadWakes(thread);
}

/// The loader without an engine (before the login): loads nothing.
final class NoWakeLoader implements WakeLoader {
  /// Creates the loader.
  const NoWakeLoader();

  @override
  Future<void> load(ThreadId thread) async {}
}
