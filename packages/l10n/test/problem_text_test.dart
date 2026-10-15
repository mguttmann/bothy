// SPDX-License-Identifier: Apache-2.0

// Problem codes (brief M4-06 Design note 5): every code of `x-problem-codes` in
// docs/api/openapi.yaml has its own text, so a new code in the specification needs a text in
// the same change; an unknown code shows the generic text with the code.

import 'dart:io';
import 'dart:ui';

import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

import 'support/repo.dart';

/// The codes of `x-problem-codes`, in the order of the file.
List<String> specCodes() {
  final lines = File('${repoRoot().path}/docs/api/openapi.yaml')
      .readAsLinesSync();
  final start = lines.indexWhere((l) => l.trim() == 'x-problem-codes:');
  expect(
    start,
    isNonNegative,
    reason: 'docs/api/openapi.yaml has no x-problem-codes',
  );
  final indent = lines[start].indexOf('x');
  final codes = <String>[];
  for (final line in lines.skip(start + 1)) {
    final text = line.trimLeft();
    if (text.isNotEmpty &&
        line.length - text.length <= indent &&
        !text.startsWith('- ')) {
      break;
    }
    final match = RegExp(r'^- code: (\S+)$').firstMatch(text);
    if (match != null) {
      codes.add(match.group(1)!);
    }
  }
  return codes;
}

void main() {
  final locales = AppLocalizations.supportedLocales;

  test('the specification lists the problem codes', () {
    expect(
      specCodes(),
      containsAll(['not_found', 'forbidden', 'setup_required']),
    );
  });

  for (final locale in locales) {
    final l = lookupAppLocalizations(locale);

    test(
      'every problem code of the specification has its own text ($locale)',
      () {
        final fallbacks = specCodes().where(
          (c) => problemText(l, c) == l.problemFallback(c),
        );
        expect(fallbacks, isEmpty, reason: 'codes without an ARB message');
      },
    );

    test('codes map to distinct texts ($locale)', () {
      final texts = specCodes().map((c) => problemText(l, c)).toSet();
      expect(texts.length, specCodes().length);
    });

    test('a reason selects the specific text ($locale)', () {
      expect(
        problemText(l, 'forbidden', reason: 'capability_missing'),
        l.problemForbiddenCapabilityMissing,
      );
      expect(
        problemText(l, 'forbidden', reason: 'capability_missing'),
        isNot(l.problemForbidden),
      );
      expect(
        problemText(l, 'forbidden', reason: 'something_new'),
        l.problemForbidden,
      );
    });

    test('an unknown code shows the fallback with the code ($locale)', () {
      final text = problemText(l, 'future_code');
      expect(text, l.problemFallback('future_code'));
      expect(text, contains('future_code'));
    });
  }

  test('the specific and the fallback texts read as written', () {
    final en = lookupAppLocalizations(const Locale('en'));
    final de = lookupAppLocalizations(const Locale('de'));
    expect(problemText(en, 'not_found'), 'Not found or no access');
    expect(
      problemText(en, 'future_code'),
      'Something went wrong (future_code)',
    );
    expect(
      problemText(de, 'future_code'),
      'Etwas ist schiefgegangen (future_code)',
    );
    expect(
      problemText(en, 'forbidden', reason: 'capability_missing'),
      'You lack an extra permission for this',
    );
  });
}
