// SPDX-License-Identifier: Apache-2.0

/// The syntax colors of code blocks (01 6.14 item 4; ticket FEAT-markdown-viewer).
library;

import 'package:flutter/foundation.dart' show immutable;
import 'package:flutter/painting.dart';

import '../../generated/design_tokens.g.dart';

/// The kind of a piece of code, for its color.
enum DsCodeToken {
  /// Everything without a color of its own.
  plain,

  /// A keyword or a literal word of the language (`if`, `true`, `null`).
  keyword,

  /// A string or character literal.
  string,

  /// A number literal.
  number,

  /// A comment.
  comment,

  /// A type name (a capitalized identifier in languages that name types that way).
  type,

  /// An added line of a diff.
  inserted,

  /// A removed line of a diff.
  deleted,
}

/// One piece of a code text: the characters from [start] to [end] (exclusive) of kind [token].
@immutable
final class DsCodeRun {
  /// Creates the run.
  const DsCodeRun(this.start, this.end, this.token);

  /// The first character.
  final int start;

  /// The end, exclusive.
  final int end;

  /// The kind.
  final DsCodeToken token;

  @override
  bool operator ==(Object other) =>
      other is DsCodeRun &&
      other.start == start &&
      other.end == end &&
      other.token == token;

  @override
  int get hashCode => Object.hash(start, end, token);

  @override
  String toString() => 'DsCodeRun($start, $end, ${token.name})';
}

/// The lexical rules of a language family.
final class _Lexicon {
  const _Lexicon({
    required this.keywords,
    this.lineComments = const ['//'],
    this.blockComment = const ('/*', '*/'),
    this.quotes = const ['"', "'"],
    this.tripleQuotes = false,
    this.capitalTypes = true,
  });

  final Set<String> keywords;
  final List<String> lineComments;
  final (String, String)? blockComment;
  final List<String> quotes;
  final bool tripleQuotes;
  final bool capitalTypes;
}

const _cLike = {
  'abstract',
  'as',
  'async',
  'await',
  'break',
  'case',
  'catch',
  'class',
  'const',
  'continue',
  'default',
  'defer',
  'do',
  'else',
  'enum',
  'export',
  'extends',
  'false',
  'final',
  'finally',
  'fn',
  'for',
  'func',
  'fun',
  'function',
  'go',
  'if',
  'impl',
  'implements',
  'import',
  'in',
  'interface',
  'is',
  'let',
  'match',
  'mut',
  'new',
  'nil',
  'null',
  'override',
  'package',
  'private',
  'protected',
  'pub',
  'public',
  'return',
  'static',
  'struct',
  'super',
  'switch',
  'this',
  'throw',
  'throws',
  'trait',
  'true',
  'try',
  'type',
  'typeof',
  'use',
  'val',
  'var',
  'void',
  'when',
  'where',
  'while',
  'yield',
  'required',
  'late',
  'library',
  'part',
  'sealed',
  'base',
  'mixin',
  'with',
  'extension',
  'factory',
  'get',
  'set',
  'operator',
  'dynamic',
  'int',
  'double',
  'bool',
  'string',
  'char',
  'long',
  'float',
  'unsigned',
  'signed',
  'short',
  'byte',
  'namespace',
  'using',
  'readonly',
  'declare',
  'module',
  'from',
  'of',
  'undefined',
  'self',
  'Self',
  'crate',
  'mod',
  'unsafe',
  'loop',
  'range',
  'chan',
  'select',
  'map',
  'goto',
  'sizeof',
  'typedef',
  'union',
  'volatile',
  'extern',
  'inline',
  'virtual',
  'template',
  'typename',
  'delete',
  'guard',
  'protocol',
  'internal',
  'open',
  'data',
  'object',
  'companion',
  'lateinit',
};

const _python = {
  'and',
  'as',
  'assert',
  'async',
  'await',
  'break',
  'class',
  'continue',
  'def',
  'del',
  'elif',
  'else',
  'except',
  'False',
  'finally',
  'for',
  'from',
  'global',
  'if',
  'import',
  'in',
  'is',
  'lambda',
  'None',
  'nonlocal',
  'not',
  'or',
  'pass',
  'raise',
  'return',
  'True',
  'try',
  'while',
  'with',
  'yield',
  'match',
  'case',
  'self',
};

