// SPDX-License-Identifier: Apache-2.0

/// The port of the live stream (docs/spec/15-client-architecture.md 3.1 rule 6; 25 5.1).
library;

import 'dart:async';

import 'package:api_client/api_client.dart' as api;
import 'package:flutter/foundation.dart';

import '../model/live_topology_model.dart';
import '../model/names.dart';
import 'viz_clock.dart';

/// Error codes of the live stream that the views tell apart (25 11).
enum LiveFrameErrorCode {
  /// `forbidden`: the user may not see the statistics of this scope.
  forbidden,

  /// `not_found`: the space does not exist or is not visible.
  notFound,

  /// `feature_disabled`: the instance switched the statistics off.
  featureDisabled,

  /// `rate_limited`: too many requests; the source retries later.
  rateLimited,

  /// Any other code; [LiveFrameError.code] keeps it.
  other;

  /// The value for the wire code [code].
  static LiveFrameErrorCode of(String code) => switch (code) {
    'forbidden' => forbidden,
    'not_found' => notFound,
    'feature_disabled' => featureDisabled,
    'rate_limited' => rateLimited,
    _ => other,
  };
}

/// One event of a [LiveFrameSource].
sealed class LiveFrameEvent {
  const LiveFrameEvent();

  /// A new frame with its derived model.
  const factory LiveFrameEvent.frame(
    api.StatsLiveFrame frame,
    LiveTopologyModel model,
  ) = LiveFrameFrame;

  /// No frame arrived for `viz.stale_after`: the view marks its data as stale.
  const factory LiveFrameEvent.stale() = LiveFrameStale;

  /// The stream failed with the wire [code] of a problem.
  factory LiveFrameEvent.error(String code) = LiveFrameError;
}

/// A frame of the stream.
final class LiveFrameFrame extends LiveFrameEvent {
  /// Creates the event.
  const LiveFrameFrame(this.frame, this.model);

  /// The frame as the server sent it, for the views that derive their own model.
  final api.StatsLiveFrame frame;

  /// The live topology of [frame].
  final LiveTopologyModel model;
}

/// No frame for `viz.stale_after`.
final class LiveFrameStale extends LiveFrameEvent {
  /// Creates the event.
  const LiveFrameStale();
}

/// An error of the stream.
final class LiveFrameError extends LiveFrameEvent {
  /// Creates the event for the wire [code].
  LiveFrameError(this.code) : kind = LiveFrameErrorCode.of(code);

  /// The wire code, for example `forbidden`.
  final String code;

  /// The known kind of [code].
  final LiveFrameErrorCode kind;
}

/// The port through which the apps and the console feed the live views.
///
/// [start] returns a fresh stream for the scope [spaceId] (`null` for the instance); a new
/// [start] closes the previous stream, so a view switching the space never sees frames of the
/// old one. [stop] closes the current stream. Implementations differ only in how frames
/// arrive (stream event, polling, fixtures); the stale rule lives in [LiveFrameSourceBase].
abstract interface class LiveFrameSource {
  /// Starts the stream for [spaceId] and returns its events.
  Stream<LiveFrameEvent> start({String? spaceId});

  /// Stops the current stream.
  Future<void> stop();
}

/// The shared part of every [LiveFrameSource]: the stream per [start] and the stale rule.
///
/// After [start] and after every frame the base waits [staleAfter] (`viz.stale_after`) on
/// [clock]; without a frame in that time it emits one [LiveFrameStale] until the next frame.
/// Subclasses implement [onStart] and [onStop] and report through [emitFrame] and [emitError].
abstract class LiveFrameSourceBase implements LiveFrameSource {
  /// Creates the base with [clock], [staleAfter] and the [names] of the models.
  LiveFrameSourceBase({
    required this.clock,
    required this.staleAfter,
    this.names = const VizNames(),
  });

  /// The clock of the stale rule.
  final VizClock clock;

  /// `viz.stale_after` of the app configuration.
  final Duration staleAfter;

  /// Display names for the models.
  final VizNames names;

  StreamController<LiveFrameEvent>? _controller;
  VizTimer? _staleTimer;
  bool _stale = false;

  /// Whether a stream is open.
  bool get isRunning => _controller != null;

  @override
  Stream<LiveFrameEvent> start({String? spaceId}) {
    _close();
    final controller = StreamController<LiveFrameEvent>(sync: true);
    _controller = controller;
    _armStale();
    onStart(spaceId);
    return controller.stream;
  }

  @override
  Future<void> stop() async {
    onStop();
    _close();
  }

  /// Begins delivering frames for [spaceId].
  @protected
  void onStart(String? spaceId);

  /// Stops delivering frames.
  @protected
  void onStop();

  /// Reports a new [frame]: emits it with its model and restarts the stale timer.
  @protected
  void emitFrame(api.StatsLiveFrame frame) {
    final controller = _controller;
    if (controller == null) return;
    _stale = false;
    controller.add(
      LiveFrameEvent.frame(
        frame,
        LiveTopologyModel.fromFrame(frame, names: names),
      ),
    );
    _armStale();
  }

  /// Reports the error [code].
  @protected
  void emitError(String code) => _controller?.add(LiveFrameEvent.error(code));

  void _armStale() {
    _staleTimer?.cancel();
    _staleTimer = clock.schedule(staleAfter, () {
      if (_stale) return;
      _stale = true;
      _controller?.add(const LiveFrameEvent.stale());
    });
  }

  void _close() {
    _staleTimer?.cancel();
    _staleTimer = null;
    _stale = false;
    final controller = _controller;
    _controller = null;
    if (controller != null) unawaited(controller.close());
  }
}
