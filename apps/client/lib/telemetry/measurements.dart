// SPDX-License-Identifier: Apache-2.0

/// The measurement points of the app metrics (docs/spec/15-client-architektur.md 17 and 19;
/// brief M4-31 Scope).
///
/// | Metric | Starts | Stops |
/// |---|---|---|
/// | `client.cold_start.duration` | the first line of `main` ([appStartWatch]) | the first frame in which the sidebar shows spaces from the local database ([AppMeasurements.sidebarShown]), once per process |
/// | `client.thread_open.duration` | the history column of a thread is created ([AppMeasurements.threadOpening]) | the first frame with messages of that thread ([AppMeasurements.threadShown]) |
/// | `client.send_local.duration` | Enter in the composer ([AppMeasurements.sendStarted]) | the frame after the outbox write that the history shows ([AppMeasurements.sendShown]) |
/// | `client.frames`, `client.frames.over_budget` | the frame timings of the engine ([AppMeasurements.attachFrameTimings]) | one count per frame, over budget when its total time exceeds one refresh interval |
/// | `client.catchup.duration`, `client.catchup.events` | the connection enters `catchingUp` | it reaches `live`; events are the `dispatch` frames of the gateway in between ([AppMeasurements.wrapTransport]) |
/// | `client.resync` | a `Resynced` notice of the engine | |
/// | `client.reconnects` | a new gateway connection after a closed one, with the close code of the closed one | |
/// | `client.outbox.depth` | every change of the outbox depth of the engine (M4-15) | |
///
/// Only durations, counts and the close code are recorded: never a thread ID (it identifies the
/// open measurement in memory only), a text or a name.
library;

import 'dart:async';

import 'package:flutter/scheduler.dart';
import 'package:sync_engine/sync_engine.dart';

import 'client_metrics.dart';

/// The time since the first line of `main`; `main` reads it first, so it starts there. The
/// engine's start before `main` is not part of the cold start.
final Stopwatch appStartWatch = Stopwatch()..start();

/// Runs a callback after the next frame; production: `SchedulerBinding.addPostFrameCallback`.
typedef AfterFrame = void Function(void Function() callback);

void _afterNextFrame(void Function() callback) =>
    SchedulerBinding.instance.addPostFrameCallback((_) => callback());

/// The measurement points of the app over a [ClientMetrics] collector.
///
/// Example:
/// ```dart
/// final measurements = AppMeasurements(metrics);
/// measurements.threadOpening(thread.value);
/// ```
final class AppMeasurements {
  /// Creates the measurements over [metrics]; [clock] is the time since the start of `main`
  /// (default [appStartWatch]), [afterFrame] runs a callback after the next frame.
  AppMeasurements(this.metrics, {Stopwatch? clock, AfterFrame? afterFrame})
    : _clock = clock ?? appStartWatch,
      _afterFrame = afterFrame ?? _afterNextFrame;

  /// The collector.
  final ClientMetrics metrics;

  final Stopwatch _clock;
  final AfterFrame _afterFrame;
  bool _coldStartDone = false;
  final Map<String, Duration> _threadOpens = {};
  Duration? _catchupStart;
  int _catchupEvents = 0;

  Duration get _now => _clock.elapsed;

  /// The sidebar shows spaces from the local database: the cold start ends after this frame
  /// (once per process).
  void sidebarShown() {
    if (_coldStartDone) return;
    _coldStartDone = true;
    _afterFrame(
      () => metrics.recordDuration(ClientMetric.coldStartDuration, _now),
    );
  }

  /// The history column of [threadId] was created: the thread opens.
  void threadOpening(String threadId) => _threadOpens[threadId] = _now;

  /// The history of [threadId] has messages: the open ends after this frame. Without an open
  /// thread of that ID nothing is recorded.
  void threadShown(String threadId) {
    final start = _threadOpens.remove(threadId);
    if (start == null) return;
    _afterFrame(
      () =>
          metrics.recordDuration(ClientMetric.threadOpenDuration, _now - start),
    );
  }

  /// A message is being sent; pass the result to [sendShown].
  Duration sendStarted() => _now;

  /// The local write of the send that started at [start] is done: the history shows it in the
  /// next frame.
  void sendShown(Duration start) => _afterFrame(
    () => metrics.recordDuration(ClientMetric.sendLocalDuration, _now - start),
  );

