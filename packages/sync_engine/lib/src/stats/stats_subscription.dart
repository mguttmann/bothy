// SPDX-License-Identifier: Apache-2.0

/// The `stats.live` subscription of the gateway connection (04 6.2 `stats.subscribe` and
/// `stats.unsubscribe`, 6.3 `resume`, 12 item 4; 25 5.1; brief M6-16 Design note 1;
/// docs/decisions.d/M6-16.md D-M6-16-1 and D-M6-16-2).
library;

import 'dart:async';

import 'package:api_client/api_client.dart';
import 'package:built_value/serializer.dart';

import '../frames.dart';

/// The scope of a subscription: the instance ([spaceId] null) or one space.
final class StatsLiveScope {
  /// Creates the scope; [spaceId] null is the instance.
  const StatsLiveScope({this.spaceId});

  /// The subscribed space, or null for the whole instance.
  final String? spaceId;

  @override
  bool operator ==(Object other) =>
      other is StatsLiveScope && other.spaceId == spaceId;

  @override
  int get hashCode => spaceId.hashCode;

  @override
  String toString() => 'StatsLiveScope(${spaceId ?? 'instance'})';
}

/// One update of the live stream for the app.
sealed class StatsLiveUpdate {
  const StatsLiveUpdate();
}

/// A `stats.live` frame of the subscribed scope.
final class StatsLiveReceived extends StatsLiveUpdate {
  /// Wraps [frame].
  const StatsLiveReceived(this.frame);

  /// The frame as the server built it for this person (25 5.2).
  final StatsLiveFrame frame;
}

/// The server refused or ended the subscription with `error {op: stats.subscribe}`; the
/// subscription is cleared and not sent again until the app subscribes anew.
final class StatsLiveEnded extends StatsLiveUpdate {
  /// Creates the update for the wire [code].
  const StatsLiveEnded(this.code);

  /// `forbidden`, `not_found`, `feature_disabled`, `rate_limited` or another problem code.
  final String code;
}

/// Keeps the subscription the app wants and sends `stats.subscribe` and `stats.unsubscribe`
/// so the server holds the same one.
///
/// Rules (Design note 1):
/// - [subscribe] and [unsubscribe] change the wanted subscription; a change is sent at once
///   on the identified connection, else after the next `ready` or `resumed`.
/// - After `ready` (a new gateway session, after `identify`) the server holds none, so a
///   wanted subscription is sent again ([onReady]).
/// - After `resumed` the server restored what it held (04 6.3); only a change made while
///   disconnected is sent ([onResumed]). A lost connection changes nothing here: the server
///   keeps the subscription in the gateway session for `resume`.
/// - `error` with op `stats.subscribe` (a refused subscription, a refused restore on `resume`,
///   or the loss of `stats.read` during the stream, 25 5.1 item 3) clears the subscription and
///   passes the code on as [StatsLiveEnded] ([onError]). The server keeps an earlier
///   subscription when it refuses a replacing one, and the error does not say which request
///   it answers, so the client sends `stats.unsubscribe` (now, or after the next `resumed`):
///   afterwards the server surely holds none (an unsubscribe without one is answered with
///   nothing).
/// - Frames arrive as the ephemeral event `stats.live` ([onLive]) and are passed on as
///   [StatsLiveReceived] when their scope is the wanted one; nothing is written to the local
///   database: a live frame is replaced by the next one and has no `seq` (04 2.1, D-M6-16-2).
///
/// Example:
/// ```dart
/// engine.stats.updates.listen(show);
/// engine.stats.subscribe(spaceId: space); // or subscribe() for the instance
/// engine.stats.unsubscribe();
/// ```
final class StatsSubscription {
  /// Creates the subscription; [trySend] sends a frame on the identified connection and
  /// returns `false` without one; [log] receives one line per notable step, never content.
  StatsSubscription({required this._trySend, this._log});

  final bool Function(GatewayFrame frame) _trySend;
  final void Function(String line)? _log;

  final StreamController<StatsLiveUpdate> _updates =
      StreamController<StatsLiveUpdate>.broadcast(sync: true);

  /// The subscription the app wants, or null.
  StatsLiveScope? _wanted;

  /// The subscription the server holds, as far as the client knows.
  StatsLiveScope? _held;

  /// Whether the server may hold a subscription the client does not know (after a refusal
  /// that could not be followed by `stats.unsubscribe` yet).
  bool _heldUnknown = false;

  /// The subscription the app wants, or null for none.
  StatsLiveScope? get current => _wanted;

  /// Frames of the wanted scope and the end of the subscription, delivered synchronously.
  Stream<StatsLiveUpdate> get updates => _updates.stream;

  /// Subscribes to the frames of [spaceId], or of the instance when null; replaces an earlier
  /// subscription (25 5.1 item 1).
  void subscribe({String? spaceId}) {
    _wanted = StatsLiveScope(spaceId: spaceId);
    _flush();
  }

  /// Ends the subscription.
  void unsubscribe() {
    _wanted = null;
    _flush();
  }

  /// A new gateway session is ready: it holds no subscription, so the wanted one is sent.
  void onReady() {
    _held = null;
    _heldUnknown = false;
    _flush();
  }

  /// The gateway session resumed: the server restored its subscription (04 6.3); only a
  /// change made while disconnected is sent.
  void onResumed() => _flush();

  /// An `error` frame for [op] with [code]: for `stats.subscribe` the subscription is cleared,
  /// `stats.unsubscribe` ends whatever the server kept, and [StatsLiveEnded] is emitted; other
  /// ops are not this part's concern.
  void onError(String op, String code) {
    if (op != 'stats.subscribe') return;
    final had = _wanted != null || _held != null || _heldUnknown;
    _wanted = null;
    if (had) _heldUnknown = true;
    _flush();
    _log?.call('stats.subscribe refused: $code');
    if (had && !_updates.isClosed) _updates.add(StatsLiveEnded(code));
  }

  /// A `dispatch` of `stats.live` with its [envelope]: the frame is decoded and passed on when
  /// it belongs to the wanted scope; a malformed frame is logged without content and skipped.
  void onLive(Map<String, Object?> envelope) {
    final wanted = _wanted;
    if (wanted == null || _updates.isClosed) return;
    final StatsLiveFrame frame;
    try {
      frame =
          standardSerializers.deserialize(
                envelope['payload'],
                specifiedType: const FullType(StatsLiveFrame),
              )!
              as StatsLiveFrame;
    } on Object {
      _log?.call('stats.live skipped: malformed frame');
      return;
    }
    // A frame of an earlier subscription may still be on the way after a change.
    if (frame.scope.spaceId != wanted.spaceId) return;
    _updates.add(StatsLiveReceived(frame));
  }

  /// Closes [updates].
  Future<void> dispose() async {
    // Not awaited: a paused listener would hold the done event (D-M4-19-14).
    unawaited(_updates.close());
  }

  void _flush() {
    final wanted = _wanted;
    if (wanted == _held && !_heldUnknown) return;
    final GatewayFrame frame = wanted == null
        ? const StatsUnsubscribeFrame()
        : StatsSubscribeFrame(
            GatewayStatsSubscribe((b) => b.spaceId = wanted.spaceId),
          );
    if (_trySend(frame)) {
      _held = wanted;
      _heldUnknown = false;
    }
  }
}
