// SPDX-License-Identifier: Apache-2.0

// Completeness of the translations (brief M4-06 Design note 2): every key in both files, the
// same ICU arguments, the same placeholder types, a description for every template key and
// no empty message. `make codegen-check` adds the generator's own untranslated list and the
// comparison with the checked-in generated code.

import 'package:flutter_test/flutter_test.dart';

import 'support/arb_rules.dart';
import 'support/repo.dart';

void main() {
  test('the real files are complete', () {
    expect(completeness(realArb('en'), realArb('de')), isEmpty);
  });

  test('the real files declare their locale', () {
    expect(realArb('en')['@@locale'], 'en');
    expect(realArb('de')['@@locale'], 'de');
  });

  test('a key missing in German fails', () {
    expect(
      completeness(fixtureArb('missing_en.arb'), fixtureArb('missing_de.arb')),
      ['onlyEnglish: missing in de'],
    );
  });

  test('a placeholder typed int in English and String in German fails', () {
    expect(completeness(fixtureArb('type_en.arb'), fixtureArb('type_de.arb')), [
      'itemCount: placeholder {count} is int in en, String in de',
    ]);
  });

  test('a template key without description fails', () {
    expect(
      completeness(fixtureArb('nodesc_en.arb'), fixtureArb('nodesc_de.arb')),
      ['plainTitle: no description in @plainTitle'],
    );
  });

  test('an empty message and differing arguments fail', () {
    final en = {
      'a': 'Hello {name}',
      '@a': {'description': 'x'},
      'b': 'B',
      '@b': {'description': 'y'},
    };
    final de = {'a': 'Hallo {who}', 'b': ' '};
    expect(completeness(en, de), [
      'a: arguments differ, en {name: plain}, de {who: plain}',
      'b: empty message in de',
    ]);
  });

  test('ICU arguments include plural and select with nested arguments', () {
    expect(
      icuArguments(
        '{count, plural, =1{One step by {name}} other{{count} steps}} {kind, select, a{Done} other{Open}}',
      ),
      {'count': 'plural', 'name': 'plain', 'kind': 'select'},
    );
    expect(() => icuArguments('Hello {name'), throwsFormatException);
    expect(() => icuArguments('Hello name}'), throwsFormatException);
  });
}
