// SPDX-License-Identifier: Apache-2.0

import 'dart:typed_data';

import 'bytes.dart';
import 'constants.dart';
import 'errors.dart';
import 'symmetric.dart';

/// The two cipher states after the handshake: one key for the messages this side sends and one
/// for the messages it receives.
///
/// [encrypt] takes its nonce synchronously when it is called, so the results belong to the call
/// order: send the messages in the order of the calls. The same holds for [decrypt], which must
/// see the peer's messages in the order they were sent. After a failed [decrypt] every call
/// throws [NoiseError.broken] (docs/DECISIONS.md D-M2-38-8).
class NoiseTransport {
  NoiseTransport._(this._sendKey, this._receiveKey);

  final Uint8List _sendKey;
  final Uint8List _receiveKey;
  final Nonce _sendN = Nonce();
  final Nonce _receiveN = Nonce();
  bool _broken = false;

  /// Returns the Noise transport message for [plaintext] (one frame of
  /// docs/spec/24-terminal.md 5.4).
  ///
  /// Throws [NoiseException] with [NoiseError.messageTooLong] for more than
  /// [maxTransportPlaintextLength] bytes without using a nonce, [NoiseError.nonceExhausted] when
  /// the direction has used every nonce, and [NoiseError.broken] after a failure.
  Future<Uint8List> encrypt(List<int> plaintext) async {
    if (_broken) {
      throw const NoiseException(NoiseError.broken, 'encrypt');
    }
    requireBytes(plaintext, 'plaintext');
    if (plaintext.length > maxTransportPlaintextLength) {
      throw NoiseException(
        NoiseError.messageTooLong,
        'encrypt',
        '${plaintext.length} bytes, limit $maxTransportPlaintextLength',
      );
    }
    if (_sendN.exhausted) {
      throw const NoiseException(NoiseError.nonceExhausted, 'encrypt');
    }
    final nonce = _sendN.bytes();
    _sendN.increment();
    return aeadEncrypt(_sendKey, nonce, const [], plaintext);
  }

  /// Authenticates and decrypts one transport message of the peer.
  ///
  /// Throws [NoiseException] with [NoiseError.messageTooLong] over [maxNoiseMessageLength]
  /// bytes and [NoiseError.decryptFailed] for a message shorter than the tag or one that does
  /// not authenticate; each of these breaks the transport. [NoiseError.nonceExhausted] when the
  /// direction has used every nonce.
  Future<Uint8List> decrypt(List<int> message) async {
    if (_broken) {
      throw const NoiseException(NoiseError.broken, 'decrypt');
    }
    if (_receiveN.exhausted) {
      throw const NoiseException(NoiseError.nonceExhausted, 'decrypt');
    }
    if (message.length > maxNoiseMessageLength) {
      _broken = true;
      throw NoiseException(
        NoiseError.messageTooLong,
        'decrypt',
        '${message.length} bytes, limit $maxNoiseMessageLength',
      );
    }
    if (!isByteList(message)) {
      _broken = true;
      throw const NoiseException(
        NoiseError.decryptFailed,
        'decrypt',
        'value outside a byte',
      );
    }
    if (message.length < noiseTagLength) {
      _broken = true;
      throw NoiseException(
        NoiseError.decryptFailed,
        'decrypt',
        '${message.length} bytes, minimum $noiseTagLength',
      );
    }
    final nonce = _receiveN.bytes();
    _receiveN.increment();
    try {
      final plaintext = await aeadDecrypt(
        _receiveKey,
        nonce,
        const [],
        message,
      );
      // An overlapping call that failed in the meantime breaks this one as well: its message
      // came after the failed one (docs/DECISIONS.md D-M2-38-8).
      if (_broken) {
        throw const NoiseException(NoiseError.broken, 'decrypt');
      }
      return plaintext;
    } on AuthenticationFailure {
      _broken = true;
      throw NoiseException(
        NoiseError.decryptFailed,
        'decrypt',
        '${message.length} bytes',
      );
    }
  }
}

/// Creates the transport of one side from its send and receive keys; internal to the package.
NoiseTransport newTransport(Uint8List sendKey, Uint8List receiveKey) =>
    NoiseTransport._(sendKey, receiveKey);

/// Sets the next nonces of [t]; a test hook for the nonce limit, not exported.
void setTransportNonces(
  NoiseTransport t, {
  int? sendHigh,
  int? sendLow,
  int? receiveHigh,
  int? receiveLow,
}) {
  t._sendN
    ..high = sendHigh ?? t._sendN.high
    ..low = sendLow ?? t._sendN.low;
  t._receiveN
    ..high = receiveHigh ?? t._receiveN.high
    ..low = receiveLow ?? t._receiveN.low;
}
