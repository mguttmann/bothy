// SPDX-License-Identifier: Apache-2.0

/// The pure rules of long texts in the composer (brief M4-47; 01 6.4 and 6.13): which paste is
/// long ([PasteRule]), the text one change of the field inserted ([insertionOf]), the file a
/// long paste becomes ([pasteFileOf]), and the cut of a draft that is larger than one message
/// may carry ([splitForLimit]): the message keeps the head, the rest goes out as one text
/// attachment of the same message; a message is never split into several.
library;

import 'dart:convert';

/// The name of the attachment that carries the rest of a draft over
/// `message_max_content_bytes` (D-M4-47-3).
const String overflowFileName = 'message.txt';

/// When a paste is long enough to become a file: more than [maxChars] characters or more than
/// [maxLines] lines (`ui.composer_paste_file_chars`, `ui.composer_paste_file_lines`).
///
/// Example:
/// ```dart
/// const rule = PasteRule(maxChars: 4000, maxLines: 100);
/// rule.isLong(clipboardText);
/// ```
final class PasteRule {
  /// Creates the rule.
  const PasteRule({required this.maxChars, required this.maxLines});

  /// The most characters (UTF-16 code units) a paste keeps as text.
  final int maxChars;

  /// The most lines a paste keeps as text.
  final int maxLines;

  /// Whether [text] is over one of the two bounds; a trailing line break starts no line.
  bool isLong(String text) =>
      text.length > maxChars || lineCount(text) > maxLines;
}

/// The number of lines of [text]; a trailing line break starts no new line.
int lineCount(String text) {
  if (text.isEmpty) return 0;
  var lines = 1;
  for (var i = 0; i < text.length - 1; i++) {
    if (text.codeUnitAt(i) == 0x0A) lines++;
  }
  return lines;
}

/// The text one change of a field inserted: at [start] the old text lost [removedLength]
/// characters (a replaced selection) and got [inserted].
final class TextInsertion {
  /// Creates the insertion.
  const TextInsertion(this.start, this.inserted, this.removedLength);

  /// Where the insertion begins, in the old and in the new text.
  final int start;

  /// The inserted text.
  final String inserted;

  /// How many characters of the old text it replaced.
  final int removedLength;
}

/// The insertion that turned [before] into [after], or null when nothing was inserted.
///
/// The longest common prefix and then the longest common suffix that does not overlap it
/// bound the change; neither splits a surrogate pair.
TextInsertion? insertionOf(String before, String after) {
  final shorter = before.length < after.length ? before.length : after.length;
  var prefix = 0;
  while (prefix < shorter &&
      before.codeUnitAt(prefix) == after.codeUnitAt(prefix)) {
    prefix++;
  }
  if (prefix > 0 && _isHigh(after.codeUnitAt(prefix - 1))) prefix--;
  var suffix = 0;
  while (suffix < shorter - prefix &&
      before.codeUnitAt(before.length - 1 - suffix) ==
          after.codeUnitAt(after.length - 1 - suffix)) {
    suffix++;
  }
  if (suffix > 0 && _isLow(after.codeUnitAt(after.length - suffix))) suffix--;
  final inserted = after.substring(prefix, after.length - suffix);
  if (inserted.isEmpty) return null;
  return TextInsertion(prefix, inserted, before.length - suffix - prefix);
}

bool _isHigh(int unit) => unit >= 0xD800 && unit <= 0xDBFF;
bool _isLow(int unit) => unit >= 0xDC00 && unit <= 0xDFFF;

/// The file a long paste becomes: its [name], [content] and declared [mimeType].
final class PasteFile {
  /// Creates the file.
  const PasteFile(this.name, this.content, this.mimeType);

  /// `paste.txt`, or `paste.<extension>` for code.
  final String name;

  /// The content of the file.
  final String content;

  /// The declared type of the upload.
  final String mimeType;
}

/// The file of the pasted [text] (D-M4-47-2): one fenced block whose language is known
/// becomes `paste.<extension>` with the code of the block, without the fence; text that is
/// clearly a file (a script with `#!`, JSON, XML, HTML, a diff) gets that extension; anything
/// else is `paste.txt` with the text as it is.
///
/// Example: `pasteFileOf('```python\nprint(1)\n```').name` is `paste.py`.
PasteFile pasteFileOf(String text) {
  final fenced = _singleFence(text);
  if (fenced != null) {
    final ext = _languages[fenced.$1.toLowerCase()] ?? 'txt';
    return PasteFile('paste.$ext', fenced.$2, _mimeOf(ext));
  }
  final ext = _contentKind(text) ?? 'txt';
  return PasteFile('paste.$ext', text, _mimeOf(ext));
}

String _mimeOf(String ext) => switch (ext) {
  'json' => 'application/json',
  'md' => 'text/markdown',
  _ => 'text/plain',
};

const Map<String, String> _languages = {
  'bash': 'sh',
  'c': 'c',
  'c++': 'cpp',
  'cpp': 'cpp',
  'cs': 'cs',
  'csharp': 'cs',
  'css': 'css',
  'dart': 'dart',
  'diff': 'diff',
  'go': 'go',
  'golang': 'go',
  'h': 'h',
  'html': 'html',
  'java': 'java',
  'javascript': 'js',
  'js': 'js',
  'json': 'json',
  'jsx': 'jsx',
  'kotlin': 'kt',
  'kt': 'kt',
  'markdown': 'md',
  'md': 'md',
  'patch': 'diff',
  'php': 'php',
  'py': 'py',
  'python': 'py',
  'rb': 'rb',
  'ruby': 'rb',
  'rs': 'rs',
  'rust': 'rs',
  'sh': 'sh',
  'shell': 'sh',
  'sql': 'sql',
  'swift': 'swift',
  'toml': 'toml',
  'ts': 'ts',
  'tsx': 'tsx',
  'typescript': 'ts',
  'xml': 'xml',
  'yaml': 'yaml',
  'yml': 'yaml',
  'zsh': 'sh',
};

