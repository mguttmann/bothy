// SPDX-License-Identifier: Apache-2.0

/// The typing throttle of the gateway connection (04 12 item 1; brief M4-13 Design note 6).
library;

import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart' show Clock;
import 'package:domain/domain.dart';

import 'frames.dart';

/// Sends `typing.start` at most once per `sync.typing_min_interval`, and `typing.stop` when
/// the input is cleared or sent.
///
/// The throttle is per connection, as on the server (04 12 item 1): `typing.stop` resets it,
/// so the next `typing.start` goes out at once. A frame that cannot be sent (no identified
/// connection) is dropped, because typing is ephemeral.
///
/// Example:
/// ```dart
/// composer.onChanged = (_) => connection.typing.onInput(threadId);
/// composer.onSubmitted = (_) => connection.typing.onSent(threadId);
/// ```
final class TypingThrottle {
  /// Creates the throttle; [interval] is `sync.typing_min_interval`, [trySend] sends a frame
  /// on the identified connection and returns `false` without one.
  TypingThrottle({
    required this._clock,
    required this.interval,
    required this._trySend,
  });

  /// `sync.typing_min_interval`.
  final Duration interval;

  final Clock _clock;
  final bool Function(GatewayFrame) _trySend;

  DateTime? _lastStart;
  ThreadId? _active;

  /// The person typed in [thread].
  void onInput(ThreadId thread) {
    final now = _clock.now();
    final last = _lastStart;
    if (last != null && _active == thread && now.difference(last) < interval) {
      return;
    }
    if (last != null && _active != thread && now.difference(last) < interval) {
      // The server throttles per connection: a start for another thread inside the interval
      // would be refused, so the previous thread is stopped first, which resets it.
      _stop(_active!);
    }
    if (_trySend(TypingStartFrame(_typing(thread)))) {
      _lastStart = now;
      _active = thread;
    }
  }

  /// The input of [thread] was cleared.
  void onCleared(ThreadId thread) => _stop(thread);

  /// The message of [thread] was sent.
  void onSent(ThreadId thread) => _stop(thread);

  /// Forgets the throttle; a new connection has a fresh one on the server.
  void reset() {
    _lastStart = null;
    _active = null;
  }

  void _stop(ThreadId thread) {
    if (_active != thread) return;
    reset();
    _trySend(TypingStopFrame(_typing(thread)));
  }

  static GatewayTyping _typing(ThreadId thread) =>
      GatewayTyping((b) => b..threadId = thread.value);
}
