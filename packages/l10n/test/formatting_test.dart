// SPDX-License-Identifier: Apache-2.0

// Formatting per locale (brief M4-06 Design note 6, docs/spec/01-produkt.md section 9).
//
// `L10nFormat` puts a non-breaking space (U+00A0) between number and currency or unit, so a
// line break never separates them; the assertions normalize it to a plain space with `n`.

import 'dart:ui';

import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

String n(String s) => s.replaceAll('\u00A0', ' ').replaceAll('\u202F', ' ');

void main() {
  const de = L10nFormat(Locale('de'));
  const en = L10nFormat(Locale('en'));

  test('money follows the locale as 01 section 9 shows', () {
    expect(n(de.money(1.84, 'USD')), '1,84 USD');
    expect(n(en.money(1.84, 'USD')), 'USD 1.84');
    expect(n(de.money(1234.5, 'EUR')), '1.234,50 EUR');
    expect(n(en.money(1234.5, 'EUR')), 'EUR 1,234.50');
    expect(n(en.money(1500, 'JPY')), 'JPY 1,500');
  });

  test('money keeps the space unbreakable', () {
    expect(de.money(1.84, 'USD'), '1,84\u00A0USD');
    expect(en.money(1.84, 'USD'), 'USD\u00A01.84');
  });

  test('counts and percentages', () {
    expect(de.count(48100), '48.100');
    expect(en.count(48100), '48,100');
    expect(n(de.percent(62)), '62 %');
    expect(n(en.percent(62)), '62%');
  });

  test('elapsed time of a running run reads as a clock', () {
    expect(de.elapsed(const Duration(minutes: 4, seconds: 12)), '4:12');
    expect(en.elapsed(const Duration(seconds: 7)), '0:07');
    expect(
      en.elapsed(const Duration(hours: 1, minutes: 2, seconds: 3)),
      '1:02:03',
    );
    expect(en.elapsed(const Duration(seconds: -5)), '0:00');
  });

  test('short durations', () {
    expect(
      n(de.durationShort(const Duration(minutes: 28, seconds: 40))),
      '28 min',
    );
    expect(
      n(en.durationShort(const Duration(hours: 2, minutes: 5))),
      '2 h 5 min',
    );
    expect(n(en.durationShort(const Duration(hours: 3))), '3 h');
    expect(n(de.durationShort(const Duration(seconds: 42))), '42 s');
    expect(n(de.durationShort(Duration.zero)), '0 s');
  });

  group('time across a daylight saving change', () {
    // Europe/Berlin leaves summer time on 2026-10-25 at 01:00 UTC; New York on 2026-11-01
    // at 06:00 UTC.
    final beforeBerlin = DateTime.utc(2026, 10, 24, 12);
    final afterBerlin = DateTime.utc(2026, 10, 26, 12);
    final beforeNewYork = DateTime.utc(2026, 10, 31, 16);
    final afterNewYork = DateTime.utc(2026, 11, 2, 16);

    test('Europe/Berlin', () {
      expect(de.time(beforeBerlin, timeZone: 'Europe/Berlin'), '14:00');
      expect(de.time(afterBerlin, timeZone: 'Europe/Berlin'), '13:00');
    });

    test('America/New_York', () {
      expect(
        n(en.time(beforeNewYork, timeZone: 'America/New_York')),
        '12:00 PM',
      );
      expect(
        n(en.time(afterNewYork, timeZone: 'America/New_York')),
        '11:00 AM',
      );
      expect(de.time(afterNewYork, timeZone: 'America/New_York'), '11:00');
    });

    test('an unknown time zone fails', () {
      expect(
        () => de.time(beforeBerlin, timeZone: 'Mars/Olympus'),
        throwsArgumentError,
      );
    });
  });

  test('the start of a calendar day in a time zone, as UTC', () {
    const f = L10nFormat(Locale('en'));
    expect(
      f.dayStart(2027, 1, 1, timeZone: 'America/New_York'),
      DateTime.utc(2027, 1, 1, 5),
    );
    expect(
      f.dayStart(2026, 7, 1, timeZone: 'Europe/Berlin'),
      DateTime.utc(2026, 6, 30, 22),
    );
    expect(
      () => f.dayStart(2026, 1, 1, timeZone: 'Mars/Base'),
      throwsArgumentError,
    );
  });

  group('relative day in the time zone of the person', () {
    final now = DateTime.utc(2026, 10, 15, 10); // Thursday, 12:00 in Berlin

    test('today and yesterday', () {
      expect(
        en.relativeDay(
          DateTime.utc(2026, 10, 15, 6),
          now: now,
          timeZone: 'Europe/Berlin',
        ),
        'Today',
      );
      expect(
        de.relativeDay(
          DateTime.utc(2026, 10, 14, 6),
          now: now,
          timeZone: 'Europe/Berlin',
        ),
        'Gestern',
      );
    });

    test('the day boundary is the one of the time zone', () {
      // 22:30 UTC on the 14th is already the 15th in Berlin, still the 14th in New York.
      final late = DateTime.utc(2026, 10, 14, 22, 30);
      expect(
        en.relativeDay(late, now: now, timeZone: 'Europe/Berlin'),
        'Today',
      );
      expect(
        en.relativeDay(late, now: now, timeZone: 'America/New_York'),
        'Yesterday',
      );
    });

    test('the weekday within the last week, a date before', () {
      expect(
        en.relativeDay(
          DateTime.utc(2026, 10, 12, 10),
          now: now,
          timeZone: 'Europe/Berlin',
        ),
        'Monday',
      );
      expect(
        de.relativeDay(
          DateTime.utc(2026, 10, 12, 10),
          now: now,
          timeZone: 'Europe/Berlin',
        ),
        'Montag',
      );
      expect(
        n(
          en.relativeDay(
            DateTime.utc(2026, 10, 1, 10),
            now: now,
            timeZone: 'Europe/Berlin',
          ),
        ),
        'Oct 1, 2026',
      );
      expect(
        n(
          de.relativeDay(
            DateTime.utc(2026, 10, 1, 10),
            now: now,
            timeZone: 'Europe/Berlin',
          ),
        ),
        '1. Okt. 2026',
      );
    });

    test('a day after now shows the date', () {
      expect(
        n(
          de.relativeDay(
            DateTime.utc(2026, 10, 16, 10),
            now: now,
            timeZone: 'Europe/Berlin',
          ),
        ),
        '16. Okt. 2026',
      );
    });
  });

  test('an unsupported locale formats as German', () {
    expect(n(const L10nFormat(Locale('fr')).money(1.84, 'USD')), '1,84 USD');
  });
}
