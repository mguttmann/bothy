// SPDX-License-Identifier: Apache-2.0

import 'dart:typed_data';

/// Whether every value of [b] is a byte (0 to 255). Every public input of type `List<int>`
/// passes this check: `Uint8List.fromList` and the cryptographic primitives would otherwise
/// truncate a larger value silently, while the Go side can only receive bytes
/// (docs/DECISIONS.md D-M2-38-21).
bool isByteList(List<int> b) {
  if (b is Uint8List) {
    return true;
  }
  for (final v in b) {
    if (v < 0 || v > 0xff) {
      return false;
    }
  }
  return true;
}

/// Throws an [ArgumentError] naming [what] when [b] holds a value outside a byte; for inputs of
/// the local caller (payloads, plaintext), whose mistake is a programming error. The message
/// never contains the values.
void requireBytes(List<int> b, String what) {
  if (!isByteList(b)) {
    throw ArgumentError('$what holds a value outside 0 to 255');
  }
}
