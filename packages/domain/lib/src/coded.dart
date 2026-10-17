// SPDX-License-Identifier: Apache-2.0

/// Shared pieces of the entities: wire codes of closed enums and value equality of lists.
library;

/// An enum member with the text it has on the wire and in the database (03 `CHECK` lists).
abstract interface class Coded {
  /// The wire text of this member; empty for the `unknown` member of a newer server's value.
  String get code;
}

/// Finds the member of [values] whose [Coded.code] is [code], or [unknown].
///
/// Never throws: a value that a newer server adds decodes to [unknown] (06 13 item 3), and the
/// caller shows a neutral state instead of failing.
///
/// Example: `decodeCode(ChannelKind.values, 'rocket', ChannelKind.unknown)` is
/// `ChannelKind.unknown`.
T decodeCode<T extends Coded>(List<T> values, String code, T unknown) {
  for (final value in values) {
    if (value.code == code && code.isNotEmpty) return value;
  }
  return unknown;
}

/// Whether [a] and [b] have equal elements in the same order.
bool listEquals<T>(List<T> a, List<T> b) {
  if (identical(a, b)) return true;
  if (a.length != b.length) return false;
  for (var i = 0; i < a.length; i++) {
    if (a[i] != b[i]) return false;
  }
  return true;
}

/// Whether [a] and [b] hold equal JSON-like values (maps, lists, scalars), compared deeply.
bool jsonEquals(Object? a, Object? b) {
  if (identical(a, b)) return true;
  if (a is Map && b is Map) {
    if (a.length != b.length) return false;
    for (final key in a.keys) {
      if (!b.containsKey(key) || !jsonEquals(a[key], b[key])) return false;
    }
    return true;
  }
  if (a is List && b is List) {
    if (a.length != b.length) return false;
    for (var i = 0; i < a.length; i++) {
      if (!jsonEquals(a[i], b[i])) return false;
    }
    return true;
  }
  return a == b;
}

/// A hash of a JSON-like value that agrees with [jsonEquals].
int jsonHash(Object? value) {
  if (value is Map) {
    // Order-independent, as map equality ignores the order of keys.
    var hash = 0;
    for (final entry in value.entries) {
      hash ^= Object.hash(entry.key, jsonHash(entry.value));
    }
    return hash;
  }
  if (value is List) return Object.hashAll(value.map(jsonHash));
  return value.hashCode;
}
