// SPDX-License-Identifier: Apache-2.0

/// Shared conversions between columns and domain values: JSON arrays of IDs, optional IDs,
/// enum codes and local times in milliseconds.
library;

import 'dart:convert';

/// Encodes a list of IDs or texts as a JSON array; typed IDs are their canonical text.
String encodeTexts(Iterable<Object> values) =>
    jsonEncode([for (final value in values) value.toString()]);

/// Decodes a JSON array of texts with [parse]; throws a [FormatException] for anything else.
List<T> decodeTexts<T>(String json, T Function(String text) parse) {
  final decoded = jsonDecode(json);
  if (decoded is! List) throw const FormatException('expected a JSON array');
  return List.unmodifiable([for (final item in decoded) parse(item as String)]);
}

/// Parses [text] with [parse] when it is not null.
T? optional<T>(String? text, T Function(String text) parse) =>
    text == null ? null : parse(text);

/// A local time as milliseconds since the epoch.
int toMillis(DateTime time) => time.millisecondsSinceEpoch;

/// Milliseconds since the epoch as a UTC time.
DateTime fromMillis(int millis) =>
    DateTime.fromMillisecondsSinceEpoch(millis, isUtc: true);

/// [fromMillis] for a nullable column.
DateTime? fromMillisOrNull(int? millis) =>
    millis == null ? null : fromMillis(millis);
