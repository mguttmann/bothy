// SPDX-License-Identifier: Apache-2.0

/// "Copy as text" of a text block (01 6.12 item 7, R-PR-35; ticket F-KOPIER-FLIESSTEXT): the
/// running text form of a block that holds prose to forward, not code.
library;

/// The fence languages of a block that holds text rather than code; compared in lower case.
const Set<String> _proseLanguages = {
  'text',
  'plaintext',
  'txt',
  'markdown',
  'md',
};

/// Whether a code block with the fence [language] offers "Copy as text": a block without a
/// language or with a text language (`text`, `plaintext`, `txt`, `markdown`, `md`). A block in
/// a programming language never does, so code is never changed.
bool dsIsProseBlock(String? language) {
  final l = language?.trim().toLowerCase() ?? '';
  return l.isEmpty || _proseLanguages.contains(l);
}

/// A list item: `-`, `*` or `+`, or a number with `.` or `)`, then a space or the line end.
final RegExp _listItem = RegExp(r'^\s*(?:[-*+]|\d{1,9}[.)])(?:\s|$)');

/// An ATX heading: up to three spaces, one to six `#`, then a space or the line end.
final RegExp _heading = RegExp(r'^ {0,3}#{1,6}(?:\s|$)');

/// A Markdown hard break at the end of a line: two or more spaces, or a backslash.
final RegExp _hardBreak = RegExp(r'(?: {2,}|\\)$');

/// [text] as running text: the line breaks inside a paragraph become one space, so a paste into
/// a mail or a chat reflows; a blank line stays the break between paragraphs.
///
/// * A line that starts a list item (`-`, `*`, `+`, `1.`, `1)`) or is a heading stays its own
///   line; a following line without such a start joins the item, a nested item keeps its
///   indentation.
/// * A line that ends in a Markdown hard break (two spaces or a backslash) keeps its line break;
///   the marker is dropped.
/// * CRLF and a lone CR count as line breaks; a line of only white space is blank; leading and
///   trailing blank lines are dropped and several blank lines are one paragraph break.
/// * Every other character, Unicode included, stays as it is; only the white space at a join
///   collapses to one space.
String dsProseCopy(String text) {
  final lines = text
      .replaceAll('\r\n', '\n')
      .replaceAll('\r', '\n')
      .split('\n');
  final paragraphs = <List<String>>[];
  var current = <String>[];
  var breakAfter = false;
  for (final raw in lines) {
    if (raw.trim().isEmpty) {
      if (current.isNotEmpty) paragraphs.add(current);
      current = <String>[];
      breakAfter = false;
      continue;
    }
    final hard = _hardBreak.hasMatch(raw);
    final line = (hard ? raw.replaceFirst(_hardBreak, '') : raw).trimRight();
    final heading = _heading.hasMatch(line);
    if (current.isEmpty || breakAfter || heading || _listItem.hasMatch(line)) {
      current.add(line);
    } else {
      current[current.length - 1] = '${current.last} ${line.trimLeft()}';
    }
    breakAfter = hard || heading;
  }
  if (current.isNotEmpty) paragraphs.add(current);
  return paragraphs.map((p) => p.join('\n')).join('\n\n');
}
