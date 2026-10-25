// SPDX-License-Identifier: Apache-2.0

/// The live stream of the apps: `LiveFrameSource` over the `stats.subscribe` subscription of
/// the sync engine (docs/spec/25-visualization.md 5.1, 7.1; docs/spec/15-client-architecture.md
/// 3.1 rule 6; brief M6-16 Design notes 2 and 3).
library;

import 'dart:async';

import 'package:bothy_viz/bothy_viz.dart';
import 'package:sync_engine/sync_engine.dart';

/// [LiveFrameSource] of the desktop and mobile app: [onStart] subscribes the gateway
/// connection to the scope ([StatsSubscription.subscribe]), each `stats.live` of that scope
/// becomes a frame, an `error {op: stats.subscribe}` becomes the error event with its code, and
/// [onStop] ends the subscription.
///
/// The stale rule is the one of [LiveFrameSourceBase]: no frame for `viz.stale_after` marks the
/// view stale. On mobile the connection closes in the background (15 7.2 `Suspended`), so the
/// server ends the subscription and the view turns stale; back in the foreground `resume`
/// restores it, or the engine subscribes again after `identify` (D-M6-16-3). The source keeps
/// nothing: a frame is shown and replaced, never stored.
///
/// Example:
/// ```dart
/// final live = EngineLiveFrameSource(
///   engine.stats,
///   clock: const SystemVizClock(),
///   staleAfter: config.staleAfter,
/// );
/// final events = live.start(spaceId: space);
/// ```
final class EngineLiveFrameSource extends LiveFrameSourceBase {
  /// Creates the source over [_subscription] (`SyncEngine.stats`).
  EngineLiveFrameSource(
    this._subscription, {
    required super.clock,
    required super.staleAfter,
    super.names,
  });

  final StatsSubscription _subscription;
  StreamSubscription<StatsLiveUpdate>? _updates;

  @override
  void onStart(String? spaceId) {
    unawaited(_updates?.cancel());
    _updates = _subscription.updates.listen(
      (update) => switch (update) {
        StatsLiveReceived(:final frame) => emitFrame(frame),
        StatsLiveEnded(:final code) => emitError(code),
      },
    );
    _subscription.subscribe(spaceId: spaceId);
  }

  @override
  void onStop() {
    unawaited(_updates?.cancel());
    _updates = null;
    _subscription.unsubscribe();
  }
}
