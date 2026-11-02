// SPDX-License-Identifier: Apache-2.0

/// The marks of a search in rendered text (ticket FEAT-markdown-viewer): every match of a
/// query in `color.attention.subtle`, the current one in `color.attention.bg`.
library;

import 'package:flutter/painting.dart';
import 'package:flutter/widgets.dart' show immutable;

import '../../generated/design_tokens.g.dart';

/// A search over a text that is drawn in pieces: the [query], the number of the match where
/// this piece begins ([first]) and the number of the current match of the whole text
/// ([active], -1 for none). Matches are counted in reading order from 0.
///
/// ```dart
/// DsMarkdown(source, onOpenLink: open, search: const DsTextSearch('token', active: 2));
/// ```
@immutable
final class DsTextSearch {
  /// Creates the search for [query].
  const DsTextSearch(this.query, {this.active = -1, this.first = 0});

  /// The searched text; case does not matter. Empty: nothing matches.
  final String query;

  /// The number of the current match of the whole text, or -1.
  final int active;

  /// The number of the first match of this piece.
  final int first;

  /// This search for a piece whose first match is number [first].
  DsTextSearch from(int first) =>
      DsTextSearch(query, active: active, first: first);

  @override
  bool operator ==(Object other) =>
      other is DsTextSearch &&
      other.query == query &&
      other.active == active &&
      other.first == first;

  @override
  int get hashCode => Object.hash(query, active, first);
}

/// The ranges (start, end exclusive) of the matches of [query] in [text], ignoring case, in
/// order and without overlap; none for an empty query.
List<(int, int)> dsMatchRanges(String text, String query) {
  if (query.isEmpty || text.isEmpty) return const [];
  final pattern = RegExp(
    RegExp.escape(query),
    caseSensitive: false,
    unicode: true,
  );
  return [
    for (final m in pattern.allMatches(text))
      if (m.end > m.start) (m.start, m.end),
  ];
}

/// One styled piece of a text: the characters from [start] to [end] in [style].
typedef DsStyledRange = (int start, int end, TextStyle style);

/// The spans of [text] in the styled [ranges] (in order, covering the text), with the matches
/// of [search] marked: every match on `color.attention.subtle`, the match numbered
/// `search.active` on `color.attention.bg` in `color.attention.on-bg`; the first match of
/// [text] is number `search.first`. Returns the spans and the number of matches in [text].
///
/// Without a search (or with an empty query) the spans are the ranges as they are.
(List<TextSpan>, int) dsMarkMatches(
  String text,
  List<DsStyledRange> ranges,
  DsTextSearch? search,
  DsColors colors,
) {
  final matches = search == null
      ? const <(int, int)>[]
      : dsMatchRanges(text, search.query);
  if (matches.isEmpty) {
    return (
      [
        for (final (s, e, style) in ranges)
          if (e > s) TextSpan(text: text.substring(s, e), style: style),
      ],
      0,
    );
  }
  final first = search!.first;
  final out = <TextSpan>[];
  var m = 0;
  for (final (s, e, style) in ranges) {
    var at = s;
    while (at < e) {
      while (m < matches.length && matches[m].$2 <= at) {
        m++;
      }
      if (m >= matches.length || matches[m].$1 >= e) {
        out.add(TextSpan(text: text.substring(at, e), style: style));
        break;
      }
      final (ms, me) = matches[m];
      if (ms > at) {
        out.add(TextSpan(text: text.substring(at, ms), style: style));
        at = ms;
      }
      final end = me < e ? me : e;
      final active = first + m == search.active;
      out.add(
        TextSpan(
          text: text.substring(at, end),
          style: active
              ? style.copyWith(
                  backgroundColor: colors.attentionBg,
                  color: colors.attentionOnBg,
                )
              : style.copyWith(backgroundColor: colors.attentionSubtle),
        ),
      );
      at = end;
    }
  }
  return (out, matches.length);
}
