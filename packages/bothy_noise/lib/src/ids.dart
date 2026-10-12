// SPDX-License-Identifier: Apache-2.0

import 'dart:convert';
import 'dart:typed_data';

import 'package:cryptography/cryptography.dart';

import 'bytes.dart';
import 'constants.dart';
import 'errors.dart';

const int _uuidLength = 36;

/// Returns the 16 bytes of a session or host ID in the octet order of RFC 9562, the bytes as
/// the string reads from left to right (docs/DECISIONS.md D-M2-38-7).
///
/// Only the canonical 36-character form 8-4-4-4-12 is accepted, with hex digits in either case;
/// braces, a `urn:uuid:` prefix or other lengths throw a [NoiseException] with
/// [NoiseError.invalidId]. The version is not checked: the prologue binds bytes.
///
/// Example: `uuidToBytes('0192f4a0-7b1c-7b22-9d4f-5a6b7c8d9e0f')` starts with `01 92 f4 a0`.
Uint8List uuidToBytes(String uuid) {
  if (uuid.length != _uuidLength) {
    throw NoiseException(
      NoiseError.invalidId,
      'parse id',
      '${uuid.length} characters, want $_uuidLength',
    );
  }
  final out = Uint8List(idLength);
  var o = 0;
  for (var i = 0; i < uuid.length;) {
    if (i == 8 || i == 13 || i == 18 || i == 23) {
      if (uuid.codeUnitAt(i) != 0x2d) {
        throw NoiseException(
          NoiseError.invalidId,
          'parse id',
          'no hyphen at position $i',
        );
      }
      i++;
      continue;
    }
    out[o++] =
        _hexValue(uuid.codeUnitAt(i)) << 4 | _hexValue(uuid.codeUnitAt(i + 1));
    i += 2;
  }
  return out;
}

int _hexValue(int c) {
  if (c >= 0x30 && c <= 0x39) return c - 0x30;
  if (c >= 0x61 && c <= 0x66) return c - 0x61 + 10;
  if (c >= 0x41 && c <= 0x46) return c - 0x41 + 10;
  throw const NoiseException(
    NoiseError.invalidId,
    'parse id',
    'not hexadecimal',
  );
}

/// Returns the prologue `"bothy-terminal/1" || sessionId || hostId`, 48 bytes
/// (docs/spec/24-terminal.md 5.2). Both IDs must be 16 bytes ([uuidToBytes]), else a
/// [NoiseException] with [NoiseError.invalidId] is thrown.
Uint8List buildPrologue(List<int> sessionId, List<int> hostId) {
  if (sessionId.length != idLength ||
      hostId.length != idLength ||
      !isByteList(sessionId) ||
      !isByteList(hostId)) {
    throw NoiseException(
      NoiseError.invalidId,
      'build prologue',
      'ids of ${sessionId.length} and ${hostId.length} bytes, want $idLength',
    );
  }
  return Uint8List.fromList([
    ...ascii.encode(prologueLabel),
    ...sessionId,
    ...hostId,
  ]);
}

/// Generates a fresh X25519 key pair for message 1 of a handshake. The device computes
/// [ephemeralKeyHash] of its public key before it opens the session
/// (docs/spec/15-client-architektur.md 23.2, step 2) and passes the same pair to
/// `NkInitiator.start`.
Future<SimpleKeyPair> generateEphemeralKeyPair() => X25519().newKeyPair();

/// Returns the SHA-256 of a 32-byte X25519 public key in base64url without padding: the
/// `client_ephemeral_key_hash` of `TerminalSessionOpen` and the claim `cek` of the session token
/// (docs/spec/24-terminal.md 5.3). Another length throws a [NoiseException] with
/// [NoiseError.invalidKey].
Future<String> ephemeralKeyHash(List<int> ephemeralPublicKey) async {
  if (ephemeralPublicKey.length != keyLength ||
      !isByteList(ephemeralPublicKey)) {
    throw NoiseException(
      NoiseError.invalidKey,
      'ephemeral key hash',
      '${ephemeralPublicKey.length} bytes, want $keyLength',
    );
  }
  final hash = await Sha256().hash(ephemeralPublicKey);
  return base64Url.encode(hash.bytes).replaceAll('=', '');
}
