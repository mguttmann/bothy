// SPDX-License-Identifier: Apache-2.0

// Locale resolution (brief M4-06 Design note 7, docs/spec/01-product.md section 9).

import 'dart:ui';

import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

void main() {
  test('the chosen user locale wins over the system', () {
    expect(
      resolveAppLocale(userLocale: 'en', systemLocales: const [Locale('de')]),
      const Locale('en'),
    );
    expect(
      resolveAppLocale(
        userLocale: 'de',
        systemLocales: const [Locale('en', 'US')],
      ),
      const Locale('de'),
    );
  });

  test('without a user locale the first supported system language wins', () {
    expect(
      resolveAppLocale(
        systemLocales: const [Locale('fr'), Locale('en', 'GB'), Locale('de')],
      ),
      const Locale('en'),
    );
    expect(
      resolveAppLocale(
        userLocale: '',
        systemLocales: const [Locale('de', 'AT')],
      ),
      const Locale('de'),
    );
  });

  test('an unsupported user locale counts as not chosen', () {
    expect(
      resolveAppLocale(userLocale: 'fr', systemLocales: const [Locale('en')]),
      const Locale('en'),
    );
  });

  test('another system language falls back to German', () {
    expect(
      resolveAppLocale(systemLocales: const [Locale('fr')]),
      const Locale('de'),
    );
    expect(resolveAppLocale(systemLocales: const []), const Locale('de'));
  });

  test('German is the first supported locale', () {
    expect(AppLocalizations.supportedLocales.first, const Locale('de'));
    expect(AppLocalizations.supportedLocales, contains(const Locale('en')));
  });
}
