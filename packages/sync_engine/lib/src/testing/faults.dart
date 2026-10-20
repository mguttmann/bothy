// SPDX-License-Identifier: Apache-2.0

/// Fault injection of the fake server (brief M4-17 Design note 4; 15 20 row "Integration"):
/// REST answers, a lost response after the commit, delays, gateway closes with any code,
/// duplicate delivery, `resync_required` and revocation. Faults are deterministic.
library;

import 'dart:async';

/// What a REST fault does to one request.
sealed class RequestFault {
  const RequestFault(this.operationId);

  /// The operation the fault applies to, or null for any.
  final String? operationId;
}

/// Answer with [status] and a problem with [code] (and [reason]) instead of handling the
/// request (nothing is stored); [retryAfter] sets `Retry-After`.
final class AnswerFault extends RequestFault {
  /// Creates the fault.
  const AnswerFault(
    super.operationId,
    this.status,
    this.code,
    this.retryAfter, {
    this.reason,
  });

  /// The HTTP status.
  final int status;

  /// The problem code.
  final String code;

  /// The `Retry-After` wait, if any.
  final Duration? retryAfter;

  /// The `reason` member, if any (for example `part_conflict` of a `409 conflict`).
  final String? reason;
}

/// Handle the request (the write is stored and its events are appended), then close the
/// connection before the response (04 16.2).
final class DropResponseFault extends RequestFault {
  /// Creates the fault.
  const DropResponseFault(super.operationId);
}

/// Handle the request, then hold the response for [duration], so the echo on the gateway can
/// arrive before it.
final class DelayFault extends RequestFault {
  /// Creates the fault.
  const DelayFault(super.operationId, this.duration);

  /// How long the response waits.
  final Duration duration;
}

/// The gateway side the gateway faults act on; the fake server implements it.
abstract interface class GatewayFaultTarget {
  /// Closes every open gateway connection with [code] and [reason].
  void closeAll(int code, String? reason);

  /// Sends the next `dispatch` frame twice.
  void duplicateNextDispatch();

  /// Answers the next session start with a cursor with `resync_required` and [reason]
  /// instead of the catch-up.
  void forceResync(String reason);

  /// Closes every connection of [deviceId] (every device when null) with `4012` and revokes
  /// their sessions.
  void revoke(String? deviceId);

  /// Holds every `dispatch` until [releaseDispatches], as a slow gateway delivery would.
  void holdDispatches();

  /// Sends the held `dispatch` frames in order and delivers again.
  void releaseDispatches();
}

class _Rule {
  _Rule(this.fault, this.times);

  final RequestFault fault;
  int times;
}

/// A queue of fault rules, matched in order (Design note 4).
///
/// REST rules wait for the next request of their operation and apply [times] times; gateway
/// faults act at once (or after `after`). Nothing is random, so a test sees the same faults
/// on every run.
///
/// Example:
/// ```dart
/// server.faults
///   ..onRequest('createMessage', status: 503, code: 'unavailable',
///       retryAfter: const Duration(milliseconds: 20))
///   ..dropResponseAfterCommit('createMessage');
/// server.faults.closeGateway(4000);
/// ```
final class FaultPlan {
  /// Creates the plan acting on the gateway of the fake server.
  FaultPlan(this._gateway);

  final GatewayFaultTarget _gateway;
  final List<_Rule> _rules = [];
  final List<Timer> _timers = [];

  /// The REST rules that have not been used up, in order.
  List<RequestFault> get pending => [for (final r in _rules) r.fault];

  /// Answers the next [times] requests of [operationId] with [status] and a problem with
  /// [code] and [reason] (`409`, `410`, `412`, `429`, `5xx`), with `Retry-After` when
  /// [retryAfter] is set.
  void onRequest(
    String operationId, {
    required int status,
    required String code,
    String? reason,
    Duration? retryAfter,
    int times = 1,
  }) => _rules.add(
    _Rule(
      AnswerFault(operationId, status, code, retryAfter, reason: reason),
      times,
    ),
  );

  /// Stores the next [times] writes of [operationId] and closes their connection before the
  /// response (a lost response after the commit, 04 16.2).
  void dropResponseAfterCommit(String operationId, {int times = 1}) =>
      _rules.add(_Rule(DropResponseFault(operationId), times));

  /// Holds the responses of the next [times] requests of [operationId] (of any operation when
  /// null) for [duration] after they were handled.
  void delay(Duration duration, {String? operationId, int times = 1}) =>
      _rules.add(_Rule(DelayFault(operationId, duration), times));

  /// Closes every gateway connection with [code] and [reason], now or [after] a delay.
  void closeGateway(int code, {String? reason, Duration? after}) {
    if (after == null) {
      _gateway.closeAll(code, reason);
      return;
    }
    _timers.add(Timer(after, () => _gateway.closeAll(code, reason)));
  }

  /// Delivers the next `dispatch` twice (at-least-once delivery, 04 9).
  void duplicateNextDispatch() => _gateway.duplicateNextDispatch();

  /// Answers the next session start with a cursor with `resync_required` and [reason]
  /// (`epoch_changed`, `retention`, `too_far_behind`, `cursor_ahead`, 04 7.3).
  void resyncRequired(String reason) => _gateway.forceResync(reason);

  /// Closes every connection of [deviceId] (every device when null) with `4012` and marks
  /// their sessions revoked (04 8.5).
  void revoke({String? deviceId}) => _gateway.revoke(deviceId);

  /// Holds every `dispatch` until [releaseDispatches].
  void holdDispatches() => _gateway.holdDispatches();

  /// Sends the held `dispatch` frames and delivers again.
  void releaseDispatches() => _gateway.releaseDispatches();

  /// The fault for the next request of [operationId], used up by one; null when no rule
  /// matches. The fake server calls this once per request.
  RequestFault? take(String operationId) {
    for (final rule in _rules) {
      final op = rule.fault.operationId;
      if (op != null && op != operationId) continue;
      if (--rule.times <= 0) _rules.remove(rule);
      return rule.fault;
    }
    return null;
  }

  /// Cancels the gateway faults that wait for their delay.
  void cancelTimers() {
    for (final t in _timers) {
      t.cancel();
    }
    _timers.clear();
  }
}
