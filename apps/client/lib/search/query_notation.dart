// SPDX-License-Identifier: Apache-2.0

/// The notation of the search field (brief M4-23 Design note 2, docs/spec/14-search.md 4.1):
/// the app offers `from:`, `in:`, `has:file`, `before:` and `after:` and turns them into the
/// filter parameters of 14 4.4; the server never sees the notation and never interprets `q`.
library;

import 'package:domain/domain.dart';

/// The typed text split into the free text for the server and the parameters of the notation.
final class QueryNotation {
  /// Creates the result; [channelIds] in the order they were resolved.
  const QueryNotation({
    required this.text,
    this.authorId,
    this.channelIds = const [],
    this.hasFile,
    this.after,
    this.before,
  });

  /// The free text `q`: everything that is not a resolved notation token, in its order, with
  /// phrases, `or` and `-word` untouched (14 4.1).
  final String text;

  /// `author_id` from `from:<handle>`, or null.
  final PrincipalId? authorId;

  /// `channel_id` from `in:<channel>`: every visible channel of that name.
  final List<ChannelId> channelIds;

  /// `has_file` from `has:file`, or null.
  final bool? hasFile;

  /// `after` from `after:YYYY-MM-DD`: the start of that day in UTC, or null.
  final DateTime? after;

  /// `before` from `before:YYYY-MM-DD`: the start of that day in UTC, or null. The interval
  /// is half-open (14 4.4), so the named day itself is not included.
  final DateTime? before;
}

/// Splits [input] into the free text and the parameters of the notation.
///
/// [principals] are the people and agents stored on this device and [channels] the channels it
/// holds (the visible ones, 15 1 principle 1): `from:<handle>` resolves a handle (case-insensitive,
/// an optional leading `@`) to `author_id`; `in:<name>` resolves every channel of that name
/// (case-insensitive, an optional leading `#`, a name with spaces in double quotes) to
/// `channel_id`; `has:file` sets `has_file`; `before:` and `after:` take a date `YYYY-MM-DD` as
/// the start of that day in UTC (docs/decisions.d/M4-23.md). A token that does not resolve (an
/// unknown handle or channel, another `has:` value, an invalid date, a key without a value)
/// stays text, and so does everything inside a quoted phrase. The last `from:` and the last
/// date of a kind win; several `in:` add up. Never throws.
///
/// Example: `parseQueryNotation('from:sam deploy', principals: p, channels: c)` gives the text
/// `deploy` and Sam's ID as `authorId`.
QueryNotation parseQueryNotation(
  String input, {
  required List<Principal> principals,
  required List<Channel> channels,
}) {
  final text = <String>[];
  PrincipalId? authorId;
  final channelIds = <ChannelId>[];
  bool? hasFile;
  DateTime? after;
  DateTime? before;
  for (final token in _tokens(input)) {
    final colon = token.indexOf(':');
    final key = colon <= 0 ? '' : token.substring(0, colon).toLowerCase();
    final value = colon <= 0 ? '' : _unquote(token.substring(colon + 1));
    var resolved = false;
    if (value.isNotEmpty && !token.startsWith('"')) {
      switch (key) {
        case 'from':
          final handle = _strip(value, '@').toLowerCase();
          for (final p in principals) {
            if (p.handle.toLowerCase() == handle) {
              authorId = p.id;
              resolved = true;
              break;
            }
          }
        case 'in':
          final name = _strip(value, '#').toLowerCase();
          for (final c in channels) {
            if (c.name.toLowerCase() == name) {
              if (!channelIds.contains(c.id)) channelIds.add(c.id);
              resolved = true;
            }
          }
        case 'has':
          if (value.toLowerCase() == 'file') {
            hasFile = true;
            resolved = true;
          }
        case 'after' || 'before':
          final day = _day(value);
          if (day != null) {
            if (key == 'after') {
              after = day;
            } else {
              before = day;
            }
            resolved = true;
          }
      }
    }
    if (!resolved) text.add(token);
  }
  return QueryNotation(
    text: text.join(' '),
    authorId: authorId,
    channelIds: List.unmodifiable(channelIds),
    hasFile: hasFile,
    after: after,
    before: before,
  );
}

/// The tokens of [input] separated by white space; a double-quoted part (a phrase, or the value
/// of `in:"two words"`) stays inside its token. An unclosed quote runs to the end.
List<String> _tokens(String input) {
  final tokens = <String>[];
  final current = StringBuffer();
  var quoted = false;
  for (final rune in input.runes) {
    final char = String.fromCharCode(rune);
    if (char == '"') quoted = !quoted;
    if (!quoted && char.trim().isEmpty) {
      if (current.isNotEmpty) tokens.add(current.toString());
      current.clear();
    } else {
      current.write(char);
    }
  }
  if (current.isNotEmpty) tokens.add(current.toString());
  return tokens;
}

String _unquote(String value) =>
    value.length >= 2 && value.startsWith('"') && value.endsWith('"')
    ? value.substring(1, value.length - 1)
    : value;

String _strip(String value, String prefix) =>
    value.startsWith(prefix) ? value.substring(prefix.length) : value;

final RegExp _date = RegExp(r'^(\d{4})-(\d{2})-(\d{2})$');

/// The start of the day [value] (`YYYY-MM-DD`) in UTC, or null for a malformed or impossible
/// date (`2026-13-40` would roll over in [DateTime.utc]).
DateTime? _day(String value) {
  final m = _date.firstMatch(value);
  if (m == null) return null;
  final year = int.parse(m.group(1)!);
  final month = int.parse(m.group(2)!);
  final day = int.parse(m.group(3)!);
  final at = DateTime.utc(year, month, day);
  return at.year == year && at.month == month && at.day == day ? at : null;
}
