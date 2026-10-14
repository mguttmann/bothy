// SPDX-License-Identifier: Apache-2.0

// CipherState and SymmetricState of the Noise Protocol Framework, revision 34, sections 5.1
// and 5.2, for the cipher ChaChaPoly and the hash SHA-256 (section 12.3, 12.4). Internal: the
// public surface is NkInitiator, NkResponder and NoiseTransport.

import 'dart:typed_data';

import 'package:cryptography/cryptography.dart';

import 'constants.dart';

final _aead = Chacha20.poly1305Aead();
final _hmac = Hmac.sha256();
final _sha256 = Sha256();

/// Thrown internally when a ciphertext does not authenticate; the callers map it to a
/// NoiseException.
class AuthenticationFailure implements Exception {
  /// Creates the marker exception.
  const AuthenticationFailure();
}

/// A 64-bit nonce kept as two 32-bit words, so that the counter is exact on the web as well,
/// where Dart integers are doubles.
class Nonce {
  /// The high and low 32-bit words of the counter.
  int high = 0, low = 0;

  /// Whether the counter holds 2^64 - 1, the value Noise revision 34, section 5.1 reserves.
  bool get exhausted => high == 0xffffffff && low == 0xffffffff;

  /// The 12-byte ChaChaPoly nonce: 4 zero bytes, then the counter little-endian (section 12.3).
  Uint8List bytes() {
    final b = Uint8List(12);
    final view = ByteData.sublistView(b);
    view.setUint32(4, low, Endian.little);
    view.setUint32(8, high, Endian.little);
    return b;
  }

  /// Advances the counter by one.
  void increment() {
    if (low == 0xffffffff) {
      low = 0;
      high++;
    } else {
      low++;
    }
  }
}

/// ChaChaPoly with associated data under a 32-byte key; ciphertext and tag concatenated.
Future<Uint8List> aeadEncrypt(
  List<int> key,
  Uint8List nonce,
  List<int> ad,
  List<int> plaintext,
) async {
  final box = await _aead.encrypt(
    plaintext,
    secretKey: SecretKeyData(key),
    nonce: nonce,
    aad: ad,
  );
  return Uint8List.fromList([...box.cipherText, ...box.mac.bytes]);
}

/// Opens a ChaChaPoly ciphertext with its tag; throws [AuthenticationFailure].
Future<Uint8List> aeadDecrypt(
  List<int> key,
  Uint8List nonce,
  List<int> ad,
  List<int> message,
) async {
  if (message.length < noiseTagLength) {
    throw const AuthenticationFailure();
  }
  final split = message.length - noiseTagLength;
  final box = SecretBox(
    message.sublist(0, split),
    nonce: nonce,
    mac: Mac(message.sublist(split)),
  );
  try {
    return Uint8List.fromList(
      await _aead.decrypt(box, secretKey: SecretKeyData(key), aad: ad),
    );
  } on SecretBoxAuthenticationError {
    throw const AuthenticationFailure();
  }
}

Future<Uint8List> _hmacOf(List<int> key, List<int> data) async =>
    Uint8List.fromList(
      (await _hmac.calculateMac(data, secretKey: SecretKeyData(key))).bytes,
    );

/// The HKDF of Noise revision 34, section 4.3, with two outputs.
Future<(Uint8List, Uint8List)> hkdf2(
  List<int> chainingKey,
  List<int> ikm,
) async {
  final temp = await _hmacOf(chainingKey, ikm);
  final out1 = await _hmacOf(temp, const [0x01]);
  final out2 = await _hmacOf(temp, [...out1, 0x02]);
  return (out1, out2);
}

/// The SymmetricState of the handshake (section 5.2) with its CipherState (k, n).
class SymmetricState {
  SymmetricState._(this.h, this.ck);

  /// InitializeSymmetric with a protocol name of exactly 32 bytes: h is the name itself.
  factory SymmetricState.initialize(List<int> protocolName) {
    assert(protocolName.length == keyLength);
    return SymmetricState._(
      Uint8List.fromList(protocolName),
      Uint8List.fromList(protocolName),
    );
  }

  /// The handshake hash.
  Uint8List h;

  /// The chaining key.
  Uint8List ck;

  Uint8List? _k;
  final Nonce _n = Nonce();

  /// MixHash: h = SHA-256(h || data).
  Future<void> mixHash(List<int> data) async {
    h = Uint8List.fromList((await _sha256.hash([...h, ...data])).bytes);
  }

  /// MixKey: ck, k = HKDF(ck, ikm); n = 0.
  Future<void> mixKey(List<int> ikm) async {
    final (ck2, k) = await hkdf2(ck, ikm);
    ck = ck2;
    _k = k;
    _n
      ..high = 0
      ..low = 0;
  }

  /// EncryptAndHash: encrypts with h as associated data once a key exists, then mixes the
  /// ciphertext into h.
  Future<Uint8List> encryptAndHash(List<int> plaintext) async {
    final k = _k;
    final out = k == null
        ? Uint8List.fromList(plaintext)
        : await aeadEncrypt(k, _n.bytes(), h, plaintext);
    if (k != null) _n.increment();
    await mixHash(out);
    return out;
  }

  /// DecryptAndHash: the inverse of [encryptAndHash]; throws [AuthenticationFailure].
  Future<Uint8List> decryptAndHash(List<int> ciphertext) async {
    final k = _k;
    final out = k == null
        ? Uint8List.fromList(ciphertext)
        : await aeadDecrypt(k, _n.bytes(), h, ciphertext);
    if (k != null) _n.increment();
    await mixHash(ciphertext);
    return out;
  }

  /// Split: the two transport keys, the first for messages from initiator to responder.
  Future<(Uint8List, Uint8List)> split() => hkdf2(ck, const []);
}
