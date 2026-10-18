// SPDX-License-Identifier: Apache-2.0

/// The reaction to every close code of the gateway, as data (04 6.4, 15 7.2; brief M4-13
/// Design notes 3 and 4).
library;

import 'connection_state.dart';

/// How the connection reconnects after a close.
enum Reconnect {
  /// Wait the full-jitter backoff of 04 6.5 (raises the attempt counter).
  backoff,

  /// Reconnect at once (after a token renewal).
  immediate,

  /// `4013`: wait uniform in `[0, sync.reconnect_backoff_base]`, without raising the counter.
  serverRestart,

  /// Wait until the app returns to the foreground (or a user action).
  onForeground,

  /// Wait for a user action (`GatewayConnection.connectNow`).
  onUserAction,

  /// Poll the pending device until it is `active` (`4016`).
  afterPendingDevice,

  /// Never on its own (`4007`, `4012`).
  never,
}

/// The reaction to one close.
final class CloseReaction {
  /// Creates a reaction.
  const CloseReaction({
    required this.code,
    required this.phase,
    required this.reason,
    required this.reconnect,
    this.identify = false,
    this.renewToken = false,
    this.signOut = false,
    this.unexpected = false,
    this.retryAfter,
  });

  /// The close code, or `null` for a drop without one.
  final int? code;

  /// The next phase.
  final ConnectionPhase phase;

  /// The reason shown with it.
  final ConnectionReason reason;

  /// How to reconnect.
  final Reconnect reconnect;

  /// Whether the next attempt must send `identify` instead of `resume` (`4006`).
  final bool identify;

  /// Whether to renew the access token first (`4004`, `4011`).
  final bool renewToken;

  /// Whether to sign out and request the deletion of local data (`4012`, R-SY-14).
  final bool signOut;

  /// Whether the code is not expected by a person's client (`4015` is for agents only).
  final bool unexpected;

  /// The wait the server named in the close reason (`4008`), replacing the jitter.
  final Duration? retryAfter;
}

CloseReaction _backoff(int code) => CloseReaction(
  code: code,
  phase: ConnectionPhase.backoff,
  reason: ConnectionReason.closedByServer(code),
  reconnect: Reconnect.backoff,
);

/// The reaction to every close code of 04 6.4, by code; [reactTo] resolves the cases that
/// depend on the close reason or the app state.
final Map<int, CloseReaction> closeCodeTable = {
  // 1000 reconnects only in the foreground; reactTo turns it into onForeground otherwise.
  1000: _backoff(1000),
  1001: _backoff(1001),
  4000: _backoff(4000),
  4001: _backoff(4001),
  4002: _backoff(4002),
  4003: _backoff(4003),
  4004: const CloseReaction(
    code: 4004,
    phase: ConnectionPhase.backoff,
    reason: ConnectionReason.closedByServer(4004),
    reconnect: Reconnect.immediate,
    renewToken: true,
  ),
  4005: _backoff(4005),
  4006: const CloseReaction(
    code: 4006,
    phase: ConnectionPhase.backoff,
    reason: ConnectionReason.closedByServer(4006),
    reconnect: Reconnect.backoff,
    identify: true,
  ),
  4007: const CloseReaction(
    code: 4007,
    phase: ConnectionPhase.updateRequired,
    reason: ConnectionReason.protocolUnsupported,
    reconnect: Reconnect.never,
  ),
  4008: const CloseReaction(
    code: 4008,
    phase: ConnectionPhase.backoff,
    reason: ConnectionReason.rateLimited,
    reconnect: Reconnect.backoff,
  ),
  4009: _backoff(4009),
  4011: const CloseReaction(
    code: 4011,
    phase: ConnectionPhase.backoff,
    reason: ConnectionReason.closedByServer(4011),
    reconnect: Reconnect.immediate,
    renewToken: true,
  ),
  4012: const CloseReaction(
    code: 4012,
    phase: ConnectionPhase.signedOut,
    reason: ConnectionReason.revoked,
    reconnect: Reconnect.never,
    signOut: true,
  ),
  4013: const CloseReaction(
    code: 4013,
    phase: ConnectionPhase.backoff,
    reason: ConnectionReason.closedByServer(4013),
    reconnect: Reconnect.serverRestart,
  ),
  4014: _backoff(4014),
  4015: const CloseReaction(
    code: 4015,
    phase: ConnectionPhase.offline,
    reason: ConnectionReason.closedByServer(4015),
    reconnect: Reconnect.onUserAction,
    unexpected: true,
  ),
  4016: const CloseReaction(
    code: 4016,
    phase: ConnectionPhase.offline,
    reason: ConnectionReason.devicePending,
    reconnect: Reconnect.afterPendingDevice,
  ),
  4017: const CloseReaction(
    code: 4017,
    phase: ConnectionPhase.offline,
    reason: ConnectionReason.deviceReplaced,
    reconnect: Reconnect.onForeground,
  ),
  4018: const CloseReaction(
    code: 4018,
    phase: ConnectionPhase.offline,
    reason: ConnectionReason.forbidden,
    reconnect: Reconnect.onUserAction,
  ),
};

final RegExp _retryAfter = RegExp(
  r'(?:^|[\s;,])retry_after=(\d{1,6})(?:$|[\s;,])',
);

/// The reaction to a close with [code] and [reason] while the app is in the foreground
/// ([foreground]).
///
/// `null` (a drop without a close frame) and a code outside the table back off and try
/// `resume`; a code of `4000` to `4099` outside the table is a newer server's code and backs
/// off too. `1000` reconnects only in the foreground. `4008` waits the seconds the server
/// writes as `retry_after=<seconds>` (M1-64); without a readable value the normal backoff
/// applies.
CloseReaction reactTo(int? code, String? reason, {required bool foreground}) {
  final entry = code == null ? null : closeCodeTable[code];
  if (entry == null) {
    return CloseReaction(
      code: code,
      phase: ConnectionPhase.backoff,
      reason: code == null
          ? ConnectionReason.none
          : ConnectionReason.closedByServer(code),
      reconnect: Reconnect.backoff,
    );
  }
  if (code == 1000 && !foreground) {
    return const CloseReaction(
      code: 1000,
      phase: ConnectionPhase.offline,
      reason: ConnectionReason.closedByServer(1000),
      reconnect: Reconnect.onForeground,
    );
  }
  if (code == 4008) {
    final m = reason == null ? null : _retryAfter.firstMatch(reason);
    if (m != null) {
      return CloseReaction(
        code: 4008,
        phase: entry.phase,
        reason: entry.reason,
        reconnect: entry.reconnect,
        retryAfter: Duration(seconds: int.parse(m.group(1)!)),
      );
    }
  }
  return entry;
}
