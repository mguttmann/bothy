// SPDX-License-Identifier: Apache-2.0

/// The kinds of failure of the Noise layer. Every failure of a handshake or of a decryption
/// ends the session (close code 4104, docs/spec/24-terminal.md 5.3).
enum NoiseError {
  /// A handshake step failed: wrong prologue, wrong static key, tampered or short message, or
  /// a Diffie-Hellman result of 32 zero bytes.
  handshakeFailed,

  /// A transport message did not authenticate or was shorter than the tag.
  decryptFailed,

  /// A Noise message over 65535 bytes, or a payload or plaintext that would make one.
  messageTooLong,

  /// A direction has used every nonce below the reserved 2^64 - 1 (Noise revision 34, 5.1).
  nonceExhausted,

  /// A call after an earlier failure of the same object.
  broken,

  /// A call out of the order of the handshake.
  invalidState,

  /// A key of the wrong length or type, or a public key that does not belong to its pair.
  invalidKey,

  /// A session or host ID that is not a canonical UUID or not 16 bytes.
  invalidId,
}

/// An error of the Noise layer. Its text names the operation, the kind and lengths only, never
/// payload, plaintext, key or token bytes (SEC-TERM-1, R-TM-10).
class NoiseException implements Exception {
  /// Creates an exception of kind [error] for [operation]; [detail] holds lengths only.
  const NoiseException(this.error, this.operation, [this.detail = '']);

  /// The kind of failure.
  final NoiseError error;

  /// The operation that failed, for example `read message 1`.
  final String operation;

  /// Lengths or limits that explain the failure; never content.
  final String detail;

  @override
  String toString() =>
      'NoiseException(${error.name}): $operation${detail.isEmpty ? '' : ' ($detail)'}';
}

/// The kinds of failure of the frame parser (docs/spec/24-terminal.md 5.4). Every one of them
/// ends the session like an unknown type (close code 4104, docs/DECISIONS.md D-M2-38-10).
enum FrameError {
  /// No bytes at all.
  empty,

  /// A type byte outside 0x01 to 0x07.
  unknownType,

  /// A known type sent by the wrong side.
  wrongDirection,

  /// A body against the rules of its type.
  malformed,

  /// More than [maxFrameLength] bytes.
  tooLong,
}

/// An error of the frame parser. Its text names the type byte and lengths, never body bytes.
class FrameException implements Exception {
  /// Creates an exception of kind [error] with a [detail] of type byte and lengths.
  const FrameException(this.error, this.detail);

  /// The kind of failure.
  final FrameError error;

  /// The type byte and lengths that explain the failure; never content.
  final String detail;

  @override
  String toString() => 'FrameException(${error.name}): $detail';
}
