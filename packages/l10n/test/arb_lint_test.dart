// SPDX-License-Identifier: Apache-2.0

// ARB lint (brief M4-06 Design note 3): no en or em dash and no emoji in interface texts, no
// dash in the descriptions. The fixtures write the forbidden characters as JSON escapes, so
// the files themselves pass the repository's text rules while the parsed values hold them.

import 'package:flutter_test/flutter_test.dart';

import 'support/arb_rules.dart';
import 'support/repo.dart';

void main() {
  for (final locale in ['en', 'de']) {
    test('app_$locale.arb has no dash and no emoji', () {
      expect(lintArb(realArb(locale)), isEmpty);
    });
  }

  test('an em dash in a value fails with key and code point', () {
    expect(lintArb(fixtureArb('bad_dash.arb')), [
      'dashTitle: value has U+2014',
    ]);
  });

  test('an emoji in a value fails', () {
    expect(lintArb(fixtureArb('bad_emoji.arb')), [
      'rocketTitle: value has U+1F680',
    ]);
  });

  test('an en dash in a description fails', () {
    expect(lintArb(fixtureArb('bad_description_dash.arb')), [
      '@rangeTitle: description has U+2013',
    ]);
  });

  test('flags, the variation selector and pictographs count as emoji', () {
    for (final rune in [0x1F1E9, 0xFE0F, 0x2705, 0x1F600, 0x00A9]) {
      expect(isEmoji(rune), isTrue, reason: rune.toRadixString(16));
    }
    for (final rune in [0x41, 0x00E4, 0x2192, 0x2026, 0x00B7]) {
      expect(isEmoji(rune), isFalse, reason: rune.toRadixString(16));
    }
  });
}
