// SPDX-License-Identifier: Apache-2.0

/// The state of one terminal tab (docs/spec/15-client-architecture.md 23.2,
/// docs/design/components.md 12 "States").
library;

import 'package:flutter/foundation.dart';

/// Where the flow of 15 23.2 stands.
enum TerminalPhase {
  /// Nothing opened yet, or the opening was cancelled before a session existed.
  idle,

  /// Loading the host, checking the pin, the warnings and the dialog "Open terminal".
  preparing,

  /// Step-up, opening or attaching, stream, `identify` and handshake run; no input yet.
  connecting,

  /// Message 2 checked and `start` sent: input goes to the host.
  connected,

  /// The process ended (`exit`); the tab and the screen stay.
  exited,

  /// The stream is closed, the session runs on the host; "Reattach" attaches again.
  detached,

  /// The session ended (`4100`, `1000` or "End"); the screen stays readable.
  ended,

  /// A hard warning without a new attempt: tampering, a changed key or a revocation.
  blocked,

  /// The opening failed before a stream existed (a problem of the server).
  failed,
}

/// Why a tab is [TerminalPhase.detached].
enum DetachCause {
  /// The person chose "Detach".
  person,

  /// The stream broke without a close code of the server.
  connectionLost,

  /// `4102`: the host service is not connected; attach later.
  hostUnavailable,

  /// `4101`: the session token expired or was already used; attach again with a new step-up.
  tokenStale,

  /// `4103`: another device attached; no own new attempt.
  replaced,

  /// `1001`, `4013`: the server node shut down or restarted.
  serverRestart,

  /// `4014`: the relay buffer was full.
  slowConsumer,

  /// `4004`: the stream's login failed.
  authenticationFailed,

  /// `4008`: rate limited.
  rateLimited,
}

/// Why a tab is [TerminalPhase.blocked].
enum BlockCause {
  /// `4104` or a Noise or frame error on this device (docs/spec/24-terminal.md 5.3, 5.4).
  handshakeFailed,

  /// The key or key generation of the grant differs from the checked one (15 23.2 item 3).
  keyMismatch,

  /// The session or the device was revoked (`4100` with a revocation reason, `4012`).
  revoked,

  /// `4002`: the stream broke the protocol.
  protocolError,
}

/// One immutable snapshot of a tab.
@immutable
final class TerminalTabState {
  /// Creates a snapshot.
  const TerminalTabState({
    required this.phase,
    this.sessionId,
    this.profile,
    this.openedAt,
    this.exitCode,
    this.closeReason,
    this.detachCause,
    this.blockCause,
    this.problemCode,
    this.problemReason,
    this.replaying = false,
  });

  /// The snapshot before anything happened.
  static const TerminalTabState initial = TerminalTabState(
    phase: TerminalPhase.idle,
  );

  /// Where the flow stands.
  final TerminalPhase phase;

  /// The session, once the server created it.
  final String? sessionId;

  /// The profile of the session.
  final String? profile;

  /// `opened_at` of the session: the start of the remaining time in the header.
  final DateTime? openedAt;

  /// The exit code of the process in [TerminalPhase.exited].
  final int? exitCode;

  /// The `close_reason` of an ended or revoked session (docs/spec/24-terminal.md 10.3).
  final String? closeReason;

  /// Why the tab is detached.
  final DetachCause? detachCause;

  /// Why the tab is blocked.
  final BlockCause? blockCause;

  /// The problem code of a [TerminalPhase.failed] opening (for example `terminal_denied`).
  final String? problemCode;

  /// The `reason` the server gave with [problemCode] (for example `no_session_user` of
  /// `terminal_denied`, docs/spec/24-terminal.md 13), or null; the view shows its text.
  final String? problemReason;

  /// Whether a replay between `replay_begin` and `replay_end` is running.
  final bool replaying;

  /// Whether input reaches the host (15 23.2 item 4: only after message 2 and `start`).
  bool get acceptsInput => phase == TerminalPhase.connected && !replaying;

  /// Returns a copy with the given members replaced; [clearCauses] drops the causes, the exit
  /// code, the close reason and the problem code of the previous phase.
  TerminalTabState copyWith({
    TerminalPhase? phase,
    String? sessionId,
    String? profile,
    DateTime? openedAt,
    int? exitCode,
    String? closeReason,
    DetachCause? detachCause,
    BlockCause? blockCause,
    String? problemCode,
    String? problemReason,
    bool? replaying,
    bool clearCauses = false,
  }) => TerminalTabState(
    phase: phase ?? this.phase,
    sessionId: sessionId ?? this.sessionId,
    profile: profile ?? this.profile,
    openedAt: openedAt ?? this.openedAt,
    exitCode: exitCode ?? (clearCauses ? null : this.exitCode),
    closeReason: closeReason ?? (clearCauses ? null : this.closeReason),
    detachCause: detachCause ?? (clearCauses ? null : this.detachCause),
    blockCause: blockCause ?? (clearCauses ? null : this.blockCause),
    problemCode: problemCode ?? (clearCauses ? null : this.problemCode),
    problemReason:
        problemReason ??
        (clearCauses || problemCode != null ? null : this.problemReason),
    replaying: replaying ?? this.replaying,
  );

  @override
  String toString() =>
      'TerminalTabState(${phase.name}, detach: ${detachCause?.name}, '
      'block: ${blockCause?.name}, reason: $closeReason, problem: $problemCode '
      '$problemReason)';
}