const _ruby = {
  'alias',
  'and',
  'begin',
  'break',
  'case',
  'class',
  'def',
  'defined?',
  'do',
  'else',
  'elsif',
  'end',
  'ensure',
  'false',
  'for',
  'if',
  'in',
  'module',
  'next',
  'nil',
  'not',
  'or',
  'redo',
  'require',
  'rescue',
  'retry',
  'return',
  'self',
  'super',
  'then',
  'true',
  'undef',
  'unless',
  'until',
  'when',
  'while',
  'yield',
  'attr_reader',
  'attr_accessor',
};

const _shell = {
  'if',
  'then',
  'else',
  'elif',
  'fi',
  'case',
  'esac',
  'for',
  'while',
  'until',
  'do',
  'done',
  'in',
  'function',
  'return',
  'local',
  'export',
  'readonly',
  'set',
  'unset',
  'echo',
  'exit',
  'source',
  'cd',
  'true',
  'false',
  'shift',
  'trap',
  'eval',
  'exec',
};

const _sql = {
  'select',
  'from',
  'where',
  'and',
  'or',
  'not',
  'insert',
  'into',
  'values',
  'update',
  'set',
  'delete',
  'create',
  'table',
  'index',
  'drop',
  'alter',
  'add',
  'primary',
  'key',
  'foreign',
  'references',
  'join',
  'left',
  'right',
  'inner',
  'outer',
  'on',
  'group',
  'by',
  'order',
  'having',
  'limit',
  'offset',
  'as',
  'distinct',
  'null',
  'is',
  'in',
  'exists',
  'case',
  'when',
  'then',
  'else',
  'end',
  'union',
  'all',
  'with',
  'returning',
  'default',
  'true',
  'false',
  'begin',
  'commit',
  'rollback',
  'constraint',
  'unique',
  'check',
  'view',
  'like',
  'between',
  'count',
  'sum',
  'avg',
  'min',
  'max',
  'coalesce',
  'cascade',
  'if',
};

const _literals = {'true', 'false', 'null', 'yes', 'no', 'on', 'off', '~'};

const _cLexicon = _Lexicon(keywords: _cLike, quotes: ['"', "'", '`']);
const _pythonLexicon = _Lexicon(
  keywords: _python,
  lineComments: ['#'],
  blockComment: null,
  tripleQuotes: true,
);
const _rubyLexicon = _Lexicon(
  keywords: _ruby,
  lineComments: ['#'],
  blockComment: null,
);
const _shellLexicon = _Lexicon(
  keywords: _shell,
  lineComments: ['#'],
  blockComment: null,
  capitalTypes: false,
);
const _sqlLexicon = _Lexicon(
  keywords: _sql,
  lineComments: ['--'],
  capitalTypes: false,
);
const _jsonLexicon = _Lexicon(
  keywords: _literals,
  lineComments: [],
  blockComment: null,
  quotes: ['"'],
  capitalTypes: false,
);
const _yamlLexicon = _Lexicon(
  keywords: _literals,
  lineComments: ['#'],
  blockComment: null,
  capitalTypes: false,
);

