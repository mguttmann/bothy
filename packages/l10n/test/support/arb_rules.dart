// SPDX-License-Identifier: Apache-2.0

// The rules the ARB gates check (docs/spec/01-product.md section 9, brief M4-06 Design notes
// 2 to 4). They live with the tests because they are gates over the source files, not code
// the apps ship.

import 'package:l10n/l10n.dart';

/// The emoji code point ranges, inclusive.
///
/// Source: Unicode 13.0, `emoji-data.txt`, property `Extended_Pictographic` (derived from the
/// Unicode Character Database of Perl 5.34, which ships Unicode 13.0), plus the regional
/// indicators U+1F1E6 to U+1F1FF and the emoji variation selector U+FE0F. The property is
/// stable by design: its ranges reserve the unassigned code points of the emoji blocks, so a
/// later Unicode version assigns new emoji inside them.
const List<(int, int)> emojiRanges = [
  (0x00A9, 0x00A9), (0x00AE, 0x00AE), (0x203C, 0x203C), (0x2049, 0x2049),
  (0x2122, 0x2122), (0x2139, 0x2139), (0x2194, 0x2199), (0x21A9, 0x21AA),
  (0x231A, 0x231B), (0x2328, 0x2328), (0x2388, 0x2388), (0x23CF, 0x23CF),
  (0x23E9, 0x23F3), (0x23F8, 0x23FA), (0x24C2, 0x24C2), (0x25AA, 0x25AB),
  (0x25B6, 0x25B6), (0x25C0, 0x25C0), (0x25FB, 0x25FE), (0x2600, 0x2605),
  (0x2607, 0x2612), (0x2614, 0x2685), (0x2690, 0x2705), (0x2708, 0x2712),
  (0x2714, 0x2714), (0x2716, 0x2716), (0x271D, 0x271D), (0x2721, 0x2721),
  (0x2728, 0x2728), (0x2733, 0x2734), (0x2744, 0x2744), (0x2747, 0x2747),
  (0x274C, 0x274C), (0x274E, 0x274E), (0x2753, 0x2755), (0x2757, 0x2757),
  (0x2763, 0x2767), (0x2795, 0x2797), (0x27A1, 0x27A1), (0x27B0, 0x27B0),
  (0x27BF, 0x27BF), (0x2934, 0x2935), (0x2B05, 0x2B07), (0x2B1B, 0x2B1C),
  (0x2B50, 0x2B50), (0x2B55, 0x2B55), (0x3030, 0x3030), (0x303D, 0x303D),
  (0x3297, 0x3297), (0x3299, 0x3299), (0x1F000, 0x1F0FF), (0x1F10D, 0x1F10F),
  (0x1F12F, 0x1F12F),
  (0x1F16C, 0x1F171),
  (0x1F17E, 0x1F17F),
  (0x1F18E, 0x1F18E),
  (0x1F191, 0x1F19A),
  (0x1F1AD, 0x1F1E5),
  (0x1F201, 0x1F20F),
  (0x1F21A, 0x1F21A),
  (0x1F22F, 0x1F22F),
  (0x1F232, 0x1F23A),
  (0x1F23C, 0x1F23F),
  (0x1F249, 0x1F3FA),
  (0x1F400, 0x1F53D),
  (0x1F546, 0x1F64F),
  (0x1F680, 0x1F6FF),
  (0x1F774, 0x1F77F),
  (0x1F7D5, 0x1F7FF),
  (0x1F80C, 0x1F80F),
  (0x1F848, 0x1F84F),
  (0x1F85A, 0x1F85F),
  (0x1F888, 0x1F88F),
  (0x1F8AE, 0x1F8FF),
  (0x1F90C, 0x1F93A),
  (0x1F93C, 0x1F945),
  (0x1F947, 0x1FAFF), (0x1FC00, 0x1FFFD),
  // Regional indicators (flags) and the emoji variation selector.
  (0x1F1E6, 0x1F1FF), (0xFE0F, 0xFE0F),
];

/// The en dash and the em dash, which no interface text may contain.
const Set<int> dashes = {0x2013, 0x2014};

