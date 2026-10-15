// SPDX-License-Identifier: Apache-2.0

/// Delivery and sync live (docs/spec/25-visualisierung.md 6.3).
library;

import 'package:api_client/api_client.dart' as api;

import 'series_model.dart';

/// One SLI of the live frame over `stats.live_sli_window`.
final class SliModel {
  /// Creates an SLI value.
  const SliModel({
    required this.metric,
    required this.total,
    required this.good,
    this.ratio,
  });

  /// The `sli.*` metric.
  final api.StatsMetric metric;

  /// Events in the window.
  final int total;

  /// Good events in the window.
  final int good;

  /// Good share; `null` when there was no event.
  final double? ratio;
}

/// Gateway connections of the node by kind; only with one node.
final class ConnectionsModel {
  /// Creates the counts.
  const ConnectionsModel({required this.user, required this.agent, required this.app});

  /// Connections of people.
  final int user;

  /// Connections of agents.
  final int agent;

  /// Connections of apps.
  final int app;
}

/// SLIs, catch-ups, resyncs, closes of slow clients and connections of one live frame.
///
/// Instance scope only (owner): a space-scoped frame carries none of these, and the model
/// stays empty instead of inventing zeros (25 4 rule 1).
final class DeliveryModel {
  /// Creates the model.
  const DeliveryModel({
    required this.at,
    required this.slis,
    required this.gatewayRates,
    this.connections,
  });

  /// Derives the model from [frame].
  factory DeliveryModel.fromFrame(api.StatsLiveFrame frame) {
    const gateway = {
      api.StatsMetric.catchups,
      api.StatsMetric.resyncs,
      api.StatsMetric.slowConsumerCloses,
    };
    final rates = frame.rates.toList();
    final connections = frame.connections;
    return DeliveryModel(
      at: frame.at,
      slis: List.unmodifiable([
        for (final s in frame.sli ?? <api.StatsLiveSli>[])
          SliModel(metric: s.metric, total: s.total, good: s.good, ratio: s.ratio),
      ]),
      gatewayRates: List.unmodifiable([
        for (var i = 0; i < rates.length; i++)
          if (gateway.contains(rates[i].metric)) RateModel.fromRate(rates[i], i),
      ]),
      connections: connections == null
          ? null
          : ConnectionsModel(
              user: connections.user,
              agent: connections.agent,
              app: connections.app,
            ),
    );
  }

  /// Time of the frame.
  final DateTime at;

  /// SLIs in the order of the frame.
  final List<SliModel> slis;

  /// Rates of `catchups`, `resyncs` (per reason) and `slow_consumer_closes`.
  final List<RateModel> gatewayRates;

  /// Connections; `null` with several nodes or a space scope.
  final ConnectionsModel? connections;
}