/// The lexicon of a fence label, or null for a language without colors.
_Lexicon? _lexiconOf(String? language) {
  final name = language?.trim().toLowerCase() ?? '';
  return switch (name) {
    'dart' ||
    'js' ||
    'javascript' ||
    'jsx' ||
    'mjs' ||
    'cjs' ||
    'ts' ||
    'typescript' ||
    'tsx' ||
    'java' ||
    'kotlin' ||
    'kt' ||
    'kts' ||
    'swift' ||
    'go' ||
    'golang' ||
    'rust' ||
    'rs' ||
    'c' ||
    'h' ||
    'cpp' ||
    'c++' ||
    'cc' ||
    'hpp' ||
    'cs' ||
    'csharp' ||
    'c#' ||
    'scala' ||
    'groovy' ||
    'gradle' ||
    'php' ||
    'objc' ||
    'objective-c' ||
    'proto' ||
    'protobuf' => _cLexicon,
    'python' || 'py' || 'python3' => _pythonLexicon,
    'ruby' || 'rb' => _rubyLexicon,
    'sh' ||
    'bash' ||
    'shell' ||
    'zsh' ||
    'console' ||
    'shellsession' ||
    'fish' ||
    'dockerfile' ||
    'docker' ||
    'makefile' ||
    'make' => _shellLexicon,
    'sql' || 'postgres' || 'postgresql' || 'mysql' || 'sqlite' => _sqlLexicon,
    'json' || 'jsonc' || 'json5' => _jsonLexicon,
    'yaml' || 'yml' || 'toml' || 'ini' || 'env' => _yamlLexicon,
    _ => null,
  };
}

/// Whether [language] is a diff (`diff` or `patch`), colored by line.
bool _isDiff(String? language) {
  final name = language?.trim().toLowerCase();
  return name == 'diff' || name == 'patch';
}

final RegExp _number = RegExp(
  r'0[xX][0-9a-fA-F_]+|0[bB][01_]+|\d[\d_]*(?:\.\d[\d_]*)?(?:[eE][+-]?\d+)?',
);
final RegExp _word = RegExp(r'[A-Za-z_$][\w$]*\??');

/// The longest code [dsCodeTokens] colors; a longer one stays one plain run, because the
/// layout of one paragraph of that many colored spans grows faster than its length.
const int dsCodeHighlightMaxChars = 32 * 1024;

/// The colored pieces of [code] in [language] (the label of a fenced block), in order and
/// without gaps; a language without colors (or none) is one plain run.
///
/// A small lexer, not a parser: keywords, strings, numbers, comments and capitalized type names
/// of the common languages of agent answers, and the added and removed lines of a diff. It
/// never fails; text it does not understand stays plain, and so does a code longer than
/// [dsCodeHighlightMaxChars]. The runs index [code] as it is, so a
/// copy is never touched by the colors.
///
/// ```dart
/// dsCodeTokens('final x = 1;', 'dart').first.token; // DsCodeToken.keyword
/// ```
List<DsCodeRun> dsCodeTokens(String code, String? language) {
  if (code.isEmpty) return const [];
  if (code.length > dsCodeHighlightMaxChars) {
    return [DsCodeRun(0, code.length, DsCodeToken.plain)];
  }
  if (_isDiff(language)) return _diffRuns(code);
  final lexicon = _lexiconOf(language);
  if (lexicon == null) return [DsCodeRun(0, code.length, DsCodeToken.plain)];
  return _lex(code, lexicon);
}

List<DsCodeRun> _diffRuns(String code) {
  final runs = <DsCodeRun>[];
  var at = 0;
  while (at < code.length) {
    final nl = code.indexOf('\n', at);
    final end = nl < 0 ? code.length : nl + 1;
    final line = code.substring(at, end);
    final token = line.startsWith('+') && !line.startsWith('+++')
        ? DsCodeToken.inserted
        : line.startsWith('-') && !line.startsWith('---')
        ? DsCodeToken.deleted
        : line.startsWith('@@')
        ? DsCodeToken.keyword
        : DsCodeToken.plain;
    _add(runs, at, end, token);
    at = end;
  }
  return runs;
}

/// Appends a run, merging it into the previous one of the same kind.
void _add(List<DsCodeRun> runs, int start, int end, DsCodeToken token) {
  if (end <= start) return;
  if (runs.isNotEmpty && runs.last.token == token && runs.last.end == start) {
    final last = runs.removeLast();
    runs.add(DsCodeRun(last.start, end, token));
    return;
  }
  runs.add(DsCodeRun(start, end, token));
}

