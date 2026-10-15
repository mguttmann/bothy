// SPDX-License-Identifier: Apache-2.0

/// UUIDv7 generator for the IDs the client creates itself (04 5.1: `client_msg_id`,
/// `client_interaction_id`, outbox entries).
library;

/// Creates RFC 9562 version 7 UUIDs that sort in creation order on one device.
///
/// Layout (RFC 9562 5.7): 48 bits Unix time in milliseconds, 4 bits version `7`, 12 bits
/// `rand_a`, 2 bits variant `10`, 62 bits `rand_b`. `rand_a` is a counter (method 1 of
/// RFC 9562 6.2): seeded randomly with 11 bits at each new millisecond, so at least 2048
/// increments fit before it overflows, incremented for every further ID in the same
/// millisecond, and on overflow the timestamp advances by one millisecond. A clock that goes
/// backwards never makes an ID smaller than the previous one.
///
/// Clock and randomness are injected (15 1 principle 5), so tests are deterministic.
///
/// Example:
/// ```dart
/// final gen = Uuid7Generator(
///   nowMillis: () => DateTime.now().millisecondsSinceEpoch,
///   random: (bits) => secureRandom.nextInt(1 << bits),
/// );
/// final id = gen.next(); // '0192f4a0-7b1c-7...'
/// ```
final class Uuid7Generator {
  /// Creates a generator.
  ///
  /// [nowMillis] returns the current Unix time in milliseconds. [random] returns a uniformly
  /// distributed non-negative integer below `2^bits`; it is called with `bits` from 1 to 32
  /// and must come from a cryptographically secure source in the apps.
  Uuid7Generator({required this.nowMillis, required this.random});

  /// The injected clock: current Unix time in milliseconds.
  final int Function() nowMillis;

  /// The injected source of randomness: an integer below `2^bits`.
  final int Function(int bits) random;
  int _lastMillis = -1;
  int _counter = 0;

  static const int _counterMax = 0xfff;
  static const int _maxMillis = 0xffffffffffff;

  /// Returns the next UUID in canonical lowercase form, strictly greater than the previous one
  /// of this generator.
  ///
  /// Throws a [StateError] when the clock reports a time outside the 48-bit range.
  String next() {
    final now = nowMillis();
    if (now < 0 || now > _maxMillis) {
      throw StateError('clock outside the 48-bit range of UUIDv7');
    }
    if (now > _lastMillis) {
      _lastMillis = now;
      _counter = _bits(11);
    } else {
      _counter++;
      if (_counter > _counterMax) {
        // Overflow: advance the timestamp instead of reusing a value (RFC 9562 6.2 method 1).
        _lastMillis++;
        _counter = _bits(11);
      }
    }
    final ms = _lastMillis;
    final timeHigh = ms ~/ 0x10000; // upper 32 bits
    final timeLow = ms % 0x10000; // lower 16 bits
    final versionCounter = 0x7000 | _counter;
    final variantRand = 0x8000 | _bits(14);
    final nodeHigh = _bits(16);
    final nodeLow = _bits(32);
    return '${_hex(timeHigh, 8)}-${_hex(timeLow, 4)}-${_hex(versionCounter, 4)}-'
        '${_hex(variantRand, 4)}-${_hex(nodeHigh, 4)}${_hex(nodeLow, 8)}';
  }

  int _bits(int bits) {
    final value = random(bits);
    // Masking keeps the layout valid even if an injected source returns too many bits.
    return value & ((1 << bits) - 1);
  }

  static String _hex(int value, int width) =>
      value.toRadixString(16).padLeft(width, '0');
}
