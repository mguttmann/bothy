// SPDX-License-Identifier: Apache-2.0

// The handshake pattern NK of Noise revision 34, section 7.5: `<- s`, `...`, `-> e, es`,
// `<- e, ee`, with the prologue of docs/spec/24-terminal.md 5.2.

import 'dart:convert';
import 'dart:typed_data';

import 'package:cryptography/cryptography.dart';

import 'bytes.dart';
import 'constants.dart';
import 'errors.dart';
import 'ids.dart';
import 'symmetric.dart';
import 'transport.dart';

/// The part of a handshake message that is not payload: the ephemeral key and the tag.
const int _handshakeOverhead = keyLength + noiseTagLength;

final X25519 _defaultX25519 = X25519();
KeyExchangeAlgorithm _x25519 = _defaultX25519;

/// Replaces the X25519 implementation for tests (null restores the default); not exported. It
/// lets the tests reproduce the failure mode of the web implementation, which throws its own
/// errors for a low-order key.
void debugUseKeyExchange(KeyExchangeAlgorithm? algorithm) {
  _x25519 = algorithm ?? _defaultX25519;
}

/// X25519 that fails on a result of 32 zero bytes, which a low-order public key produces
/// (docs/DECISIONS.md D-M2-38-9); the Go side behaves the same.
Future<Uint8List> _dh(
  SimpleKeyPair own,
  List<int> remotePublic,
  String operation,
) async {
  final Uint8List out;
  try {
    final secret = await _x25519.sharedSecretKey(
      keyPair: own,
      remotePublicKey: SimplePublicKey(remotePublic, type: KeyPairType.x25519),
    );
    out = Uint8List.fromList(await secret.extractBytes());
  } on Object {
    // Every failure of the curve function, whatever its type: the Dart VM throws ArgumentError
    // for a malformed key, the web implementation a StateError around the OperationError of
    // Web Crypto for a low-order key (docs/DECISIONS.md D-M2-38-9).
    throw NoiseException(
      NoiseError.handshakeFailed,
      operation,
      'Diffie-Hellman failed',
    );
  }
  var acc = 0;
  for (final b in out) {
    acc |= b;
  }
  if (acc == 0) {
    throw NoiseException(
      NoiseError.handshakeFailed,
      operation,
      'zero Diffie-Hellman result',
    );
  }
  return out;
}

/// Returns the public key of [pair] after checking that it is an X25519 pair whose public key
/// belongs to its private key.
Future<Uint8List> _checkedPublic(SimpleKeyPair pair, String operation) async {
  final SimplePublicKey public;
  final List<int> private;
  try {
    public = await pair.extractPublicKey();
    private = await pair.extractPrivateKeyBytes();
  } on Object {
    throw NoiseException(
      NoiseError.invalidKey,
      operation,
      'key pair cannot be read',
    );
  }
  if (public.type != KeyPairType.x25519 ||
      public.bytes.length != keyLength ||
      private.length != keyLength) {
    throw NoiseException(
      NoiseError.invalidKey,
      operation,
      'not an X25519 key pair',
    );
  }
  final derived = await (await _defaultX25519.newKeyPairFromSeed(private))
      .extractPublicKey();
  if (!_equal(derived.bytes, public.bytes)) {
    throw NoiseException(
      NoiseError.invalidKey,
      operation,
      'public key does not belong to the pair',
    );
  }
  return Uint8List.fromList(public.bytes);
}

bool _equal(List<int> a, List<int> b) {
  if (a.length != b.length) return false;
  var acc = 0;
  for (var i = 0; i < a.length; i++) {
    acc |= a[i] ^ b[i];
  }
  return acc == 0;
}

Future<SymmetricState> _initialState(
  List<int> prologue,
  List<int> responderStatic,
) async {
  final ss = SymmetricState.initialize(ascii.encode(noiseProtocolName));
  await ss.mixHash(prologue);
  await ss.mixHash(responderStatic);
  return ss;
}

/// The result of a completed handshake on the device.
class NkResult {
  /// Creates the result; built by [NkInitiator.readMessage2].
  NkResult(this.payload, this.transport, this.handshakeHash);

