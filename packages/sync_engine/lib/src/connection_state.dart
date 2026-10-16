// SPDX-License-Identifier: Apache-2.0

/// The observable state of the gateway connection: the phases of 15 7.2 and a reason (brief
/// M4-13 Design note 3).
library;

/// The phases of the state machine of 15 7.2.
enum ConnectionPhase {
  /// No connection and no timer; see [ConnectionState.reason] for what wakes it.
  offline,

  /// The WebSocket is opening or waits for `hello`.
  connecting,

  /// `identify` or `resume` is sent; waiting for `ready` or `resumed`.
  identifying,

  /// After `ready` or `resumed` while the catch-up runs (set by the listener of M4-14).
  catchingUp,

  /// Live events flow.
  live,

  /// A resync reloads the snapshot (set by the listener of M4-14, 04 7.3).
  resyncing,

  /// Waiting before the next attempt (04 6.5).
  backoff,

  /// Mobile app in the background: closed with `1000`, reconnects in the foreground.
  suspended,

  /// The session ended (`4012`, a rejected refresh); the app shows the login.
  signedOut,

  /// The server does not speak the client's protocol version (`4007`, `hello`).
  updateRequired,
}

/// The kinds of [ConnectionReason].
enum ConnectionReasonKind {
  /// No particular reason.
  none,

  /// The device has no network.
  networkUnavailable,

  /// `4016`: the device waits for approval.
  devicePending,

  /// `4017`: a newer connection of the same device is active.
  deviceReplaced,

  /// `4018`: login valid, but `gateway.connect` denied.
  forbidden,

  /// `4008`: `rl.gateway_connect` exhausted.
  rateLimited,

  /// `4012`: session, device or principal revoked.
  revoked,

  /// `4007` or a `hello` without the client's version.
  protocolUnsupported,

  /// Any other close of the server; [ConnectionReason.code] holds the code.
  closedByServer,
}

/// Why the connection is in its phase; the interface explains [ConnectionPhase.offline] and
/// [ConnectionPhase.backoff] with it.
final class ConnectionReason {
  const ConnectionReason._(this.kind) : code = null;

  /// The server closed with [code] and no other reason applies.
  const ConnectionReason.closedByServer(int this.code)
    : kind = ConnectionReasonKind.closedByServer;

  /// No particular reason.
  static const ConnectionReason none = ConnectionReason._(
    ConnectionReasonKind.none,
  );

  /// The device has no network.
  static const ConnectionReason networkUnavailable = ConnectionReason._(
    ConnectionReasonKind.networkUnavailable,
  );

  /// `4016`.
  static const ConnectionReason devicePending = ConnectionReason._(
    ConnectionReasonKind.devicePending,
  );

  /// `4017`.
  static const ConnectionReason deviceReplaced = ConnectionReason._(
    ConnectionReasonKind.deviceReplaced,
  );

  /// `4018`.
  static const ConnectionReason forbidden = ConnectionReason._(
    ConnectionReasonKind.forbidden,
  );

  /// `4008`.
  static const ConnectionReason rateLimited = ConnectionReason._(
    ConnectionReasonKind.rateLimited,
  );

  /// `4012`.
  static const ConnectionReason revoked = ConnectionReason._(
    ConnectionReasonKind.revoked,
  );

  /// `4007` or `hello` without the client's version.
  static const ConnectionReason protocolUnsupported = ConnectionReason._(
    ConnectionReasonKind.protocolUnsupported,
  );

  /// The kind.
  final ConnectionReasonKind kind;

  /// The close code for [ConnectionReasonKind.closedByServer], else `null`.
  final int? code;

  @override
  bool operator ==(Object other) =>
      other is ConnectionReason && other.kind == kind && other.code == code;

  @override
  int get hashCode => Object.hash(kind, code);

  @override
  String toString() => code == null ? kind.name : '${kind.name}($code)';
}

/// The state of the connection: a [phase] of 15 7.2 and its [reason].
final class ConnectionState {
  /// Creates the state.
  const ConnectionState(this.phase, [this.reason = ConnectionReason.none]);

  /// The phase.
  final ConnectionPhase phase;

  /// The reason.
  final ConnectionReason reason;

  @override
  bool operator ==(Object other) =>
      other is ConnectionState &&
      other.phase == phase &&
      other.reason == reason;

  @override
  int get hashCode => Object.hash(phase, reason);

  @override
  String toString() => 'ConnectionState(${phase.name}, $reason)';
}