  /// Counts [totals] as frames and those above [budget] as over budget.
  void recordFrames(Iterable<Duration> totals, {required Duration budget}) {
    var count = 0;
    var over = 0;
    for (final t in totals) {
      count++;
      if (t > budget) over++;
    }
    if (count == 0) return;
    metrics
      ..record(ClientMetric.frames, count)
      ..record(ClientMetric.framesOverBudget, over);
  }

  /// Counts the frames of [binding] from now on; the budget is one refresh interval of the
  /// display (60 Hz when the platform reports none). Returns the function that stops it.
  void Function() attachFrameTimings(SchedulerBinding binding) {
    void onTimings(List<FrameTiming> timings) {
      final displays = binding.platformDispatcher.displays;
      final rate = displays.isEmpty ? 60.0 : displays.first.refreshRate;
      final budget = Duration(
        microseconds: (Duration.microsecondsPerSecond / (rate > 0 ? rate : 60))
            .round(),
      );
      recordFrames([for (final t in timings) t.totalSpan], budget: budget);
    }

    binding.addTimingsCallback(onTimings);
    return () => binding.removeTimingsCallback(onTimings);
  }

  /// Follows the engine: catch-up from the [states], resyncs from the [notices], the outbox
  /// depth. Returns the function that stops following.
  Future<void> Function() watchEngine({
    required Stream<ConnectionState> states,
    required Stream<SyncNotice> notices,
    required Stream<int> outboxDepth,
  }) {
    final subs = [
      states.listen(_onState),
      notices.listen((n) {
        if (n is Resynced) metrics.record(ClientMetric.resync, 1);
      }),
      outboxDepth.listen((d) => metrics.record(ClientMetric.outboxDepth, d)),
    ];
    return () async {
      for (final s in subs) {
        await s.cancel();
      }
    };
  }

  void _onState(ConnectionState s) {
    switch (s.phase) {
      case ConnectionPhase.catchingUp:
        if (_catchupStart == null) {
          _catchupStart = _now;
          _catchupEvents = 0;
        }
      case ConnectionPhase.live:
        final start = _catchupStart;
        if (start != null) {
          metrics
            ..recordDuration(ClientMetric.catchupDuration, _now - start)
            ..record(ClientMetric.catchupEvents, _catchupEvents);
        }
        _catchupStart = null;
      case ConnectionPhase.resyncing:
        // A resync continues the catch-up of the same session.
        break;
      default:
        // The connection dropped before live: no complete catch-up to report.
        _catchupStart = null;
    }
  }

  /// [inner] with the counting of this class: the `dispatch` frames during a catch-up and the
  /// close code of every connection that a new one replaces.
  GatewayTransport wrapTransport(GatewayTransport inner) =>
      _MeasuredTransport(inner, this);

  void _onFrame(String frame) {
    if (_catchupStart != null && frame.contains(_dispatchOp)) _catchupEvents++;
  }

  /// The op member of a dispatch frame as the gateway writes it (`{"op":"dispatch",...}`); the
  /// sequence cannot occur unescaped inside a JSON string of the payload.
  static const String _dispatchOp = '"op":"dispatch"';
}

/// The transport wrapper of [AppMeasurements.wrapTransport].
final class _MeasuredTransport implements GatewayTransport {
  _MeasuredTransport(this._inner, this._m);

  final GatewayTransport _inner;
  final AppMeasurements _m;
  GatewayChannel? _last;

  @override
  Future<GatewayChannel> connect(Uri url) async {
    final channel = await _inner.connect(url);
    final previous = _last;
    if (previous != null) {
      _m.metrics.record(
        ClientMetric.reconnects,
        1,
        closeCode: previous.closeCode,
      );
    }
    _last = channel;
    return _MeasuredChannel(channel, _m);
  }
}

/// A channel that counts its frames for the catch-up.
final class _MeasuredChannel implements GatewayChannel {
  _MeasuredChannel(this._inner, this._m);

  final GatewayChannel _inner;
  final AppMeasurements _m;

  @override
  Stream<String> get frames => _inner.frames.map((f) {
    _m._onFrame(f);
    return f;
  });

  @override
  void send(String text) => _inner.send(text);

  @override
  Future<void> close(int code, [String? reason]) => _inner.close(code, reason);

  @override
  int? get closeCode => _inner.closeCode;

  @override
  String? get closeReason => _inner.closeReason;
}
