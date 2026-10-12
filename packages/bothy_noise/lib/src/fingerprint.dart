// SPDX-License-Identifier: Apache-2.0

import 'dart:convert';

import 'package:cryptography/cryptography.dart';

import 'bytes.dart';
import 'constants.dart';
import 'errors.dart';

/// The domain separation prefix of the fingerprint hash (docs/spec/24-terminal.md 4.3).
const String _fingerprintLabel = 'bothy-host-fp/1';

/// The Crockford base32 alphabet: no I, L, O and U.
const String _alphabet = '0123456789ABCDEFGHJKMNPQRSTVWXYZ';

/// Returns [bytes] in Crockford base32: 5-bit groups from the most significant bit, the last
/// group padded with zero bits on the right, upper case, no padding character and no check
/// symbol (docs/spec/24-terminal.md 4.3).
///
/// Example: `crockfordBase32(utf8.encode('foobar'))` is `CSQPYRK1E8`.
String crockfordBase32(List<int> bytes) {
  requireBytes(bytes, 'crockfordBase32 input');
  final out = StringBuffer();
  var acc = 0;
  var bits = 0;
  for (final b in bytes) {
    acc = (acc << 8 | (b & 0xff)) & 0xfff;
    bits += 8;
    while (bits >= 5) {
      bits -= 5;
      out.write(_alphabet[(acc >> bits) & 0x1f]);
    }
  }
  if (bits > 0) {
    out.write(_alphabet[(acc << (5 - bits)) & 0x1f]);
  }
  return out.toString();
}

/// Returns the display form of the host key fingerprint: 52 Crockford base32 characters of
/// `SHA-256("bothy-host-fp/1" || noisePublicKey || authPublicKey)` in 13 groups of four joined
/// by `-` (docs/spec/24-terminal.md 4.3, `key_fingerprint`).
///
/// [noisePublicKey] is the raw X25519 key, [authPublicKey] the raw Ed25519 key (the decoded `x`
/// of the JWK); both must be 32 bytes, else a [NoiseException] with [NoiseError.invalidKey] is
/// thrown.
Future<String> hostKeyFingerprint(
  List<int> noisePublicKey,
  List<int> authPublicKey,
) async {
  if (noisePublicKey.length != keyLength ||
      authPublicKey.length != keyLength ||
      !isByteList(noisePublicKey) ||
      !isByteList(authPublicKey)) {
    throw NoiseException(
      NoiseError.invalidKey,
      'fingerprint',
      'keys of ${noisePublicKey.length} and ${authPublicKey.length} bytes, want $keyLength',
    );
  }
  final hash = await Sha256().hash([
    ...ascii.encode(_fingerprintLabel),
    ...noisePublicKey,
    ...authPublicKey,
  ]);
  final encoded = crockfordBase32(hash.bytes);
  final groups = [
    for (var i = 0; i < encoded.length; i += 4) encoded.substring(i, i + 4),
  ];
  return groups.join('-');
}
