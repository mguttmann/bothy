// SPDX-License-Identifier: Apache-2.0

/// Mention suggestions of the composer (brief M4-20 Design note 3; 01 6.7; 03 content format;
/// 07 4.6): the token at the cursor and the people and agents of the local database that match
/// it. The local database holds only principals the person may see, so nothing here decides
/// visibility (15 1 principle 1).
library;

import 'package:domain/domain.dart';

/// The mention token being typed: where its `@` stands and the typed part of the handle.
final class MentionQuery {
  /// Creates the query.
  const MentionQuery(this.start, this.prefix);

  /// The index of the `@`.
  final int start;

  /// The typed handle part, lower case; empty right after the `@`.
  final String prefix;
}

/// The characters that may stand before an `@` that starts a mention (03 content format
/// item 1): not a letter, digit or one of `_ . - @ /`.
final RegExp _tokenStart = RegExp(r'[\p{L}\p{N}_.\-@/]', unicode: true);

/// The handle characters of 03 content format item 1.
final RegExp _handleChar = RegExp('[a-z0-9._-]');

/// The mention token that ends at [cursor] in [text], or null when the cursor is not in one.
///
/// Example: `mentionQuery('Ask @ro', 7)` has start 4 and prefix `ro`.
MentionQuery? mentionQuery(String text, int cursor) {
  if (cursor < 0 || cursor > text.length) return null;
  var i = cursor;
  while (i > 0 && _handleChar.hasMatch(text[i - 1].toLowerCase())) {
    i--;
  }
  if (i == 0 || text[i - 1] != '@') return null;
  final at = i - 1;
  if (at > 0 && _tokenStart.hasMatch(text[at - 1])) return null;
  return MentionQuery(at, text.substring(i, cursor).toLowerCase());
}

/// The people and agents of [principals] whose handle starts with [prefix], sorted by handle,
/// at most [limit]; apps are no mention target (03 content format item 3).
List<Principal> suggestMentions(
  Iterable<Principal> principals,
  String prefix, {
  int limit = 8,
}) {
  final found = [
    for (final p in principals)
      if ((p.kind == PrincipalKind.user || p.kind == PrincipalKind.agent) &&
          p.handle.toLowerCase().startsWith(prefix))
        p,
  ]..sort((a, b) => a.handle.compareTo(b.handle));
  return found.length > limit ? found.sublist(0, limit) : found;
}

/// [text] with the token of [query] (up to [cursor]) replaced by `@handle` and a space;
/// returns the new text and the new cursor.
(String, int) insertMention(
  String text,
  int cursor,
  MentionQuery query,
  String handle,
) {
  final inserted = '@$handle ';
  final next =
      text.substring(0, query.start) + inserted + text.substring(cursor);
  return (next, query.start + inserted.length);
}

/// The handles (lower case) of the principals in [mentionIds] that [principals] knows; the
/// message renders only these tokens as mentions (03 content format item 4).
Set<String> mentionHandles(
  List<PrincipalId> mentionIds,
  Map<PrincipalId, Principal> principals,
) => {
  for (final id in mentionIds)
    if (principals[id] case final p?) p.handle.toLowerCase(),
};
