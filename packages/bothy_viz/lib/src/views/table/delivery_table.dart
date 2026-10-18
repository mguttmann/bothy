// SPDX-License-Identifier: Apache-2.0

/// The table view of delivery and sync live (docs/spec/25-visualization.md 6.3, 7.4).
library;

import 'package:api_client/api_client.dart' as api;

import '../../marks/area_mark.dart';
import '../../marks/viz_texts.dart';
import '../../model/delivery_model.dart';
import '../../model/series_model.dart';
import '../view_texts.dart';
import 'viz_table.dart';

/// The SLIs of delivery, reconnecting and push acceptance (25 6.3), in the order of the frame.
const Set<api.StatsMetric> deliverySlis = {
  api.StatsMetric.sliPeriodDeliveryLatency,
  api.StatsMetric.sliPeriodReconnect,
  api.StatsMetric.sliPeriodPushAcceptance,
};

/// The metrics of the history from `getStatsTimeline`.
const List<api.StatsMetric> deliveryHistoryMetrics = [
  api.StatsMetric.sliPeriodDeliveryLatency,
  api.StatsMetric.sliPeriodReconnect,
  api.StatsMetric.sliPeriodPushAcceptance,
  api.StatsMetric.catchups,
  api.StatsMetric.resyncs,
  api.StatsMetric.slowConsumerCloses,
];

/// One labelled value of the view; the panel and the table show the same list.
typedef DeliveryLine = ({String label, String value, num sortBy});

/// The good share [ratio] (0 to 1) in percent with two decimals in the format of the locale;
/// an SLI share needs more than the whole percent of the tiles.
String deliveryShare(VizTexts texts, double ratio) {
  final digits = (ratio * 100).toStringAsFixed(2);
  final german = texts.format.locale.languageCode != 'en';
  return german ? '${digits.replaceAll('.', ',')} %' : '$digits%';
}

/// The lines of [model]: every delivery SLI with its count and share, catch-ups with their
/// duration, resyncs per reason, closes of slow clients and the connections of one node.
List<DeliveryLine> deliveryLines(VizTexts texts, DeliveryModel model) {
  final l10n = texts.l10n;
  return [
    for (final sli in model.slis)
      if (deliverySlis.contains(sli.metric))
        (
          label: texts.metric(sli.metric),
          value: sli.ratio == null
              ? l10n.vizSliNoEvents
              : l10n.vizSliValue(sli.total, deliveryShare(texts, sli.ratio!)),
          sortBy: sli.ratio ?? -1,
        ),
    for (final rate in model.gatewayRates)
      (
        label: vizRateLabel(texts, rate),
        value:
            rate.metric == api.StatsMetric.catchups &&
                rate.lastMinuteSum != null
            ? l10n.vizCatchupValue(
                texts.format.count(rate.lastMinute),
                texts.format.count(rate.lastMinuteSum!),
              )
            : texts.format.count(rate.lastMinute),
        sortBy: rate.lastMinute,
      ),
    if (model.connections case final c?) ...[
      (
        label: l10n.vizConnectionsUser,
        value: texts.format.count(c.user),
        sortBy: c.user,
      ),
      (
        label: l10n.vizConnectionsAgent,
        value: texts.format.count(c.agent),
        sortBy: c.agent,
      ),
      (
        label: l10n.vizConnectionsApp,
        value: texts.format.count(c.app),
        sortBy: c.app,
      ),
    ],
  ];
}

/// The lines as one table and the history as a second table: per series its label and its
/// last value, the same the legend of the history shows.
List<VizTableData> deliveryTables(
  VizTexts texts,
  DeliveryModel model,
  List<SeriesModel> history,
) {
  final l10n = texts.l10n;
  return [
    VizTableData(
      caption: l10n.vizViewDelivery,
      columns: [l10n.vizColMetric, l10n.vizColValue],
      rows: [
        for (final line in deliveryLines(texts, model))
          [VizCell(line.label), VizCell(line.value, sortBy: line.sortBy)],
      ],
    ),
    if (history.isNotEmpty) seriesTable(texts, l10n.vizChartHistory, history),
  ];
}

/// [series] as a table of label and last value, the legend of the area mark.
VizTableData seriesTable(
  VizTexts texts,
  String caption,
  List<SeriesModel> series,
) => VizTableData(
  caption: caption,
  columns: [texts.l10n.vizColMetric, texts.l10n.vizColValue],
  rows: [
    for (final s in series)
      [
        VizCell(AreaMark.seriesLabel(texts, s)),
        VizCell(
          s.last == null
              ? texts.l10n.vizNotShown
              : texts.format.count(s.last!.round()),
          sortBy: s.last ?? -1,
        ),
      ],
  ],
);
