// SPDX-License-Identifier: Apache-2.0

import 'package:admin_ui/admin_ui.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

// The labels of day buckets name the calendar day of the server's stats.time_zone, where the
// server starts those buckets, whatever the zone of the signed-in person; hour buckets are
// instants and keep the person's zone.
void main() {
  const format = L10nFormat(Locale('en'));
  // Far from every bucket below, so the labels are dates, not "Today" or a weekday.
  final now = DateTime.utc(2027, 6, 1, 12);

  StatsTexts texts({required String person, required String server}) =>
      StatsTexts(
        l10n: lookupAppLocalizations(const Locale('en')),
        format: format,
        timeZone: person,
        bucketZone: server,
        now: now,
        agentNames: const {},
        spaceName: (id) => id,
      );

  test('a day bucket names the server day for a person west of it', () {
    final t = texts(person: 'America/New_York', server: 'UTC');
    // The bucket of 16 Oct starts at UTC midnight, the evening of 15 Oct in New York.
    final start = DateTime.utc(2026, 10, 16);
    expect(
      t.bucket(start, StatsBucketSize.day),
      format.relativeDay(start, now: now, timeZone: 'UTC'),
    );
    expect(
      t.bucket(start, StatsBucketSize.day),
      isNot(format.relativeDay(start, now: now, timeZone: 'America/New_York')),
    );
  });

  test('an hour bucket keeps the zone of the person', () {
    final t = texts(person: 'America/New_York', server: 'UTC');
    final start = DateTime.utc(2026, 10, 16, 3);
    expect(
      t.bucket(start, StatsBucketSize.hour),
      '${format.relativeDay(start, now: now, timeZone: 'America/New_York')} '
      '${format.time(start, timeZone: 'America/New_York')}',
    );
  });

  test(
    'the last day of a range ending after a spring-forward day is that day',
    () {
      // 28 Mar 2027 has 23 hours in Berlin; the range of 22 to 28 Mar ends at the start of 29 Mar.
      final days = StatsDays('Europe/Berlin');
      final t = texts(person: 'Europe/Berlin', server: 'Europe/Berlin');
      final to = days.start(DateTime.utc(2027, 3, 29));
      expect(
        t.lastDay(to, days),
        t.bucket(days.start(DateTime.utc(2027, 3, 28)), StatsBucketSize.day),
      );
    },
  );
}