  /// The host's payload of message 2.
  final Uint8List payload;

  /// The transport of the device.
  final NoiseTransport transport;

  /// The handshake hash h, equal on both sides (a channel binding).
  final Uint8List handshakeHash;
}

/// The device side of the handshake `Noise_NK_25519_ChaChaPoly_SHA256`: [writeMessage1], then
/// [readMessage2]. Not safe for overlapping calls: await each call before the next.
class NkInitiator {
  NkInitiator._(
    this._ss,
    this._hostStatic,
    this._ephemeral,
    this._ephemeralPublic,
  );

  final SymmetricState _ss;
  final Uint8List _hostStatic;
  final SimpleKeyPair _ephemeral;
  final Uint8List _ephemeralPublic;
  int _step = 0;
  bool _broken = false;

  /// Starts the handshake to the host with [hostStaticPublicKey] (its pinned
  /// `noise_public_key`) for the session [sessionId] on the host [hostId] (16 bytes each,
  /// [uuidToBytes]). [ephemeral] is the pair whose [ephemeralKeyHash] the device sent when it
  /// opened the session.
  ///
  /// Throws [NoiseException] with [NoiseError.invalidKey] for a host key that is not 32 bytes
  /// or an ephemeral pair that is not X25519, and [NoiseError.invalidId] for IDs that are not
  /// 16 bytes.
  static Future<NkInitiator> start({
    required List<int> hostStaticPublicKey,
    required List<int> sessionId,
    required List<int> hostId,
    required SimpleKeyPair ephemeral,
  }) async => startInitiatorWithPrologue(
    hostStaticPublicKey: hostStaticPublicKey,
    prologue: buildPrologue(sessionId, hostId),
    ephemeral: ephemeral,
  );

  /// Returns message 1 (`-> e, es`) with [payload], the session token, encrypted to the host.
  ///
  /// A payload that would make the message longer than [maxNoiseMessageLength] throws
  /// [NoiseError.messageTooLong] and changes no state; a failed step (a low-order host key)
  /// throws [NoiseError.handshakeFailed] and breaks the initiator.
  Future<Uint8List> writeMessage1(List<int> payload) async {
    _guard('write message 1', 0);
    requireBytes(payload, 'payload');
    if (_handshakeOverhead + payload.length > maxNoiseMessageLength) {
      throw NoiseException(
        NoiseError.messageTooLong,
        'write message 1',
        'payload of ${payload.length} bytes, limit ${maxNoiseMessageLength - _handshakeOverhead}',
      );
    }
    _step = -1; // in progress
    try {
      await _ss.mixHash(_ephemeralPublic);
      await _ss.mixKey(await _dh(_ephemeral, _hostStatic, 'write message 1'));
      final ct = await _ss.encryptAndHash(payload);
      _step = 1;
      return Uint8List.fromList([..._ephemeralPublic, ...ct]);
    } on NoiseException {
      _broken = true;
      rethrow;
    } on Object {
      // Any other failure of a step ends the handshake as well (D-M2-38-8).
      _broken = true;
      throw const NoiseException(NoiseError.handshakeFailed, 'handshake step');
    }
  }

