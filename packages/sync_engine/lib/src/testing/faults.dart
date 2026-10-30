// SPDX-License-Identifier: Apache-2.0

/// Fault injection of the fake server (brief M4-17 Design note 4; 15 20 row "Integration"):
/// REST answers, a lost response after the commit, delays, gateway closes with any code,
/// duplicate delivery, `resync_required` and revocation. Faults are deterministic.
library;

import 'dart:async';
import 'dart:math';

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

/// A token bucket over a set of operations, as the server's rate limiter keeps one per class
/// and principal (02, 06 9): [burst] tokens at most, one more every [refill].
final class _Bucket {
  _Bucket(this.operationIds, this.burst, this.refill, this._now)
    : _tokens = burst.toDouble(),
      _at = _now();

  final Set<String> operationIds;
  final int burst;
  final Duration refill;
  final DateTime Function() _now;
  double _tokens;
  DateTime _at;

  /// Takes a token, or returns the wait until one is there.
  Duration? take() {
    final now = _now();
    _tokens = min(
      burst.toDouble(),
      _tokens + now.difference(_at).inMicroseconds / refill.inMicroseconds,
    );
    _at = now;
    if (_tokens >= 1) {
      _tokens -= 1;
      return null;
    }
    return Duration(
      microseconds: ((1 - _tokens) * refill.inMicroseconds).ceil(),
    );
  }
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
  final List<_Bucket> _buckets = [];

  /// Requests refused by [rateLimit], by operation.
  final Map<String, int> limited = {};

  /// Limits the requests of [operationIds] together with one token bucket of [burst] tokens
  /// that gains one token every [refill], as one rate-limit class of the server does for one
  /// principal (06 9). A request without a token is answered `429 rate_limited` with
  /// `Retry-After` in whole seconds until the next token, at least `1`, like the server
  /// (06 9 item 5); nothing is handled or stored. [now] replaces the clock.
  void rateLimit(
    Set<String> operationIds, {
    required int burst,
    required Duration refill,
    DateTime Function()? now,
  }) => _buckets.add(_Bucket(operationIds, burst, refill, now ?? DateTime.now));

  /// The wait of a request of [operationId] that a [rateLimit] bucket refuses, or null when it
  /// may pass (it took a token). The fake server calls this once per request, before [take].
  Duration? limit(String operationId) {
    for (final b in _buckets) {
      if (!b.operationIds.contains(operationId)) continue;
      final wait = b.take();
      if (wait != null) {
        limited.update(operationId, (n) => n + 1, ifAbsent: () => 1);
        return wait;
      }
    }
    return null;
  }

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
