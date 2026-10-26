// SPDX-License-Identifier: Apache-2.0

/// The live stream of the console: `getStatsLive` polled every `viz.live_poll_interval`
/// (docs/spec/25-visualization.md 5.1, 7.1; R-VZ-06; brief M6-16 Design note 4;
/// docs/decisions.d/M6-16.md D-M6-16-4).
library;

import 'dart:async';

import 'package:api_client/api_client.dart' as api;
import 'package:bothy_viz/bothy_viz.dart';

import '../platform/browser.dart';

/// [LiveFrameSource] of the console, which holds no gateway connection (15 3.1 rule 3; ADR
/// 0028): it fetches the frame of the scope through [queries] (`getStatsLive`) at once and then
/// every [pollInterval] (`viz.live_poll_interval`).
///
/// - A hidden tab ([visibility]) stops polling; it starts again at once when the tab is shown.
///   The stale rule of [LiveFrameSourceBase] still runs, so a tab shown again marks old data
///   "stale" until the next frame.
/// - `feature_disabled`, `forbidden` and `not_found` end polling and pass the code to the view,
///   which shows its text (25 11): retrying cannot change the answer.
/// - Any other failure (`rate_limited`, a network error, a server error) backs off: the wait
///   doubles from [pollInterval] up to [staleAfter] (`viz.stale_after`) and returns to
///   [pollInterval] after a frame; the view shows "stale" meanwhile.
///
/// Example:
/// ```dart
/// final live = PollingLiveFrameSource(
///   queries: ApiStatsQuerySource(controller.api),
///   clock: const SystemVizClock(),
///   pollInterval: config.livePollInterval,
///   staleAfter: config.staleAfter,
///   visibility: environment.visibility,
/// );
/// ```
final class PollingLiveFrameSource extends LiveFrameSourceBase {
  /// Creates the source.
  PollingLiveFrameSource({
    required this.queries,
    required super.clock,
    required this.pollInterval,
    required super.staleAfter,
    this.visibility,
    super.names,
  });

  /// The REST queries (`getStatsLive`).
  final StatsQuerySource queries;

  /// `viz.live_poll_interval`.
  final Duration pollInterval;

  /// The visibility of the tab, or null (always visible).
  final PageVisibility? visibility;

  /// The codes that end polling: the answer stays the same on a retry.
  static const Set<String> finalCodes = {
    'feature_disabled',
    'forbidden',
    'not_found',
  };

  String? _spaceId;
  bool _running = false;
  int _generation = 0;
  Duration _wait = Duration.zero;
  VizTimer? _next;
  StreamSubscription<bool>? _visibility;

  @override
  void onStart(String? spaceId) {
    _cancel();
    _spaceId = spaceId;
    _running = true;
    _wait = pollInterval;
    _visibility = visibility?.changes.listen((hidden) {
      if (!_running) return;
      if (hidden) {
        _next?.cancel();
        _next = null;
        _generation++;
      } else if (_next == null) {
        unawaited(_poll());
      }
    });
    if (!(visibility?.hidden ?? false)) unawaited(_poll());
  }

  @override
  void onStop() => _cancel();

  void _cancel() {
    _running = false;
    _generation++;
    _next?.cancel();
    _next = null;
    unawaited(_visibility?.cancel());
    _visibility = null;
  }

  Future<void> _poll() async {
    final generation = ++_generation;
    _next = null;
    try {
      final frame = await queries.live(spaceId: _spaceId);
      if (generation != _generation || !_running) return;
      _wait = pollInterval;
      emitFrame(frame);
    } on api.ApiProblem catch (e) {
      if (generation != _generation || !_running) return;
      if (finalCodes.contains(e.code)) {
        _cancel();
        emitError(e.code);
        return;
      }
      _backOff();
    } on Object {
      if (generation != _generation || !_running) return;
      _backOff();
    }
    if (generation != _generation || !_running) return;
    _schedule(generation);
  }

  void _backOff() {
    final doubled = _wait * 2;
    _wait = doubled > staleAfter ? staleAfter : doubled;
  }

  void _schedule(int generation) {
    if (visibility?.hidden ?? false) return;
    _next = clock.schedule(_wait, () {
      if (generation != _generation || !_running) return;
      unawaited(_poll());
    });
  }
}
