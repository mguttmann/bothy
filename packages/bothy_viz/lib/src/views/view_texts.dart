// SPDX-License-Identifier: Apache-2.0

/// Texts the views share: rates, clock times and cost cells.
library;

import 'package:api_client/api_client.dart' as api;

import '../marks/viz_texts.dart';
import '../model/costs.dart';
import '../model/series_model.dart';

/// The label of [rate]: its metric, with the billing mode of a cost rate or the dimension
/// code (for example the reason of `resyncs`).
String vizRateLabel(VizTexts texts, RateModel rate) {
  final name = texts.metric(rate.metric);
  final dimension = rate.dimension;
  if (dimension == null || dimension.isEmpty) return name;
  final mode = _mode(dimension);
  return mode == null ? '$name $dimension' : '$name, ${texts.badge(mode)}';
}

/// The value of [rate] over the last full minute: the amount of a cost rate (with
/// "information only" in mode `personal`), else the count.
///
/// A cost rate without its sum or currency is not shown at all ([vizRateShown]).
String vizRateValue(VizTexts texts, RateModel rate) {
  final sum = rate.lastMinuteSum;
  final currency = rate.currency;
  if (rate.metric == api.StatsMetric.cost && sum != null && currency != null) {
    final mode = _mode(rate.dimension ?? '');
    return texts.cost(
      VizCost.of(
        micros: sum,
        currency: currency,
        mode: mode ?? api.BillingMode.personal,
        // The frame carries no estimate flag for rates; only `personal` gets its label.
        isEstimate: false,
      ),
    );
  }
  return texts.format.count(rate.lastMinute);
}

/// Whether [rate] is shown: a cost rate only with its sum and currency and only where costs
/// are visible ([costsVisible] is `false` in TV mode without `viz.tv_show_costs`).
bool vizRateShown(RateModel rate, {required bool costsVisible}) {
  if (rate.metric != api.StatsMetric.cost) return true;
  return costsVisible && rate.lastMinuteSum != null && rate.currency != null;
}

api.BillingMode? _mode(String code) => switch (code) {
  'organization' => api.BillingMode.organization,
  'personal' => api.BillingMode.personal,
  _ => null,
};

/// The clock time of [at] in [timeZone], or the UTC time when the zone is unknown.
String vizClockTime(VizTexts texts, DateTime at, String timeZone) {
  try {
    return texts.format.time(at, timeZone: timeZone);
  } on ArgumentError {
    return texts.format.time(at, timeZone: 'UTC');
  }
}

/// The day of [at] relative to [now] and its clock time, for example "Yesterday, 11:58 PM".
String vizDayTime(VizTexts texts, DateTime at, DateTime now, String timeZone) {
  final zone = _zone(texts, timeZone);
  final day = texts.format.relativeDay(at, now: now, timeZone: zone);
  return '$day, ${texts.format.time(at, timeZone: zone)}';
}

String _zone(VizTexts texts, String timeZone) {
  try {
    texts.format.time(DateTime.utc(2026), timeZone: timeZone);
    return timeZone;
  } on ArgumentError {
    return 'UTC';
  }
}