  /// Processes message 2 (`<- e, ee`) and returns the host's payload, the transport of the
  /// device and the handshake hash.
  ///
  /// Throws [NoiseException] with [NoiseError.messageTooLong] or [NoiseError.handshakeFailed]
  /// (wrong prologue, wrong host key, tampered or short message); both break the initiator.
  Future<NkResult> readMessage2(List<int> message) async {
    _guard('read message 2', 1);
    _step = -1;
    if (message.length > maxNoiseMessageLength) {
      _broken = true;
      throw NoiseException(
        NoiseError.messageTooLong,
        'read message 2',
        '${message.length} bytes, limit $maxNoiseMessageLength',
      );
    }
    if (!isByteList(message)) {
      _broken = true;
      throw const NoiseException(
        NoiseError.handshakeFailed,
        'read message 2',
        'value outside a byte',
      );
    }
    if (message.length < _handshakeOverhead) {
      _broken = true;
      throw NoiseException(
        NoiseError.handshakeFailed,
        'read message 2',
        '${message.length} bytes, minimum $_handshakeOverhead',
      );
    }
    try {
      final re = message.sublist(0, keyLength);
      await _ss.mixHash(re);
      await _ss.mixKey(await _dh(_ephemeral, re, 'read message 2'));
      final payload = await _ss.decryptAndHash(message.sublist(keyLength));
      final (toResponder, toInitiator) = await _ss.split();
      _step = 2;
      return NkResult(
        payload,
        newTransport(toResponder, toInitiator),
        Uint8List.fromList(_ss.h),
      );
    } on AuthenticationFailure {
      _broken = true;
      throw NoiseException(
        NoiseError.handshakeFailed,
        'read message 2',
        '${message.length} bytes',
      );
    } on NoiseException {
      _broken = true;
      rethrow;
    } on Object {
      // Any other failure of a step ends the handshake as well (D-M2-38-8).
      _broken = true;
      throw const NoiseException(NoiseError.handshakeFailed, 'handshake step');
    }
  }

  void _guard(String operation, int step) {
    if (_broken) {
      throw NoiseException(NoiseError.broken, operation);
    }
    if (_step != step) {
      throw NoiseException(NoiseError.invalidState, operation);
    }
  }
}

/// Starts an initiator with a raw [prologue]; the public test vectors of TM-T01 use their own
/// prologues. Not exported by the package.
Future<NkInitiator> startInitiatorWithPrologue({
  required List<int> hostStaticPublicKey,
  required List<int> prologue,
  required SimpleKeyPair ephemeral,
}) async {
  requireBytes(prologue, 'prologue');
  if (hostStaticPublicKey.length != keyLength ||
      !isByteList(hostStaticPublicKey)) {
    throw NoiseException(
      NoiseError.invalidKey,
      'start initiator',
      'host key of ${hostStaticPublicKey.length} bytes, want $keyLength',
    );
  }
  final ephemeralPublic = await _checkedPublic(ephemeral, 'start initiator');
  final hostStatic = Uint8List.fromList(hostStaticPublicKey);
  return NkInitiator._(
    await _initialState(prologue, hostStatic),
    hostStatic,
    ephemeral,
    ephemeralPublic,
  );
}

/// The host side of the handshake: readMessage1, then writeMessage2. Only for tests and fake
/// hosts (docs/spec/15-client-architektur.md 23.1); exported by `package:bothy_noise/testing.dart`.
class NkResponder {
  NkResponder._(this._ss, this._static, this._ephemeral, this._ephemeralPublic);

  final SymmetricState _ss;
  final SimpleKeyPair _static;
  final SimpleKeyPair _ephemeral;
  final Uint8List _ephemeralPublic;
  Uint8List? _initiatorEphemeral;
  int _step = 0;
  bool _broken = false;

  /// Starts the host side with its [staticKeyPair] for [sessionId] on [hostId]. [ephemeral]
  /// fixes the ephemeral key of message 2 for recordings; null generates a fresh one.
  static Future<NkResponder> start({
    required SimpleKeyPair staticKeyPair,
    required List<int> sessionId,
    required List<int> hostId,
    SimpleKeyPair? ephemeral,
  }) async => startResponderWithPrologue(
    staticKeyPair: staticKeyPair,
    prologue: buildPrologue(sessionId, hostId),
    ephemeral: ephemeral,
  );

  /// The initiator's ephemeral public key from message 1; the host compares its
  /// [ephemeralKeyHash] with the claim `cek`. Throws [NoiseError.invalidState] before message 1.
  Uint8List get initiatorEphemeralPublicKey {
    final e = _initiatorEphemeral;
    if (e == null) {
      throw const NoiseException(
        NoiseError.invalidState,
        'initiator ephemeral key',
      );
    }
    return Uint8List.fromList(e);
  }

