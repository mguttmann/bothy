// SPDX-License-Identifier: Apache-2.0

import 'package:alchemist/alchemist.dart';
import 'package:bothy_viz/bothy_viz.dart';
import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

import '../support/marks.dart';

/// The golden matrix of every mark (brief M6-11, design note 8; docs/spec/15-client-architektur.md
/// 20): light and dark, the three densities, text scale 1 and 2, de and en.
GoldenTestGroup matrix(
  String name,
  Widget Function() builder, {
  double width = DsLayout.overlayWidth,
}) => dsGoldenMatrix(
  name: name,
  builder: () => _Localized(width: width, child: builder()),
  densities: DsDensityMode.values,
  textScales: const [1.0, 2.0],
  locales: AppLocalizations.supportedLocales,
);

/// Adds the `AppLocalizations` delegate for the locale of the matrix cell; the alchemist app
/// carries only the Material delegates.
class _Localized extends StatelessWidget {
  const _Localized({required this.width, required this.child});

  final double width;
  final Widget child;

  @override
  Widget build(BuildContext context) => Localizations(
    locale: Localizations.localeOf(context),
    delegates: AppLocalizations.localizationsDelegates,
    child: SizedBox(width: width, child: child),
  );
}

void main() {
  late MarkModels models;
  setUpAll(() async => models = await MarkModels.load());

  goldenTest(
    'lane mark with the normal frame',
    fileName: 'lane_mark',
    pumpBeforeTest: dsPumpIcons,
    builder: () =>
        matrix('lane', () => LaneMark(model: models.normal.lanes.last)),
  );

  goldenTest(
    'run card mark, normal and without cost permission',
    fileName: 'run_card_mark',
    pumpBeforeTest: dsPumpIcons,
    builder: () => matrix(
      'run card',
      () => Wrap(
        spacing: DsSpace.s8,
        runSpacing: DsSpace.s8,
        children: [
          RunCardMark(model: models.normal.lanes.first.cards.first),
          RunCardMark(model: models.normal.lanes[1].cards.first),
          RunCardMark(model: models.noCost.cards.first),
        ],
      ),
    ),
  );

  goldenTest(
    'area mark with the usage series',
    fileName: 'area_mark',
    builder: () => matrix(
      'area',
      () => AreaMark(title: 'Rates', series: models.usage.take(4).toList()),
    ),
  );

  goldenTest(
    'bar mark with the timeline runs',
    fileName: 'bar_mark',
    pumpBeforeTest: dsPumpIcons,
    builder: () => matrix('bar', () => BarMark(model: models.timeline.bars)),
  );

  goldenTest(
    'traffic light mark in all four states',
    fileName: 'traffic_light_mark',
    pumpBeforeTest: dsPumpIcons,
    builder: () => matrix(
      'traffic light',
      () => Wrap(
        spacing: DsSpace.s8,
        runSpacing: DsSpace.s8,
        children: [
          for (final status in TrafficLightStatus.values)
            TrafficLightMark(
              model: models.lights.firstWhere((l) => l.status == status),
            ),
        ],
      ),
    ),
  );

  goldenTest(
    'metric tile mark with usage and cost tiles',
    fileName: 'metric_tile_mark',
    builder: () => matrix(
      'metric tile',
      () => Wrap(
        spacing: DsSpace.s8,
        runSpacing: DsSpace.s8,
        children: [
          MetricTileMark(model: models.tiles.first),
          for (final tile in models.costTiles) MetricTileMark(model: tile),
          const MetricTileMark(
            model: MetricTileModel(
              value: null,
              unit: TileUnit.count,
              queryMetric: 'messages',
            ),
          ),
        ],
      ),
    ),
  );
}
