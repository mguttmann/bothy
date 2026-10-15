// SPDX-License-Identifier: Apache-2.0

/// A fixed pair of terms from the term table of docs/spec/01-produkt.md section 9.
library;

/// One German term and the English term that means the same, with the code name where the
/// specification has one.
///
/// Rows with alternatives ("Forum, Beitrag" and "Forum, Post") become one pair per
/// alternative; a parenthesis in the German cell is a note and not part of the term.
///
/// Example: `TermPair('Bereich', 'Space', 'space')`.
class TermPair {
  /// Creates a pair of [german] and [english] with the optional [code].
  const TermPair(this.german, this.english, this.code);

  /// The German term, for example `Bereich`.
  final String german;

  /// The English term, for example `Space`.
  final String english;

  /// The name in code, for example `space`; null when the table names none.
  final String? code;

  @override
  bool operator ==(Object other) =>
      other is TermPair &&
      other.german == german &&
      other.english == english &&
      other.code == code;

  @override
  int get hashCode => Object.hash(german, english, code);

  @override
  String toString() => 'TermPair($german, $english, $code)';
}