  /// Processes message 1 and returns its payload, the session token.
  ///
  /// Throws [NoiseException] with [NoiseError.messageTooLong] or [NoiseError.handshakeFailed]
  /// (wrong prologue, wrong static key, tampered or short message, low-order key); both break
  /// the responder.
  Future<Uint8List> readMessage1(List<int> message) async {
    _guard('read message 1', 0);
    _step = -1;
    if (message.length > maxNoiseMessageLength) {
      _broken = true;
      throw NoiseException(
        NoiseError.messageTooLong,
        'read message 1',
        '${message.length} bytes, limit $maxNoiseMessageLength',
      );
    }
    if (!isByteList(message)) {
      _broken = true;
      throw const NoiseException(
        NoiseError.handshakeFailed,
        'read message 1',
        'value outside a byte',
      );
    }
    if (message.length < _handshakeOverhead) {
      _broken = true;
      throw NoiseException(
        NoiseError.handshakeFailed,
        'read message 1',
        '${message.length} bytes, minimum $_handshakeOverhead',
      );
    }
    try {
      final re = Uint8List.fromList(message.sublist(0, keyLength));
      await _ss.mixHash(re);
      await _ss.mixKey(await _dh(_static, re, 'read message 1'));
      final payload = await _ss.decryptAndHash(message.sublist(keyLength));
      _initiatorEphemeral = re;
      _step = 1;
      return payload;
    } on AuthenticationFailure {
      _broken = true;
      throw NoiseException(
        NoiseError.handshakeFailed,
        'read message 1',
        '${message.length} bytes',
      );
    } on NoiseException {
      _broken = true;
      rethrow;
    } on Object {
      // Any other failure of a step ends the handshake as well (D-M2-38-8).
      _broken = true;
      throw const NoiseException(NoiseError.handshakeFailed, 'handshake step');
    }
  }

  /// Returns message 2 (`<- e, ee`) with [payload] and the transport of the host.
  ///
  /// A payload that would make the message longer than [maxNoiseMessageLength] throws
  /// [NoiseError.messageTooLong] and changes no state; a failed step throws
  /// [NoiseError.handshakeFailed] and breaks the responder.
  Future<(Uint8List, NoiseTransport)> writeMessage2(List<int> payload) async {
    _guard('write message 2', 1);
    requireBytes(payload, 'payload');
    if (_handshakeOverhead + payload.length > maxNoiseMessageLength) {
      throw NoiseException(
        NoiseError.messageTooLong,
        'write message 2',
        'payload of ${payload.length} bytes, limit ${maxNoiseMessageLength - _handshakeOverhead}',
      );
    }
    _step = -1;
    try {
      await _ss.mixHash(_ephemeralPublic);
      await _ss.mixKey(
        await _dh(_ephemeral, _initiatorEphemeral!, 'write message 2'),
      );
      final ct = await _ss.encryptAndHash(payload);
      final (toResponder, toInitiator) = await _ss.split();
      _step = 2;
      return (
        Uint8List.fromList([..._ephemeralPublic, ...ct]),
        newTransport(toInitiator, toResponder),
      );
    } on NoiseException {
      _broken = true;
      rethrow;
    } on Object {
      // Any other failure of a step ends the handshake as well (D-M2-38-8).
      _broken = true;
      throw const NoiseException(NoiseError.handshakeFailed, 'handshake step');
    }
  }

  void _guard(String operation, int step) {
    if (_broken) {
      throw NoiseException(NoiseError.broken, operation);
    }
    if (_step != step) {
      throw NoiseException(NoiseError.invalidState, operation);
    }
  }
}

/// Starts a responder with a raw [prologue]; the public test vectors of TM-T01 use their own
/// prologues. Not exported by the package.
Future<NkResponder> startResponderWithPrologue({
  required SimpleKeyPair staticKeyPair,
  required List<int> prologue,
  SimpleKeyPair? ephemeral,
}) async {
  requireBytes(prologue, 'prologue');
  final staticPublic = await _checkedPublic(staticKeyPair, 'start responder');
  final eph = ephemeral ?? await _defaultX25519.newKeyPair();
  final ephPublic = await _checkedPublic(eph, 'start responder');
  return NkResponder._(
    await _initialState(prologue, staticPublic),
    staticKeyPair,
    eph,
    ephPublic,
  );
}
