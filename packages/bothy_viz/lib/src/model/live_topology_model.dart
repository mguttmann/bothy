// SPDX-License-Identifier: Apache-2.0

/// The live topology (docs/spec/25-visualization.md 6.1).
library;

import 'package:api_client/api_client.dart' as api;

import 'names.dart';
import 'run_card_model.dart';
import 'series_model.dart';

/// Lanes of active runs per space and channel, the rates of the last minute and the
/// "more runs than shown" marker of one live frame.
final class LiveTopologyModel {
  /// Creates the model; [LiveTopologyModel.fromFrame] is the normal way.
  const LiveTopologyModel({
    required this.at,
    required this.lanes,
    required this.runsTruncated,
    required this.rates,
    this.spaceId,
  });

  /// Derives the model from [frame]; [names] gives the display names. Never calls the
  /// network.
  factory LiveTopologyModel.fromFrame(
    api.StatsLiveFrame frame, {
    VizNames names = const VizNames(),
  }) {
    final rates = frame.rates.toList();
    return LiveTopologyModel(
      at: frame.at,
      spaceId: frame.scope.spaceId,
      lanes: List.unmodifiable(
        LaneModel.group(frame.runs, names, at: frame.at),
      ),
      runsTruncated: frame.runsTruncated,
      rates: List.unmodifiable([
        for (var i = 0; i < rates.length; i++) RateModel.fromRate(rates[i], i),
      ]),
    );
  }

  /// Time of the frame (server clock).
  final DateTime at;

  /// Space of a space-scoped frame; `null` for the instance.
  final String? spaceId;

  /// Lanes in the order of the frame.
  final List<LaneModel> lanes;

  /// `runs_truncated`: the view shows "More runs than shown".
  final bool runsTruncated;

  /// Rates in the order of the frame; costs only when the server sent them.
  final List<RateModel> rates;

  /// Every card of every lane.
  Iterable<RunCardModel> get cards => lanes.expand((lane) => lane.cards);
}