/// Whether [rune] lies in one of the [emojiRanges].
bool isEmoji(int rune) => emojiRanges.any((r) => rune >= r.$1 && rune <= r.$2);

String _hex(int rune) =>
    'U+${rune.toRadixString(16).toUpperCase().padLeft(4, '0')}';

/// The message keys of an ARB file: every key without `@` (metadata) at the start.
List<String> messageKeys(Map<String, Object?> arb) =>
    arb.keys.where((k) => !k.startsWith('@')).toList();

/// The findings of the ARB lint: dashes and emoji in message values, dashes in descriptions.
///
/// Each finding names the key, the field and the code point, for example
/// `title: value has U+2014`.
List<String> lintArb(Map<String, Object?> arb) {
  final findings = <String>[];
  for (final key in messageKeys(arb)) {
    final value = arb[key];
    if (value is! String) {
      continue;
    }
    for (final rune in value.runes) {
      if (dashes.contains(rune) || isEmoji(rune)) {
        findings.add('$key: value has ${_hex(rune)}');
      }
    }
  }
  for (final entry in arb.entries) {
    final meta = entry.value;
    if (!entry.key.startsWith('@') ||
        entry.key.startsWith('@@') ||
        meta is! Map) {
      continue;
    }
    final description = meta['description'];
    if (description is String) {
      for (final rune in description.runes.where(dashes.contains)) {
        findings.add('${entry.key}: description has ${_hex(rune)}');
      }
    }
  }
  return findings;
}

/// The arguments of an ICU message: name mapped to its kind (`plain`, `plural`, `select`, ...).
///
/// Walks the message like `flutter gen-l10n` does without escaping: `{name}` is a plain
/// argument, `{name, kind, case{...} ...}` a complex one whose case bodies are messages
/// again. Throws a [FormatException] on unbalanced braces.
Map<String, String> icuArguments(String message) {
  final out = <String, String>{};
  var i = 0;

  void skipSpace() {
    while (i < message.length && message[i].trim().isEmpty) {
      i++;
    }
  }

  String readUntil(Set<String> stops) {
    final start = i;
    while (i < message.length && !stops.contains(message[i])) {
      i++;
    }
    return message.substring(start, i).trim();
  }

  void parseMessage({required bool nested}) {
    while (i < message.length) {
      final c = message[i];
      if (c == '}') {
        if (!nested) {
          throw FormatException('unbalanced "}" at $i', message);
        }
        return;
      }
      if (c != '{') {
        i++;
        continue;
      }
      i++; // the argument's "{"
      final name = readUntil({',', '}', '{'});
      if (i >= message.length || message[i] == '{') {
        throw FormatException('argument without name end at $i', message);
      }
      if (message[i] == '}') {
        out[name] = out[name] ?? 'plain';
        i++;
        continue;
      }
      i++; // ","
      final kind = readUntil({',', '}'});
      out[name] = kind;
      if (i < message.length && message[i] == ',') {
        i++;
        while (true) {
          skipSpace();
          if (i >= message.length) {
            throw FormatException('unterminated argument "$name"', message);
          }
          if (message[i] == '}') {
            i++;
            break;
          }
          readUntil({'{', '}'});
          if (i >= message.length || message[i] != '{') {
            throw FormatException('case without body in "$name"', message);
          }
          i++;
          parseMessage(nested: true);
          if (i >= message.length) {
            throw FormatException('unterminated case in "$name"', message);
          }
          i++; // the case's "}"
        }
      } else if (i < message.length) {
        i++; // "}" after the kind
      } else {
        throw FormatException('unterminated argument "$name"', message);
      }
    }
    if (nested) {
      throw FormatException('unbalanced "{"', message);
    }
  }

  parseMessage(nested: false);
  return out;
}

Map<String, String> _declaredPlaceholders(
  Map<String, Object?> arb,
  String key,
) {
  final meta = arb['@$key'];
  if (meta is! Map || meta['placeholders'] is! Map) {
    return const {};
  }
  final placeholders = (meta['placeholders'] as Map).cast<String, Object?>();
  return {
    for (final e in placeholders.entries)
      e.key:
          (e.value is Map ? (e.value as Map)['type'] : null)?.toString() ??
          'Object',
  };
}

