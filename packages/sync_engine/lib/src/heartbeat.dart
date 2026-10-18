// SPDX-License-Identifier: Apache-2.0

/// The client heartbeat of 04 6.3 item 5.
library;

import 'package:auth/auth.dart' show Clock;

import 'ports.dart';

/// Sends a heartbeat every `heartbeat_interval_ms` of `hello` and reports a missing
/// `heartbeat_ack`.
///
/// The first heartbeat waits a random fraction of the interval, so that after a server restart
/// not every client sends at the same moment. After each heartbeat the next one follows one
/// interval later; when no acknowledgement arrived by then, [onMissedAck] runs instead and the
/// heartbeat stops (the connection closes and reconnects).
///
/// Example:
/// ```dart
/// final heartbeat = Heartbeat(
///   clock: clock,
///   random: random,
///   interval: Duration(milliseconds: hello.heartbeatIntervalMs),
///   send: () => channel.send(encodeFrame(HeartbeatFrame(...))),
///   onMissedAck: reconnect,
/// )..start();
/// ```
final class Heartbeat {
  /// Creates the heartbeat; [send] sends one heartbeat, [onMissedAck] reports a missing
  /// acknowledgement.
  Heartbeat({
    required this._clock,
    required this._random,
    required this.interval,
    required this._send,
    required this._onMissedAck,
  });

  /// `heartbeat_interval_ms` of `hello`.
  final Duration interval;

  final Clock _clock;
  final RandomSource _random;
  final void Function() _send;
  final void Function() _onMissedAck;

  int _generation = 0;
  bool _awaitingAck = false;

  /// Starts the loop; a running loop is replaced.
  void start() {
    final generation = ++_generation;
    _awaitingAck = false;
    _run(generation);
  }

  /// Stops the loop.
  void stop() => _generation++;

  /// Records a `heartbeat_ack`.
  void acknowledge() => _awaitingAck = false;

  Future<void> _run(int generation) async {
    await _clock.delay(interval * _random.nextDouble());
    while (generation == _generation) {
      _awaitingAck = true;
      _send();
      await _clock.delay(interval);
      if (generation != _generation) return;
      if (_awaitingAck) {
        _generation++;
        _onMissedAck();
        return;
      }
    }
  }
}