List<DsCodeRun> _lex(String code, _Lexicon lx) {
  final runs = <DsCodeRun>[];
  var i = 0;
  final n = code.length;
  bool at(String s) => code.startsWith(s, i);
  while (i < n) {
    final start = i;
    // Comments.
    final line = lx.lineComments.where(at).firstOrNull;
    if (line != null) {
      final nl = code.indexOf('\n', i);
      i = nl < 0 ? n : nl;
      _add(runs, start, i, DsCodeToken.comment);
      continue;
    }
    if (lx.blockComment case (final open, final close) when at(open)) {
      final endAt = code.indexOf(close, i + open.length);
      i = endAt < 0 ? n : endAt + close.length;
      _add(runs, start, i, DsCodeToken.comment);
      continue;
    }
    // Strings.
    final ch = code[i];
    if (lx.quotes.contains(ch)) {
      if (lx.tripleQuotes && at('$ch$ch$ch')) {
        final endAt = code.indexOf('$ch$ch$ch', i + 3);
        i = endAt < 0 ? n : endAt + 3;
      } else {
        i++;
        while (i < n && code[i] != ch) {
          // A string ends at the end of its line unless it is a template string.
          if (code[i] == '\n' && ch != '`') break;
          i += code[i] == r'\' ? 2 : 1;
        }
        i = i < n && code[i] == ch ? i + 1 : (i > n ? n : i);
      }
      _add(runs, start, i, DsCodeToken.string);
      continue;
    }
    // Numbers, not inside a word.
    final prev = i == 0 ? '' : code[i - 1];
    if (_isDigit(ch) && !_isWordChar(prev)) {
      final m = _number.matchAsPrefix(code, i);
      if (m != null && m.end > i) {
        i = m.end;
        _add(runs, start, i, DsCodeToken.number);
        continue;
      }
    }
    // Words.
    if (_isWordStart(ch)) {
      final m = _word.matchAsPrefix(code, i)!;
      var word = m[0]!;
      // `defined?` is a word of Ruby only; elsewhere the question mark is an operator.
      if (word.endsWith('?') && !lx.keywords.contains(word)) {
        word = word.substring(0, word.length - 1);
      }
      i += word.length;
      final key = lx == _sqlLexicon ? word.toLowerCase() : word;
      final token = lx.keywords.contains(key)
          ? DsCodeToken.keyword
          : lx.capitalTypes && _isUpper(word.codeUnitAt(0))
          ? DsCodeToken.type
          : DsCodeToken.plain;
      _add(runs, start, i, token);
      continue;
    }
    i++;
    _add(runs, start, i, DsCodeToken.plain);
  }
  return runs;
}

bool _isDigit(String c) => c.codeUnitAt(0) >= 0x30 && c.codeUnitAt(0) <= 0x39;

bool _isUpper(int c) => c >= 0x41 && c <= 0x5a;

bool _isWordStart(String c) {
  final u = c.codeUnitAt(0);
  return (u >= 0x41 && u <= 0x5a) ||
      (u >= 0x61 && u <= 0x7a) ||
      u == 0x5f ||
      u == 0x24;
}

bool _isWordChar(String c) => c.isNotEmpty && (_isWordStart(c) || _isDigit(c));

/// The style of [token] on top of the code style [base], in the colors of [colors]: keywords in
/// `color.category.heather.fg`, strings in `color.category.moss.fg`, numbers in
/// `color.category.amber.fg`, types in `color.category.loch.fg`, comments in `color.text.muted`
/// and italic, added and removed diff lines in `color.success.fg` and `color.danger.fg`.
TextStyle dsCodeTokenStyle(
  DsCodeToken token,
  TextStyle base,
  DsColors colors,
) => switch (token) {
  DsCodeToken.plain => base,
  DsCodeToken.keyword => base.copyWith(color: colors.categoryHeatherFg),
  DsCodeToken.string => base.copyWith(color: colors.categoryMossFg),
  DsCodeToken.number => base.copyWith(color: colors.categoryAmberFg),
  DsCodeToken.type => base.copyWith(color: colors.categoryLochFg),
  DsCodeToken.comment => base.copyWith(
    color: colors.textMuted,
    fontStyle: FontStyle.italic,
  ),
  DsCodeToken.inserted => base.copyWith(color: colors.successFg),
  DsCodeToken.deleted => base.copyWith(color: colors.dangerFg),
};
