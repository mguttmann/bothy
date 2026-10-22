// SPDX-License-Identifier: Apache-2.0

/// The pin check before every opening and attaching (docs/spec/15-client-architecture.md 23.3,
/// R-CL-16; docs/spec/24-terminal.md 5.5).
library;

import 'dart:convert';

import 'package:flutter/foundation.dart';
import 'package:terminal_ui/src/ports.dart';

/// The result of comparing the pin of a device with the key `getHost` delivers.
sealed class PinCheck {
  const PinCheck();
}

/// No pin yet: open, and pin the key after the first successful handshake.
final class PinFirstUse extends PinCheck {
  /// Creates the result.
  const PinFirstUse();
}

/// The pinned key is the delivered key: no notice.
final class PinMatches extends PinCheck {
  /// Creates the result.
  const PinMatches();
}

/// The delivered key differs from the pin: the hard warning with both fingerprints.
///
/// Only the key decides. A higher `key_generation` alone does not lift the warning, because
/// the person must consciously accept even a re-pairing by an owner (TM-T18); a different
/// generation with the same key is no key change.
@immutable
final class PinChanged extends PinCheck {
  /// Creates the result.
  const PinChanged({required this.pinned, required this.currentFingerprint});

  /// The stored pin with the old fingerprint.
  final HostKeyPin pinned;

  /// The `key_fingerprint` the server delivers now.
  final String currentFingerprint;
}

/// Compares [pin] with the key [noisePublicKey] (base64url, as `Host.noise_public_key`) and
/// returns what the view does.
///
/// ```dart
/// final check = checkPin(await pins.read(host.id), host.noisePublicKey!, host.keyFingerprint!);
/// ```
PinCheck checkPin(
  HostKeyPin? pin,
  String noisePublicKey,
  String currentFingerprint,
) {
  if (pin == null) return const PinFirstUse();
  if (sameNoiseKey(pin.noisePublicKey, noisePublicKey)) {
    return const PinMatches();
  }
  return PinChanged(pinned: pin, currentFingerprint: currentFingerprint);
}

/// Whether two base64url keys (with or without padding) carry the same bytes; a value that
/// does not decode never matches.
bool sameNoiseKey(String a, String b) {
  final x = decodeNoiseKey(a);
  final y = decodeNoiseKey(b);
  if (x == null || y == null || x.length != y.length) return false;
  var diff = 0;
  for (var i = 0; i < x.length; i++) {
    diff |= x[i] ^ y[i];
  }
  return diff == 0;
}

/// Decodes a base64url key with or without padding; null when it does not decode.
Uint8List? decodeNoiseKey(String text) {
  try {
    return base64Url.decode(base64Url.normalize(text));
  } on FormatException {
    return null;
  }
}
