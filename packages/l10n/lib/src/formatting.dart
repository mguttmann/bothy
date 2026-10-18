// SPDX-License-Identifier: Apache-2.0

/// Numbers, money, durations and times per app locale (docs/spec/01-product.md section 9).
library;

import 'dart:ui';

import 'package:intl/date_symbol_data_local.dart';
import 'package:intl/intl.dart';
import 'package:timezone/data/latest_all.dart' as tz_data;
import 'package:timezone/timezone.dart' as tz;

import 'generated/app_localizations.dart';

/// A non-breaking space: it keeps a number with its currency or unit on one line.
const String _nbsp = '\u00A0';

bool _dataLoaded = false;

/// Loads the date symbols of `intl` and the IANA time zone database once.
///
/// Both loaders of the local data fill their tables synchronously; the future of
/// [initializeDateFormatting] only reports that, so it is not awaited.
void _ensureData() {
  if (_dataLoaded) {
    return;
  }
  // ignore: discarded_futures, unawaited_futures
  initializeDateFormatting();
  tz_data.initializeTimeZones();
  _dataLoaded = true;
}

/// Formats values for the app locale [locale]; agent content is never passed through here.
///
/// The locale is the result of `resolveAppLocale`; a language other than `de` or `en`
/// formats as German, the default language. Nothing reads a clock: the caller passes `now`
/// and the IANA time zone of the person (`users.time_zone`).
///
/// Example: `L10nFormat(Locale('de')).money(1.84, 'USD')` is `1,84 USD` and
/// `L10nFormat(Locale('en')).money(1.84, 'USD')` is `USD 1.84`, each with a non-breaking
/// space.
class L10nFormat {
  /// Creates a formatter for [locale].
  const L10nFormat(this.locale);

  /// The app locale.
  final Locale locale;

  String get _tag => locale.languageCode == 'en' ? 'en' : 'de';

  bool get _english => _tag == 'en';

  /// [amount] in [currency] (ISO 4217 code), with the decimals of the currency: `1,84 USD`
  /// in German, `USD 1.84` in English.
  String money(num amount, String currency) {
    final number = NumberFormat.currency(
      locale: _tag,
      name: currency,
      symbol: '',
    ).format(amount).trim();
    return _english ? '$currency$_nbsp$number' : '$number$_nbsp$currency';
  }

  /// [value] with the grouping of the locale: `48.100` in German, `48,100` in English.
  String count(int value) => NumberFormat.decimalPattern(_tag).format(value);

  /// [percent] given in percent (62 for 62 %): `62 %` in German, `62%` in English.
  String percent(num percent) =>
      NumberFormat.percentPattern(_tag).format(percent / 100);

  /// The running time of a run as a clock: `4:12`, `1:02:03`; a negative duration is `0:00`.
  String elapsed(Duration duration) {
    final seconds = duration.isNegative ? 0 : duration.inSeconds;
    final h = seconds ~/ 3600;
    final m = (seconds % 3600) ~/ 60;
    final s = (seconds % 60).toString().padLeft(2, '0');
    return h > 0 ? '$h:${m.toString().padLeft(2, '0')}:$s' : '$m:$s';
  }

  /// A finished duration in short form: `42 s`, `28 min`, `2 h 5 min`, `3 h`; minutes and
  /// seconds are cut, not rounded. The units are the same in German and English.
  String durationShort(Duration duration) {
    final seconds = duration.isNegative ? 0 : duration.inSeconds;
    if (seconds < 60) {
      return '$seconds${_nbsp}s';
    }
    final h = seconds ~/ 3600;
    final m = (seconds % 3600) ~/ 60;
    if (h == 0) {
      return '$m${_nbsp}min';
    }
    return m == 0 ? '$h${_nbsp}h' : '$h${_nbsp}h $m${_nbsp}min';
  }

  tz.TZDateTime _inZone(DateTime instant, String timeZone) {
    _ensureData();
    final tz.Location location;
    try {
      location = tz.getLocation(timeZone);
    } on tz.LocationNotFoundException {
      throw ArgumentError.value(timeZone, 'timeZone', 'not an IANA time zone');
    }
    return tz.TZDateTime.from(instant, location);
  }

  /// The instant (UTC) at which the calendar day [year]-[month]-[day] starts in [timeZone]
  /// (IANA name), for a date the person typed in their own time zone.
  ///
  /// Example: `dayStart(2027, 1, 1, timeZone: 'America/New_York')` is 2027-01-01 05:00 UTC.
  /// Throws an [ArgumentError] when [timeZone] is not in the time zone database.
  DateTime dayStart(int year, int month, int day, {required String timeZone}) {
    _ensureData();
    final tz.Location location;
    try {
      location = tz.getLocation(timeZone);
    } on tz.LocationNotFoundException {
      throw ArgumentError.value(timeZone, 'timeZone', 'not an IANA time zone');
    }
    final start = tz.TZDateTime(location, year, month, day);
    return DateTime.fromMicrosecondsSinceEpoch(
      start.microsecondsSinceEpoch,
      isUtc: true,
    );
  }

  /// The clock time of [instant] in [timeZone] (IANA name): `14:05` in German, `2:05 PM` in
  /// English. The space before AM or PM is the no-break space of the product font: `intl`
  /// writes U+202F, which the font lacks, so each platform drew it with another fallback font.
  ///
  /// Throws an [ArgumentError] when [timeZone] is not in the time zone database.
  String time(DateTime instant, {required String timeZone}) {
    final local = _inZone(instant, timeZone);
    return DateFormat.jm(_tag).format(local).replaceAll('\u202F', _nbsp);
  }

  /// The day of [instant] relative to [now], both seen in [timeZone]: `Today`, `Yesterday`,
  /// the weekday for the five days before, the date otherwise (also for a day after [now]).
  ///
  /// Throws an [ArgumentError] when [timeZone] is not in the time zone database.
  String relativeDay(
    DateTime instant, {
    required DateTime now,
    required String timeZone,
  }) {
    final local = _inZone(instant, timeZone);
    final today = _inZone(now, timeZone);
    // Calendar days in UTC, so that a change of the UTC offset does not shift the difference.
    final days = DateTime.utc(
      today.year,
      today.month,
      today.day,
    ).difference(DateTime.utc(local.year, local.month, local.day)).inDays;
    final l = lookupAppLocalizations(Locale(_tag));
    if (days == 0) {
      return l.relativeDayToday;
    }
    if (days == 1) {
      return l.relativeDayYesterday;
    }
    if (days > 1 && days < 7) {
      return DateFormat.EEEE(_tag).format(local);
    }
    return DateFormat.yMMMd(_tag).format(local);
  }
}
