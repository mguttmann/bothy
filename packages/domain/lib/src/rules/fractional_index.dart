// SPDX-License-Identifier: Apache-2.0

/// The fractional index of orders that change one row per move (03 principle 5): the key rule
/// the server validates (`fracindex` of M1-52), shared by every client that writes a key
/// (docs/DECISIONS.md D-M4-19-2).
library;

/// Order keys of the base-62 scheme with an integer part.
///
/// Digits are `0-9A-Za-z` in ASCII order, so the byte order of two keys is their order (the
/// server compares them with `COLLATE "C"`). A key is an integer part, a head letter followed by
/// its digits (`a` to `z`: 1 to 26 digits, `A` to `Z`: 26 to 1 digits for the negative range),
/// plus an optional fraction that never ends with `0`. The smallest integer, `A` followed by 26
/// zeros, is no key by itself, so there is always room before every key. The algorithm is the
/// server's, step by step, so every key [between] returns passes `putMySidebarSpace`.
///
/// Example:
/// ```dart
/// final first = FractionalIndex.between(null, null); // 'a0'
/// final next = FractionalIndex.between(first, null); // 'a1'
/// final mid = FractionalIndex.between(first, next); // 'a0V'
/// ```
abstract final class FractionalIndex {
  static const String _digits =
      '0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz';

  static final String _smallestInteger = 'A${'0' * 26}';

  /// Whether [key] is a key of the grammar: a valid integer part, only base-62 digits, no
  /// fraction ending in `0`, not the smallest integer.
  static bool isValid(String key) => _split(key) != null;

  /// A key strictly between [a] and [b]; null is an open end.
  ///
  /// Both null give `a0`; with [b] null the integer part of [a] grows (or, when it cannot, a
  /// fraction is appended); with [a] null the integer part of [b] shrinks likewise; otherwise
  /// the shared prefix plus the digit midpoint.
  ///
  /// Throws an [ArgumentError] for an invalid bound, for [a] not below [b], and when the key
  /// range is exhausted (26 integer digits on one side; it does not happen in practice).
  static String between(String? a, String? b) {
    final ia = a == null ? '' : _bound(a, 'a');
    final ib = b == null ? '' : _bound(b, 'b');
    if (a != null && b != null && a.compareTo(b) >= 0) {
      throw ArgumentError.value(b, 'b', 'is not above $a');
    }
    if (a == null && b == null) return 'a0';
    if (a == null) return _before(b!, ib);
    if (b == null) {
      return _increment(ia) ?? ia + _midpoint(a.substring(ia.length), '');
    }
    if (ia == ib) {
      return ia + _midpoint(a.substring(ia.length), b.substring(ib.length));
    }
    final i = _increment(ia);
    if (i == null) throw ArgumentError('the key range is exhausted');
    if (i.compareTo(b) < 0) return i;
    return ia + _midpoint(a.substring(ia.length), '');
  }

  static String _bound(String key, String name) =>
      _split(key) ?? (throw ArgumentError.value(key, name, 'is no valid key'));

  static String _before(String b, String ib) {
    if (ib == _smallestInteger) {
      return ib + _midpoint('', b.substring(ib.length));
    }
    if (ib.compareTo(b) < 0) return ib;
    return _decrement(ib) ??
        (throw ArgumentError('the key range is exhausted'));
  }

  static int _integerLength(String head) {
    final h = head.codeUnitAt(0);
    if (h >= 0x61 && h <= 0x7a) return h - 0x61 + 2;
    if (h >= 0x41 && h <= 0x5a) return 0x5a - h + 2;
    return 0;
  }

  /// The integer part of [key], or null when [key] is no key.
  static String? _split(String key) {
    if (key.isEmpty) return null;
    final n = _integerLength(key[0]);
    if (n == 0 || n > key.length) return null;
    for (var i = 1; i < key.length; i++) {
      if (!_digits.contains(key[i])) return null;
    }
    if (key == _smallestInteger || (key.length > n && key.endsWith('0'))) {
      return null;
    }
    return key.substring(0, n);
  }

  /// A fraction strictly between the fractions [a] and [b]; an empty [b] has no upper bound.
  static String _midpoint(String a, String b) {
    if (b.isNotEmpty) {
      var n = 0;
      while (n < b.length && _digitAt(a, n) == b[n]) {
        n++;
      }
      if (n > 0) {
        return b.substring(0, n) + _midpoint(_tail(a, n), b.substring(n));
      }
    }
    final da = a.isEmpty ? 0 : _digits.indexOf(a[0]);
    final db = b.isEmpty ? _digits.length : _digits.indexOf(b[0]);
    if (db - da > 1) return _digits[(da + db + 1) ~/ 2];
    if (b.length > 1) return b.substring(0, 1);
    return _digits[da] + _midpoint(_tail(a, 1), '');
  }

  static String _digitAt(String s, int i) => i < s.length ? s[i] : '0';

  static String _tail(String s, int n) => n >= s.length ? '' : s.substring(n);

  /// The integer after [x], or null at the end of the range.
  static String? _increment(String x) {
    final head = x[0];
    final ds = x.substring(1).split('');
    var carry = true;
    for (var i = ds.length - 1; carry && i >= 0; i--) {
      final d = _digits.indexOf(ds[i]) + 1;
      if (d == _digits.length) {
        ds[i] = '0';
      } else {
        ds[i] = _digits[d];
        carry = false;
      }
    }
    if (!carry) return head + ds.join();
    if (head == 'Z') return 'a0';
    if (head == 'z') return null;
    final h = String.fromCharCode(head.codeUnitAt(0) + 1);
    if (h.compareTo('a') > 0) {
      ds.add('0');
    } else {
      ds.removeLast();
    }
    return h + ds.join();
  }

  /// The integer before [x], or null at the start of the range.
  static String? _decrement(String x) {
    final head = x[0];
    final ds = x.substring(1).split('');
    var borrow = true;
    for (var i = ds.length - 1; borrow && i >= 0; i--) {
      final d = _digits.indexOf(ds[i]) - 1;
      if (d == -1) {
        ds[i] = _digits[_digits.length - 1];
      } else {
        ds[i] = _digits[d];
        borrow = false;
      }
    }
    if (!borrow) return head + ds.join();
    if (head == 'a') return 'Z${_digits[_digits.length - 1]}';
    if (head == 'A') return null;
    final h = String.fromCharCode(head.codeUnitAt(0) - 1);
    if (h.compareTo('Z') < 0) {
      ds.add(_digits[_digits.length - 1]);
    } else {
      ds.removeLast();
    }
    return h + ds.join();
  }
}
