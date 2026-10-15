// SPDX-License-Identifier: Apache-2.0

/// The language the app shows (docs/spec/01-produkt.md section 9).
library;

import 'dart:ui';

const Set<String> _languages = {'de', 'en'};

/// The locale of the app: the language the person chose, else the system language, else German.
///
/// [userLocale] is `users.locale` (`de` or `en`, null or empty while the person has not chosen
/// one; another value counts as not chosen). [systemLocales] are the preferred locales of the
/// platform in order; the first whose language is `de` or `en` wins. Without a match the app
/// shows German, the default language. The result carries the language only, because the ARB
/// files have no regional variants.
///
/// Example: `resolveAppLocale(userLocale: null, systemLocales: [Locale('fr'), Locale('en', 'GB')])`
/// is `Locale('en')`.
Locale resolveAppLocale({
  String? userLocale,
  required List<Locale> systemLocales,
}) {
  if (userLocale != null && _languages.contains(userLocale)) {
    return Locale(userLocale);
  }
  for (final locale in systemLocales) {
    if (_languages.contains(locale.languageCode)) {
      return Locale(locale.languageCode);
    }
  }
  return const Locale('de');
}
