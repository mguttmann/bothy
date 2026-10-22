// SPDX-License-Identifier: Apache-2.0

/// The reactions to the close codes of the stream (docs/spec/15-client-architecture.md 23.2
/// item 8; docs/spec/24-terminal.md 13; asyncapi channel `terminalStream`, `x-close-codes`).
library;

import 'package:flutter/foundation.dart';
import 'package:terminal_ui/src/controller/session_state.dart';

/// The close codes of the stream the view reacts to by name.
abstract final class StreamCloseCode {
  /// Normal end: the session ended; do not reconnect.
  static const int normal = 1000;

  /// The node shuts down; attach again.
  static const int goingAway = 1001;

  /// Text frame after `identify`, invalid JSON or an oversized frame.
  static const int decodeError = 4002;

  /// The stream's login failed.
  static const int authenticationFailed = 4004;

  /// Rate limited.
  static const int rateLimited = 4008;

  /// Session, device or person revoked.
  static const int revoked = 4012;

  /// The server restarts; attach again.
  static const int serverRestart = 4013;

  /// The relay buffer was full; attach again.
  static const int slowConsumer = 4014;

  /// The session ended; the close reason holds `close_reason`.
  static const int terminalClosed = 4100;

  /// The session token expired or was already used; a new step-up is needed.
  static const int tokenStale = 4101;

  /// The host service is not connected; attach later.
  static const int hostUnavailable = 4102;

  /// Another device attached.
  static const int replaced = 4103;

  /// Handshake, token or frame tampered with: hard warning, no new attempt.
  static const int handshakeFailed = 4104;
}

/// The `close_reason` values (docs/spec/24-terminal.md 10.3) that mean a revocation: the view
/// shows them as blocked with a danger banner instead of a plain end
/// (docs/design/components.md 12 "States").
const Set<String> revocationCloseReasons = {
  'access_revoked',
  'i6_violation',
  'user_disabled',
  'device_revoked',
  'session_revoked',
  'host_disabled',
  'host_retired',
  'host_rekeyed',
};

/// What the view does after a stream closed.
@immutable
final class CloseReaction {
  /// Creates a reaction.
  const CloseReaction(
    this.phase, {
    this.detachCause,
    this.blockCause,
    this.closeReason,
  });

  /// The phase the tab moves to.
  final TerminalPhase phase;

  /// The detach cause for [TerminalPhase.detached].
  final DetachCause? detachCause;

  /// The block cause for [TerminalPhase.blocked].
  final BlockCause? blockCause;

  /// The `close_reason` for an ended or revoked session.
  final String? closeReason;

  /// Whether "Reattach" is offered: only a detached session; no reaction retries by itself.
  bool get offersReattach => phase == TerminalPhase.detached;

  /// Whether reattaching needs a step-up before `attachTerminalSession` (`4101`).
  bool get needsStepUp => detachCause == DetachCause.tokenStale;
}

/// Returns the reaction to a stream closed with [code] and [reason] (the close reason text).
///
/// A null [code] (the connection broke) and every code without its own row detach, so the
/// person can attach again; the session still runs on the host until
/// `terminal.detach_grace` (docs/spec/24-terminal.md 6.2).
///
/// ```dart
/// closeReactionFor(4100, 'access_revoked').phase; // TerminalPhase.blocked
/// ```
CloseReaction closeReactionFor(int? code, String? reason) {
  final text = (reason == null || reason.isEmpty) ? null : reason;
  return switch (code) {
    StreamCloseCode.terminalClosed when revocationCloseReasons.contains(text) =>
      CloseReaction(
        TerminalPhase.blocked,
        blockCause: BlockCause.revoked,
        closeReason: text,
      ),
    StreamCloseCode.terminalClosed || StreamCloseCode.normal => CloseReaction(
      TerminalPhase.ended,
      closeReason: text,
    ),
    StreamCloseCode.revoked => const CloseReaction(
      TerminalPhase.blocked,
      blockCause: BlockCause.revoked,
    ),
    StreamCloseCode.handshakeFailed => const CloseReaction(
      TerminalPhase.blocked,
      blockCause: BlockCause.handshakeFailed,
    ),
    StreamCloseCode.decodeError => const CloseReaction(
      TerminalPhase.blocked,
      blockCause: BlockCause.protocolError,
    ),
    StreamCloseCode.tokenStale => const CloseReaction(
      TerminalPhase.detached,
      detachCause: DetachCause.tokenStale,
    ),
    StreamCloseCode.hostUnavailable => const CloseReaction(
      TerminalPhase.detached,
      detachCause: DetachCause.hostUnavailable,
    ),
    StreamCloseCode.replaced => const CloseReaction(
      TerminalPhase.detached,
      detachCause: DetachCause.replaced,
    ),
    StreamCloseCode.goingAway ||
    StreamCloseCode.serverRestart => const CloseReaction(
      TerminalPhase.detached,
      detachCause: DetachCause.serverRestart,
    ),
    StreamCloseCode.slowConsumer => const CloseReaction(
      TerminalPhase.detached,
      detachCause: DetachCause.slowConsumer,
    ),
    StreamCloseCode.authenticationFailed => const CloseReaction(
      TerminalPhase.detached,
      detachCause: DetachCause.authenticationFailed,
    ),
    StreamCloseCode.rateLimited => const CloseReaction(
      TerminalPhase.detached,
      detachCause: DetachCause.rateLimited,
    ),
    _ => const CloseReaction(
      TerminalPhase.detached,
      detachCause: DetachCause.connectionLost,
    ),
  };
}
