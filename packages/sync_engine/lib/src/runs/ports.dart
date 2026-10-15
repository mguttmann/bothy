// SPDX-License-Identifier: Apache-2.0

/// Ports of the run store: how it fetches a whole run document and how it learns about the
/// next frame. The store depends on no transport (docs/DECISIONS.md D-M4-16-1); M4-17 wires
/// the fetcher to `GET /api/v1/runs/{id}` and the app passes a ticker on its frame scheduler.
library;

import 'package:domain/domain.dart';

/// Fetches the whole run document (`run_document.v1`, API schema `Run`) of one run.
///
/// The production implementation calls `GET /api/v1/runs/{id}` (operation `getRun`) and
/// returns the decoded JSON body; tests pass a fake that counts calls.
abstract interface class RunDocumentFetcher {
  /// Returns the decoded document of the run [id].
  ///
  /// The future completes with an error when the run cannot be loaded (network, permission,
  /// not found); the store then keeps the last good document and retries on the next patch or
  /// focus (04 11.3 item 4).
  Future<Map<String, Object?>> fetchRun(RunId id);
}

/// Calls back once at the next frame the interface draws.
///
/// The app implements it with `SchedulerBinding.instance.scheduleFrameCallback` (and
/// `scheduleFrame`), so the store publishes at most one state per drawn frame (15 7.4); tests
/// pass a ticker that runs the callbacks when the test says so.
abstract interface class FrameTicker {
  /// Runs [onFrame] once at the next frame.
  ///
  /// The store calls this only while it holds unpublished changes and at most once until the
  /// callback has run, so an idle store requests no frames.
  void requestFrame(void Function() onFrame);
}