/// The language and the code of [text] when it is exactly one fenced block, else null.
(String, String)? _singleFence(String text) {
  final lines = text.trim().split('\n');
  if (lines.length < 2) return null;
  final open = _fenceOf(lines.first);
  if (open == null) return null;
  final (char, run, info) = open;
  if (char == '`' && info.contains('`')) return null;
  for (var i = 1; i < lines.length; i++) {
    if (_closes(lines[i], char, run)) {
      if (i != lines.length - 1) return null;
      final inner = lines.sublist(1, i).join('\n');
      final language = info.trim().split(RegExp(r'\s+')).first;
      return (language, inner.isEmpty ? '' : '$inner\n');
    }
  }
  return null;
}

String? _contentKind(String text) {
  final t = text.trimLeft();
  if (t.startsWith('#!')) {
    final first = t.split('\n').first;
    if (first.contains('python')) return 'py';
    if (first.contains('node')) return 'js';
    if (first.contains('sh')) return 'sh';
    return null;
  }
  if (t.startsWith('<?xml')) return 'xml';
  final lower = t.length > 20
      ? t.substring(0, 20).toLowerCase()
      : t.toLowerCase();
  if (lower.startsWith('<!doctype html') || lower.startsWith('<html')) {
    return 'html';
  }
  if (t.startsWith('diff --git ') ||
      (t.startsWith('--- ') && t.contains('\n+++ '))) {
    return 'diff';
  }
  if (t.startsWith('{') || t.startsWith('[')) {
    try {
      jsonDecode(t);
      return 'json';
    } on FormatException {
      return null;
    }
  }
  return null;
}

/// A fence line: up to three spaces, then three or more backticks or tildes; the character,
/// its run and the rest of the line (CommonMark 4.5).
(String, int, String)? _fenceOf(String line) {
  final stripped = line.trimLeft();
  if (line.length - stripped.length > 3 || stripped.isEmpty) return null;
  final char = stripped[0];
  if (char != '`' && char != '~') return null;
  var run = 0;
  while (run < stripped.length && stripped[run] == char) {
    run++;
  }
  if (run < 3) return null;
  return (char, run, stripped.substring(run));
}

bool _closes(String line, String char, int run) {
  final fence = _fenceOf(line);
  return fence != null &&
      fence.$1 == char &&
      fence.$2 >= run &&
      fence.$3.trim().isEmpty;
}

/// The cut of a draft: [head] is sent as the message's text, [overflow] (null when the text
/// fits) as one attachment of the same message.
final class LimitSplit {
  /// Creates the cut.
  const LimitSplit(this.head, this.overflow);

  /// The text of the message.
  final String head;

  /// The rest, or null.
  final String? overflow;
}

/// Whether [text] has more than [maxBytes] UTF-8 bytes; encodes only when the length alone
/// does not decide (a code unit is one to three bytes).
bool exceedsBytes(String text, int maxBytes) {
  if (text.length > maxBytes) return true;
  if (text.length * 3 <= maxBytes) return false;
  return utf8.encode(text).length > maxBytes;
}

/// Cuts [text] so the head has at most [maxBytes] UTF-8 bytes (D-M4-47-3).
///
/// The head is the longest prefix within the limit that never splits a character, shortened
/// to its last line break when it has one. When the cut lies inside a fenced code block, the
/// head ends before the line that opens the block, so the block goes whole into the rest
/// (01 6.4: a code block stays one block); a block that starts the text stays cut, because
/// the head would be empty. Head and rest together are always exactly [text].
///
/// Example: `splitForLimit(longAnswer, 262144).overflow` is the rest, or null.
LimitSplit splitForLimit(String text, int maxBytes) {
  if (!exceedsBytes(text, maxBytes)) return LimitSplit(text, null);
  var used = 0;
  var cut = 0;
  for (final rune in text.runes) {
    final size = rune < 0x80
        ? 1
        : rune < 0x800
        ? 2
        : rune < 0x10000
        ? 3
        : 4;
    if (used + size > maxBytes) break;
    used += size;
    cut += rune > 0xFFFF ? 2 : 1;
  }
  if (cut > 0) {
    final newline = text.lastIndexOf('\n', cut - 1);
    if (newline >= 0) cut = newline + 1;
  }
  final open = _openFenceStart(text.substring(0, cut));
  if (open != null && open > 0) cut = open;
  return LimitSplit(text.substring(0, cut), text.substring(cut));
}

/// The offset of the line that opens a fenced block still open at the end of [head], or
/// null when no block is open there.
int? _openFenceStart(String head) {
  (String, int, int)? open;
  var start = 0;
  while (start <= head.length) {
    var end = head.indexOf('\n', start);
    if (end < 0) end = head.length;
    final line = head.substring(start, end);
    if (open == null) {
      final fence = _fenceOf(line);
      if (fence != null && !(fence.$1 == '`' && fence.$3.contains('`'))) {
        open = (fence.$1, fence.$2, start);
      }
    } else if (_closes(line, open.$1, open.$2)) {
      open = null;
    }
    if (end == head.length) break;
    start = end + 1;
  }
  return open?.$3;
}
