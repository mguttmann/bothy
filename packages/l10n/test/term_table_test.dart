// SPDX-License-Identifier: Apache-2.0

// The term table (brief M4-06 Design note 4): lib/src/term_table.dart equals the table of
// docs/spec/01-produkt.md section 9, and every English message that uses an English term has
// the German term of the same row in its German message (direction English to German, the
// template is English).

import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

import '../tool/term_table.dart' as generator;
import 'support/arb_rules.dart';
import 'support/repo.dart';

/// Keys allowed to use an English term without the German one, each with its reason.
const Map<String, String> exceptions = {
  // The German plural of Lauf is Läufe; the stem check does not see through the umlaut.
  'vizMetricRunsStarted': 'plural Läufe of Lauf',
  'vizMetricRunsFinished': 'plural Läufe of Lauf',
  'vizMoreRunsThanShown': 'plural Läufe of Lauf',
  'vizChartRuns': 'plural Läufe of Lauf',
};

void main() {
  test('the checked-in table equals the generation from 01 section 9', () {
    final root = repoRoot();
    final expected = generator.render(
      generator.parse(generator.specFile(root).readAsStringSync()),
    );
    final actual = File('${packageDir().path}/lib/src/term_table.dart')
        .readAsStringSync();
    expect(
      actual,
      expected,
      reason: 'stale; run: cd packages/l10n && dart run tool/term_table.dart',
    );
  });

  test('the table splits the alternatives of a row', () {
    expect(termTable, contains(const TermPair('Bereich', 'Space', 'space')));
    expect(termTable, contains(const TermPair('Beitrag', 'Post', 'thread')));
    // language-check: off
    expect(
      termTable,
      contains(const TermPair('Agenten auslösen', 'Trigger agents', 'trigger')),
    );
    // language-check: on
    expect(
      termTable,
      contains(const TermPair('Kopplungs-Code', 'pairing code', 'host.pair')),
    );
    expect(
      termTable,
      contains(const TermPair('Sondertasten-Leiste', 'Extra keys bar', null)),
    );
  });

  test('the real files use the terms of the table', () {
    expect(
      termFindings(
        realArb('en'),
        realArb('de'),
        termTable,
        exceptions: exceptions,
      ),
      isEmpty,
    );
  });

  test('an English term without the German term fails', () {
    expect(
      termFindings(
        fixtureArb('term_en.arb'),
        fixtureArb('term_de.arb'),
        termTable,
      ),
      ['spaceTitle: "Space" in en needs "Bereich" in de'],
    );
  });

  test('term matching: whole words, plural s, German stems', () {
    expect(hasEnglishTerm('Two Spaces left', 'Space'), isTrue);
    expect(hasEnglishTerm('Running', 'Run'), isFalse);
    expect(hasEnglishTerm('The live  box', 'Live box'), isTrue);
    expect(hasGermanTerm('Zwei Bereiche', 'Bereich'), isTrue);
    expect(hasGermanTerm('Der Kopplung fehlt', 'Kopplung'), isTrue);
    // language-check: off
    expect(hasGermanTerm('Die Kanäle', 'Kanal'), isFalse);
    expect(hasGermanTerm('Agenten auslösen', 'Agenten auslösen'), isTrue);
    // language-check: on
  });

  test('the parser reads rows between the markers of section 9 only', () {
    const markdown = '''
## 8. Other

<!-- language-check: off -->
| German | English | Code |
|---|---|---|
| Nein | No | `no` |
<!-- language-check: on -->

## 9. Languages

| German | English | Code |
|---|---|---|
| Vorher | Before | |

<!-- language-check: off -->

| German | English | Code |
|---|---|---|
| Forum, Beitrag | Forum, Post | `forum`, `thread` |
| Kopplung, Kopplungs-Code | Pairing, pairing code | `host.pair` |
| Live-Topologie | Live topology | |

<!-- language-check: on -->

## 10. Later
''';
    expect(generator.parse(markdown), const [
      TermPair('Forum', 'Forum', 'forum'),
      TermPair('Beitrag', 'Post', 'thread'),
      TermPair('Kopplung', 'Pairing', 'host.pair'),
      TermPair('Kopplungs-Code', 'pairing code', 'host.pair'),
      TermPair('Live-Topologie', 'Live topology', null),
    ]);
  });

  test('the parser fails without the table', () {
    expect(
      () => generator.parse('## 9. Languages\n\nNo table.\n'),
      throwsFormatException,
    );
    expect(
      () => generator.parse(
        '## 9. Languages\n<!-- language-check: off -->\n| A, B | C | |\n',
      ),
      throwsFormatException,
    );
  });
}
