// SPDX-License-Identifier: Apache-2.0

// TOTP of RFC 6238 for the password login of the E2E suite (08 2.5: HMAC-SHA-1, 30 s steps, six
// digits). The server accepts a step once, so the suite remembers the steps it used.

import 'package:crypto/crypto.dart';

/// The TOTP step of [time] (period 30 s, T0 = 0).
int totpStep(DateTime time) => time.millisecondsSinceEpoch ~/ 30000;

/// The six-digit code of [base32Secret] (RFC 4648 base32 without padding) for [step].
String totpCode(String base32Secret, int step) {
  final key = _base32(base32Secret);
  final message = List<int>.generate(8, (i) => (step >> (56 - 8 * i)) & 0xff);
  final sum = Hmac(sha1, key).convert(message).bytes;
  final offset = sum.last & 0x0f;
  final value =
      ((sum[offset] & 0x7f) << 24) |
      (sum[offset + 1] << 16) |
      (sum[offset + 2] << 8) |
      sum[offset + 3];
  return (value % 1000000).toString().padLeft(6, '0');
}

List<int> _base32(String text) {
  const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';
  final out = <int>[];
  var buffer = 0;
  var bits = 0;
  for (final char in text.toUpperCase().replaceAll('=', '').split('')) {
    final value = alphabet.indexOf(char);
    if (value < 0) throw FormatException('not base32: $char');
    buffer = (buffer << 5) | value;
    bits += 5;
    if (bits >= 8) {
      bits -= 8;
      out.add((buffer >> bits) & 0xff);
    }
  }
  return out;
}