/// The findings of the completeness check between the template [en] and [de].
///
/// Fails a key that is missing in one file, an empty message, a key whose ICU arguments
/// (names and kinds) differ, a placeholder declared with another type in the two files, and a
/// template key without an `@key` entry holding a non-empty `description`.
List<String> completeness(Map<String, Object?> en, Map<String, Object?> de) {
  final findings = <String>[];
  final enKeys = messageKeys(en).toSet();
  final deKeys = messageKeys(de).toSet();
  for (final key in enKeys.difference(deKeys)) {
    findings.add('$key: missing in de');
  }
  for (final key in deKeys.difference(enKeys)) {
    findings.add('$key: missing in en');
  }
  for (final key in enKeys) {
    final meta = en['@$key'];
    final description = meta is Map ? meta['description'] : null;
    if (description is! String || description.trim().isEmpty) {
      findings.add('$key: no description in @$key');
    }
  }
  for (final (name, arb) in [('en', en), ('de', de)]) {
    for (final key in messageKeys(arb)) {
      final value = arb[key];
      if (value is! String || value.trim().isEmpty) {
        findings.add('$key: empty message in $name');
      }
    }
  }
  for (final key in enKeys.intersection(deKeys)) {
    final enValue = en[key];
    final deValue = de[key];
    if (enValue is! String || deValue is! String) {
      continue;
    }
    final Map<String, String> enArgs;
    final Map<String, String> deArgs;
    try {
      enArgs = icuArguments(enValue);
      deArgs = icuArguments(deValue);
    } on FormatException catch (e) {
      findings.add('$key: ${e.message}');
      continue;
    }
    if (!_sameMap(enArgs, deArgs)) {
      findings.add('$key: arguments differ, en $enArgs, de $deArgs');
    }
    final enTypes = _declaredPlaceholders(en, key);
    final deTypes = _declaredPlaceholders(de, key);
    for (final name in deTypes.keys) {
      if (enTypes[name] != deTypes[name]) {
        findings.add(
          '$key: placeholder {$name} is ${enTypes[name]} in en, ${deTypes[name]} in de',
        );
      }
    }
  }
  findings.sort();
  return findings;
}

bool _sameMap(Map<String, String> a, Map<String, String> b) =>
    a.length == b.length && a.entries.every((e) => b[e.key] == e.value);

/// Whether [english] contains [term] as a whole word, case-insensitive, also with a plural `s`.
bool hasEnglishTerm(String english, String term) {
  final words = term.split(RegExp(r'\s+')).map(RegExp.escape).join(r'\s+');
  return RegExp(
    '(?<![A-Za-z0-9])${words}s?(?![A-Za-z0-9])',
    caseSensitive: false,
  ).hasMatch(english);
}

/// Whether [german] contains [term] or its stem: the term without a final `en`, `e`, `n` or
/// `s`, case-insensitive.
bool hasGermanTerm(String german, String term) {
  final text = german.toLowerCase();
  final lower = term.toLowerCase();
  final stem = lower.replaceFirst(RegExp(r'(en|e|n|s)$'), '');
  return text.contains(lower) || (stem.isNotEmpty && text.contains(stem));
}

/// The findings of the term check, in the direction English to German.
///
/// For every key whose English message contains an English term of [table], the German
/// message of the same key must contain the German term of that row. Keys in [exceptions]
/// are skipped (each with its reason in the test).
List<String> termFindings(
  Map<String, Object?> en,
  Map<String, Object?> de,
  List<TermPair> table, {
  Map<String, String> exceptions = const {},
}) {
  final findings = <String>[];
  for (final key in messageKeys(en)) {
    final english = en[key];
    final german = de[key];
    if (exceptions.containsKey(key) ||
        english is! String ||
        german is! String) {
      continue;
    }
    for (final pair in table) {
      if (hasEnglishTerm(english, pair.english) &&
          !hasGermanTerm(german, pair.german)) {
        findings.add(
          '$key: "${pair.english}" in en needs "${pair.german}" in de',
        );
      }
    }
  }
  return findings;
}
